package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/TigerKim9/happy-trading/internal/exchange"
	"github.com/TigerKim9/happy-trading/internal/models"
)

// Handler holds all HTTP handlers
type Handler struct {
	exchange *exchange.Exchange
}

// NewHandler creates a new handler
func NewHandler(ex *exchange.Exchange) *Handler {
	return &Handler{exchange: ex}
}

// Response helpers
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Health check
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "emotion-exchange",
	})
}

// Emotion handlers
func (h *Handler) GetEmotions(w http.ResponseWriter, r *http.Request) {
	emotions := h.exchange.GetAllEmotions()
	writeJSON(w, http.StatusOK, emotions)
}

func (h *Handler) GetMarketData(w http.ResponseWriter, r *http.Request) {
	emotionID := r.URL.Query().Get("emotion_id")

	if emotionID != "" {
		data, err := h.exchange.GetMarketData(emotionID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, data)
		return
	}

	data := h.exchange.GetAllMarketData()
	writeJSON(w, http.StatusOK, data)
}

// User handlers
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username       string  `json:"username"`
		InitialBalance float64 `json:"initial_balance"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}

	if req.InitialBalance <= 0 {
		req.InitialBalance = 10000.0 // Default starting balance
	}

	user := h.exchange.CreateUser(req.Username, req.InitialBalance)
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	user, err := h.exchange.GetUser(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// Order handlers
func (h *Handler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID    string  `json:"user_id"`
		EmotionID string  `json:"emotion_id"`
		Side      string  `json:"side"`
		Type      string  `json:"type"`
		Price     float64 `json:"price"`
		Quantity  float64 `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate
	if req.UserID == "" || req.EmotionID == "" || req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	var side models.OrderSide
	switch req.Side {
	case "buy":
		side = models.Buy
	case "sell":
		side = models.Sell
	default:
		writeError(w, http.StatusBadRequest, "invalid side")
		return
	}

	var orderType models.OrderType
	switch req.Type {
	case "limit":
		orderType = models.LimitOrder
		if req.Price <= 0 {
			writeError(w, http.StatusBadRequest, "price required for limit order")
			return
		}
	case "market":
		orderType = models.MarketOrder
	default:
		writeError(w, http.StatusBadRequest, "invalid order type")
		return
	}

	order, trades, err := h.exchange.PlaceOrder(req.UserID, req.EmotionID, side, orderType, req.Price, req.Quantity)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"order":  order,
		"trades": trades,
	})
}

func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID string `json:"order_id"`
		UserID  string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.exchange.CancelOrder(req.OrderID, req.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	emotionID := r.URL.Query().Get("emotion_id")
	if emotionID == "" {
		writeError(w, http.StatusBadRequest, "emotion_id is required")
		return
	}

	depth := 20
	if d := r.URL.Query().Get("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			depth = parsed
		}
	}

	ob, err := h.exchange.GetOrderBook(emotionID, depth)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ob)
}

func (h *Handler) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	orders := h.exchange.GetUserOrders(userID)
	writeJSON(w, http.StatusOK, orders)
}

func (h *Handler) GetRecentTrades(w http.ResponseWriter, r *http.Request) {
	emotionID := r.URL.Query().Get("emotion_id")
	if emotionID == "" {
		writeError(w, http.StatusBadRequest, "emotion_id is required")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	trades, err := h.exchange.GetRecentTrades(emotionID, limit)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, trades)
}

// Store handlers
func (h *Handler) GetStoreItems(w http.ResponseWriter, r *http.Request) {
	items := h.exchange.GetStoreItems()
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) PurchaseFromStore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		ItemID   string `json:"item_id"`
		Quantity int    `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	if err := h.exchange.PurchaseFromStore(req.UserID, req.ItemID, req.Quantity); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "purchased"})
}

// P2P handlers
func (h *Handler) CreateP2POffer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID    string  `json:"user_id"`
		EmotionID string  `json:"emotion_id"`
		Quantity  float64 `json:"quantity"`
		Price     float64 `json:"price"`
		Message   string  `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	trade, err := h.exchange.CreateP2POffer(req.UserID, req.EmotionID, req.Quantity, req.Price, req.Message)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, trade)
}

func (h *Handler) AcceptP2POffer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TradeID string `json:"trade_id"`
		BuyerID string `json:"buyer_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.exchange.AcceptP2POffer(req.TradeID, req.BuyerID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (h *Handler) CancelP2POffer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TradeID string `json:"trade_id"`
		UserID  string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.exchange.CancelP2POffer(req.TradeID, req.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *Handler) GetP2POffers(w http.ResponseWriter, r *http.Request) {
	emotionID := r.URL.Query().Get("emotion_id")

	var offers []models.P2PTrade
	if emotionID != "" {
		offers = h.exchange.GetP2POffersByEmotion(emotionID)
	} else {
		offers = h.exchange.GetP2POffers()
	}

	writeJSON(w, http.StatusOK, offers)
}

// Admin handlers
func (h *Handler) AddEmotion(w http.ResponseWriter, r *http.Request) {
	var emotion models.Emotion

	if err := json.NewDecoder(r.Body).Decode(&emotion); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if emotion.ID == "" || emotion.Name == "" || emotion.NameKR == "" {
		writeError(w, http.StatusBadRequest, "id, name, and name_kr are required")
		return
	}

	if emotion.BasePrice <= 0 {
		writeError(w, http.StatusBadRequest, "base_price must be positive")
		return
	}

	if err := h.exchange.AddEmotion(emotion); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, emotion)
}

func (h *Handler) UpdateEmotion(w http.ResponseWriter, r *http.Request) {
	var emotion models.Emotion

	if err := json.NewDecoder(r.Body).Decode(&emotion); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if emotion.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.exchange.UpdateEmotion(emotion); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, emotion)
}

func (h *Handler) DeleteEmotion(w http.ResponseWriter, r *http.Request) {
	emotionID := r.URL.Query().Get("id")
	if emotionID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.exchange.DeleteEmotion(emotionID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Wallet handlers
func (h *Handler) GetChains(w http.ResponseWriter, r *http.Request) {
	chains := h.exchange.GetSupportedChains()
	writeJSON(w, http.StatusOK, chains)
}

func (h *Handler) GetNonce(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		writeError(w, http.StatusBadRequest, "address is required")
		return
	}

	nonce, err := h.exchange.GetNonce(address)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	message := h.exchange.GetSignMessage(nonce)
	writeJSON(w, http.StatusOK, map[string]string{
		"nonce":   nonce,
		"message": message,
	})
}

func (h *Handler) WalletLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address   string `json:"address"`
		Nonce     string `json:"nonce"`
		Signature string `json:"signature"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, isNew, err := h.exchange.WalletLogin(req.Address, req.Nonce, req.Signature)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":   user,
		"is_new": isNew,
	})
}

func (h *Handler) GetDepositAddress(w http.ResponseWriter, r *http.Request) {
	chainIDStr := r.URL.Query().Get("chain_id")
	if chainIDStr == "" {
		writeError(w, http.StatusBadRequest, "chain_id is required")
		return
	}

	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chain_id")
		return
	}

	address := h.exchange.GetDepositAddress(chainID)
	writeJSON(w, http.StatusOK, map[string]string{
		"address":  address,
		"chain_id": chainIDStr,
	})
}

func (h *Handler) RegisterDeposit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  string  `json:"user_id"`
		ChainID int64   `json:"chain_id"`
		TxHash  string  `json:"tx_hash"`
		Amount  float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deposit, err := h.exchange.RegisterDeposit(req.UserID, req.ChainID, req.TxHash, req.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, deposit)
}

func (h *Handler) GetUserDeposits(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	deposits := h.exchange.GetUserDeposits(userID)
	writeJSON(w, http.StatusOK, deposits)
}

func (h *Handler) RequestWithdrawal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID    string  `json:"user_id"`
		ChainID   int64   `json:"chain_id"`
		ToAddress string  `json:"to_address"`
		Amount    float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	withdrawal, err := h.exchange.RequestWithdrawal(req.UserID, req.ChainID, req.ToAddress, req.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, withdrawal)
}

func (h *Handler) GetUserWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	withdrawals := h.exchange.GetUserWithdrawals(userID)
	writeJSON(w, http.StatusOK, withdrawals)
}
