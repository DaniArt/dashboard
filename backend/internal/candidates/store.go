package candidates

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store manages candidate data persistence
type Store struct {
	mu       sync.RWMutex
	path     string
	candidates []Candidate
}

// NewStore creates a new store loading from the given path
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		// File doesn't exist yet — start empty
		s.candidates = []Candidate{}
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.candidates)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.candidates, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

// GetAll returns all candidates
func (s *Store) GetAll() []Candidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Candidate, len(s.candidates))
	copy(result, s.candidates)
	return result
}

// GetByMonth returns candidates for a specific month
func (s *Store) GetByMonth(year int, month int) []Candidate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Candidate
	for _, c := range s.candidates {
		if c.Date.Year() == year && int(c.Date.Month()) == month {
			result = append(result, c)
		}
	}
	return result
}

// GetByRange returns candidates in a date range
func (s *Store) GetByRange(from, to time.Time) []Candidate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Candidate
	for _, c := range s.candidates {
		if (c.Date.Equal(from) || c.Date.After(from)) && c.Date.Before(to) {
			result = append(result, c)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.After(result[j].Date)
	})
	return result
}

// Add adds a new candidate
func (s *Store) Add(c Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate ID
	c.ID = fmt.Sprintf("c_%d", time.Now().UnixNano())

	// Calculate avg score and level
	if len(c.Competencies) > 0 {
		total := 0
		for _, comp := range c.Competencies {
			total += comp.Score
		}
		c.AvgScore = math.Round(float64(total)/float64(len(c.Competencies))*10) / 10
		c.Level, c.Grade = LevelFromScore(c.AvgScore)
	}

	s.candidates = append(s.candidates, c)
	return s.save()
}

// Delete removes a candidate by ID
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, c := range s.candidates {
		if c.ID == id {
			s.candidates = append(s.candidates[:i], s.candidates[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("candidate %s not found", id)
}

// UpdateConclusion updates the conclusion text for a candidate
func (s *Store) UpdateConclusion(id string, conclusion string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, c := range s.candidates {
		if c.ID == id {
			s.candidates[i].Conclusion = conclusion
			return s.save()
		}
	}
	return fmt.Errorf("candidate %s not found", id)
}

// Update replaces a candidate's data (keeps same ID)
func (s *Store) Update(c Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.candidates {
		if existing.ID == c.ID {
			// Recalculate score and level
			if len(c.Competencies) > 0 {
				total := 0
				for _, comp := range c.Competencies {
					total += comp.Score
				}
				c.AvgScore = math.Round(float64(total)/float64(len(c.Competencies))*10) / 10
				c.Level, c.Grade = LevelFromScore(c.AvgScore)
			}
			s.candidates[i] = c
			return s.save()
		}
	}
	return fmt.Errorf("candidate %s not found", c.ID)
}

// CalculateStats computes stats for a set of candidates
func CalculateStats(candidates []Candidate) Stats {
	stats := Stats{Total: len(candidates)}
	if stats.Total == 0 {
		return stats
	}

	var totalScore float64
	for _, c := range candidates {
		totalScore += c.AvgScore
		switch c.Result {
		case "accepted":
			stats.Accepted++
		case "accepted_no_sb":
			stats.NoSB++
		case "pending":
			// pending - not counted in stats
		default:
			stats.Rejected++
		}
	}

	stats.AvgScore = math.Round(totalScore/float64(stats.Total)*10) / 10
	stats.Conversion = math.Round(float64(stats.Accepted)/float64(stats.Total)*100*10) / 10

	return stats
}

// GenerateConclusion creates auto-generated conclusion
func GenerateConclusion(candidates []Candidate, stats Stats) string {
	if stats.Total == 0 {
		return "Нет данных о собеседованиях за выбранный период."
	}

	var parts []string

	parts = append(parts, fmt.Sprintf("Проведено %d собеседований", stats.Total))

	if stats.Accepted > 0 {
		parts = append(parts, fmt.Sprintf("%d принято", stats.Accepted))
	}
	if stats.NoSB > 0 {
		parts = append(parts, fmt.Sprintf("%d не прошли СБ", stats.NoSB))
	}
	parts = append(parts, fmt.Sprintf("конверсия %.0f%%", stats.Conversion))

	conclusion := strings.Join(parts, ", ") + "."

	// Find weak competencies
	if len(candidates) >= 3 {
		compScores := make(map[string]float64)
		compCounts := make(map[string]int)
		for _, c := range candidates {
			for _, comp := range c.Competencies {
				compScores[comp.Name] += float64(comp.Score)
				compCounts[comp.Name]++
			}
		}

		var weakest string
		weakestScore := 10.0
		for name, total := range compScores {
			avg := total / float64(compCounts[name])
			if avg < weakestScore {
				weakestScore = avg
				weakest = name
			}
		}
		if weakest != "" && weakestScore < 3 {
			conclusion += fmt.Sprintf(" Слабая зона кандидатов: %s (средний балл %.1f).", weakest, weakestScore)
		}
	}

	return conclusion
}
