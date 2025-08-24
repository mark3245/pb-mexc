package monitor

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"mexc-monitor/internal/config"
	"mexc-monitor/internal/database"
	"mexc-monitor/internal/mexc"
	"mexc-monitor/internal/telegram"

	log "github.com/sirupsen/logrus"
)

type Monitor struct {
	cfg          *config.Config
	db           *database.Database
	bot          *telegram.Bot
	client       *mexc.Client
	mu           sync.RWMutex
	priceHistory map[string][]*PriceData
	volumeData   map[string]*VolumeData
	stopChan     chan struct{}
}

type PriceData struct {
	Price     float64
	Timestamp time.Time
}

type VolumeData struct {
	Volume    int
	Timestamp time.Time
}

func New(cfg *config.Config, db *database.Database, bot *telegram.Bot) (*Monitor, error) {
	client := mexc.NewClient(cfg.MEXC.WebSocketURL)

	return &Monitor{
		cfg:          cfg,
		db:           db,
		bot:          bot,
		client:       client,
		priceHistory: make(map[string][]*PriceData),
		volumeData:   make(map[string]*VolumeData),
		stopChan:     make(chan struct{}),
	}, nil
}

func (m *Monitor) Start(ctx context.Context) error {
	log.Info("Starting MEXC monitor...")

	symbols, err := m.client.GetSpotSymbols()
	if err != nil {
		return fmt.Errorf("failed to get symbols: %w", err)
	}

	log.Infof("Monitoring %d symbols", len(symbols))

	go m.restPollingRoutine(ctx, symbols)

	go m.cleanupRoutine(ctx)

	go m.analysisRoutine(ctx)

	<-ctx.Done()

	log.Info("Stopping MEXC monitor...")
	return nil
}

func (m *Monitor) handleTrade(data interface{}) {
	trade, ok := data.(mexc.TradeData)
	if !ok {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	price, err := strconv.ParseFloat(trade.Price, 64)
	if err != nil {
		log.Errorf("Failed to parse price: %v", err)
		return
	}

	quantity, err := strconv.ParseFloat(trade.Quantity, 64)
	if err != nil {
		log.Errorf("Failed to parse quantity: %v", err)
		return
	}

	volumeUSD := int(price * quantity)

	if volData, exists := m.volumeData[trade.Symbol]; exists {
		volData.Volume += volumeUSD
		volData.Timestamp = time.Now()
	} else {
		m.volumeData[trade.Symbol] = &VolumeData{
			Volume:    volumeUSD,
			Timestamp: time.Now(),
		}
	}
}

func (m *Monitor) handleTicker(data interface{}) {
	ticker, ok := data.(mexc.TickerData)
	if !ok {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	price, err := strconv.ParseFloat(ticker.Price, 64)
	if err != nil {
		log.Errorf("Failed to parse ticker price: %v", err)
		return
	}

	priceData := &PriceData{
		Price:     price,
		Timestamp: time.Now(),
	}

	if history, exists := m.priceHistory[ticker.Symbol]; exists {
		m.priceHistory[ticker.Symbol] = append(history, priceData)
	} else {
		m.priceHistory[ticker.Symbol] = []*PriceData{priceData}
	}
}

func (m *Monitor) analysisRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.analyzeData()
		}
	}
}

func (m *Monitor) analyzeData() {
	log.Debug("Starting data analysis...")

	settings, err := m.db.GetSettings()
	if err != nil {
		log.Errorf("Failed to get settings: %v", err)
		return
	}

	now := time.Now()
	cutoffTime := now.Add(-time.Duration(settings.TimeInterval) * time.Second)

	log.Debugf("Analysis settings: time_interval=%d, price_change=%.2f%%, min_volume=%d",
		settings.TimeInterval, settings.PriceChange, settings.MinVolume)

	m.mu.Lock()
	defer m.mu.Unlock()

	log.Debugf("Analyzing %d symbols", len(m.priceHistory))

	for symbol, history := range m.priceHistory {
		if len(history) == 0 {
			log.Debugf("Skipping %s: no price history", symbol)
			continue
		}

		currentPrice := history[len(history)-1].Price
		currentTime := history[len(history)-1].Timestamp

		log.Debugf("Analyzing %s: current price=%.6f, time=%s",
			symbol, currentPrice, currentTime.Format("15:04:05"))

		if currentTime.Before(cutoffTime) {
			log.Debugf("Skipping %s: price too old", symbol)
			continue
		}

		if blacklisted, err := m.db.IsBlacklisted(symbol); err != nil {
			log.Errorf("Failed to check blacklist for %s: %v", symbol, err)
			continue
		} else if blacklisted {
			continue
		}

		volData, exists := m.volumeData[symbol]
		if !exists || volData.Timestamp.Before(cutoffTime) {
			continue
		}

		var startPrice float64
		found := false
		targetTime := now.Add(-time.Duration(settings.TimeInterval) * time.Second)

		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Timestamp.Before(targetTime) ||
				history[i].Timestamp.Equal(targetTime) {
				startPrice = history[i].Price
				found = true
				break
			}
		}

		if !found && len(history) > 0 {
			startPrice = history[0].Price
		}

		log.Debugf("Price analysis for %s: start=%.6f, current=%.6f",
			symbol, startPrice, currentPrice)

		priceChange := 0.0
		if startPrice > 0 {
			priceChange = ((currentPrice - startPrice) / startPrice) * 100
		}

		log.Debugf("Price change for %s: %.4f%%", symbol, priceChange)

		log.Debugf("Checking conditions for %s: volume=%d (min=%d), price_change=%.4f%% (threshold=%.2f%%)",
			symbol, volData.Volume, settings.MinVolume, priceChange, settings.PriceChange)

		if volData.Volume >= settings.MinVolume &&
			(priceChange >= settings.PriceChange || priceChange <= -settings.PriceChange) {
			log.Infof("Conditions met for %s! Sending alert...", symbol)
			if err := m.bot.SendAlert(symbol, priceChange, volData.Volume, now); err != nil {
				log.Errorf("Failed to send alert for %s: %v", symbol, err)
			} else {
				log.Infof("Alert sent for %s: %.2f%% change, $%d volume",
					symbol, priceChange, volData.Volume)
			}

			delete(m.volumeData, symbol)
		} else {
			log.Debugf("Conditions not met for %s", symbol)
		}
	}
}

func (m *Monitor) restPollingRoutine(ctx context.Context, symbols []string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Info("Starting REST API polling for price data")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pollPrices(symbols)
		}
	}
}

func (m *Monitor) pollPrices(symbols []string) {
	restClient := mexc.NewRESTClient()

	tickers, err := restClient.GetAllTickers()
	if err != nil {
		log.Errorf("Failed to get tickers: %v", err)
		return
	}

	for _, ticker := range tickers {
		found := false
		for _, symbol := range symbols {
			if symbol == ticker.Symbol {
				found = true
				break
			}
		}

		if !found {
			continue
		}

		price, err := strconv.ParseFloat(ticker.Price, 64)
		if err != nil {
			log.Errorf("Failed to parse price for %s: %v", ticker.Symbol, err)
			continue
		}

		priceData := &PriceData{
			Price:     price,
			Timestamp: time.Now(),
		}

		m.mu.Lock()
		if history, exists := m.priceHistory[ticker.Symbol]; exists {
			m.priceHistory[ticker.Symbol] = append(history, priceData)
		} else {
			m.priceHistory[ticker.Symbol] = []*PriceData{priceData}
		}
		m.mu.Unlock()

		log.Debugf("Updated price for %s: %f", ticker.Symbol, price)
	}

	for _, symbol := range symbols {
		trades, err := restClient.GetRecentTrades(symbol)
		if err != nil {
			log.Debugf("Failed to get trades for %s: %v", symbol, err)
			continue
		}

		totalVolume := 0
		for _, trade := range trades {
			price, err := strconv.ParseFloat(trade.Price, 64)
			if err != nil {
				continue
			}
			qty, err := strconv.ParseFloat(trade.Qty, 64)
			if err != nil {
				continue
			}
			totalVolume += int(price * qty)
		}

		m.mu.Lock()
		m.volumeData[symbol] = &VolumeData{
			Volume:    totalVolume,
			Timestamp: time.Now(),
		}
		m.mu.Unlock()

		log.Debugf("Updated volume for %s: $%d", symbol, totalVolume)
	}
}

func (m *Monitor) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

func (m *Monitor) cleanup() {
	if err := m.db.CleanupExpiredBlacklist(); err != nil {
		log.Errorf("Failed to cleanup blacklist: %v", err)
	}

	now := time.Now()
	cutoffTime := now.Add(-10 * time.Minute)

	m.mu.Lock()
	defer m.mu.Unlock()

	for symbol, history := range m.priceHistory {
		var newHistory []*PriceData
		for _, priceData := range history {
			if priceData.Timestamp.After(cutoffTime) {
				newHistory = append(newHistory, priceData)
			}
		}
		m.priceHistory[symbol] = newHistory
	}

	for symbol, volData := range m.volumeData {
		if volData.Timestamp.Before(cutoffTime) {
			delete(m.volumeData, symbol)
		}
	}
}
