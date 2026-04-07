package tsu

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"SuperBotGo/internal/model"
)

const (
	stateTTL    = 10 * time.Minute
	stateLength = 32 // bytes → 64 hex chars
)

type stateEntry struct {
	UserID    model.GlobalUserID
	ExpiresAt time.Time
}

type StateStore struct {
	mu       sync.Mutex
	states   map[string]*stateEntry
	loginURL string // e.g. "https://bot.example.com/api/auth/tsu/login"
	stop     chan struct{}
}

func NewStateStore(loginURL string) *StateStore {
	s := &StateStore{
		states:   make(map[string]*stateEntry),
		loginURL: loginURL,
		stop:     make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

func (s *StateStore) Stop() {
	close(s.stop)
}

// GenerateAuthURL creates a state token for the given user and returns the login URL.
func (s *StateStore) GenerateAuthURL(userID model.GlobalUserID) (string, error) {
	token, err := generateStateToken()
	if err != nil {
		return "", fmt.Errorf("generate state token: %w", err)
	}

	s.mu.Lock()
	s.states[token] = &stateEntry{
		UserID:    userID,
		ExpiresAt: time.Now().Add(stateTTL),
	}
	s.mu.Unlock()

	return s.loginURL + "?state=" + token, nil
}

// Verify checks that a state token exists and is not expired, without consuming it.
func (s *StateStore) Verify(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.states[state]
	if !ok {
		return false
	}
	return !time.Now().After(entry.ExpiresAt)
}

// Consume validates and removes a state token, returning the associated user ID.
func (s *StateStore) Consume(state string) (model.GlobalUserID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.states[state]
	if !ok {
		return 0, false
	}
	delete(s.states, state)

	if time.Now().After(entry.ExpiresAt) {
		return 0, false
	}
	return entry.UserID, true
}

func (s *StateStore) cleanupLoop() {
	ticker := time.NewTicker(stateTTL)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for token, entry := range s.states {
				if now.After(entry.ExpiresAt) {
					delete(s.states, token)
				}
			}
			s.mu.Unlock()
		case <-s.stop:
			return
		}
	}
}

func generateStateToken() (string, error) {
	b := make([]byte, stateLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
