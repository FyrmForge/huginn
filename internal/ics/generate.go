package ics

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/FyrmForge/huginn/internal/repo"
)

// Write writes events as a VCALENDAR iCalendar stream to w.
// For recurring events, includes the master VEVENT (with RRULE/EXDATE) and any exception VEVENTs.
func Write(w io.Writer, calName string, events []*repo.Event) error {
	return WriteWithExceptions(w, calName, events, nil)
}

// WriteWithExceptions is like Write but also emits exception VEVENTs for recurring masters.
func WriteWithExceptions(w io.Writer, calName string, events []*repo.Event, exsByMaster map[string][]*repo.EventException) error {
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//Huginn//Huginn Calendar//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"X-WR-CALNAME:" + escape(calName),
		"X-WR-TIMEZONE:UTC",
	}

	for _, e := range events {
		lines = append(lines, eventLines(e)...)
		if exsByMaster != nil {
			for _, ex := range exsByMaster[e.ID] {
				lines = append(lines, exceptionLines(e, ex)...)
			}
		}
	}

	lines = append(lines, "END:VCALENDAR")

	_, err := io.WriteString(w, strings.Join(lines, "\r\n")+"\r\n")
	return err
}

func eventLines(e *repo.Event) []string {
	lines := []string{"BEGIN:VEVENT"}
	lines = append(lines, "UID:"+e.UID)
	lines = append(lines, "SUMMARY:"+escape(e.Title))
	if e.AllDay {
		lines = append(lines, "DTSTART;VALUE=DATE:"+e.StartAt.UTC().Format("20060102"))
		lines = append(lines, "DTEND;VALUE=DATE:"+e.EndAt.UTC().Format("20060102"))
	} else {
		lines = append(lines, "DTSTART:"+e.StartAt.UTC().Format("20060102T150405Z"))
		lines = append(lines, "DTEND:"+e.EndAt.UTC().Format("20060102T150405Z"))
	}
	if e.RRule != "" {
		lines = append(lines, "RRULE:"+e.RRule)
	}
	if e.Exdates != "" {
		for _, raw := range strings.Split(e.Exdates, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339, raw)
			if err == nil {
				lines = append(lines, "EXDATE:"+t.UTC().Format("20060102T150405Z"))
			}
		}
	}
	if e.Description != "" {
		lines = append(lines, fold("DESCRIPTION:"+escape(e.Description)))
	}
	if e.Location != "" {
		lines = append(lines, "LOCATION:"+escape(e.Location))
	}
	lines = append(lines, "STATUS:"+strings.ToUpper(e.Status))
	transp := "OPAQUE"
	if e.BusyStatus == "free" {
		transp = "TRANSPARENT"
	}
	lines = append(lines, "TRANSP:"+transp)
	lines = append(lines, "DTSTAMP:"+e.CreatedAt.UTC().Format("20060102T150405Z"))
	lines = append(lines, "CREATED:"+e.CreatedAt.UTC().Format("20060102T150405Z"))
	lines = append(lines, "LAST-MODIFIED:"+e.UpdatedAt.UTC().Format("20060102T150405Z"))
	lines = append(lines, fmt.Sprintf("SEQUENCE:%d", e.UpdatedAt.Unix()))
	lines = append(lines, "END:VEVENT")
	return lines
}

// exceptionLines emits a VEVENT with RECURRENCE-ID for a modified or cancelled occurrence.
func exceptionLines(master *repo.Event, ex *repo.EventException) []string {
	status := strings.ToUpper(ex.Status)
	if ex.IsDeleted {
		status = "CANCELLED"
	}

	lines := []string{"BEGIN:VEVENT"}
	lines = append(lines, "UID:"+master.UID)
	lines = append(lines, "RECURRENCE-ID:"+ex.RecurrenceID.UTC().Format("20060102T150405Z"))
	lines = append(lines, "SUMMARY:"+escape(ex.Title))
	if ex.AllDay {
		lines = append(lines, "DTSTART;VALUE=DATE:"+ex.StartAt.UTC().Format("20060102"))
		lines = append(lines, "DTEND;VALUE=DATE:"+ex.EndAt.UTC().Format("20060102"))
	} else {
		lines = append(lines, "DTSTART:"+ex.StartAt.UTC().Format("20060102T150405Z"))
		lines = append(lines, "DTEND:"+ex.EndAt.UTC().Format("20060102T150405Z"))
	}
	if ex.Description != "" {
		lines = append(lines, fold("DESCRIPTION:"+escape(ex.Description)))
	}
	if ex.Location != "" {
		lines = append(lines, "LOCATION:"+escape(ex.Location))
	}
	lines = append(lines, "STATUS:"+status)
	lines = append(lines, "DTSTAMP:"+ex.CreatedAt.UTC().Format("20060102T150405Z"))
	lines = append(lines, "LAST-MODIFIED:"+ex.UpdatedAt.UTC().Format("20060102T150405Z"))
	lines = append(lines, fmt.Sprintf("SEQUENCE:%d", ex.UpdatedAt.Unix()))
	lines = append(lines, "END:VEVENT")
	return lines
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// fold wraps long lines at 75 octets per RFC 5545.
func fold(line string) string {
	if len(line) <= 75 {
		return line
	}
	var b strings.Builder
	b.WriteString(line[:75])
	line = line[75:]
	for len(line) > 0 {
		b.WriteString("\r\n ")
		if len(line) > 74 {
			b.WriteString(line[:74])
			line = line[74:]
		} else {
			b.WriteString(line)
			line = ""
		}
	}
	return b.String()
}
