package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// Config holds database configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// Connect establishes database connection
func Connect(cfg Config) error {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)
	DB.SetConnMaxLifetime(5 * time.Minute)

	if err := DB.Ping(); err != nil {
		return err
	}

	log.Println("Database connected successfully")
	return nil
}

// GetConfigFromEnv loads config from environment
func GetConfigFromEnv() Config {
	return Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "emotion_exchange"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// Migrate runs database migrations
func Migrate() error {
	migrations := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(100) PRIMARY KEY,
			username VARCHAR(100) NOT NULL,
			wallet_address VARCHAR(42) UNIQUE,
			balance DECIMAL(20, 8) DEFAULT 0,
			nonce VARCHAR(64),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Emotions table
		`CREATE TABLE IF NOT EXISTS emotions (
			id VARCHAR(50) PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			name_kr VARCHAR(100) NOT NULL,
			description TEXT,
			base_price DECIMAL(20, 8) NOT NULL,
			category VARCHAR(20) NOT NULL,
			total_supply DECIMAL(20, 8) DEFAULT 0,
			circulating_supply DECIMAL(20, 8) DEFAULT 0
		)`,

		// Orders table
		`CREATE TABLE IF NOT EXISTS orders (
			id VARCHAR(100) PRIMARY KEY,
			user_id VARCHAR(100) REFERENCES users(id),
			emotion_id VARCHAR(50) REFERENCES emotions(id),
			side VARCHAR(10) NOT NULL,
			type VARCHAR(20) NOT NULL,
			price DECIMAL(20, 8) NOT NULL,
			quantity DECIMAL(20, 8) NOT NULL,
			filled DECIMAL(20, 8) DEFAULT 0,
			status VARCHAR(20) DEFAULT 'pending',
			stop_price DECIMAL(20, 8),
			take_profit_price DECIMAL(20, 8),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Trades table
		`CREATE TABLE IF NOT EXISTS trades (
			id VARCHAR(100) PRIMARY KEY,
			buy_order_id VARCHAR(100),
			sell_order_id VARCHAR(100),
			buyer_id VARCHAR(100),
			seller_id VARCHAR(100),
			emotion_id VARCHAR(50),
			price DECIMAL(20, 8) NOT NULL,
			quantity DECIMAL(20, 8) NOT NULL,
			buyer_fee DECIMAL(20, 8) DEFAULT 0,
			seller_fee DECIMAL(20, 8) DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Portfolio table
		`CREATE TABLE IF NOT EXISTS portfolios (
			user_id VARCHAR(100) REFERENCES users(id),
			emotion_id VARCHAR(50) REFERENCES emotions(id),
			quantity DECIMAL(20, 8) DEFAULT 0,
			PRIMARY KEY (user_id, emotion_id)
		)`,

		// Deposits table
		`CREATE TABLE IF NOT EXISTS deposits (
			id VARCHAR(100) PRIMARY KEY,
			user_id VARCHAR(100) REFERENCES users(id),
			chain_id BIGINT NOT NULL,
			tx_hash VARCHAR(100),
			amount DECIMAL(20, 8) NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			confirmed_at TIMESTAMP
		)`,

		// Withdrawals table
		`CREATE TABLE IF NOT EXISTS withdrawals (
			id VARCHAR(100) PRIMARY KEY,
			user_id VARCHAR(100) REFERENCES users(id),
			chain_id BIGINT NOT NULL,
			to_address VARCHAR(42) NOT NULL,
			amount DECIMAL(20, 8) NOT NULL,
			tx_hash VARCHAR(100),
			status VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP
		)`,

		// OHLCV candles table
		`CREATE TABLE IF NOT EXISTS candles (
			emotion_id VARCHAR(50) REFERENCES emotions(id),
			interval VARCHAR(10) NOT NULL,
			open_time TIMESTAMP NOT NULL,
			open DECIMAL(20, 8) NOT NULL,
			high DECIMAL(20, 8) NOT NULL,
			low DECIMAL(20, 8) NOT NULL,
			close DECIMAL(20, 8) NOT NULL,
			volume DECIMAL(20, 8) NOT NULL,
			PRIMARY KEY (emotion_id, interval, open_time)
		)`,

		// Rankings table
		`CREATE TABLE IF NOT EXISTS rankings (
			user_id VARCHAR(100) PRIMARY KEY REFERENCES users(id),
			total_pnl DECIMAL(20, 8) DEFAULT 0,
			total_volume DECIMAL(20, 8) DEFAULT 0,
			trade_count INT DEFAULT 0,
			win_count INT DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_emotion ON orders(emotion_id)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_emotion ON trades(emotion_id)`,
		`CREATE INDEX IF NOT EXISTS idx_candles_time ON candles(open_time)`,
	}

	for _, migration := range migrations {
		if _, err := DB.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	log.Println("Database migrations completed")
	return nil
}

// Close closes database connection
func Close() {
	if DB != nil {
		DB.Close()
	}
}
