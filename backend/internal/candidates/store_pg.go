package candidates

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	_ "github.com/lib/pq"
)

// PgStore is a PostgreSQL-backed store for candidates
type PgStore struct {
	db *sql.DB
}

// NewPgStore creates a new PostgreSQL store
func NewPgStore(dsn string) (*PgStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	log.Printf("Connected to PostgreSQL for candidates")
	return &PgStore{db: db}, nil
}

func (s *PgStore) GetAll() []Candidate {
	return s.queryCandidates("SELECT id, name, date, type, conclusion, result, avg_score, level, grade FROM candidates ORDER BY date DESC")
}

func (s *PgStore) GetByRange(from, to time.Time) []Candidate {
	return s.queryCandidates("SELECT id, name, date, type, conclusion, result, avg_score, level, grade FROM candidates WHERE date >= $1 AND date < $2 ORDER BY date DESC", from, to)
}

func (s *PgStore) queryCandidates(query string, args ...interface{}) []Candidate {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("PgStore query error: %v", err)
		return []Candidate{}
	}
	defer rows.Close()

	var candidates []Candidate
	for rows.Next() {
		var c Candidate
		var conclusion, result, cType, level sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Date, &cType, &conclusion, &result, &c.AvgScore, &level, &c.Grade); err != nil {
			log.Printf("PgStore scan error: %v", err)
			continue
		}
		c.Type = cType.String
		c.Conclusion = conclusion.String
		c.Result = result.String
		c.Level = level.String
		c.Competencies = s.getCompetencies(c.ID)
		candidates = append(candidates, c)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Date.After(candidates[j].Date)
	})
	return candidates
}

func (s *PgStore) getCompetencies(candidateID string) []Competency {
	rows, err := s.db.Query("SELECT name, score, comment FROM candidate_competencies WHERE candidate_id = $1", candidateID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var comps []Competency
	for rows.Next() {
		var c Competency
		var comment sql.NullString
		if err := rows.Scan(&c.Name, &c.Score, &comment); err != nil {
			continue
		}
		c.Comment = comment.String
		comps = append(comps, c)
	}
	return comps
}

func (s *PgStore) Add(c Candidate) error {
	c.ID = fmt.Sprintf("c_%d", time.Now().UnixNano())
	if len(c.Competencies) > 0 {
		total := 0
		for _, comp := range c.Competencies {
			total += comp.Score
		}
		c.AvgScore = math.Round(float64(total)/float64(len(c.Competencies))*10) / 10
		c.Level, c.Grade = LevelFromScore(c.AvgScore)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT INTO candidates (id, name, date, type, conclusion, result, avg_score, level, grade) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		c.ID, c.Name, c.Date, c.Type, c.Conclusion, c.Result, c.AvgScore, c.Level, c.Grade)
	if err != nil {
		return err
	}

	for _, comp := range c.Competencies {
		_, err = tx.Exec("INSERT INTO candidate_competencies (candidate_id, name, score, comment) VALUES ($1, $2, $3, $4)",
			c.ID, comp.Name, comp.Score, comp.Comment)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PgStore) Update(c Candidate) error {
	if len(c.Competencies) > 0 {
		total := 0
		for _, comp := range c.Competencies {
			total += comp.Score
		}
		c.AvgScore = math.Round(float64(total)/float64(len(c.Competencies))*10) / 10
		c.Level, c.Grade = LevelFromScore(c.AvgScore)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec("UPDATE candidates SET name=$1, date=$2, type=$3, conclusion=$4, result=$5, avg_score=$6, level=$7, grade=$8, updated_at=NOW() WHERE id=$9",
		c.Name, c.Date, c.Type, c.Conclusion, c.Result, c.AvgScore, c.Level, c.Grade, c.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("candidate %s not found", c.ID)
	}

	// Replace competencies
	tx.Exec("DELETE FROM candidate_competencies WHERE candidate_id=$1", c.ID)
	for _, comp := range c.Competencies {
		tx.Exec("INSERT INTO candidate_competencies (candidate_id, name, score, comment) VALUES ($1, $2, $3, $4)",
			c.ID, comp.Name, comp.Score, comp.Comment)
	}

	return tx.Commit()
}

func (s *PgStore) Delete(id string) error {
	res, err := s.db.Exec("DELETE FROM candidates WHERE id=$1", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("candidate %s not found", id)
	}
	return nil
}

func (s *PgStore) UpdateConclusion(id string, conclusion string) error {
	res, err := s.db.Exec("UPDATE candidates SET conclusion=$1, updated_at=NOW() WHERE id=$2", conclusion, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("candidate %s not found", id)
	}
	return nil
}
