package candidates

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Handler struct {
	store CandidateStore
}

func NewHandler(dataPath string) *Handler {
	store, err := NewStore(dataPath)
	if err != nil {
		log.Printf("Warning: candidates store init: %v", err)
	}
	return &Handler{store: store}
}

// NewHandlerWithDB creates a handler backed by PostgreSQL
func NewHandlerWithDB(dsn string) *Handler {
	store, err := NewPgStore(dsn)
	if err != nil {
		log.Printf("Warning: PG candidates store init failed: %v, falling back to JSON", err)
		fallback, _ := NewStore("data/candidates.json")
		return &Handler{store: fallback}
	}
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/candidates", h.handleCandidates)
	mux.HandleFunc("/api/candidates/add", h.handleAddCandidate)
	mux.HandleFunc("/api/candidates/update", h.handleUpdateCandidate)
	mux.HandleFunc("/api/candidates/delete", h.handleDeleteCandidate)
	mux.HandleFunc("/api/candidates/conclusion", h.handleUpdateConclusion)
	// Also register under /qa-dashboard/ prefix for local testing
	mux.HandleFunc("/qa-dashboard/api/candidates", h.handleCandidates)
	mux.HandleFunc("/qa-dashboard/api/candidates/add", h.handleAddCandidate)
	mux.HandleFunc("/qa-dashboard/api/candidates/update", h.handleUpdateCandidate)
	mux.HandleFunc("/qa-dashboard/api/candidates/delete", h.handleDeleteCandidate)
	mux.HandleFunc("/qa-dashboard/api/candidates/conclusion", h.handleUpdateConclusion)
}

func (h *Handler) handleCandidates(w http.ResponseWriter, r *http.Request) {
	monthParam := r.URL.Query().Get("month")

	var candidates []Candidate

	if monthParam == "all" {
		candidates = h.store.GetAll()
	} else if monthParam != "" {
		t, err := time.Parse("2006-01", monthParam)
		if err == nil {
			start := t
			end := t.AddDate(0, 1, 0)
			candidates = h.store.GetByRange(start, end)
		} else {
			candidates = h.store.GetAll()
		}
	} else {
		// Default: current month
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 1, 0)
		candidates = h.store.GetByRange(start, end)
	}

	stats := CalculateStats(candidates)
	conclusion := GenerateConclusion(candidates, stats)

	resp := Response{
		Candidates: candidates,
		Stats:      stats,
		Conclusion: conclusion,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleAddCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var c Candidate
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if c.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if err := h.store.Add(c); err != nil {
		http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": c.ID})
}

func (h *Handler) handleUpdateCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var c Candidate
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if c.ID == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	if err := h.store.Update(c); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleDeleteCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) handleUpdateConclusion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID         string `json:"id"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateConclusion(req.ID, req.Conclusion); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
