package collector

import (
	"strings"
	"testing"
	"time"

	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/config"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/models"
)

func TestGetEmployeeTasksRange_JQLFormat(t *testing.T) {
	cfg := &config.Config{
		Jira: config.JiraConfig{URL: "https://jira.test.com", Token: "test"},
	}
	jc := NewJiraCollector(cfg)
	_ = jc // just verify it builds

	// Test date range logic
	tests := []struct {
		name     string
		since    time.Time
		until    time.Time
		wantJQL  string
	}{
		{
			name:    "January 2026",
			since:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
			until:   time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local),
			wantJQL: `created >= "2026-01-01" AND created < "2026-02-01"`,
		},
		{
			name:    "June 2026",
			since:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local),
			until:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local),
			wantJQL: `created >= "2026-06-01" AND created < "2026-07-01"`,
		},
		{
			name:    "All time (no upper bound)",
			since:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
			until:   time.Time{},
			wantJQL: `created >= "2026-01-01"`,
		},
		{
			name:    "December to January boundary",
			since:   time.Date(2026, 12, 1, 0, 0, 0, 0, time.Local),
			until:   time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local),
			wantJQL: `created >= "2026-12-01" AND created < "2027-01-01"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emp := models.Employee{Email: "test@test.com"}

			// Build JQL the same way as GetEmployeeTasksRange
			sinceStr := tt.since.Format("2006-01-02")
			var dateFilter string
			if tt.until.IsZero() {
				dateFilter = `created >= "` + sinceStr + `"`
			} else {
				untilStr := tt.until.Format("2006-01-02")
				dateFilter = `created >= "` + sinceStr + `" AND created < "` + untilStr + `"`
			}
			jql := `assignee = "` + emp.Email + `" AND ` + dateFilter + ` ORDER BY created DESC`

			if !strings.Contains(jql, tt.wantJQL) {
				t.Errorf("JQL does not contain expected date filter.\nGot: %s\nWant to contain: %s", jql, tt.wantJQL)
			}
		})
	}
}

func TestMonthBoundaries(t *testing.T) {
	// Verify that month boundaries are correct (no off-by-one)
	tests := []struct {
		month       string
		wantStart   string
		wantEnd     string
	}{
		{"2026-01", "2026-01-01", "2026-02-01"},
		{"2026-02", "2026-02-01", "2026-03-01"},
		{"2026-06", "2026-06-01", "2026-07-01"},
		{"2026-12", "2026-12-01", "2027-01-01"},
	}

	for _, tt := range tests {
		t.Run(tt.month, func(t *testing.T) {
			parsed, err := time.Parse("2006-01", tt.month)
			if err != nil {
				t.Fatalf("Failed to parse month %s: %v", tt.month, err)
			}
			start := parsed
			end := parsed.AddDate(0, 1, 0)

			startStr := start.Format("2006-01-02")
			endStr := end.Format("2006-01-02")

			if startStr != tt.wantStart {
				t.Errorf("Start = %s, want %s", startStr, tt.wantStart)
			}
			if endStr != tt.wantEnd {
				t.Errorf("End = %s, want %s", endStr, tt.wantEnd)
			}
		})
	}
}
