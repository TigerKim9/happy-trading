package exchange

import (
	"errors"
	"sync"
	"time"

	"github.com/TigerKim9/happy-trading/internal/models"
	"github.com/TigerKim9/happy-trading/internal/orderbook"
	"github.com/TigerKim9/happy-trading/internal/p2p"
	"github.com/TigerKim9/happy-trading/internal/store"
	"github.com/TigerKim9/happy-trading/internal/wallet"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmotionNotFound    = errors.New("emotion not found")
	ErrEmotionExists      = errors.New("emotion already exists")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrInsufficientAssets = errors.New("insufficient assets")
	ErrInvalidOrder       = errors.New("invalid order")
)

// Exchange is the main trading engine
type Exchange struct {
	emotions      map[string]models.Emotion
	users         map[string]*models.User
	orderBooks    map[string]*orderbook.Book
	store         *store.Store
	p2pMarket     *p2p.P2PMarket
	walletService *wallet.WalletService
	orders        map[string]*models.Order
	mu            sync.RWMutex
	orderIDCtr    int64
}

// NewExchange creates a new exchange
func NewExchange() *Exchange {
	ex := &Exchange{
		emotions:      make(map[string]models.Emotion),
		users:         make(map[string]*models.User),
		orderBooks:    make(map[string]*orderbook.Book),
		store:         store.NewStore(),
		p2pMarket:     p2p.NewP2PMarket(),
		walletService: wallet.NewWalletService(),
		orders:        make(map[string]*models.Order),
	}

	// Initialize emotions
	for _, emotion := range models.DefaultEmotions() {
		ex.emotions[emotion.ID] = emotion
		ex.orderBooks[emotion.ID] = orderbook.NewBook(emotion.ID, emotion.BasePrice)
	}

	return ex
}

// Wallet operations
func (ex *Exchange) GetNonce(address string) (string, error) {
	return ex.walletService.GenerateNonce(address)
}

func (ex *Exchange) GetSignMessage(nonce string) string {
	return ex.walletService.GetSignMessage(nonce)
}

func (ex *Exchange) WalletLogin(address, nonce, signature string) (*models.User, bool, error) {
	// Verify signature
	if err := ex.walletService.VerifySignature(address, nonce, signature); err != nil {
		return nil, false, err
	}

	// Get or create user
	user, isNew, err := ex.walletService.GetOrCreateUser(address)
	if err != nil {
		return nil, false, err
	}

	// Also store in exchange users map
	ex.mu.Lock()
	ex.users[user.ID] = user
	ex.mu.Unlock()

	return user, isNew, nil
}

func (ex *Exchange) GetSupportedChains() []models.Chain {
	return models.SupportedChains()
}

func (ex *Exchange) GetDepositAddress(chainID int64) string {
	return ex.walletService.GetDepositAddress(chainID)
}

func (ex *Exchange) RegisterDeposit(userID string, chainID int64, txHash string, amount float64) (*models.Deposit, error) {
	return ex.walletService.RegisterDeposit(userID, chainID, txHash, amount)
}

func (ex *Exchange) ConfirmDeposit(depositID, userID string) error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	user, exists := ex.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	return ex.walletService.ConfirmDeposit(depositID, user)
}

func (ex *Exchange) GetUserDeposits(userID string) []models.Deposit {
	return ex.walletService.GetUserDeposits(userID)
}

func (ex *Exchange) RequestWithdrawal(userID string, chainID int64, toAddress string, amount float64) (*models.Withdrawal, error) {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	user, exists := ex.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}

	return ex.walletService.RequestWithdrawal(userID, chainID, toAddress, amount, user)
}

func (ex *Exchange) GetUserWithdrawals(userID string) []models.Withdrawal {
	return ex.walletService.GetUserWithdrawals(userID)
}

// CreateUser creates a new user with initial balance
func (ex *Exchange) CreateUser(username string, initialBalance float64) *models.User {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	user := &models.User{
		ID:        generateUserID(username),
		Username:  username,
		Balance:   initialBalance,
		Portfolio: make(map[string]float64),
		CreatedAt: time.Now(),
	}

	ex.users[user.ID] = user
	return user
}

// GetUser returns a user by ID
func (ex *Exchange) GetUser(userID string) (*models.User, error) {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	user, exists := ex.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// GetAllEmotions returns all tradeable emotions
func (ex *Exchange) GetAllEmotions() []models.Emotion {
	emotions := make([]models.Emotion, 0, len(ex.emotions))
	for _, e := range ex.emotions {
		emotions = append(emotions, e)
	}
	return emotions
}

// PlaceOrder places a new order on the exchange
func (ex *Exchange) PlaceOrder(userID, emotionID string, side models.OrderSide, orderType models.OrderType, price, quantity float64) (*models.Order, []models.Trade, error) {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	user, exists := ex.users[userID]
	if !exists {
		return nil, nil, ErrUserNotFound
	}

	book, exists := ex.orderBooks[emotionID]
	if !exists {
		return nil, nil, ErrEmotionNotFound
	}

	// Validate order
	if quantity <= 0 {
		return nil, nil, ErrInvalidOrder
	}

	// Check funds/assets
	if side == models.Buy {
		estimatedCost := price * quantity
		if orderType == models.MarketOrder {
			// For market orders, estimate based on current ask
			md := book.GetMarketData()
			if md.AskPrice > 0 {
				estimatedCost = md.AskPrice * quantity * 1.1 // 10% buffer
			}
		}
		if user.Balance < estimatedCost {
			return nil, nil, ErrInsufficientFunds
		}
		// Reserve funds for limit orders
		if orderType == models.LimitOrder {
			user.Balance -= price * quantity
		}
	} else {
		if user.Portfolio[emotionID] < quantity {
			return nil, nil, ErrInsufficientAssets
		}
		// Reserve assets
		user.Portfolio[emotionID] -= quantity
	}

	// Create order
	ex.orderIDCtr++
	order := &models.Order{
		ID:        generateOrderID(ex.orderIDCtr),
		UserID:    userID,
		EmotionID: emotionID,
		Side:      side,
		Type:      orderType,
		Price:     price,
		Quantity:  quantity,
		Status:    models.OrderPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ex.orders[order.ID] = order

	// Match order
	trades := book.AddOrder(order)

	// Process trades
	for _, trade := range trades {
		ex.processTrade(trade)
	}

	// Return unused funds/assets if order partially filled
	ex.settleOrder(order, user)

	return order, trades, nil
}

// processTrade updates user balances after a trade
func (ex *Exchange) processTrade(trade models.Trade) {
	buyer := ex.users[trade.BuyerID]
	seller := ex.users[trade.SellerID]

	cost := trade.Price * trade.Quantity

	// Transfer funds (for market orders where funds weren't reserved)
	order := ex.orders[trade.BuyOrderID]
	if order != nil && order.Type == models.MarketOrder {
		buyer.Balance -= cost
	}

	// Transfer funds to seller
	seller.Balance += cost

	// Transfer assets to buyer
	if buyer.Portfolio == nil {
		buyer.Portfolio = make(map[string]float64)
	}
	buyer.Portfolio[trade.EmotionID] += trade.Quantity
}

// settleOrder returns unused funds/assets after order completion
func (ex *Exchange) settleOrder(order *models.Order, user *models.User) {
	if order.Status == models.OrderFilled || order.Status == models.OrderCancelled {
		if order.Side == models.Buy && order.Type == models.LimitOrder {
			// Return unused funds
			unused := (order.Quantity - order.Filled) * order.Price
			user.Balance += unused
		} else if order.Side == models.Sell {
			// Return unsold assets
			unsold := order.Quantity - order.Filled
			user.Portfolio[order.EmotionID] += unsold
		}
	}
}

// CancelOrder cancels an existing order
func (ex *Exchange) CancelOrder(orderID, userID string) error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	order, exists := ex.orders[orderID]
	if !exists {
		return errors.New("order not found")
	}

	if order.UserID != userID {
		return errors.New("not authorized")
	}

	book := ex.orderBooks[order.EmotionID]
	if !book.CancelOrder(orderID) {
		return errors.New("order not in book")
	}

	user := ex.users[userID]
	ex.settleOrder(order, user)

	return nil
}

// GetOrderBook returns the order book for an emotion
func (ex *Exchange) GetOrderBook(emotionID string, depth int) (*models.OrderBook, error) {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	book, exists := ex.orderBooks[emotionID]
	if !exists {
		return nil, ErrEmotionNotFound
	}

	ob := book.GetOrderBook(depth)
	return &ob, nil
}

// GetMarketData returns market data for an emotion
func (ex *Exchange) GetMarketData(emotionID string) (*models.MarketData, error) {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	book, exists := ex.orderBooks[emotionID]
	if !exists {
		return nil, ErrEmotionNotFound
	}

	md := book.GetMarketData()
	return &md, nil
}

// GetAllMarketData returns market data for all emotions
func (ex *Exchange) GetAllMarketData() []models.MarketData {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	data := make([]models.MarketData, 0, len(ex.orderBooks))
	for _, book := range ex.orderBooks {
		data = append(data, book.GetMarketData())
	}
	return data
}

// GetRecentTrades returns recent trades for an emotion
func (ex *Exchange) GetRecentTrades(emotionID string, limit int) ([]models.Trade, error) {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	book, exists := ex.orderBooks[emotionID]
	if !exists {
		return nil, ErrEmotionNotFound
	}

	return book.GetRecentTrades(limit), nil
}

// GetUserOrders returns all orders for a user
func (ex *Exchange) GetUserOrders(userID string) []models.Order {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	orders := make([]models.Order, 0)
	for _, order := range ex.orders {
		if order.UserID == userID {
			orders = append(orders, *order)
		}
	}
	return orders
}

// Store operations
func (ex *Exchange) GetStoreItems() []models.StoreItem {
	return ex.store.GetAllItems()
}

func (ex *Exchange) PurchaseFromStore(userID, itemID string, quantity int) error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	user, exists := ex.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	return ex.store.Purchase(itemID, user, quantity)
}

// P2P operations
func (ex *Exchange) CreateP2POffer(userID, emotionID string, quantity, price float64, message string) (*models.P2PTrade, error) {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	user, exists := ex.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}

	return ex.p2pMarket.CreateOffer(user, emotionID, quantity, price, message)
}

func (ex *Exchange) AcceptP2POffer(tradeID, buyerID string) error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	trade, err := ex.p2pMarket.GetOffer(tradeID)
	if err != nil {
		return err
	}

	buyer, exists := ex.users[buyerID]
	if !exists {
		return ErrUserNotFound
	}

	seller, exists := ex.users[trade.SellerID]
	if !exists {
		return ErrUserNotFound
	}

	return ex.p2pMarket.AcceptOffer(tradeID, buyer, seller)
}

func (ex *Exchange) CancelP2POffer(tradeID, userID string) error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	user, exists := ex.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	return ex.p2pMarket.CancelOffer(tradeID, user)
}

func (ex *Exchange) GetP2POffers() []models.P2PTrade {
	return ex.p2pMarket.GetAllOffers()
}

func (ex *Exchange) GetP2POffersByEmotion(emotionID string) []models.P2PTrade {
	return ex.p2pMarket.GetOffersByEmotion(emotionID)
}

// Admin: Emotion management
func (ex *Exchange) AddEmotion(emotion models.Emotion) error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	if _, exists := ex.emotions[emotion.ID]; exists {
		return ErrEmotionExists
	}

	ex.emotions[emotion.ID] = emotion
	ex.orderBooks[emotion.ID] = orderbook.NewBook(emotion.ID, emotion.BasePrice)

	return nil
}

func (ex *Exchange) UpdateEmotion(emotion models.Emotion) error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	if _, exists := ex.emotions[emotion.ID]; !exists {
		return ErrEmotionNotFound
	}

	ex.emotions[emotion.ID] = emotion
	return nil
}

func (ex *Exchange) DeleteEmotion(emotionID string) error {
	ex.mu.Lock()
	defer ex.mu.Unlock()

	if _, exists := ex.emotions[emotionID]; !exists {
		return ErrEmotionNotFound
	}

	delete(ex.emotions, emotionID)
	delete(ex.orderBooks, emotionID)

	return nil
}

func (ex *Exchange) GetEmotion(emotionID string) (*models.Emotion, error) {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	emotion, exists := ex.emotions[emotionID]
	if !exists {
		return nil, ErrEmotionNotFound
	}

	return &emotion, nil
}

// Helper functions
func generateUserID(username string) string {
	return "user-" + username + "-" + time.Now().Format("150405")
}

func generateOrderID(counter int64) string {
	return time.Now().Format("20060102150405") + "-" + string(rune('A'+counter%26)) + string(rune('0'+counter%10))
}
