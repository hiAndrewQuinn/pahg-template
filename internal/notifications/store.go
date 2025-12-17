package notifications

import (
	"sync"
	"time"
)

// Notification represents a single notification
type Notification struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Store is a thread-safe in-memory notification store
type Store struct {
	mu            sync.RWMutex
	notifications []Notification
	nextID        int
}

// NewStore creates a new notification store
func NewStore() *Store {
	return &Store{
		notifications: make([]Notification, 0),
		nextID:        1,
	}
}

// Add creates a new notification
func (s *Store) Add(title, message string) Notification {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := Notification{
		ID:        s.nextID,
		Title:     title,
		Message:   message,
		Timestamp: time.Now(),
	}
	s.nextID++

	s.notifications = append(s.notifications, n)
	return n
}

// GetAll returns all notifications (newest first)
func (s *Store) GetAll() []Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Notification, len(s.notifications))
	for i, n := range s.notifications {
		result[len(s.notifications)-1-i] = n
	}

	return result
}

// Count returns the number of notifications
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.notifications)
}

// Clear removes all notifications
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications = make([]Notification, 0)
}
