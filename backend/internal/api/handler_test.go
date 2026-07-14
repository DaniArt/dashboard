package api

import (
	"strings"
	"testing"
	"time"

	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/models"
)

func TestIsCompletedStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		// Positive cases
		{"готово", true},
		{"Готово", true},
		{"ГОТОВО", true},
		{"done", true},
		{"Done", true},
		{"DONE", true},
		{"closed", true},
		{"Закрыт", true},
		{"закрыт", true},
		{"выполнено", true},
		{"Выполнено", true},
		{"ready for development", true},
		{"Ready for development", true},
		{"READY FOR DEVELOPMENT", true},
		{"готово к оценке", true},
		{"Готово к оценке", true},
		{"готова к archqg", true},
		{"готово к тестированию", true},
		{"resolved", true},
		{"Resolved", true},
		{"завершено", true},

		// Negative cases
		{"в работе", false},
		{"В работе", false},
		{"открытый", false},
		{"Открытый", false},
		{"analysis", false},
		{"ANALYSIS", false},
		{"analytics", false},
		{"на анализе", false},
		{"backlog", false},
		{"Backlog", false},
		{"blocked", false},
		{"сделать", false},
		{"разработка", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := isCompletedStatus(strings.ToLower(tt.status))
			if result != tt.expected {
				t.Errorf("isCompletedStatus(%q) = %v, want %v", tt.status, result, tt.expected)
			}
		})
	}
}

func TestCountMRMetrics(t *testing.T) {
	mrs := []models.MergeRequest{
		{State: "merged", Reviewers: []string{"reviewer1"}},
		{State: "merged", Reviewers: []string{"reviewer2"}},
		{State: "opened", Reviewers: []string{"reviewer1"}},
		{State: "opened", Reviewers: []string{}},
		{State: "closed", Reviewers: []string{}},
	}

	metrics := countMRMetrics(mrs)

	if metrics.MRsCreatedMonth != 5 {
		t.Errorf("MRsCreatedMonth = %d, want 5", metrics.MRsCreatedMonth)
	}
	if metrics.MRsMergedMonth != 2 {
		t.Errorf("MRsMergedMonth = %d, want 2", metrics.MRsMergedMonth)
	}
	if metrics.MRsOpen != 2 {
		t.Errorf("MRsOpen = %d, want 2", metrics.MRsOpen)
	}
	if metrics.MRsWithoutReview != 1 {
		t.Errorf("MRsWithoutReview = %d, want 1", metrics.MRsWithoutReview)
	}
}

func TestGenerateTasksConclusion_NoIssues(t *testing.T) {
	issues := []models.JiraIssue{}
	metrics := models.TaskMetrics{ActiveTasks: 3, CompletedMonth: 5}

	result := generateTasksConclusion("Test User", issues, metrics)

	if result != "Задачи обрабатываются в нормальном режиме. Замечаний нет." {
		t.Errorf("Expected no issues conclusion, got: %s", result)
	}
}

func TestGenerateTasksConclusion_WithStale(t *testing.T) {
	now := time.Now()
	issues := []models.JiraIssue{
		{Key: "PMB-1", StatusSince: now.AddDate(0, 0, -10)}, // 10 days stale
		{Key: "PMB-2", StatusSince: now.AddDate(0, 0, -2)},  // 2 days, fine
		{Key: "PMB-3", StatusSince: now.AddDate(0, 0, -7)},  // 7 days stale
	}
	metrics := models.TaskMetrics{ActiveTasks: 3, CompletedMonth: 0}

	result := generateTasksConclusion("Test User", issues, metrics)

	if result == "Задачи обрабатываются в нормальном режиме. Замечаний нет." {
		t.Errorf("Expected issues in conclusion, got: %s", result)
	}
}

func TestGenerateTasksConclusion_HighLoad(t *testing.T) {
	issues := []models.JiraIssue{}
	metrics := models.TaskMetrics{ActiveTasks: 15, CompletedMonth: 2}

	result := generateTasksConclusion("Test User", issues, metrics)

	if result == "Задачи обрабатываются в нормальном режиме. Замечаний нет." {
		t.Errorf("Expected overload warning, got: %s", result)
	}
}

func TestGenerateMRConclusion_AllGood(t *testing.T) {
	mrs := []models.MergeRequest{
		{State: "merged", DaysOpen: 1, Reviewers: []string{"r1"}},
	}
	metrics := countMRMetrics(mrs)

	result := generateMRConclusion("Test User", mrs, metrics)

	if result != "Всё в порядке. MR обрабатываются в нормальном режиме." {
		t.Errorf("Expected all good, got: %s", result)
	}
}

func TestGenerateMRConclusion_LongOpen(t *testing.T) {
	mrs := []models.MergeRequest{
		{State: "opened", DaysOpen: 5, Reviewers: []string{"r1"}},
		{State: "opened", DaysOpen: 10, Reviewers: []string{}},
	}
	metrics := countMRMetrics(mrs)

	result := generateMRConclusion("Test User", mrs, metrics)

	if result == "Всё в порядке. MR обрабатываются в нормальном режиме." {
		t.Errorf("Expected issues, got all good")
	}
}

func TestGenerateConfluenceConclusion_NoActivity(t *testing.T) {
	pages := []models.ConfluencePage{}
	metrics := models.ConfluenceMetrics{QualityScore: 20}

	result := generateConfluenceConclusion("Test User", pages, metrics)

	if result == "Документация ведётся активно. Замечаний нет." {
		t.Errorf("Expected no activity warning, got all good")
	}
}

func TestGenerateConfluenceConclusion_Active(t *testing.T) {
	pages := []models.ConfluencePage{
		{Title: "Page 1", BodyLength: 2000, DaysSinceUpdate: 2},
		{Title: "Page 2", BodyLength: 5000, DaysSinceUpdate: 5},
	}
	metrics := models.ConfluenceMetrics{QualityScore: 90, PagesCreatedMonth: 5}

	result := generateConfluenceConclusion("Test User", pages, metrics)

	if result != "Документация ведётся активно. Замечаний нет." {
		t.Errorf("Expected all good, got: %s", result)
	}
}
