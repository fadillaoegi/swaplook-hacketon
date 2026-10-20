package httpapi

import (
	"sync"
	"time"
)

// limiter is a fixed-window, in-memory request limiter keyed by salted client hash and route class.
// It protects a single-instance demo; a multi-instance deployment needs a shared store.
type limiter struct {
	mu      sync.Mutex
	windows map[string]*window
	now     func() time.Time
}
type window struct {
	start time.Time
	count int
}
type rule struct {
	limit int
	every time.Duration
}

var rules = map[string]rule{
	"session":  {30, time.Hour},
	"upload":   {15, time.Hour},
	"merchant": {30, time.Minute},
	"write":    {120, time.Minute},
}

func (l *limiter) allow(client, class string) bool {
	r, ok := rules[class]
	if !ok || client == "" {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windows == nil {
		l.windows = map[string]*window{}
	}
	now := time.Now()
	if l.now != nil {
		now = l.now()
	}
	// Bound memory: drop expired windows when the map grows.
	if len(l.windows) > 10000 {
		for k, w := range l.windows {
			if now.Sub(w.start) > time.Hour {
				delete(l.windows, k)
			}
		}
	}
	key := class + ":" + client
	w := l.windows[key]
	if w == nil || now.Sub(w.start) >= r.every {
		w = &window{start: now}
		l.windows[key] = w
	}
	if w.count >= r.limit {
		return false
	}
	w.count++
	return true
}
