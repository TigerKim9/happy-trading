package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/TigerKim9/happy-trading/internal/auth"
	"github.com/TigerKim9/happy-trading/internal/database"
	"github.com/TigerKim9/happy-trading/internal/exchange"
	"github.com/TigerKim9/happy-trading/internal/handlers"
	"github.com/TigerKim9/happy-trading/internal/websocket"
)

func main() {
	// Initialize database (optional - for production)
	dbHost := os.Getenv("DB_HOST")
	if dbHost != "" {
		dbConfig := database.Config{
			Host:     dbHost,
			Port:     getEnvOrDefault("DB_PORT", "5432"),
			User:     getEnvOrDefault("DB_USER", "postgres"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   getEnvOrDefault("DB_NAME", "emotion_exchange"),
			SSLMode:  getEnvOrDefault("DB_SSLMODE", "disable"),
		}
		if err := database.Connect(dbConfig); err != nil {
			log.Printf("Warning: Database connection failed: %v", err)
			log.Printf("Running in memory-only mode")
		} else {
			log.Printf("Connected to PostgreSQL database")
			if err := database.Migrate(); err != nil {
				log.Printf("Warning: Database migration failed: %v", err)
			} else {
				log.Printf("Database migrations completed")
			}
		}
	} else {
		log.Printf("Running in memory-only mode (set DB_HOST for PostgreSQL)")
	}

	// Initialize JWT auth
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "emotion-exchange-secret-key-change-in-production"
	}
	auth.Init(jwtSecret)
	log.Printf("JWT authentication initialized")

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()
	log.Printf("WebSocket hub started")

	// Initialize exchange
	ex := exchange.NewExchange()

	// Create demo users
	user1 := ex.CreateUser("감정상인", 100000.0)
	user2 := ex.CreateUser("충동구매자", 50000.0)

	log.Printf("Demo users created:")
	log.Printf("  - %s (ID: %s)", user1.Username, user1.ID)
	log.Printf("  - %s (ID: %s)", user2.Username, user2.ID)

	// Initialize handler
	h := handlers.NewHandler(ex)

	// Setup routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", h.HealthCheck)

	// Emotions
	mux.HandleFunc("GET /api/emotions", h.GetEmotions)
	mux.HandleFunc("GET /api/market", h.GetMarketData)

	// Users
	mux.HandleFunc("POST /api/users", h.CreateUser)
	mux.HandleFunc("GET /api/users", h.GetUser)

	// Orders
	mux.HandleFunc("POST /api/orders", h.PlaceOrder)
	mux.HandleFunc("DELETE /api/orders", h.CancelOrder)
	mux.HandleFunc("GET /api/orders", h.GetUserOrders)
	mux.HandleFunc("GET /api/orderbook", h.GetOrderBook)
	mux.HandleFunc("GET /api/trades", h.GetRecentTrades)

	// Store
	mux.HandleFunc("GET /api/store", h.GetStoreItems)
	mux.HandleFunc("POST /api/store/purchase", h.PurchaseFromStore)

	// P2P
	mux.HandleFunc("GET /api/p2p", h.GetP2POffers)
	mux.HandleFunc("POST /api/p2p", h.CreateP2POffer)
	mux.HandleFunc("POST /api/p2p/accept", h.AcceptP2POffer)
	mux.HandleFunc("DELETE /api/p2p", h.CancelP2POffer)

	// Admin API
	mux.HandleFunc("POST /api/admin/emotions", h.AddEmotion)
	mux.HandleFunc("PUT /api/admin/emotions", h.UpdateEmotion)
	mux.HandleFunc("DELETE /api/admin/emotions", h.DeleteEmotion)

	// Admin page
	mux.Handle("GET /admin/", http.StripPrefix("/admin/", http.FileServer(http.Dir("web"))))

	// Trading UI
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "web/index.html")
	})

	// WebSocket
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWs(w, r)
	})

	// Wallet
	mux.HandleFunc("GET /api/wallet/chains", h.GetChains)
	mux.HandleFunc("GET /api/wallet/nonce", h.GetNonce)
	mux.HandleFunc("POST /api/wallet/login", h.WalletLogin)
	mux.HandleFunc("GET /api/wallet/deposit-address", h.GetDepositAddress)
	mux.HandleFunc("POST /api/wallet/deposit", h.RegisterDeposit)
	mux.HandleFunc("GET /api/wallet/deposits", h.GetUserDeposits)
	mux.HandleFunc("POST /api/wallet/withdraw", h.RequestWithdrawal)
	mux.HandleFunc("GET /api/wallet/withdrawals", h.GetUserWithdrawals)

	// CORS middleware
	handler := corsMiddleware(mux)

	// Get port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Print banner
	printBanner()

	log.Printf("Starting Emotion Exchange server on :%s", port)
	log.Printf("Trading UI at http://localhost:%s/", port)
	log.Printf("API available at http://localhost:%s/api", port)
	log.Printf("Admin page at http://localhost:%s/admin/", port)
	log.Printf("WebSocket at ws://localhost:%s/ws", port)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func printBanner() {
	banner := `
╔════════════════════════════════════════════════════════════╗
║                                                            ║
║   ███████╗███╗   ███╗ ██████╗ ████████╗██╗ ██████╗ ███╗   ║
║   ██╔════╝████╗ ████║██╔═══██╗╚══██╔══╝██║██╔═══██╗████╗  ║
║   █████╗  ██╔████╔██║██║   ██║   ██║   ██║██║   ██║██╔██╗ ║
║   ██╔══╝  ██║╚██╔╝██║██║   ██║   ██║   ██║██║   ██║██║╚██╗║
║   ███████╗██║ ╚═╝ ██║╚██████╔╝   ██║   ██║╚██████╔╝██║ ╚██║
║   ╚══════╝╚═╝     ╚═╝ ╚═════╝    ╚═╝   ╚═╝ ╚═════╝ ╚═╝  ╚═║
║                                                            ║
║   ███████╗██╗  ██╗ ██████╗██╗  ██╗ █████╗ ███╗   ██╗ ██████╗║
║   ██╔════╝╚██╗██╔╝██╔════╝██║  ██║██╔══██╗████╗  ██║██╔════╝║
║   █████╗   ╚███╔╝ ██║     ███████║███████║██╔██╗ ██║██║  ███║
║   ██╔══╝   ██╔██╗ ██║     ██╔══██║██╔══██║██║╚██╗██║██║   ██║
║   ███████╗██╔╝ ██╗╚██████╗██║  ██║██║  ██║██║ ╚████║╚██████╔╝
║   ╚══════╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝║
║                                                            ║
║   Trade your emotions: Joy, Impulse, Madness, and more!    ║
║   감정을 거래하세요: 기쁨, 충동, 광기, 그리고 더 많은 것들! ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
}
