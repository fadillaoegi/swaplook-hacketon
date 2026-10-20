package domain

import "time"

type Session struct {
	ID        string     `json:"-"`
	Shortlist *Shortlist `json:"shortlist"`
	Decision  *Decision  `json:"decision"`
	Demo      bool       `json:"demo"`
	ExpiresAt time.Time  `json:"expires_at"`
}
type Decision struct {
	ID         string    `json:"id"`
	ProductID  string    `json:"product_id"`
	OriginalID string    `json:"original_id"`
	Size       string    `json:"size"`
	Price      int       `json:"price"`
	Kind       string    `json:"kind"`
	Reason     string    `json:"reason"`
	Demo       bool      `json:"demo"`
	CreatedAt  time.Time `json:"created_at"`
}
type Photo struct {
	ID        string    `json:"id"`
	SessionID string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}
type Job struct {
	ID              string `json:"id"`
	SessionID       string `json:"-"`
	ProductID       string `json:"product_id"`
	Size            string `json:"size"`
	SourceID        string `json:"source_id"`
	Provider        string `json:"provider"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	Image           string `json:"image,omitempty"`
	TaskID          string `json:"-"`
	Polls           int    `json:"polls"`
	Reconciliations int    `json:"reconciliations"`
	// Reused marks a copy of an earlier demo-model result; no new generation was submitted.
	Reused         bool      `json:"reused,omitempty"`
	PollLeaseUntil time.Time `json:"poll_lease_until,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
	CreatedAt      time.Time `json:"created_at"`
}
type Summary struct {
	Decisions     []Decision     `json:"decisions"`
	Sessions      int            `json:"sessions"`
	Choices       int            `json:"choices"`
	Rejections    int            `json:"rejections"`
	Demo          bool           `json:"demo"`
	Substitutions []Substitution `json:"substitutions"`
	Previews      PreviewStats   `json:"previews"`
}

// Substitution counts demo choices of an alternative when an original was unavailable.
type Substitution struct {
	OriginalID string `json:"original_id"`
	ProductID  string `json:"product_id"`
	Count      int    `json:"count"`
}
type PreviewStats struct {
	Provider string `json:"provider"`
	// Available means an actual provider is configured; Enabled means new previews use it.
	Available      bool    `json:"available"`
	Enabled        bool    `json:"enabled"`
	Generations    int     `json:"generations"`
	Today          int     `json:"today"`
	Succeeded      int     `json:"succeeded"`
	Failed         int     `json:"failed"`
	Reused         int     `json:"reused"`
	AverageSeconds float64 `json:"average_seconds"`
	DailyLimit     int     `json:"daily_limit"`
	TotalLimit     int     `json:"total_limit"`
	RemainingToday int     `json:"remaining_today"`
	RemainingTotal int     `json:"remaining_total"`
}

// Generation is a ledger entry for one provider submission. ClientKey is a salted hash, never an address.
type Generation struct {
	ID        string
	Provider  string
	ClientKey string
	CreatedAt time.Time
}
