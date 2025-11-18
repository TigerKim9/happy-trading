package store

import (
	"errors"
	"sync"
	"time"

	"github.com/TigerKim9/happy-trading/internal/models"
)

var (
	ErrItemNotFound    = errors.New("item not found")
	ErrOutOfStock      = errors.New("out of stock")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// Store manages the emotion store
type Store struct {
	items map[string]*models.StoreItem
	mu    sync.RWMutex
}

// NewStore creates a new store
func NewStore() *Store {
	s := &Store{
		items: make(map[string]*models.StoreItem),
	}
	s.initializeDefaultItems()
	return s
}

// initializeDefaultItems sets up the initial store inventory
func (s *Store) initializeDefaultItems() {
	emotions := models.DefaultEmotions()

	for _, emotion := range emotions {
		// Create store items with markup from base price
		item := &models.StoreItem{
			ID:          "store-" + emotion.ID,
			EmotionID:   emotion.ID,
			Quantity:    1.0,
			Price:       emotion.BasePrice * 1.1, // 10% markup
			Description: "Purchase " + emotion.NameKR + " directly from the store",
			Stock:       -1, // unlimited
		}
		s.items[item.ID] = item
	}

	// Add special bundles
	s.items["bundle-starter"] = &models.StoreItem{
		ID:          "bundle-starter",
		EmotionID:   "joy",
		Quantity:    5.0,
		Price:       400.0,
		Description: "스타터 팩: 기쁨 5개 (20% 할인)",
		Stock:       100,
	}

	s.items["bundle-chaos"] = &models.StoreItem{
		ID:          "bundle-chaos",
		EmotionID:   "impulse",
		Quantity:    3.0,
		Price:       600.0,
		Description: "카오스 팩: 충동 3개 (20% 할인)",
		Stock:       50,
	}

	s.items["bundle-darkness"] = &models.StoreItem{
		ID:          "bundle-darkness",
		EmotionID:   "despair",
		Quantity:    2.0,
		Price:       320.0,
		Description: "다크니스 팩: 절망 2개 (20% 할인)",
		Stock:       30,
	}
}

// GetAllItems returns all store items
func (s *Store) GetAllItems() []models.StoreItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]models.StoreItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, *item)
	}
	return items
}

// GetItem returns a specific store item
func (s *Store) GetItem(itemID string) (*models.StoreItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.items[itemID]
	if !exists {
		return nil, ErrItemNotFound
	}
	return item, nil
}

// Purchase processes a purchase from the store
func (s *Store) Purchase(itemID string, user *models.User, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[itemID]
	if !exists {
		return ErrItemNotFound
	}

	// Check stock
	if item.Stock != -1 && item.Stock < quantity {
		return ErrOutOfStock
	}

	// Calculate total cost
	totalCost := item.Price * float64(quantity)
	if user.Balance < totalCost {
		return ErrInsufficientFunds
	}

	// Process purchase
	user.Balance -= totalCost

	// Add emotions to user portfolio
	totalQuantity := item.Quantity * float64(quantity)
	if user.Portfolio == nil {
		user.Portfolio = make(map[string]float64)
	}
	user.Portfolio[item.EmotionID] += totalQuantity

	// Reduce stock
	if item.Stock != -1 {
		item.Stock -= quantity
	}

	return nil
}

// AddItem adds a new item to the store
func (s *Store) AddItem(item *models.StoreItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
}

// UpdateStock updates the stock of an item
func (s *Store) UpdateStock(itemID string, newStock int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.items[itemID]
	if !exists {
		return ErrItemNotFound
	}
	item.Stock = newStock
	return nil
}

// GetItemsByEmotion returns all store items for a specific emotion
func (s *Store) GetItemsByEmotion(emotionID string) []models.StoreItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]models.StoreItem, 0)
	for _, item := range s.items {
		if item.EmotionID == emotionID {
			items = append(items, *item)
		}
	}
	return items
}

// PurchaseRecord represents a record of a purchase
type PurchaseRecord struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ItemID    string    `json:"item_id"`
	Quantity  int       `json:"quantity"`
	TotalCost float64   `json:"total_cost"`
	CreatedAt time.Time `json:"created_at"`
}
