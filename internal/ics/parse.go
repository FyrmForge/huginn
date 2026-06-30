// Package ics provides minimal iCalendar parsing and generation for Huginn.
// Handles VEVENT fields including recurrence (RRULE, EXDATE, RDATE, RECURRENCE-ID).
package ics

import (
	"bufio"
	"io"
	"strings"
	"time"
)

// Event holds the parsed fields of a single VEVENT.
type Event struct {
	UID          string
	Summary      string
	Description  string
	Location     string
	StartAt      time.Time
	EndAt        time.Time
	AllDay       bool
	Status       string
	RRule        string // RRULE value without "RRULE:" prefix
	Exdates      []time.Time
	Rdates       []time.Time
	RecurrenceID time.Time // non-zero = this is an exception VEVENT
	RawBlock     string    // original lines of this VEVENT
}

// Parse reads an iCalendar stream and returns all VEVENTs found.
func Parse(r io.Reader) ([]Event, error) {
	var events []Event
	var cur *Event
	var rawLines []string

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := unfoldLine(scanner.Text())

		switch {
		case line == "BEGIN:VEVENT":
			cur = &Event{}
			rawLines = []string{line}
		case line == "END:VEVENT" && cur != nil:
			rawLines = append(rawLines, line)
			cur.RawBlock = strings.Join(rawLines, "\r\n")
			events = append(events, *cur)
			cur = nil
			rawLines = nil
		case cur != nil:
			rawLines = append(rawLines, line)
			key, val := splitProp(line)
			keyBase := strings.SplitN(key, ";", 2)[0] // strip parameters for matching

			switch keyBase {
			case "UID":
				cur.UID = val
			case "SUMMARY":
				cur.Summary = unescape(val)
			case "DESCRIPTION":
				cur.Description = unescape(val)
			case "LOCATION":
				cur.Location = unescape(val)
			case "STATUS":
				cur.Status = strings.ToLower(val)
			case "RRULE":
				cur.RRule = val
			case "RECURRENCE-ID":
				if t, _, ok := parseDateTimeParam(key, val); ok {
					cur.RecurrenceID = t
				}
			case "EXDATE":
				for _, raw := range strings.Split(val, ",") {
					if t, _, ok := parseDateTimeParam(key, strings.TrimSpace(raw)); ok {
						cur.Exdates = append(cur.Exdates, t)
					}
				}
			case "RDATE":
				for _, raw := range strings.Split(val, ",") {
					if t, _, ok := parseDateTimeParam(key, strings.TrimSpace(raw)); ok {
						cur.Rdates = append(cur.Rdates, t)
					}
				}
			}

			// Handle DTSTART / DTEND with any parameters (TZID, VALUE=DATE, etc).
			if strings.HasPrefix(keyBase, "DTSTART") {
				if cur.StartAt.IsZero() {
					t, allDay, ok := parseDateTimeParam(key, val)
					if ok {
						cur.StartAt = t
						cur.AllDay = allDay
					}
				}
			}
			if strings.HasPrefix(keyBase, "DTEND") {
				if cur.EndAt.IsZero() {
					t, _, ok := parseDateTimeParam(key, val)
					if ok {
						cur.EndAt = t
					}
				}
			}
		}
	}
	return events, scanner.Err()
}

func splitProp(line string) (key, val string) {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return line, ""
	}
	return line[:idx], line[idx+1:]
}

// parseDateTimeParam parses a DTSTART/DTEND/EXDATE value, handling TZID and VALUE=DATE parameters.
func parseDateTimeParam(key, val string) (t time.Time, allDay bool, ok bool) {
	// Check VALUE=DATE in the key parameters.
	if strings.Contains(key, "VALUE=DATE") {
		parsed, err := time.Parse("20060102", val)
		if err == nil {
			return parsed.UTC(), true, true
		}
		return
	}

	// Date-only (8 chars).
	if len(val) == 8 {
		parsed, err := time.Parse("20060102", val)
		if err == nil {
			return parsed.UTC(), true, true
		}
	}

	// Check for TZID parameter — e.g. DTSTART;TZID=America/New_York:20260101T090000
	if idx := strings.Index(key, "TZID="); idx >= 0 {
		tzName := key[idx+5:]
		if semi := strings.Index(tzName, ";"); semi >= 0 {
			tzName = tzName[:semi]
		}
		loc, err := time.LoadLocation(tzName)
		if err == nil {
			parsed, err := time.ParseInLocation("20060102T150405", val, loc)
			if err == nil {
				return parsed.UTC(), false, true
			}
		}
	}

	// UTC: 20060102T150405Z
	if strings.HasSuffix(val, "Z") {
		parsed, err := time.Parse("20060102T150405Z", val)
		if err == nil {
			return parsed.UTC(), false, true
		}
	}

	// Local (no TZ info — treat as UTC, best effort).
	parsed, err := time.Parse("20060102T150405", val)
	if err == nil {
		return parsed.UTC(), false, true
	}
	return
}


// unfoldLine removes iCalendar line folding (continuation lines start with space/tab).
func unfoldLine(line string) string {
	return strings.TrimRight(line, "\r")
}

func unescape(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}
