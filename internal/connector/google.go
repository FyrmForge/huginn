package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const googleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
const googleTokenURL = "https://oauth2.googleapis.com/token"
const googleCalBase = "https://www.googleapis.com/calendar/v3"

// GoogleProvider implements OAuthProvider and CalendarConnector for Google Calendar.
type GoogleProvider struct {
	clientID     string
	clientSecret string
}

func NewGoogleProvider(clientID, clientSecret string) *GoogleProvider {
	return &GoogleProvider{clientID: clientID, clientSecret: clientSecret}
}

func (g *GoogleProvider) AuthorizeURL(state, redirectURI string) string {
	q := url.Values{
		"client_id":     {g.clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"https://www.googleapis.com/auth/calendar.readonly openid email"},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
		"state":         {state},
	}
	return googleAuthURL + "?" + q.Encode()
}

func (g *GoogleProvider) Exchange(ctx context.Context, code, redirectURI string) (*Token, error) {
	return g.tokenRequest(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
	})
}

func (g *GoogleProvider) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	return g.tokenRequest(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
	})
}

func (g *GoogleProvider) tokenRequest(ctx context.Context, v url.Values) (*Token, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google token: %s", body)
	}
	var t struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("google token parse: %w", err)
	}
	return &Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(t.ExpiresIn) * time.Second),
		Scope:        t.Scope,
	}, nil
}

// ListCalendars returns all calendars in the user's calendar list.
func (g *GoogleProvider) ListCalendars(ctx context.Context, tok *Token) ([]ExternalCalendar, error) {
	body, err := googleGET(ctx, tok.AccessToken, googleCalBase+"/users/me/calendarList")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []struct {
			ID              string `json:"id"`
			Summary         string `json:"summary"`
			Description     string `json:"description"`
			BackgroundColor string `json:"backgroundColor"`
			Primary         bool   `json:"primary"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse calendar list: %w", err)
	}
	cals := make([]ExternalCalendar, len(resp.Items))
	for i, item := range resp.Items {
		cals[i] = ExternalCalendar{
			ID: item.ID, Name: item.Summary, Description: item.Description,
			Color: item.BackgroundColor, Primary: item.Primary,
		}
	}
	return cals, nil
}

// FullSync fetches all events for a calendar and returns a sync token.
func (g *GoogleProvider) FullSync(ctx context.Context, tok *Token, calendarID string) ([]ExternalEvent, string, error) {
	return g.fetchEvents(ctx, tok, calendarID, "")
}

// IncrementalSync fetches events changed since syncToken.
func (g *GoogleProvider) IncrementalSync(ctx context.Context, tok *Token, calendarID, syncToken string) ([]ExternalEvent, string, error) {
	return g.fetchEvents(ctx, tok, calendarID, syncToken)
}

func (g *GoogleProvider) fetchEvents(ctx context.Context, tok *Token, calendarID, syncToken string) ([]ExternalEvent, string, error) {
	apiURL := fmt.Sprintf("%s/calendars/%s/events", googleCalBase, url.PathEscape(calendarID))
	params := url.Values{"maxResults": {"2500"}}
	if syncToken != "" {
		params.Set("syncToken", syncToken)
	} else {
		params.Set("timeMin", time.Now().AddDate(-1, 0, 0).Format(time.RFC3339))
	}

	var events []ExternalEvent
	nextSyncToken := ""

	for {
		u := apiURL + "?" + params.Encode()
		body, err := googleGET(ctx, tok.AccessToken, u)
		if err != nil {
			return nil, "", err
		}
		var page struct {
			Items []struct {
				ID          string `json:"id"`
				ICalUID     string `json:"iCalUID"`
				Summary     string `json:"summary"`
				Description string `json:"description"`
				Location    string `json:"location"`
				Status      string `json:"status"`
				Etag        string `json:"etag"`
				Updated     string `json:"updated"`
				Start       struct {
					DateTime string `json:"dateTime"`
					Date     string `json:"date"`
				} `json:"start"`
				End struct {
					DateTime string `json:"dateTime"`
					Date     string `json:"date"`
				} `json:"end"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
			NextSyncToken string `json:"nextSyncToken"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, "", fmt.Errorf("parse events: %w", err)
		}
		for _, item := range page.Items {
			ev := ExternalEvent{
				ProviderID:   item.ID,
				UID:          item.ICalUID,
				Title:        item.Summary,
				Description:  item.Description,
				Location:     item.Location,
				Status:       item.Status,
				ProviderETag: item.Etag,
			}
			if item.Updated != "" {
				ev.UpdatedAt, _ = time.Parse(time.RFC3339, item.Updated)
			}
			if item.Start.DateTime != "" {
				ev.StartAt, _ = time.Parse(time.RFC3339, item.Start.DateTime)
				ev.EndAt, _ = time.Parse(time.RFC3339, item.End.DateTime)
			} else {
				ev.AllDay = true
				ev.StartAt, _ = time.Parse("2006-01-02", item.Start.Date)
				ev.EndAt, _ = time.Parse("2006-01-02", item.End.Date)
			}
			events = append(events, ev)
		}
		if page.NextPageToken != "" {
			params.Set("pageToken", page.NextPageToken)
			params.Del("syncToken")
			continue
		}
		nextSyncToken = page.NextSyncToken
		break
	}
	return events, nextSyncToken, nil
}

func googleGET(ctx context.Context, accessToken, u string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google api %s: %s", u, body)
	}
	return body, nil
}
