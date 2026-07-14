package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/alerts"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/collector"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/config"
	"github.com/timurberkenov-lgtm/qa-dashboards/backend/internal/models"
)

// Data collection start date
var dataStartDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)

type Handler struct {
	cfg        *config.Config
	jira       *collector.JiraCollector
	confluence *collector.ConfluenceCollector
	gitlab     *collector.GitLabCollector
	alertEng   *alerts.AlertEngine

	mu          sync.RWMutex
	dashboard   *models.DashboardResponse
	cache       map[string]interface{} // key: "section:month" -> cached response
	lastUpdated time.Time
}

func NewHandler(cfg *config.Config) *Handler {
	h := &Handler{
		cfg:        cfg,
		jira:       collector.NewJiraCollector(cfg),
		confluence: collector.NewConfluenceCollector(cfg),
		gitlab:     collector.NewGitLabCollector(cfg),
		alertEng:   alerts.NewAlertEngine(cfg),
		cache:      make(map[string]interface{}),
	}

	go h.collectData()
	go h.startPolling()

	return h
}

func (h *Handler) startPolling() {
	ticker := time.NewTicker(h.cfg.Server.PollInterval)
	defer ticker.Stop()
	for range ticker.C {
		h.collectData()
	}
}

// getMonthRange parses ?month=2026-03, ?month=all, or defaults to current month
func getMonthRange(r *http.Request) (time.Time, time.Time) {
	monthParam := r.URL.Query().Get("month")
	now := time.Now()

	if monthParam == "all" {
		// All time since project start
		return dataStartDate, now
	}

	if monthParam != "" {
		t, err := time.Parse("2006-01", monthParam)
		if err == nil {
			start := t
			end := t.AddDate(0, 1, 0)
			return start, end
		}
	}

	// Default: current month
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	return start, end
}

func (h *Handler) collectData() {
	log.Println("Collecting data from Jira, Confluence, GitLab...")

	var employees []models.EmployeeDashboard
	var allAlerts []models.Alert
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, emp := range h.cfg.Employees {
		ed := models.EmployeeDashboard{
			Employee:    emp,
			LastUpdated: now,
		}

		// Jira active tasks
		activeTasks, err := h.jira.GetEmployeeTasks(emp)
		if err != nil {
			log.Printf("Error fetching Jira tasks for %s: %v", emp.Name, err)
		} else {
			ed.Tasks.ActiveTasks = len(activeTasks)
			ed.Tasks.ByStatus = make(map[string]int)
			ed.Tasks.ByType = make(map[string]int)
			for _, task := range activeTasks {
				ed.Tasks.ByStatus[task.Status]++
				ed.Tasks.ByType[task.Type]++
				daysInStatus := int(now.Sub(task.StatusSince).Hours() / 24)
				if daysInStatus >= h.cfg.Alerts.StaleTaskDays {
					ed.Tasks.StaleTasks++
				}
			}
		}

		// Completed tasks - current month
		completedMonth, err := h.jira.GetCompletedTasks(emp, monthStart)
		if err != nil {
			log.Printf("Error fetching completed tasks for %s: %v", emp.Name, err)
		} else {
			ed.Tasks.CompletedMonth = len(completedMonth)
			for _, task := range completedMonth {
				if task.Updated.After(todayStart) {
					ed.Tasks.CompletedToday++
				}
			}
			if len(completedMonth) > 0 {
				var totalDays float64
				for _, task := range completedMonth {
					days := task.Updated.Sub(task.Created).Hours() / 24
					totalDays += days
				}
				ed.Tasks.AvgCycleTimeDays = totalDays / float64(len(completedMonth))
			}
		}

		ed.Tasks.TotalTasks = ed.Tasks.ActiveTasks + ed.Tasks.CompletedMonth

		// Confluence
		confMetrics, err := h.confluence.GetEmployeeMetrics(emp)
		if err != nil {
			log.Printf("Error fetching Confluence for %s: %v", emp.Name, err)
		} else {
			ed.Confluence = confMetrics
		}

		// GitLab
		gitMetrics, err := h.gitlab.GetEmployeeMetrics(emp)
		if err != nil {
			log.Printf("Error fetching GitLab for %s: %v", emp.Name, err)
		} else {
			ed.GitLab = gitMetrics
		}

		// Alerts
		empAlerts := h.alertEng.CheckAlerts(emp, activeTasks, gitMetrics)
		ed.Alerts = empAlerts
		allAlerts = append(allAlerts, empAlerts...)

		employees = append(employees, ed)
	}

	// Summary
	summary := models.TeamSummary{}
	for _, ed := range employees {
		summary.TotalActiveTasks += ed.Tasks.ActiveTasks
		summary.TotalCompletedToday += ed.Tasks.CompletedToday
		summary.TotalCompletedMonth += ed.Tasks.CompletedMonth
		summary.TotalMRsMonth += ed.GitLab.MRsMergedMonth
		summary.TotalPagesMonth += ed.Confluence.PagesCreatedMonth + ed.Confluence.PagesUpdatedMonth
	}
	summary.TotalAlerts = len(allAlerts)
	for _, a := range allAlerts {
		if a.Severity == "critical" {
			summary.CriticalAlerts++
		}
	}

	h.mu.Lock()
	h.dashboard = &models.DashboardResponse{
		Employees:   employees,
		Summary:     summary,
		Alerts:      allAlerts,
		LastUpdated: now,
	}
	h.lastUpdated = now
	// Invalidate cache on fresh data
	h.cache = make(map[string]interface{})
	h.mu.Unlock()

	log.Printf("Data collection complete. %d employees, %d alerts", len(employees), len(allAlerts))
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/dashboard", h.handleDashboard)
	mux.HandleFunc("/api/alerts", h.handleAlerts)
	mux.HandleFunc("/api/tasks", h.handleTasks)
	mux.HandleFunc("/api/tasks/export", h.handleTasksExport)
	mux.HandleFunc("/api/tasks/comments", h.handleTaskComments)
	mux.HandleFunc("/api/merge-requests", h.handleMergeRequests)
	mux.HandleFunc("/api/pushes", h.handlePushes)
	mux.HandleFunc("/api/vpn/", h.handleVPNProxy)
	mux.HandleFunc("/api/confluence", h.handleConfluence)
	mux.HandleFunc("/api/health", h.handleHealth)
	// Also register under /qa-dashboard/ prefix for local testing
	mux.HandleFunc("/qa-dashboard/api/dashboard", h.handleDashboard)
	mux.HandleFunc("/qa-dashboard/api/alerts", h.handleAlerts)
	mux.HandleFunc("/qa-dashboard/api/tasks", h.handleTasks)
	mux.HandleFunc("/qa-dashboard/api/tasks/export", h.handleTasksExport)
	mux.HandleFunc("/qa-dashboard/api/tasks/comments", h.handleTaskComments)
	mux.HandleFunc("/qa-dashboard/api/merge-requests", h.handleMergeRequests)
	mux.HandleFunc("/qa-dashboard/api/pushes", h.handlePushes)
	mux.HandleFunc("/qa-dashboard/api/confluence", h.handleConfluence)
	mux.HandleFunc("/qa-dashboard/api/health", h.handleHealth)

}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Always use filtered logic (default = current month)
	monthParam := r.URL.Query().Get("month")
	if monthParam == "" {
		monthParam = time.Now().Format("2006-01")
	}
	h.handleDashboardFiltered(w, r, monthParam)
}

func (h *Handler) handleDashboardFiltered(w http.ResponseWriter, r *http.Request, monthParam string) {
	// Check cache first
	h.mu.RLock()
	if cached, ok := h.cache["dashboard:"+monthParam]; ok {
		h.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(cached)
		return
	}
	h.mu.RUnlock()

	// Calculate date range
	var monthStart, monthEnd time.Time
	now := time.Now()
	if monthParam == "all" {
		monthStart = dataStartDate
		monthEnd = time.Time{} // no upper bound
	} else {
		t, err := time.Parse("2006-01", monthParam)
		if err == nil {
			monthStart = t
			monthEnd = t.AddDate(0, 1, 0)
		} else {
			monthStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			monthEnd = monthStart.AddDate(0, 1, 0)
		}
	}

	// Determine until for queries
	var until time.Time
	if monthParam != "all" {
		until = monthEnd
	}

	var employees []models.EmployeeDashboard
	var allAlerts []models.Alert

	for _, emp := range h.cfg.Employees {
		ed := models.EmployeeDashboard{Employee: emp, LastUpdated: now}

		// Tasks in date range
		issues, _ := h.jira.GetEmployeeTasksRange(emp, monthStart, until)
		ed.Tasks.ActiveTasks = 0
		ed.Tasks.BacklogTasks = 0
		ed.Tasks.ClosedTasks = 0
		ed.Tasks.CompletedMonth = 0
		ed.Tasks.ByStatus = make(map[string]int)
		ed.Tasks.ByType = make(map[string]int)
		for _, issue := range issues {
			ed.Tasks.ByStatus[issue.Status]++
			ed.Tasks.ByType[issue.Type]++
			// Check if task is completed (case-insensitive check)
			status := strings.ToLower(issue.Status)

			// Бэклог: "Готово к тестированию"
			if isBacklogStatus(status) {
				ed.Tasks.BacklogTasks++
			}
			// Активные: "Тестирование", "Готово к внедрению", "Регрессионное тестирование"
			if isActiveTestingStatus(status) {
				ed.Tasks.ActiveTasks++
				// Зависшие: > 3 дней в активных статусах
				days := int(now.Sub(issue.StatusSince).Hours() / 24)
				if days >= 3 {
					ed.Tasks.StaleTasks++
				}
			}
			// Закрытые: "Готово к релизу", "Выполнено", "Готово"
			if isClosedStatus(status) {
				ed.Tasks.ClosedTasks++
			}
			// CompletedMonth для summary (оставляем совместимость)
			if isCompletedStatus(status) {
				ed.Tasks.CompletedMonth++
			}
		}
		ed.Tasks.TotalTasks = len(issues)

		// Confluence
		confMetrics, _ := h.confluence.GetEmployeeMetrics(emp)
		ed.Confluence = confMetrics

		// GitLab — use filtered period
		var mrs []models.MergeRequest
		if monthParam == "all" {
			mrs, _ = h.gitlab.GetEmployeeMRDetailsSince(emp, monthStart)
		} else {
			mrs, _ = h.gitlab.GetEmployeeMRDetailsRange(emp, monthStart, until)
		}
		gitMetrics := countMRMetrics(mrs)
		ed.GitLab = gitMetrics

		// Alerts
		activeIssues, _ := h.jira.GetEmployeeTasks(emp)
		empAlerts := h.alertEng.CheckAlerts(emp, activeIssues, gitMetrics)
		ed.Alerts = empAlerts
		allAlerts = append(allAlerts, empAlerts...)

		employees = append(employees, ed)
	}

	summary := models.TeamSummary{}
	for _, ed := range employees {
		summary.TotalActiveTasks += ed.Tasks.ActiveTasks
		summary.TotalCompletedMonth += ed.Tasks.CompletedMonth
		summary.TotalMRsMonth += ed.GitLab.MRsMergedMonth
		summary.TotalPagesMonth += ed.Confluence.PagesCreatedMonth + ed.Confluence.PagesUpdatedMonth
	}
	summary.TotalAlerts = len(allAlerts)
	for _, a := range allAlerts {
		if a.Severity == "critical" {
			summary.CriticalAlerts++
		}
	}

	resp := models.DashboardResponse{
		Employees:   employees,
		Summary:     summary,
		Alerts:      allAlerts,
		LastUpdated: now,
	}

	// Store in cache
	h.mu.Lock()
	h.cache["dashboard:"+monthParam] = &resp
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleAlerts(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	data := h.dashboard
	h.mu.RUnlock()
	if data == nil {
		http.Error(w, "Data not yet available", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(data.Alerts)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"last_updated": h.lastUpdated,
	})
}

func (h *Handler) handleTasks(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	data := h.dashboard
	h.mu.RUnlock()
	if data == nil {
		http.Error(w, "Data not yet available", http.StatusServiceUnavailable)
		return
	}

	monthParam := r.URL.Query().Get("month")
	cacheKey := "tasks:" + monthParam

	// Check cache
	h.mu.RLock()
	if cached, ok := h.cache[cacheKey]; ok {
		h.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(cached)
		return
	}
	h.mu.RUnlock()

	// Parse month filter — get both start and end
	monthStart, monthEnd := getMonthRange(r)

	type TasksResponse struct {
		Employee   string             `json:"employee"`
		Issues     []models.JiraIssue `json:"issues"`
		Conclusion string             `json:"conclusion"`
	}

	// Determine until: if "all" → no upper bound (zero time), otherwise use monthEnd
	var until time.Time
	if monthParam != "all" {
		until = monthEnd
	}

	var result []TasksResponse
	for _, emp := range data.Employees {
		issues, err := h.jira.GetEmployeeTasksRange(emp.Employee, monthStart, until)
		if err != nil {
			issues = []models.JiraIssue{}
		}

		conclusion := generateTasksConclusion(emp.Employee.Name, issues, emp.Tasks)
		result = append(result, TasksResponse{
			Employee:   emp.Employee.Name,
			Issues:     issues,
			Conclusion: conclusion,
		})
	}

	// Cache result
	h.mu.Lock()
	h.cache[cacheKey] = result
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleTaskComments(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	comments, err := h.jira.GetIssueComments(key)
	if err != nil {
		comments = []models.JiraComment{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	json.NewEncoder(w).Encode(comments)
}

func (h *Handler) handleTasksExport(w http.ResponseWriter, r *http.Request) {
	// Parse params: month, employee (comma-separated emails)
	monthStart, monthEnd := getMonthRange(r)
	employeeFilter := r.URL.Query().Get("employees") // comma-separated names

	var until time.Time
	if r.URL.Query().Get("month") != "all" {
		until = monthEnd
	}

	type ExportIssue struct {
		Key      string `json:"key"`
		Employee string `json:"employee"`
		Summary  string `json:"summary"`
		Type     string `json:"type"`
		Status   string `json:"status"`
		Project  string `json:"project"`
		Created  string `json:"created"`
		Updated  string `json:"updated"`
		URL      string `json:"url"`
		Comments []models.JiraComment `json:"comments"`
	}

	var result []ExportIssue

	for _, emp := range h.cfg.Employees {
		// Filter by employee if specified
		if employeeFilter != "" {
			found := false
			for _, name := range strings.Split(employeeFilter, ",") {
				if strings.TrimSpace(name) == emp.Name {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		issues, err := h.jira.GetEmployeeTasksRange(emp, monthStart, until)
		if err != nil {
			continue
		}

		for _, issue := range issues {
			// Fetch comments for this issue
			comments, _ := h.jira.GetIssueComments(issue.Key)

			result = append(result, ExportIssue{
				Key:      issue.Key,
				Employee: emp.Name,
				Summary:  issue.Summary,
				Type:     issue.Type,
				Status:   issue.Status,
				Project:  issue.Project,
				Created:  issue.Created.Format("2006-01-02"),
				Updated:  issue.Updated.Format("2006-01-02"),
				URL:      issue.URL,
				Comments: comments,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleMergeRequests(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	data := h.dashboard
	h.mu.RUnlock()
	if data == nil {
		http.Error(w, "Data not yet available", http.StatusServiceUnavailable)
		return
	}

	monthParam := r.URL.Query().Get("month")
	cacheKey := "mr:" + monthParam

	// Check cache
	h.mu.RLock()
	if cached, ok := h.cache[cacheKey]; ok {
		h.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(cached)
		return
	}
	h.mu.RUnlock()

	monthStart, monthEnd := getMonthRange(r)

	var result []models.MRDetailResponse
	for _, emp := range data.Employees {
		// Only show automation testers in MR tab
		if !emp.Employee.ShowInMR {
			continue
		}
		var mrs []models.MergeRequest
		var err error
		if r.URL.Query().Get("month") == "all" {
			mrs, err = h.gitlab.GetEmployeeMRDetailsRangeMulti(emp.Employee, monthStart, time.Time{})
		} else {
			mrs, err = h.gitlab.GetEmployeeMRDetailsRangeMulti(emp.Employee, monthStart, monthEnd)
		}
		if err != nil {
			mrs = []models.MergeRequest{}
		}

		metrics := countMRMetrics(mrs)
		conclusion := generateMRConclusion(emp.Employee.Name, mrs, metrics)

		result = append(result, models.MRDetailResponse{
			Employee:   emp.Employee.Name,
			MRs:        mrs,
			Metrics:    metrics,
			Conclusion: conclusion,
		})
	}

	// Cache result
	h.mu.Lock()
	h.cache[cacheKey] = result
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handlePushes(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	data := h.dashboard
	h.mu.RUnlock()
	if data == nil {
		http.Error(w, "Data not yet available", http.StatusServiceUnavailable)
		return
	}

	monthStart, _ := getMonthRange(r)

	type PushEvent struct {
		Employee  string `json:"employee"`
		Repo      string `json:"repo"`
		Team      string `json:"team"`
		Date      string `json:"date"`
		CommitMsg string `json:"commit_msg"`
	}

	var result []PushEvent
	for _, emp := range data.Employees {
		if !emp.Employee.ShowInMR {
			continue
		}
		events, err := h.gitlab.GetUserPushEvents(emp.Employee, monthStart)
		if err != nil {
			continue
		}
		for _, ev := range events {
			result = append(result, PushEvent{
				Employee:  emp.Employee.Name,
				Repo:      ev.Project,
				Team:      emp.Employee.Team,
				Date:      ev.Date,
				CommitMsg: ev.CommitTitle,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleVPNProxy(w http.ResponseWriter, r *http.Request) {
	// Proxy /api/vpn/* to Flask app on localhost:5000/api/*
	targetPath := strings.TrimPrefix(r.URL.Path, "/api/vpn")
	if targetPath == "" {
		targetPath = "/"
	}

	// For upload - handle directly to avoid multipart proxy issues
	if targetPath == "/upload" && r.Method == "POST" {
		h.handleVPNUploadDirect(w, r)
		return
	}

	targetURL := "http://localhost:5000/api" + targetPath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, "Proxy error", http.StatusInternalServerError)
		return
	}

	if ct := r.Header.Get("Content-Type"); ct != "" {
		proxyReq.Header.Set("Content-Type", ct)
	}
	proxyReq.ContentLength = int64(len(bodyBytes))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, "VPN service unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) handleVPNUploadDirect(w http.ResponseWriter, r *http.Request) {
	// Save multipart file to disk, then call Flask with file path
	err := r.ParseMultipartForm(50 << 20) // 50MB max
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		fmt.Fprintf(w, `{"error":"Ошибка парсинга формы: %s"}`, err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		fmt.Fprintf(w, `{"error":"Нет файла: %s"}`, err.Error())
		return
	}
	defer file.Close()

	// Save to temp file
	tmpPath := "/tmp/vpn_upload_" + header.Filename
	dst, err := os.Create(tmpPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprintf(w, `{"error":"Не удалось сохранить файл"}`)
		return
	}
	io.Copy(dst, file)
	dst.Close()

	// Re-upload to Flask from Go (localhost, no proxy issues)
	f, _ := os.Open(tmpPath)
	defer f.Close()
	defer os.Remove(tmpPath)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("file", header.Filename)
	io.Copy(part, f)
	mw.Close()

	req, _ := http.NewRequest("POST", "http://localhost:5000/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		fmt.Fprintf(w, `{"error":"Flask недоступен"}`)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *Handler) handleConfluence(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	data := h.dashboard
	h.mu.RUnlock()
	if data == nil {
		http.Error(w, "Data not yet available", http.StatusServiceUnavailable)
		return
	}

	monthParam := r.URL.Query().Get("month")
	cacheKey := "confluence:" + monthParam

	// Check cache
	h.mu.RLock()
	if cached, ok := h.cache[cacheKey]; ok {
		h.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(cached)
		return
	}
	h.mu.RUnlock()

	monthStart, monthEnd := getMonthRange(r)

	var result []models.ConfluenceDetailResponse
	for _, emp := range data.Employees {
		var pages []models.ConfluencePage
		var err error
		if r.URL.Query().Get("month") == "all" {
			pages, err = h.confluence.GetEmployeePageDetailsSince(emp.Employee, monthStart)
		} else {
			pages, err = h.confluence.GetEmployeePageDetailsRange(emp.Employee, monthStart, monthEnd)
		}
		if err != nil {
			pages = []models.ConfluencePage{}
		}

		metrics := emp.Confluence
		conclusion := generateConfluenceConclusion(emp.Employee.Name, pages, metrics)

		result = append(result, models.ConfluenceDetailResponse{
			Employee:   emp.Employee.Name,
			Pages:      pages,
			Metrics:    metrics,
			Conclusion: conclusion,
		})
	}

	// Cache result
	h.mu.Lock()
	h.cache[cacheKey] = result
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(result)
}

// === Conclusion generators ===

func generateTasksConclusion(name string, issues []models.JiraIssue, metrics models.TaskMetrics) string {
	var issues2 []string
	now := time.Now()

	// Count stale
	staleCount := 0
	noDescription := 0
	for _, issue := range issues {
		days := int(now.Sub(issue.StatusSince).Hours() / 24)
		if days >= 5 {
			staleCount++
		}
		if issue.Summary == "" {
			noDescription++
		}
	}

	if staleCount > 0 {
		issues2 = append(issues2, fmt.Sprintf("%d задач зависли (>5 дней в одном статусе)", staleCount))
	}

	if noDescription > 0 {
		issues2 = append(issues2, fmt.Sprintf("%d задач без описания — необходимо заполнить", noDescription))
	}

	if metrics.ActiveTasks > 10 {
		issues2 = append(issues2, fmt.Sprintf("Много активных задач (%d) — возможна перегрузка", metrics.ActiveTasks))
	}

	if metrics.CompletedMonth == 0 && metrics.ActiveTasks > 0 {
		issues2 = append(issues2, "Нет завершённых задач за месяц при наличии активных")
	}

	if metrics.AvgCycleTimeDays > 10 {
		issues2 = append(issues2, fmt.Sprintf("Высокий cycle time: %.1f дней в среднем", metrics.AvgCycleTimeDays))
	}

	if len(issues2) == 0 {
		return "Задачи обрабатываются в нормальном режиме. Замечаний нет."
	}

	result := "Рекомендации: "
	for i, issue := range issues2 {
		if i > 0 {
			result += "; "
		}
		result += issue
	}
	return result
}

func generateMRConclusion(name string, mrs []models.MergeRequest, metrics models.GitLabMetrics) string {
	var issues []string

	longOpen := 0
	for _, mr := range mrs {
		if mr.State == "opened" && mr.DaysOpen > 3 {
			longOpen++
		}
	}
	if longOpen > 0 {
		issues = append(issues, fmt.Sprintf("%d MR открыты более 3 дней — требуют внимания", longOpen))
	}

	failedPipelines := 0
	for _, mr := range mrs {
		if mr.PipelineStatus == "failed" {
			failedPipelines++
		}
	}
	if failedPipelines > 0 {
		issues = append(issues, fmt.Sprintf("%d MR с упавшим pipeline", failedPipelines))
	}

	if metrics.MRsWithoutReview > 0 {
		issues = append(issues, fmt.Sprintf("%d MR без ревьюера", metrics.MRsWithoutReview))
	}

	conflicts := 0
	for _, mr := range mrs {
		if mr.HasConflicts {
			conflicts++
		}
	}
	if conflicts > 0 {
		issues = append(issues, fmt.Sprintf("%d MR с конфликтами", conflicts))
	}

	if len(mrs) == 0 {
		issues = append(issues, "Нет MR за выбранный период")
	}

	if len(issues) == 0 {
		return "Всё в порядке. MR обрабатываются в нормальном режиме."
	}

	result := "Обратить внимание: "
	for i, issue := range issues {
		if i > 0 {
			result += "; "
		}
		result += issue
	}
	return result
}

func generateConfluenceConclusion(name string, pages []models.ConfluencePage, metrics models.ConfluenceMetrics) string {
	var issues []string

	if len(pages) == 0 {
		issues = append(issues, "Нет активности в Confluence за выбранный период")
	}

	shortPages := 0
	for _, p := range pages {
		if p.BodyLength > 0 && p.BodyLength < 500 {
			shortPages++
		}
	}
	if shortPages > 0 {
		issues = append(issues, fmt.Sprintf("%d страниц с минимальным содержимым (<500 символов)", shortPages))
	}

	stalePages := 0
	for _, p := range pages {
		if p.DaysSinceUpdate > 30 {
			stalePages++
		}
	}
	if stalePages > 0 {
		issues = append(issues, fmt.Sprintf("%d страниц не обновлялись >30 дней", stalePages))
	}

	if metrics.QualityScore < 50 {
		issues = append(issues, "Низкий показатель качества документации")
	}

	if len(issues) == 0 {
		return "Документация ведётся активно. Замечаний нет."
	}

	result := "Обратить внимание: "
	for i, issue := range issues {
		if i > 0 {
			result += "; "
		}
		result += issue
	}
	return result
}

func countMRMetrics(mrs []models.MergeRequest) models.GitLabMetrics {
	var m models.GitLabMetrics
	m.MRsCreatedMonth = len(mrs)
	for _, mr := range mrs {
		if mr.State == "merged" {
			m.MRsMergedMonth++
		}
		if mr.State == "opened" {
			m.MRsOpen++
			if len(mr.Reviewers) == 0 {
				m.MRsWithoutReview++
			}
		}
	}
	return m
}

// helper for unused import
var _ = strconv.Itoa

// isActiveWorkStatus checks if a status is "В работе", "На анализе", or "Analysis"
func isActiveWorkStatus(status string) bool {
	return strings.Contains(status, "в работе") ||
		strings.Contains(status, "на анализе") ||
		status == "analysis" ||
		status == "analytics"
}

// isBacklogStatus checks if a task is in "Готово к тестированию"
func isBacklogStatus(status string) bool {
	return strings.Contains(status, "готово к тестированию") ||
		strings.EqualFold(status, "Backlog") ||
		strings.EqualFold(status, "backlog")
}

// isActiveTestingStatus checks if a task is in active testing statuses
func isActiveTestingStatus(status string) bool {
	return status == "тестирование" ||
		strings.Contains(status, "готово к внедрению") ||
		strings.Contains(status, "регрессионное тестирование")
}

// isClosedStatus checks if a task is in closed/done statuses
func isClosedStatus(status string) bool {
	return strings.Contains(status, "готово к релизу") ||
		status == "выполнено" ||
		status == "готово"
}

// isCompletedStatus checks if a lowercased status name indicates completion
func isCompletedStatus(status string) bool {
	completedStatuses := []string{
		"готово", "выполнено", "done", "closed", "закрыт", "закрыта",
		"ready for development", "готово к оценке", "готова к archqg",
		"готово к тестированию", "resolved", "complete", "завершено",
	}
	for _, s := range completedStatuses {
		if status == s {
			return true
		}
	}
	// Also check if contains key words
	if strings.Contains(status, "готово") || strings.Contains(status, "done") || strings.Contains(status, "closed") || strings.Contains(status, "resolved") {
		return true
	}
	return false
}
