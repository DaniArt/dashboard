package candidates

import "time"

// CandidateStore is the interface for candidate persistence
type CandidateStore interface {
	GetAll() []Candidate
	GetByRange(from, to time.Time) []Candidate
	Add(c Candidate) error
	Update(c Candidate) error
	Delete(id string) error
	UpdateConclusion(id string, conclusion string) error
}
