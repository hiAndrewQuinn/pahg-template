package session

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

const (
	sessionIDLength   = 32
	sessionTimeout    = 24 * time.Hour
	cleanupInterval   = 1 * time.Hour
	sessionCookieName = "coinops_session"
)

// Session represents a user session
type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store manages user sessions in memory
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	stopChan chan struct{}
	clock    clockwork.Clock
}

// NewStore creates a new session store with automatic cleanup
func NewStore() *Store {
	return NewStoreWithClock(clockwork.NewRealClock())
}

// NewStoreWithClock creates a new session store with a custom clock (for testing)
func NewStoreWithClock(clock clockwork.Clock) *Store {
	s := &Store{
		sessions: make(map[string]*Session),
		stopChan: make(chan struct{}),
		clock:    clock,
	}

	// Start background cleanup goroutine
	go s.cleanupExpiredSessions()

	return s
}

// Create creates a new session for the given username
func (s *Store) Create(username string) (*Session, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()
	session := &Session{
		ID:        sessionID,
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(sessionTimeout),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return session, nil
}

// Get retrieves a session by ID
// Returns nil if session doesn't exist or has expired
func (s *Store) Get(sessionID string) *Session {
	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		return nil
	}

	// Check expiration
	if s.clock.Now().After(session.ExpiresAt) {
		s.Delete(sessionID)
		return nil
	}

	return session
}

// Delete removes a session
func (s *Store) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// Count returns the number of active sessions
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// cleanupExpiredSessions runs periodically to remove expired sessions
func (s *Store) cleanupExpiredSessions() {
	ticker := s.clock.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			s.cleanup()
		case <-s.stopChan:
			return
		}
	}
}

func (s *Store) cleanup() {
	now := s.clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

// Close stops the cleanup goroutine
func (s *Store) Close() {
	close(s.stopChan)
}

// generateSessionID creates a cryptographically secure random session ID
func generateSessionID() (string, error) {
	bytes := make([]byte, sessionIDLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GetCookieName returns the name of the session cookie
func GetCookieName() string {
	return sessionCookieName
}
