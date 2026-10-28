package middleware

import (
	"sync"
	"time"
)

type attemptWindow struct {
	count   int
	expires time.Time
}

// AttemptLimiter bounds both login work and its own memory. It is deliberately
// process-local: clustered deployments also need a shared gateway limit.
type AttemptLimiter struct {
	mu                sync.Mutex
	entries           map[string]attemptWindow
	limit, maxEntries int
	window            time.Duration
	now               func() time.Time
}

func NewAttemptLimiter(limit int, window time.Duration, maxEntries int) *AttemptLimiter {
	return &AttemptLimiter{entries: make(map[string]attemptWindow), limit: limit, window: window, maxEntries: maxEntries, now: time.Now}
}

func (l *AttemptLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	entry, exists := l.entries[key]
	if exists && now.Before(entry.expires) {
		if entry.count >= l.limit {
			return false
		}
		entry.count++
		l.entries[key] = entry
		return true
	}
	if !exists && len(l.entries) >= l.maxEntries {
		for k, item := range l.entries {
			if !now.Before(item.expires) {
				delete(l.entries, k)
			}
		}
		if len(l.entries) >= l.maxEntries {
			return false
		}
	}
	l.entries[key] = attemptWindow{count: 1, expires: now.Add(l.window)}
	return true
}

func (l *AttemptLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}
