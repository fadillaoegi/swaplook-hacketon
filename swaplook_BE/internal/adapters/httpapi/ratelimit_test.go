package httpapi

import (
	"testing"
	"time"
)

func TestLimiterWindowsArePerClientAndClass(t *testing.T) {
	now := time.Unix(0, 0)
	l := limiter{now: func() time.Time { return now }}
	for i := 0; i < 15; i++ {
		if !l.allow("a", "upload") {
			t.Fatal("allowed request rejected", i)
		}
	}
	if l.allow("a", "upload") {
		t.Fatal("upload limit not enforced")
	}
	if !l.allow("b", "upload") || !l.allow("a", "session") || !l.allow("a", "") || !l.allow("", "upload") {
		t.Fatal("limit leaked across clients, classes or unlimited routes")
	}
	now = now.Add(time.Hour)
	if !l.allow("a", "upload") {
		t.Fatal("window did not reset")
	}
}
