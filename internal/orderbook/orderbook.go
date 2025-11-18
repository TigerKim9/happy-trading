package orderbook

import (
	"container/heap"
	"sync"
	"time"

	"github.com/TigerKim9/happy-trading/internal/models"
)

// OrderHeap implements heap.Interface for orders
type OrderHeap struct {
	orders []*models.Order
	isAsk  bool // true for sell orders (ascending), false for buy orders (descending)
}

func (h OrderHeap) Len() int { return len(h.orders) }

func (h OrderHeap) Less(i, j int) bool {
	if h.isAsk {
		// Sell orders: lowest price first
		if h.orders[i].Price == h.orders[j].Price {
			return h.orders[i].CreatedAt.Before(h.orders[j].CreatedAt)
		}
		return h.orders[i].Price < h.orders[j].Price
	}
	// Buy orders: highest price first
	if h.orders[i].Price == h.orders[j].Price {
		return h.orders[i].CreatedAt.Before(h.orders[j].CreatedAt)
	}
	return h.orders[i].Price > h.orders[j].Price
}

func (h OrderHeap) Swap(i, j int) { h.orders[i], h.orders[j] = h.orders[j], h.orders[i] }

func (h *OrderHeap) Push(x interface{}) {
	h.orders = append(h.orders, x.(*models.Order))
}

func (h *OrderHeap) Pop() interface{} {
	old := h.orders
	n := len(old)
	x := old[n-1]
	h.orders = old[0 : n-1]
	return x
}

func (h *OrderHeap) Peek() *models.Order {
	if len(h.orders) == 0 {
		return nil
	}
	return h.orders[0]
}

// Book represents an order book for a single emotion
type Book struct {
	EmotionID  string
	Bids       *OrderHeap // buy orders
	Asks       *OrderHeap // sell orders
	LastPrice  float64
	High24h    float64
	Low24h     float64
	Volume24h  float64
	trades     []models.Trade
	mu         sync.RWMutex
}

// NewBook creates a new order book
func NewBook(emotionID string, basePrice float64) *Book {
	bids := &OrderHeap{orders: make([]*models.Order, 0), isAsk: false}
	asks := &OrderHeap{orders: make([]*models.Order, 0), isAsk: true}
	heap.Init(bids)
	heap.Init(asks)

	return &Book{
		EmotionID: emotionID,
		Bids:      bids,
		Asks:      asks,
		LastPrice: basePrice,
		High24h:   basePrice,
		Low24h:    basePrice,
		trades:    make([]models.Trade, 0),
	}
}

// AddOrder adds an order to the book and attempts to match it
func (b *Book) AddOrder(order *models.Order) []models.Trade {
	b.mu.Lock()
	defer b.mu.Unlock()

	var trades []models.Trade

	if order.Type == models.MarketOrder {
		trades = b.matchMarketOrder(order)
	} else {
		trades = b.matchLimitOrder(order)
	}

	// If order is not fully filled, add to book
	if order.Quantity > order.Filled && order.Type == models.LimitOrder {
		if order.Side == models.Buy {
			heap.Push(b.Bids, order)
		} else {
			heap.Push(b.Asks, order)
		}
		order.Status = models.OrderPartial
		if order.Filled == 0 {
			order.Status = models.OrderPending
		}
	} else {
		order.Status = models.OrderFilled
	}

	return trades
}

// matchMarketOrder matches a market order against the book
func (b *Book) matchMarketOrder(order *models.Order) []models.Trade {
	var trades []models.Trade
	remaining := order.Quantity - order.Filled

	var oppositeBook *OrderHeap
	if order.Side == models.Buy {
		oppositeBook = b.Asks
	} else {
		oppositeBook = b.Bids
	}

	for remaining > 0 && oppositeBook.Len() > 0 {
		best := oppositeBook.Peek()
		if best == nil {
			break
		}

		matchQty := min(remaining, best.Quantity-best.Filled)
		trade := b.executeTrade(order, best, best.Price, matchQty)
		trades = append(trades, trade)

		remaining -= matchQty

		if best.Filled >= best.Quantity {
			heap.Pop(oppositeBook)
			best.Status = models.OrderFilled
		}
	}

	return trades
}

// matchLimitOrder matches a limit order against the book
func (b *Book) matchLimitOrder(order *models.Order) []models.Trade {
	var trades []models.Trade
	remaining := order.Quantity - order.Filled

	var oppositeBook *OrderHeap
	if order.Side == models.Buy {
		oppositeBook = b.Asks
	} else {
		oppositeBook = b.Bids
	}

	for remaining > 0 && oppositeBook.Len() > 0 {
		best := oppositeBook.Peek()
		if best == nil {
			break
		}

		// Check if prices cross
		if order.Side == models.Buy && order.Price < best.Price {
			break
		}
		if order.Side == models.Sell && order.Price > best.Price {
			break
		}

		matchQty := min(remaining, best.Quantity-best.Filled)
		trade := b.executeTrade(order, best, best.Price, matchQty)
		trades = append(trades, trade)

		remaining -= matchQty

		if best.Filled >= best.Quantity {
			heap.Pop(oppositeBook)
			best.Status = models.OrderFilled
		}
	}

	return trades
}

// executeTrade creates and records a trade
func (b *Book) executeTrade(taker, maker *models.Order, price, quantity float64) models.Trade {
	var buyOrder, sellOrder *models.Order
	if taker.Side == models.Buy {
		buyOrder = taker
		sellOrder = maker
	} else {
		buyOrder = maker
		sellOrder = taker
	}

	trade := models.Trade{
		ID:          generateID(),
		BuyOrderID:  buyOrder.ID,
		SellOrderID: sellOrder.ID,
		BuyerID:     buyOrder.UserID,
		SellerID:    sellOrder.UserID,
		EmotionID:   b.EmotionID,
		Price:       price,
		Quantity:    quantity,
		CreatedAt:   time.Now(),
	}

	taker.Filled += quantity
	maker.Filled += quantity
	taker.UpdatedAt = time.Now()
	maker.UpdatedAt = time.Now()

	// Update market data
	b.LastPrice = price
	b.Volume24h += quantity
	if price > b.High24h {
		b.High24h = price
	}
	if price < b.Low24h {
		b.Low24h = price
	}

	b.trades = append(b.trades, trade)

	return trade
}

// CancelOrder cancels an order from the book
func (b *Book) CancelOrder(orderID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Search in bids
	for i, order := range b.Bids.orders {
		if order.ID == orderID {
			order.Status = models.OrderCancelled
			b.Bids.orders = append(b.Bids.orders[:i], b.Bids.orders[i+1:]...)
			heap.Init(b.Bids)
			return true
		}
	}

	// Search in asks
	for i, order := range b.Asks.orders {
		if order.ID == orderID {
			order.Status = models.OrderCancelled
			b.Asks.orders = append(b.Asks.orders[:i], b.Asks.orders[i+1:]...)
			heap.Init(b.Asks)
			return true
		}
	}

	return false
}

// GetOrderBook returns the current order book state
func (b *Book) GetOrderBook(depth int) models.OrderBook {
	b.mu.RLock()
	defer b.mu.RUnlock()

	ob := models.OrderBook{
		EmotionID: b.EmotionID,
		Bids:      make([]models.OrderBookEntry, 0),
		Asks:      make([]models.OrderBookEntry, 0),
	}

	// Aggregate bids by price
	bidPrices := make(map[float64]*models.OrderBookEntry)
	for _, order := range b.Bids.orders {
		remaining := order.Quantity - order.Filled
		if entry, exists := bidPrices[order.Price]; exists {
			entry.Quantity += remaining
			entry.Count++
		} else {
			bidPrices[order.Price] = &models.OrderBookEntry{
				Price:    order.Price,
				Quantity: remaining,
				Count:    1,
			}
		}
	}

	// Aggregate asks by price
	askPrices := make(map[float64]*models.OrderBookEntry)
	for _, order := range b.Asks.orders {
		remaining := order.Quantity - order.Filled
		if entry, exists := askPrices[order.Price]; exists {
			entry.Quantity += remaining
			entry.Count++
		} else {
			askPrices[order.Price] = &models.OrderBookEntry{
				Price:    order.Price,
				Quantity: remaining,
				Count:    1,
			}
		}
	}

	// Convert to slices and sort
	for _, entry := range bidPrices {
		ob.Bids = append(ob.Bids, *entry)
	}
	for _, entry := range askPrices {
		ob.Asks = append(ob.Asks, *entry)
	}

	// Sort bids descending, asks ascending
	sortBids(ob.Bids)
	sortAsks(ob.Asks)

	// Apply depth limit
	if depth > 0 {
		if len(ob.Bids) > depth {
			ob.Bids = ob.Bids[:depth]
		}
		if len(ob.Asks) > depth {
			ob.Asks = ob.Asks[:depth]
		}
	}

	return ob
}

// GetMarketData returns current market data
func (b *Book) GetMarketData() models.MarketData {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var bidPrice, askPrice float64
	if b.Bids.Len() > 0 {
		bidPrice = b.Bids.Peek().Price
	}
	if b.Asks.Len() > 0 {
		askPrice = b.Asks.Peek().Price
	}

	return models.MarketData{
		EmotionID: b.EmotionID,
		LastPrice: b.LastPrice,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		High24h:   b.High24h,
		Low24h:    b.Low24h,
		Volume24h: b.Volume24h,
		UpdatedAt: time.Now(),
	}
}

// GetRecentTrades returns recent trades
func (b *Book) GetRecentTrades(limit int) []models.Trade {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > len(b.trades) {
		limit = len(b.trades)
	}

	start := len(b.trades) - limit
	if start < 0 {
		start = 0
	}

	result := make([]models.Trade, limit)
	copy(result, b.trades[start:])

	// Reverse to get newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// Helper functions
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func sortBids(entries []models.OrderBookEntry) {
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Price < entries[j].Price {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

func sortAsks(entries []models.OrderBookEntry) {
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Price > entries[j].Price {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

var idCounter int64
var idMu sync.Mutex

func generateID() string {
	idMu.Lock()
	defer idMu.Unlock()
	idCounter++
	return time.Now().Format("20060102150405") + "-" + string(rune(idCounter))
}
