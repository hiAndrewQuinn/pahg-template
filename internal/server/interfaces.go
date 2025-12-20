package server

import (
	"pahg-template/internal/coingecko"
	"pahg-template/internal/notifications"
	"pahg-template/internal/session"
)

// CoinService defines the interface for cryptocurrency price operations
type CoinService interface {
	GetPrices() ([]coingecko.Coin, error)
	GetCoin(id string) (*coingecko.Coin, error)
	SearchCoins(query string) ([]coingecko.Coin, error)
}

// SessionStore defines the interface for session management
type SessionStore interface {
	Create(username string) (*session.Session, error)
	Get(sessionID string) *session.Session
	Delete(sessionID string)
	Count() int
	Close()
}

// NotificationStore defines the interface for notification management
type NotificationStore interface {
	Add(title, message string) notifications.Notification
	GetAll() []notifications.Notification
	Count() int
	Clear()
}
