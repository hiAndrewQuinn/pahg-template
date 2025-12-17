package coingecko

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"pahg-template/internal/config"
)

// Coin represents a cryptocurrency with its price data
type Coin struct {
	ID          string
	DisplayName string
	Price       float64
	Change24h   float64
}

// Service fetches cryptocurrency prices from CoinGecko
type Service struct {
	client    *http.Client
	coins     []config.CoinConfig
	cache     []Coin
	cacheMu   sync.RWMutex
	cacheTime time.Time
	cacheTTL  time.Duration
}

// NewService creates a new CoinGecko service instance
func NewService(coins []config.CoinConfig) *Service {
	return &Service{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		coins:    coins,
		cacheTTL: 30 * time.Second,
	}
}

// CoinGeckoResponse represents the API response structure
type CoinGeckoResponse map[string]struct {
	USD          float64 `json:"usd"`
	USD24hChange float64 `json:"usd_24h_change"`
}

// ErrCoinNotFound is returned when a coin is not in the tracked list
var ErrCoinNotFound = errors.New("coin not found")

// GetPrices fetches current prices for all tracked coins
func (s *Service) GetPrices() ([]Coin, error) {
	s.cacheMu.RLock()
	if time.Since(s.cacheTime) < s.cacheTTL && len(s.cache) > 0 {
		coins := make([]Coin, len(s.cache))
		copy(coins, s.cache)
		s.cacheMu.RUnlock()
		return coins, nil
	}
	s.cacheMu.RUnlock()

	// Build ID list from config
	ids := make([]string, len(s.coins))
	for i, c := range s.coins {
		ids[i] = c.ID
	}
	idStr := strings.Join(ids, ",")
	url := "https://api.coingecko.com/api/v3/simple/price?ids=" + idStr + "&vs_currencies=usd&include_24hr_change=true"

	resp, err := s.client.Get(url)
	if err != nil {
		return s.fallbackPrices(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return s.fallbackPrices(), nil
	}

	var data CoinGeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return s.fallbackPrices(), nil
	}

	coins := make([]Coin, 0, len(s.coins))
	for _, cfg := range s.coins {
		if coinData, ok := data[cfg.ID]; ok {
			coins = append(coins, Coin{
				ID:          cfg.ID,
				DisplayName: cfg.DisplayName,
				Price:       coinData.USD,
				Change24h:   coinData.USD24hChange,
			})
		}
	}

	s.cacheMu.Lock()
	s.cache = coins
	s.cacheTime = time.Now()
	s.cacheMu.Unlock()

	return coins, nil
}

// GetCoin fetches a single coin by ID
func (s *Service) GetCoin(id string) (*Coin, error) {
	coins, err := s.GetPrices()
	if err != nil {
		return nil, err
	}

	for _, coin := range coins {
		if coin.ID == id {
			return &coin, nil
		}
	}

	return nil, ErrCoinNotFound
}

// SearchCoins filters coins by search query
func (s *Service) SearchCoins(query string) ([]Coin, error) {
	coins, err := s.GetPrices()
	if err != nil {
		return nil, err
	}

	if query == "" {
		return coins, nil
	}

	query = strings.ToLower(query)
	filtered := make([]Coin, 0)
	for _, coin := range coins {
		if strings.Contains(strings.ToLower(coin.DisplayName), query) ||
			strings.Contains(strings.ToLower(coin.ID), query) {
			filtered = append(filtered, coin)
		}
	}

	return filtered, nil
}

// fallbackPrices returns cached or mock data when API is unavailable
func (s *Service) fallbackPrices() []Coin {
	s.cacheMu.RLock()
	if len(s.cache) > 0 {
		coins := make([]Coin, len(s.cache))
		copy(coins, s.cache)
		s.cacheMu.RUnlock()
		return coins
	}
	s.cacheMu.RUnlock()

	// Build display name map
	nameMap := make(map[string]string)
	for _, c := range s.coins {
		nameMap[c.ID] = c.DisplayName
	}

	// Return mock data as last resort
	mockData := []Coin{
		{ID: "bitcoin", Price: 43250.00, Change24h: 2.35},
		{ID: "ethereum", Price: 2280.50, Change24h: 1.87},
		{ID: "dogecoin", Price: 0.0825, Change24h: -0.42},
		{ID: "solana", Price: 98.75, Change24h: 5.12},
		{ID: "cardano", Price: 0.52, Change24h: -1.23},
	}

	result := make([]Coin, 0)
	for _, mock := range mockData {
		if name, ok := nameMap[mock.ID]; ok {
			mock.DisplayName = name
			result = append(result, mock)
		}
	}

	return result
}
