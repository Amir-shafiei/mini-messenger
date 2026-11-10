package httpadapter

import (
	"net"
	"sync"
	"time"
)

type attempt struct {
	count int
	until time.Time
}
type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]attempt
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{entries: make(map[string]attempt)} }

// Direct peer IP only: forwarding headers are untrusted. One process, 10/minute/IP.
func (l *loginLimiter) allow(remote string) bool {
	ip, _, err := net.SplitHostPort(remote)
	if err != nil {
		ip = remote
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, a := range l.entries {
		if !now.Before(a.until) {
			delete(l.entries, key)
		}
	}
	a, exists := l.entries[ip]
	if !exists {
		if len(l.entries) >= 10000 {
			return false
		}
		a = attempt{until: now.Add(time.Minute)}
	}
	if a.count >= 10 {
		return false
	}
	a.count++
	l.entries[ip] = a
	return true
}
