package wallet

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TigerKim9/happy-trading/internal/models"
)

var (
	ErrInvalidAddress   = errors.New("invalid wallet address")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrInvalidNonce     = errors.New("invalid or expired nonce")
)

// WalletService handles wallet authentication and deposits
type WalletService struct {
	users      map[string]*models.User // wallet_address -> user
	deposits   map[string]*models.Deposit
	withdrawals map[string]*models.Withdrawal
	nonces     map[string]time.Time // nonce -> expiry time
	mu         sync.RWMutex
}

// NewWalletService creates a new wallet service
func NewWalletService() *WalletService {
	return &WalletService{
		users:       make(map[string]*models.User),
		deposits:    make(map[string]*models.Deposit),
		withdrawals: make(map[string]*models.Withdrawal),
		nonces:      make(map[string]time.Time),
	}
}

// GenerateNonce creates a new nonce for signature verification
func (ws *WalletService) GenerateNonce(address string) (string, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if !isValidAddress(address) {
		return "", ErrInvalidAddress
	}

	// Generate random nonce
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(bytes)

	// Store nonce with 5 minute expiry
	ws.nonces[nonce] = time.Now().Add(5 * time.Minute)

	return nonce, nil
}

// GetSignMessage returns the message to be signed
func (ws *WalletService) GetSignMessage(nonce string) string {
	return fmt.Sprintf("Welcome to Emotion Exchange!\n\nSign this message to authenticate.\n\nNonce: %s", nonce)
}

// VerifySignature verifies the wallet signature
// In production, this would use go-ethereum's crypto package
func (ws *WalletService) VerifySignature(address, nonce, signature string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Check nonce validity
	expiry, exists := ws.nonces[nonce]
	if !exists || time.Now().After(expiry) {
		return ErrInvalidNonce
	}

	// Remove used nonce
	delete(ws.nonces, nonce)

	// Validate address format
	if !isValidAddress(address) {
		return ErrInvalidAddress
	}

	// TODO: Implement actual signature verification using go-ethereum
	// For MVP, we accept any properly formatted signature
	if len(signature) < 130 { // 0x + 128 hex chars
		return ErrInvalidSignature
	}

	return nil
}

// GetOrCreateUser gets existing user or creates new one by wallet address
func (ws *WalletService) GetOrCreateUser(address string) (*models.User, bool, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	address = strings.ToLower(address)

	if !isValidAddress(address) {
		return nil, false, ErrInvalidAddress
	}

	// Check if user exists
	if user, exists := ws.users[address]; exists {
		return user, false, nil
	}

	// Create new user
	user := &models.User{
		ID:            "wallet-" + address[2:10],
		Username:      address[:10] + "...",
		WalletAddress: address,
		Balance:       0, // Start with 0, need to deposit
		Portfolio:     make(map[string]float64),
		CreatedAt:     time.Now(),
	}

	ws.users[address] = user
	return user, true, nil
}

// GetUserByAddress returns user by wallet address
func (ws *WalletService) GetUserByAddress(address string) (*models.User, error) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	address = strings.ToLower(address)
	user, exists := ws.users[address]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// RegisterDeposit registers a new deposit
func (ws *WalletService) RegisterDeposit(userID string, chainID int64, txHash string, amount float64) (*models.Deposit, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	deposit := &models.Deposit{
		ID:        generateDepositID(),
		UserID:    userID,
		ChainID:   chainID,
		TxHash:    txHash,
		Amount:    amount,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	ws.deposits[deposit.ID] = deposit
	return deposit, nil
}

// ConfirmDeposit confirms a deposit and credits user balance
func (ws *WalletService) ConfirmDeposit(depositID string, user *models.User) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	deposit, exists := ws.deposits[depositID]
	if !exists {
		return errors.New("deposit not found")
	}

	if deposit.Status != "pending" {
		return errors.New("deposit already processed")
	}

	// Credit user balance
	user.Balance += deposit.Amount

	// Update deposit status
	now := time.Now()
	deposit.Status = "confirmed"
	deposit.ConfirmedAt = &now

	return nil
}

// GetUserDeposits returns all deposits for a user
func (ws *WalletService) GetUserDeposits(userID string) []models.Deposit {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	deposits := make([]models.Deposit, 0)
	for _, d := range ws.deposits {
		if d.UserID == userID {
			deposits = append(deposits, *d)
		}
	}
	return deposits
}

// RequestWithdrawal creates a withdrawal request
func (ws *WalletService) RequestWithdrawal(userID string, chainID int64, toAddress string, amount float64, user *models.User) (*models.Withdrawal, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if user.Balance < amount {
		return nil, errors.New("insufficient balance")
	}

	if !isValidAddress(toAddress) {
		return nil, ErrInvalidAddress
	}

	// Deduct from balance
	user.Balance -= amount

	withdrawal := &models.Withdrawal{
		ID:        generateWithdrawalID(),
		UserID:    userID,
		ChainID:   chainID,
		ToAddress: toAddress,
		Amount:    amount,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	ws.withdrawals[withdrawal.ID] = withdrawal
	return withdrawal, nil
}

// GetUserWithdrawals returns all withdrawals for a user
func (ws *WalletService) GetUserWithdrawals(userID string) []models.Withdrawal {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	withdrawals := make([]models.Withdrawal, 0)
	for _, w := range ws.withdrawals {
		if w.UserID == userID {
			withdrawals = append(withdrawals, *w)
		}
	}
	return withdrawals
}

// GetDepositAddress returns the deposit address for a chain
// In production, this would return a unique address per user
func (ws *WalletService) GetDepositAddress(chainID int64) string {
	// Placeholder - in production, generate unique deposit addresses
	return "0x742d35Cc6634C0532925a3b844Bc9e7595f5bE8a"
}

// Helper functions
func isValidAddress(address string) bool {
	if len(address) != 42 {
		return false
	}
	if !strings.HasPrefix(address, "0x") && !strings.HasPrefix(address, "0X") {
		return false
	}
	// Check if remaining chars are valid hex
	_, err := hex.DecodeString(address[2:])
	return err == nil
}

var depositCounter int64
var withdrawalCounter int64

func generateDepositID() string {
	depositCounter++
	return fmt.Sprintf("DEP-%d-%d", time.Now().Unix(), depositCounter)
}

func generateWithdrawalID() string {
	withdrawalCounter++
	return fmt.Sprintf("WTH-%d-%d", time.Now().Unix(), withdrawalCounter)
}
