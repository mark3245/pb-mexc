package mexc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

type RESTClient struct {
	baseURL    string
	httpClient *http.Client
}

type TickerResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type TradeResponse struct {
	Symbol    string `json:"symbol"`
	Price     string `json:"price"`
	Qty       string `json:"qty"`
	Time      int64  `json:"time"`
	IsBuyerMaker bool `json:"isBuyerMaker"`
}

type ExchangeInfoResponse struct {
	Symbols []SymbolInfo `json:"symbols"`
}

type SymbolInfo struct {
	Symbol string `json:"symbol"`
	Status string `json:"status"`
}

func NewRESTClient() *RESTClient {
	return &RESTClient{
		baseURL: "https://api.mexc.com",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *RESTClient) GetAllTickers() ([]TickerResponse, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/price", c.baseURL)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP ошибка: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	var tickers []TickerResponse
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	return tickers, nil
}

func (c *RESTClient) GetRecentTrades(symbol string) ([]TradeResponse, error) {
	url := fmt.Sprintf("%s/api/v3/trades?symbol=%s&limit=100", c.baseURL, symbol)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP ошибка: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	var trades []TradeResponse
	if err := json.Unmarshal(body, &trades); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	return trades, nil
}

func (c *RESTClient) GetExchangeInfo() (*ExchangeInfoResponse, error) {
	url := fmt.Sprintf("%s/api/v3/exchangeInfo", c.baseURL)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP ошибка: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	var exchangeInfo ExchangeInfoResponse
	if err := json.Unmarshal(body, &exchangeInfo); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
	}

	return &exchangeInfo, nil
}

func (c *RESTClient) GetActiveSymbols() ([]string, error) {
	exchangeInfo, err := c.GetExchangeInfo()
	if err != nil {
		return nil, err
	}

	var activeSymbols []string
	for _, symbol := range exchangeInfo.Symbols {
		if symbol.Status == "TRADING" {
			activeSymbols = append(activeSymbols, symbol.Symbol)
		}
	}

	log.Infof("Найдено %d активных торговых пар", len(activeSymbols))
	return activeSymbols, nil
}





