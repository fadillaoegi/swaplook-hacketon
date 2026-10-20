package domain

import (
	"errors"
	"fmt"
	"sort"
)

var (
	ErrInvalid  = errors.New("invalid request")
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("availability changed")
	ErrLimit    = errors.New("session preview limit reached")
	ErrProvider = errors.New("preview provider unavailable")
	ErrQuota    = errors.New("preview allowance reached")
	// ErrUnavailable is a stock/price conflict at selection; it is also an ErrConflict.
	ErrUnavailable = fmt.Errorf("%w: no longer available", ErrConflict)
)

type Product struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Price    int            `json:"price"`
	Currency string         `json:"currency"`
	Color    string         `json:"color"`
	Material string         `json:"material"`
	Cut      string         `json:"cut"`
	Category string         `json:"category"`
	Image    string         `json:"image"`
	Stock    map[string]int `json:"stock"`
}
type Constraints struct {
	OriginalID string `json:"original_id"`
	Size       string `json:"size"`
	Budget     int    `json:"budget"`
	Color      string `json:"color"`
	Material   string `json:"material"`
}

func (c Constraints) Validate() error {
	if c.Budget < 1 || c.Budget > 100000 || !ValidSize(c.Size) || c.OriginalID == "" {
		return ErrInvalid
	}
	return nil
}
func ValidSize(s string) bool { return s == "XS" || s == "S" || s == "M" || s == "L" || s == "XL" }

type Candidate struct {
	Product   Product  `json:"product"`
	Reasons   []string `json:"reasons"`
	Tradeoffs []string `json:"tradeoffs"`
	score     int
	delta     int
}
type Shortlist struct {
	Constraints Constraints `json:"constraints"`
	Candidates  []Candidate `json:"candidates"`
}

func Eligible(p Product, c Constraints) bool {
	return p.ID != c.OriginalID && p.Category == "tops" && p.Currency == "USD" && p.Stock[c.Size] > 0 && p.Price <= c.Budget && (c.Color == "" || p.Color == c.Color) && (c.Material == "" || p.Material == c.Material)
}
func Alternatives(products []Product, c Constraints) (Shortlist, error) {
	out := Shortlist{Constraints: c, Candidates: []Candidate{}}
	if err := c.Validate(); err != nil {
		return out, err
	}
	var original *Product
	for i := range products {
		if products[i].ID == c.OriginalID {
			original = &products[i]
			break
		}
	}
	if original == nil {
		return out, ErrNotFound
	}
	if original.Stock[c.Size] > 0 {
		return out, fmt.Errorf("%w: your original is available in this size", ErrConflict)
	}
	for _, p := range products {
		if !Eligible(p, c) {
			continue
		}
		a := Candidate{Product: p, Reasons: []string{"Size " + c.Size + " in stock", "Within your budget"}, Tradeoffs: []string{}, delta: abs(p.Price - original.Price)}
		for _, v := range [][3]string{{"color", p.Color, original.Color}, {"material", p.Material, original.Material}, {"cut", p.Cut, original.Cut}} {
			if v[1] == v[2] {
				a.score++
				a.Reasons = append(a.Reasons, "Same "+v[0]+": "+v[1])
			} else {
				a.Tradeoffs = append(a.Tradeoffs, v[0]+": "+v[1]+" instead of "+v[2])
			}
		}
		out.Candidates = append(out.Candidates, a)
	}
	sort.Slice(out.Candidates, func(i, j int) bool {
		a, b := out.Candidates[i], out.Candidates[j]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.delta != b.delta {
			return a.delta < b.delta
		}
		return a.Product.ID < b.Product.ID
	})
	if len(out.Candidates) > 3 {
		out.Candidates = out.Candidates[:3]
	}
	return out, nil
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
