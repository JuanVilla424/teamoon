package web

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const sessionCookieName = "teamoon_session"
const sessionDuration = 24 * time.Hour

type sessionEntry struct {
	createdAt time.Time
	expiresAt time.Time
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionEntry
}

func newSessionStore() *sessionStore {
	s := &sessionStore{sessions: make(map[string]sessionEntry)}
	go s.cleanupLoop()
	return s
}

func (s *sessionStore) create() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	now := time.Now()
	s.mu.Lock()
	s.sessions[token] = sessionEntry{createdAt: now, expiresAt: now.Add(sessionDuration)}
	s.mu.Unlock()
	return token, nil
}

func (s *sessionStore) validate(token string) bool {
	s.mu.RLock()
	entry, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return false
	}
	s.mu.Lock()
	s.sessions[token] = sessionEntry{
		createdAt: entry.createdAt,
		expiresAt: time.Now().Add(sessionDuration),
	}
	s.mu.Unlock()
	return true
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (s *sessionStore) invalidateAll() {
	s.mu.Lock()
	s.sessions = make(map[string]sessionEntry)
	s.mu.Unlock()
}

func (s *sessionStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for token, entry := range s.sessions {
			if now.After(entry.expiresAt) {
				delete(s.sessions, token)
			}
		}
		s.mu.Unlock()
	}
}
