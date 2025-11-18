package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/TigerKim9/happy-trading/internal/exchange"
	"github.com/TigerKim9/happy-trading/internal/handlers"
)

func main() {
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
	log.Printf("API available at http://localhost:%s/api", port)

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
