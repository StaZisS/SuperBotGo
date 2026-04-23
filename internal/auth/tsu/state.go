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

type flowKind string

const (
	flowKindLink  flowKind = "link"
	flowKindLogin flowKind = "login"
)

type FlowState struct {
	Kind     flowKind
	UserID   model.GlobalUserID
	ReturnTo string
}

type stateEntry struct {
	FlowState
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
	return s.generateURL(FlowState{Kind: flowKindLink, UserID: userID})
}

// GenerateLoginURL creates a state token for a browser login flow and returns
// the login URL that should be opened by the user agent.
func (s *StateStore) GenerateLoginURL(returnTo string) (string, error) {
	return s.generateURL(FlowState{Kind: flowKindLogin, ReturnTo: returnTo})
}

func (s *StateStore) generateURL(flow FlowState) (string, error) {
	token, err := generateStateToken()
	if err != nil {
		return "", fmt.Errorf("generate state token: %w", err)
	}

	s.mu.Lock()
	s.states[token] = &stateEntry{
		FlowState: flow,
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

// Consume validates and removes a state token, returning the associated flow payload.
func (s *StateStore) Consume(state string) (FlowState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.states[state]
	if !ok {
		return FlowState{}, false
	}
	delete(s.states, state)

	if time.Now().After(entry.ExpiresAt) {
		return FlowState{}, false
	}
	return entry.FlowState, true
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
