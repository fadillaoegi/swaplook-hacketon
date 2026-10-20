package domain

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
)

func TestAlternativesHardConstraintsAndStableRanking(t *testing.T) {
	products := []Product{{ID: "original", Price: 5000, Currency: "USD", Category: "tops", Color: "cream", Material: "cotton", Cut: "relaxed", Stock: map[string]int{"M": 0}},
		{ID: "exact", Price: 4500, Currency: "USD", Category: "tops", Color: "cream", Material: "cotton", Cut: "relaxed", Stock: map[string]int{"M": 1}},
		{ID: "tradeoff", Price: 4000, Currency: "USD", Category: "tops", Color: "sage", Material: "cotton", Cut: "relaxed", Stock: map[string]int{"M": 2}},
		{ID: "expensive", Price: 6001, Currency: "USD", Category: "tops", Color: "cream", Material: "cotton", Cut: "relaxed", Stock: map[string]int{"M": 1}},
		{ID: "unavailable", Price: 4000, Currency: "USD", Category: "tops", Color: "cream", Material: "cotton", Cut: "relaxed", Stock: map[string]int{"M": 0}}}
	c := Constraints{OriginalID: "original", Size: "M", Budget: 6000}
	got, e := Alternatives(products, c)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Candidates) != 2 || got.Candidates[0].Product.ID != "exact" {
		t.Fatalf("unexpected shortlist: %+v", got)
	}
	if len(got.Candidates[1].Tradeoffs) != 1 || got.Candidates[1].Tradeoffs[0] != "color: sage instead of cream" {
		t.Fatal("tradeoff must come from metadata")
	}
	c.Color = "cream"
	got, e = Alternatives(products, c)
	if e != nil || len(got.Candidates) != 1 {
		t.Fatal("required color was relaxed")
	}
	c.Budget = 1
	got, e = Alternatives(products, c)
	if e != nil || len(got.Candidates) != 0 || got.Candidates == nil {
		t.Fatal("zero match should be an empty array")
	}
	products[0].Stock["M"] = 1
	if _, e = Alternatives(products, c); !errors.Is(e, ErrConflict) {
		t.Fatal("available original must be explained")
	}
	c.Size = "XXL"
	if _, e = Alternatives(products, c); !errors.Is(e, ErrInvalid) {
		t.Fatal("invalid size accepted")
	}
}
func TestRandomCatalogNeverViolatesConstraints(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 99))
	colors := []string{"cream", "sage", "blue"}
	materials := []string{"cotton", "linen"}
	for run := 0; run < 500; run++ {
		products := []Product{{ID: "original", Price: 5000, Currency: "USD", Category: "tops", Color: "cream", Material: "cotton", Cut: "relaxed", Stock: map[string]int{"M": 0}}}
		for i := 0; i < 40; i++ {
			products = append(products, Product{ID: fmt.Sprint(i), Price: 1000 + rng.IntN(9000), Currency: "USD", Category: "tops", Color: colors[rng.IntN(3)], Material: materials[rng.IntN(2)], Cut: "regular", Stock: map[string]int{"M": rng.IntN(3)}})
		}
		c := Constraints{OriginalID: "original", Size: "M", Budget: 1 + rng.IntN(10000)}
		if run%2 == 0 {
			c.Color = "cream"
		}
		if run%3 == 0 {
			c.Material = "cotton"
		}
		a, e := Alternatives(products, c)
		if e != nil {
			t.Fatal(e)
		}
		if len(a.Candidates) > 3 {
			t.Fatal("more than three candidates")
		}
		for _, v := range a.Candidates {
			if v.Product.Price > c.Budget || v.Product.Stock[c.Size] <= 0 || v.Product.ID == c.OriginalID || (c.Color != "" && v.Product.Color != c.Color) || (c.Material != "" && v.Product.Material != c.Material) {
				t.Fatal("hard constraint violated")
			}
		}
		rng.Shuffle(len(products), func(i, j int) { products[i], products[j] = products[j], products[i] })
		b, e := Alternatives(products, c)
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("ranking depends on input order")
		}
	}
}
