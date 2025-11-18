package p2p

import (
	"errors"
	"sync"
	"time"

	"github.com/TigerKim9/happy-trading/internal/models"
)

var (
	ErrTradeNotFound       = errors.New("trade not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInsufficientAssets  = errors.New("insufficient assets")
	ErrCannotBuyOwnTrade   = errors.New("cannot buy your own trade")
	ErrTradeNotPending     = errors.New("trade is not pending")
)

// P2PMarket manages peer-to-peer trades
type P2PMarket struct {
	trades map[string]*models.P2PTrade
	mu     sync.RWMutex
	idCtr  int64
}

// NewP2PMarket creates a new P2P market
func NewP2PMarket() *P2PMarket {
	return &P2PMarket{
		trades: make(map[string]*models.P2PTrade),
	}
}

// CreateOffer creates a new P2P trade offer
func (m *P2PMarket) CreateOffer(seller *models.User, emotionID string, quantity, price float64, message string) (*models.P2PTrade, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if seller has enough assets
	if seller.Portfolio == nil || seller.Portfolio[emotionID] < quantity {
		return nil, ErrInsufficientAssets
	}

	// Reserve the assets
	seller.Portfolio[emotionID] -= quantity

	m.idCtr++
	trade := &models.P2PTrade{
		ID:        generateP2PID(m.idCtr),
		SellerID:  seller.ID,
		EmotionID: emotionID,
		Quantity:  quantity,
		Price:     price,
		Status:    models.OrderPending,
		Message:   message,
		CreatedAt: time.Now(),
	}

	m.trades[trade.ID] = trade
	return trade, nil
}

// AcceptOffer accepts a P2P trade offer
func (m *P2PMarket) AcceptOffer(tradeID string, buyer *models.User, seller *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	trade, exists := m.trades[tradeID]
	if !exists {
		return ErrTradeNotFound
	}

	if trade.Status != models.OrderPending {
		return ErrTradeNotPending
	}

	if buyer.ID == trade.SellerID {
		return ErrCannotBuyOwnTrade
	}

	// Check if buyer has enough balance
	if buyer.Balance < trade.Price {
		return ErrInsufficientBalance
	}

	// Process the trade
	buyer.Balance -= trade.Price
	seller.Balance += trade.Price

	if buyer.Portfolio == nil {
		buyer.Portfolio = make(map[string]float64)
	}
	buyer.Portfolio[trade.EmotionID] += trade.Quantity

	// Update trade status
	trade.BuyerID = buyer.ID
	trade.Status = models.OrderFilled
	now := time.Now()
	trade.CompletedAt = &now

	return nil
}

// CancelOffer cancels a P2P trade offer
func (m *P2PMarket) CancelOffer(tradeID string, seller *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	trade, exists := m.trades[tradeID]
	if !exists {
		return ErrTradeNotFound
	}

	if trade.SellerID != seller.ID {
		return errors.New("only seller can cancel")
	}

	if trade.Status != models.OrderPending {
		return ErrTradeNotPending
	}

	// Return assets to seller
	if seller.Portfolio == nil {
		seller.Portfolio = make(map[string]float64)
	}
	seller.Portfolio[trade.EmotionID] += trade.Quantity

	trade.Status = models.OrderCancelled
	return nil
}

// GetAllOffers returns all pending P2P offers
func (m *P2PMarket) GetAllOffers() []models.P2PTrade {
	m.mu.RLock()
	defer m.mu.RUnlock()

	offers := make([]models.P2PTrade, 0)
	for _, trade := range m.trades {
		if trade.Status == models.OrderPending {
			offers = append(offers, *trade)
		}
	}
	return offers
}

// GetOffersByEmotion returns all pending offers for a specific emotion
func (m *P2PMarket) GetOffersByEmotion(emotionID string) []models.P2PTrade {
	m.mu.RLock()
	defer m.mu.RUnlock()

	offers := make([]models.P2PTrade, 0)
	for _, trade := range m.trades {
		if trade.Status == models.OrderPending && trade.EmotionID == emotionID {
			offers = append(offers, *trade)
		}
	}
	return offers
}

// GetOffersBySeller returns all offers by a specific seller
func (m *P2PMarket) GetOffersBySeller(sellerID string) []models.P2PTrade {
	m.mu.RLock()
	defer m.mu.RUnlock()

	offers := make([]models.P2PTrade, 0)
	for _, trade := range m.trades {
		if trade.SellerID == sellerID {
			offers = append(offers, *trade)
		}
	}
	return offers
}

// GetOffer returns a specific P2P offer
func (m *P2PMarket) GetOffer(tradeID string) (*models.P2PTrade, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trade, exists := m.trades[tradeID]
	if !exists {
		return nil, ErrTradeNotFound
	}
	return trade, nil
}

// GetTradeHistory returns completed trades
func (m *P2PMarket) GetTradeHistory(userID string) []models.P2PTrade {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]models.P2PTrade, 0)
	for _, trade := range m.trades {
		if trade.Status == models.OrderFilled && (trade.SellerID == userID || trade.BuyerID == userID) {
			history = append(history, *trade)
		}
	}
	return history
}

func generateP2PID(counter int64) string {
	return time.Now().Format("20060102") + "-P2P-" + string(rune('A'+counter%26)) + string(rune('0'+counter%10))
}
