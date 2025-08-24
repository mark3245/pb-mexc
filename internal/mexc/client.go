package mexc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	conn     *websocket.Conn
	url      string
	mu       sync.RWMutex
	handlers map[string][]EventHandler
	ctx      context.Context
	cancel   context.CancelFunc
}

type EventHandler func(data interface{})

type TradeData struct {
	Symbol    string `json:"s"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	Timestamp int64  `json:"T"`
	IsBuyer   bool   `json:"m"`
}

type TickerData struct {
	Symbol    string `json:"s"`
	Price     string `json:"c"`
	Timestamp int64  `json:"E"`
}

type WebSocketMessage struct {
	Method string          `json:"method"`
	Params []string        `json:"params"`
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Stream string          `json:"stream,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

func NewClient(url string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		url:      url,
		handlers: make(map[string][]EventHandler),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	log.Infof("Connecting to MEXC WebSocket: %s", c.url)

	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	c.conn = conn
	log.Info("Successfully connected to MEXC WebSocket")

	go c.readMessages()

	return nil
}

func (c *Client) Disconnect() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) SubscribeToTrades(symbols []string) error {
	log.Infof("Trades stream will be available automatically")
	return nil
}

func (c *Client) SubscribeToTickers(symbols []string) error {
	log.Infof("Tickers stream will be available automatically")
	return nil
}

func (c *Client) OnTrade(handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers["trade"] = append(c.handlers["trade"], handler)
}

func (c *Client) OnTicker(handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers["ticker"] = append(c.handlers["ticker"], handler)
}

func (c *Client) sendMessage(msg WebSocketMessage) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) readMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				return
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Errorf("Error reading message: %v", err)

				log.Info("Attempting to reconnect...")
				if err := c.reconnect(); err != nil {
					log.Errorf("Failed to reconnect: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}
				continue
			}

			c.handleMessage(message)
		}
	}
}

func (c *Client) reconnect() error {
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	time.Sleep(2 * time.Second)

	return c.Connect()
}

func (c *Client) handleMessage(data []byte) {
	log.Debugf("Raw message received: %s", string(data))

	var streamData map[string]interface{}
	if err := json.Unmarshal(data, &streamData); err == nil {
		if stream, ok := streamData["stream"].(string); ok {
			log.Infof("Stream data received: %s", stream)
			if dataBytes, ok := streamData["data"].(json.RawMessage); ok {
				c.handleStreamData(stream, dataBytes)
			} else {
				if dataBytes, err := json.Marshal(streamData["data"]); err == nil {
					c.handleStreamData(stream, dataBytes)
				}
			}
			return
		}
	}

	var msg WebSocketMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Errorf("Error unmarshaling message: %v", err)
		return
	}

	if msg.ID > 0 {
		log.Infof("Subscription response: %+v", msg)
		return
	}

	log.Debugf("Other message: %+v", msg)
}

func (c *Client) handleStreamData(stream string, data json.RawMessage) {
	if len(data) == 0 {
		log.Debugf("Empty stream data for: %s", stream)
		return
	}

	log.Infof("Processing stream data: %s, data length: %d", stream, len(data))

	if stream == "spot@public.deals.v3.api" {
		var trades []TradeData
		if err := json.Unmarshal(data, &trades); err != nil {
			log.Errorf("Error unmarshaling trade data: %v", err)
			return
		}

		c.mu.RLock()
		handlers := c.handlers["trade"]
		c.mu.RUnlock()

		for _, trade := range trades {
			for _, handler := range handlers {
				handler(trade)
			}
		}
		return
	}

	if stream == "spot@public.ticker.v3.api" {
		var tickers []TickerData
		if err := json.Unmarshal(data, &tickers); err != nil {
			log.Errorf("Error unmarshaling ticker data: %v", err)
			return
		}

		c.mu.RLock()
		handlers := c.handlers["ticker"]
		c.mu.RUnlock()

		for _, ticker := range tickers {
			for _, handler := range handlers {
				handler(ticker)
			}
		}
		return
	}
}

func (c *Client) GetSpotSymbols() ([]string, error) {
	return []string{
		"BTCUSDT", "ETHUSDT", "BNBUSDT", "ADAUSDT", "SOLUSDT",
		"XRPUSDT", "DOTUSDT", "DOGEUSDT", "AVAXUSDT", "MATICUSDT",
		"LINKUSDT", "LTCUSDT", "UNIUSDT", "ATOMUSDT", "ETCUSDT",
		"FILUSDT", "TRXUSDT", "XLMUSDT", "VETUSDT", "ALGOUSDT",
	}, nil
}
