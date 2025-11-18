package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// MessageType represents websocket message types
type MessageType string

const (
	TypeOrderBook  MessageType = "orderbook"
	TypeTrade      MessageType = "trade"
	TypeTicker     MessageType = "ticker"
	TypeCandle     MessageType = "candle"
	TypeSubscribe  MessageType = "subscribe"
	TypeUnsubscribe MessageType = "unsubscribe"
)

// Message represents a websocket message
type Message struct {
	Type      MessageType     `json:"type"`
	Channel   string          `json:"channel,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// Client represents a websocket client
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan []byte
	subscriptions map[string]bool
	mu            sync.RWMutex
}

// Hub manages websocket connections
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	channels   map[string]map[*Client]bool // channel -> clients
	mu         sync.RWMutex
}

// NewHub creates a new websocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		channels:   make(map[string]map[*Client]bool),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				// Remove from all channels
				for channel := range client.subscriptions {
					if clients, ok := h.channels[channel]; ok {
						delete(clients, client)
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToChannel sends message to specific channel subscribers
func (h *Hub) BroadcastToChannel(channel string, data interface{}) {
	msg := Message{
		Type:      TypeTicker,
		Channel:   channel,
		Timestamp: time.Now().UnixMilli(),
	}

	if jsonData, err := json.Marshal(data); err == nil {
		msg.Data = jsonData
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	if clients, ok := h.channels[channel]; ok {
		for client := range clients {
			select {
			case client.send <- msgBytes:
			default:
			}
		}
	}
	h.mu.RUnlock()
}

// BroadcastTrade broadcasts trade to subscribers
func (h *Hub) BroadcastTrade(emotionID string, trade interface{}) {
	channel := "trade:" + emotionID
	h.BroadcastToChannel(channel, trade)
}

// BroadcastOrderBook broadcasts orderbook update
func (h *Hub) BroadcastOrderBook(emotionID string, orderbook interface{}) {
	channel := "orderbook:" + emotionID
	h.BroadcastToChannel(channel, orderbook)
}

// BroadcastTicker broadcasts ticker update
func (h *Hub) BroadcastTicker(emotionID string, ticker interface{}) {
	channel := "ticker:" + emotionID
	h.BroadcastToChannel(channel, ticker)
}

// ServeWs handles websocket connections
func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	client := &Client{
		hub:           h,
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case TypeSubscribe:
			c.subscribe(msg.Channel)
		case TypeUnsubscribe:
			c.unsubscribe(msg.Channel)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			w.Close()

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) subscribe(channel string) {
	c.mu.Lock()
	c.subscriptions[channel] = true
	c.mu.Unlock()

	c.hub.mu.Lock()
	if c.hub.channels[channel] == nil {
		c.hub.channels[channel] = make(map[*Client]bool)
	}
	c.hub.channels[channel][c] = true
	c.hub.mu.Unlock()
}

func (c *Client) unsubscribe(channel string) {
	c.mu.Lock()
	delete(c.subscriptions, channel)
	c.mu.Unlock()

	c.hub.mu.Lock()
	if clients, ok := c.hub.channels[channel]; ok {
		delete(clients, c)
	}
	c.hub.mu.Unlock()
}
