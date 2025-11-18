package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
	jwtSecret       []byte
)

// Claims represents JWT claims
type Claims struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
	jwt.RegisteredClaims
}

// Init sets up JWT secret with provided key
func Init(secret string) {
	jwtSecret = []byte(secret)
}

// Initialize sets up JWT secret from environment
func Initialize() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "emotion-exchange-secret-key-change-in-production"
	}
	jwtSecret = []byte(secret)
}

// GenerateToken creates a new JWT token
func GenerateToken(userID, walletAddress string) (string, error) {
	claims := Claims{
		UserID:        userID,
		WalletAddress: walletAddress,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "emotion-exchange",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken validates and parses JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// Middleware creates JWT auth middleware
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims, err := ValidateToken(parts[1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Add claims to request context
		r.Header.Set("X-User-ID", claims.UserID)
		r.Header.Set("X-Wallet-Address", claims.WalletAddress)

		next.ServeHTTP(w, r)
	})
}

// OptionalMiddleware allows unauthenticated requests but parses token if present
func OptionalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				if claims, err := ValidateToken(parts[1]); err == nil {
					r.Header.Set("X-User-ID", claims.UserID)
					r.Header.Set("X-Wallet-Address", claims.WalletAddress)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
