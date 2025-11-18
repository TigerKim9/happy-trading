package models

import (
	"sync"
	"time"
)

// Emotion represents a tradeable emotion
type Emotion struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	NameKR            string  `json:"name_kr"`
	Description       string  `json:"description"`
	BasePrice         float64 `json:"base_price"`
	Category          string  `json:"category"`           // basic, dark, chaotic
	TotalSupply       float64 `json:"total_supply"`       // 총 발행량 (0 = 무제한)
	CirculatingSupply float64 `json:"circulating_supply"` // 현재 유통량
}

// User represents a trader
type User struct {
	ID            string             `json:"id"`
	Username      string             `json:"username"`
	WalletAddress string             `json:"wallet_address,omitempty"` // EVM wallet address
	Balance       float64            `json:"balance"`
	Portfolio     map[string]float64 `json:"portfolio"`  // emotion_id -> quantity
	Nonce         string             `json:"nonce"`      // for signature verification
	CreatedAt     time.Time          `json:"created_at"`
}

// Chain represents a supported blockchain
type Chain struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	RPC      string `json:"rpc"`
	Explorer string `json:"explorer"`
}

// Deposit represents a deposit transaction
type Deposit struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	ChainID     int64     `json:"chain_id"`
	TxHash      string    `json:"tx_hash"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"` // pending, confirmed, failed
	CreatedAt   time.Time `json:"created_at"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}

// Withdrawal represents a withdrawal request
type Withdrawal struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	ChainID     int64      `json:"chain_id"`
	ToAddress   string     `json:"to_address"`
	Amount      float64    `json:"amount"`
	TxHash      string     `json:"tx_hash,omitempty"`
	Status      string     `json:"status"` // pending, processing, completed, failed
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// SupportedChains returns the list of supported EVM chains
func SupportedChains() []Chain {
	return []Chain{
		{ID: 1, Name: "Ethereum", Symbol: "ETH", RPC: "https://eth.llamarpc.com", Explorer: "https://etherscan.io"},
		{ID: 137, Name: "Polygon", Symbol: "MATIC", RPC: "https://polygon-rpc.com", Explorer: "https://polygonscan.com"},
		{ID: 42161, Name: "Arbitrum", Symbol: "ETH", RPC: "https://arb1.arbitrum.io/rpc", Explorer: "https://arbiscan.io"},
		{ID: 10, Name: "Optimism", Symbol: "ETH", RPC: "https://mainnet.optimism.io", Explorer: "https://optimistic.etherscan.io"},
		{ID: 8453, Name: "Base", Symbol: "ETH", RPC: "https://mainnet.base.org", Explorer: "https://basescan.org"},
		{ID: 56, Name: "BSC", Symbol: "BNB", RPC: "https://bsc-dataseed.binance.org", Explorer: "https://bscscan.com"},
		{ID: 43114, Name: "Avalanche", Symbol: "AVAX", RPC: "https://api.avax.network/ext/bc/C/rpc", Explorer: "https://snowtrace.io"},
	}
}

// OrderSide represents buy or sell
type OrderSide string

const (
	Buy  OrderSide = "buy"
	Sell OrderSide = "sell"
)

// OrderType represents the type of order
type OrderType string

const (
	LimitOrder  OrderType = "limit"
	MarketOrder OrderType = "market"
)

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderPending   OrderStatus = "pending"
	OrderFilled    OrderStatus = "filled"
	OrderPartial   OrderStatus = "partial"
	OrderCancelled OrderStatus = "cancelled"
)

// Order represents a buy/sell order
type Order struct {
	ID         string      `json:"id"`
	UserID     string      `json:"user_id"`
	EmotionID  string      `json:"emotion_id"`
	Side       OrderSide   `json:"side"`
	Type       OrderType   `json:"type"`
	Price      float64     `json:"price"`
	Quantity   float64     `json:"quantity"`
	Filled     float64     `json:"filled"`
	Status     OrderStatus `json:"status"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// Trade represents a completed trade
type Trade struct {
	ID         string    `json:"id"`
	BuyOrderID string    `json:"buy_order_id"`
	SellOrderID string   `json:"sell_order_id"`
	BuyerID    string    `json:"buyer_id"`
	SellerID   string    `json:"seller_id"`
	EmotionID  string    `json:"emotion_id"`
	Price      float64   `json:"price"`
	Quantity   float64   `json:"quantity"`
	CreatedAt  time.Time `json:"created_at"`
}

// StoreItem represents an item in the store
type StoreItem struct {
	ID          string  `json:"id"`
	EmotionID   string  `json:"emotion_id"`
	Quantity    float64 `json:"quantity"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Stock       int     `json:"stock"` // -1 for unlimited
}

// P2PTrade represents a peer-to-peer trade offer
type P2PTrade struct {
	ID           string      `json:"id"`
	SellerID     string      `json:"seller_id"`
	BuyerID      string      `json:"buyer_id,omitempty"`
	EmotionID    string      `json:"emotion_id"`
	Quantity     float64     `json:"quantity"`
	Price        float64     `json:"price"`
	Status       OrderStatus `json:"status"`
	Message      string      `json:"message,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	CompletedAt  *time.Time  `json:"completed_at,omitempty"`
}

// MarketData represents current market data for an emotion
type MarketData struct {
	EmotionID   string    `json:"emotion_id"`
	LastPrice   float64   `json:"last_price"`
	BidPrice    float64   `json:"bid_price"`
	AskPrice    float64   `json:"ask_price"`
	High24h     float64   `json:"high_24h"`
	Low24h      float64   `json:"low_24h"`
	Volume24h   float64   `json:"volume_24h"`
	Change24h   float64   `json:"change_24h"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrderBookEntry represents a single entry in the order book
type OrderBookEntry struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Count    int     `json:"count"` // number of orders at this price
}

// OrderBook represents the order book for an emotion
type OrderBook struct {
	EmotionID string           `json:"emotion_id"`
	Bids      []OrderBookEntry `json:"bids"` // buy orders, sorted desc by price
	Asks      []OrderBookEntry `json:"asks"` // sell orders, sorted asc by price
	mu        sync.RWMutex
}

// DefaultEmotions returns the list of tradeable emotions
func DefaultEmotions() []Emotion {
	return []Emotion{
		// Basic emotions - 발행량 많음
		{ID: "joy", Name: "Joy", NameKR: "기쁨", Description: "Pure, simple happiness", BasePrice: 100.0, Category: "basic", TotalSupply: 10000},
		{ID: "happiness", Name: "Happiness", NameKR: "행복", Description: "Deep contentment and satisfaction", BasePrice: 150.0, Category: "basic", TotalSupply: 8000},
		{ID: "sadness", Name: "Sadness", NameKR: "슬픔", Description: "Melancholy and sorrow", BasePrice: 80.0, Category: "basic", TotalSupply: 12000},
		{ID: "anger", Name: "Anger", NameKR: "분노", Description: "Intense displeasure and hostility", BasePrice: 90.0, Category: "basic", TotalSupply: 10000},
		{ID: "fear", Name: "Fear", NameKR: "공포", Description: "Anxiety and dread", BasePrice: 85.0, Category: "basic", TotalSupply: 9000},
		{ID: "surprise", Name: "Surprise", NameKR: "놀람", Description: "Unexpected astonishment", BasePrice: 70.0, Category: "basic", TotalSupply: 15000},

		// Dark emotions - 발행량 중간
		{ID: "jealousy", Name: "Jealousy", NameKR: "질투", Description: "Envious resentment", BasePrice: 120.0, Category: "dark", TotalSupply: 5000},
		{ID: "despair", Name: "Despair", NameKR: "절망", Description: "Complete loss of hope", BasePrice: 200.0, Category: "dark", TotalSupply: 3000},
		{ID: "emptiness", Name: "Emptiness", NameKR: "허무", Description: "Void of meaning", BasePrice: 180.0, Category: "dark", TotalSupply: 4000},
		{ID: "guilt", Name: "Guilt", NameKR: "죄책감", Description: "Remorse and self-blame", BasePrice: 110.0, Category: "dark", TotalSupply: 6000},

		// Chaotic emotions (발칙한 것들) - 발행량 적음 (희귀)
		{ID: "impulse", Name: "Impulse", NameKR: "충동", Description: "Sudden uncontrollable urge", BasePrice: 250.0, Category: "chaotic", TotalSupply: 2000},
		{ID: "desire", Name: "Desire", NameKR: "욕망", Description: "Intense wanting and craving", BasePrice: 300.0, Category: "chaotic", TotalSupply: 1500},
		{ID: "madness", Name: "Madness", NameKR: "광기", Description: "Beautiful chaos of the mind", BasePrice: 500.0, Category: "chaotic", TotalSupply: 500},
		{ID: "greed", Name: "Greed", NameKR: "탐욕", Description: "Insatiable appetite for more", BasePrice: 350.0, Category: "chaotic", TotalSupply: 1000},
		{ID: "lust", Name: "Lust", NameKR: "정욕", Description: "Overwhelming passionate desire", BasePrice: 400.0, Category: "chaotic", TotalSupply: 800},
		{ID: "rage", Name: "Rage", NameKR: "격노", Description: "Uncontrollable fury", BasePrice: 280.0, Category: "chaotic", TotalSupply: 1800},
		{ID: "obsession", Name: "Obsession", NameKR: "집착", Description: "Fixation beyond reason", BasePrice: 320.0, Category: "chaotic", TotalSupply: 1200},
		{ID: "euphoria", Name: "Euphoria", NameKR: "황홀", Description: "Transcendent ecstasy", BasePrice: 450.0, Category: "chaotic", TotalSupply: 600},
		{ID: "apathy", Name: "Apathy", NameKR: "무관심", Description: "Complete emotional detachment", BasePrice: 160.0, Category: "chaotic", TotalSupply: 2500},
		{ID: "schadenfreude", Name: "Schadenfreude", NameKR: "남의불행", Description: "Pleasure from others' misfortune", BasePrice: 380.0, Category: "chaotic", TotalSupply: 700},
	}
}
