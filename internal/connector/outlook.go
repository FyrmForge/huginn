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

const outlookAuthURL = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
const outlookTokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
const outlookCalBase = "https://graph.microsoft.com/v1.0/me"

// OutlookProvider implements OAuthProvider and CalendarConnector for Microsoft 365.
type OutlookProvider struct {
	clientID     string
	clientSecret string
}

func NewOutlookProvider(clientID, clientSecret string) *OutlookProvider {
	return &OutlookProvider{clientID: clientID, clientSecret: clientSecret}
}

func (o *OutlookProvider) AuthorizeURL(state, redirectURI string) string {
	q := url.Values{
		"client_id":     {o.clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"Calendars.Read offline_access openid email"},
		"state":         {state},
	}
	return outlookAuthURL + "?" + q.Encode()
}

func (o *OutlookProvider) Exchange(ctx context.Context, code, redirectURI string) (*Token, error) {
	return o.tokenRequest(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {o.clientID},
		"client_secret": {o.clientSecret},
	})
}

func (o *OutlookProvider) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	return o.tokenRequest(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {o.clientID},
		"client_secret": {o.clientSecret},
	})
}

func (o *OutlookProvider) tokenRequest(ctx context.Context, v url.Values) (*Token, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, outlookTokenURL, strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("outlook token: %s", body)
	}
	var t struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("outlook token parse: %w", err)
	}
	return &Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(t.ExpiresIn) * time.Second),
		Scope:        t.Scope,
	}, nil
}

func (o *OutlookProvider) ListCalendars(ctx context.Context, tok *Token) ([]ExternalCalendar, error) {
	body, err := outlookGET(ctx, tok.AccessToken, outlookCalBase+"/calendars?$top=50")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Value []struct {
			ID                string `json:"id"`
			Name              string `json:"name"`
			Color             string `json:"color"`
			IsDefaultCalendar bool   `json:"isDefaultCalendar"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse calendar list: %w", err)
	}
	cals := make([]ExternalCalendar, len(resp.Value))
	for i, item := range resp.Value {
		cals[i] = ExternalCalendar{
			ID: item.ID, Name: item.Name, Color: item.Color, Primary: item.IsDefaultCalendar,
		}
	}
	return cals, nil
}

func (o *OutlookProvider) FullSync(ctx context.Context, tok *Token, calendarID string) ([]ExternalEvent, string, error) {
	return o.fetchEvents(ctx, tok, calendarID)
}

func (o *OutlookProvider) IncrementalSync(ctx context.Context, tok *Token, calendarID, _ string) ([]ExternalEvent, string, error) {
	// ponytail: Graph delta tokens; use FullSync until delta link tracking is needed
	return o.fetchEvents(ctx, tok, calendarID)
}

func (o *OutlookProvider) fetchEvents(ctx context.Context, tok *Token, calendarID string) ([]ExternalEvent, string, error) {
	startTime := time.Now().AddDate(-1, 0, 0).Format(time.RFC3339)
	endTime := time.Now().AddDate(1, 0, 0).Format(time.RFC3339)

	var events []ExternalEvent
	nextLink := fmt.Sprintf("%s/calendars/%s/calendarView?startDateTime=%s&endDateTime=%s&$top=100",
		outlookCalBase, url.PathEscape(calendarID), url.QueryEscape(startTime), url.QueryEscape(endTime))

	for nextLink != "" {
		body, err := outlookGET(ctx, tok.AccessToken, nextLink)
		if err != nil {
			return nil, "", err
		}
		var page struct {
			Value []struct {
				ID      string `json:"id"`
				ICalUID string `json:"iCalUId"`
				Subject string `json:"subject"`
				Body    struct {
					Content string `json:"content"`
				} `json:"body"`
				Location struct {
					DisplayName string `json:"displayName"`
				} `json:"location"`
				IsAllDay             bool   `json:"isAllDay"`
				LastModifiedDateTime string `json:"lastModifiedDateTime"`
				Start                struct {
					DateTime string `json:"dateTime"`
				} `json:"start"`
				End struct {
					DateTime string `json:"dateTime"`
				} `json:"end"`
			} `json:"value"`
			NextLink string `json:"@odata.nextLink"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, "", fmt.Errorf("parse events: %w", err)
		}
		for _, item := range page.Value {
			ev := ExternalEvent{
				ProviderID:  item.ID,
				UID:         item.ICalUID,
				Title:       item.Subject,
				Description: item.Body.Content,
				Location:    item.Location.DisplayName,
				AllDay:      item.IsAllDay,
			}
			if item.LastModifiedDateTime != "" {
				ev.UpdatedAt, _ = time.Parse(time.RFC3339, item.LastModifiedDateTime)
			}
			// Graph returns UTC without Z suffix
			ev.StartAt, _ = time.Parse("2006-01-02T15:04:05.0000000", item.Start.DateTime)
			ev.EndAt, _ = time.Parse("2006-01-02T15:04:05.0000000", item.End.DateTime)
			events = append(events, ev)
		}
		nextLink = page.NextLink
	}
	return events, "", nil
}

func outlookGET(ctx context.Context, accessToken, u string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("outlook api %s: %s", u, body)
	}
	return body, nil
}
