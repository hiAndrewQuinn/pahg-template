package server

import (
	"pahg-template/internal/coingecko"
	"pahg-template/internal/notifications"
	"pahg-template/internal/session"
)

// MockCoinService is a mock implementation of CoinService for testing
type MockCoinService struct {
	Coins       []coingecko.Coin
	GetPricesErr error
	GetCoinErr   error
	SearchErr    error
}

func (m *MockCoinService) GetPrices() ([]coingecko.Coin, error) {
	if m.GetPricesErr != nil {
		return nil, m.GetPricesErr
	}
	return m.Coins, nil
}

func (m *MockCoinService) GetCoin(id string) (*coingecko.Coin, error) {
	if m.GetCoinErr != nil {
		return nil, m.GetCoinErr
	}
	for _, coin := range m.Coins {
		if coin.ID == id {
			return &coin, nil
		}
	}
	return nil, coingecko.ErrCoinNotFound
}

func (m *MockCoinService) SearchCoins(query string) ([]coingecko.Coin, error) {
	if m.SearchErr != nil {
		return nil, m.SearchErr
	}
	if query == "" {
		return m.Coins, nil
	}
	var results []coingecko.Coin
	for _, coin := range m.Coins {
		if contains(coin.DisplayName, query) || contains(coin.ID, query) {
			results = append(results, coin)
		}
	}
	return results, nil
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}

// MockSessionStore is a mock implementation of SessionStore for testing
type MockSessionStore struct {
	Sessions     map[string]*session.Session
	CreateErr    error
	LastUsername string
}

func NewMockSessionStore() *MockSessionStore {
	return &MockSessionStore{
		Sessions: make(map[string]*session.Session),
	}
}

func (m *MockSessionStore) Create(username string) (*session.Session, error) {
	if m.CreateErr != nil {
		return nil, m.CreateErr
	}
	m.LastUsername = username
	sess := &session.Session{
		ID:       "mock-session-id-" + username,
		Username: username,
	}
	m.Sessions[sess.ID] = sess
	return sess, nil
}

func (m *MockSessionStore) Get(sessionID string) *session.Session {
	return m.Sessions[sessionID]
}

func (m *MockSessionStore) Delete(sessionID string) {
	delete(m.Sessions, sessionID)
}

func (m *MockSessionStore) Count() int {
	return len(m.Sessions)
}

func (m *MockSessionStore) Close() {}

// MockNotificationStore is a mock implementation of NotificationStore for testing
type MockNotificationStore struct {
	Notifications []notifications.Notification
	NextID        int
}

func NewMockNotificationStore() *MockNotificationStore {
	return &MockNotificationStore{
		Notifications: []notifications.Notification{},
		NextID:        1,
	}
}

func (m *MockNotificationStore) Add(title, message string) notifications.Notification {
	n := notifications.Notification{
		ID:      m.NextID,
		Title:   title,
		Message: message,
	}
	m.NextID++
	m.Notifications = append(m.Notifications, n)
	return n
}

func (m *MockNotificationStore) GetAll() []notifications.Notification {
	// Return in reverse order (newest first)
	result := make([]notifications.Notification, len(m.Notifications))
	for i, n := range m.Notifications {
		result[len(m.Notifications)-1-i] = n
	}
	return result
}

func (m *MockNotificationStore) Count() int {
	return len(m.Notifications)
}

func (m *MockNotificationStore) Clear() {
	m.Notifications = []notifications.Notification{}
}
