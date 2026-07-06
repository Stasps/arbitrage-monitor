package models

import "time"

// Pair представляет связку акции и фьючерсного контракта
type Pair struct {
	ID           string `yaml:"id"`
	StockTicker  string `yaml:"stock_ticker"`
	FutureTicker string `yaml:"future_ticker"`
	FutureUID    string `yaml:"future_uid"` // уникальный идентификатор фьючерса
}

// Config хранит глобальную конфигурацию приложения
type Config struct {
	UpdateInterval int     `yaml:"update_interval"`
	Commission     float64 `yaml:"commission"`
	Pairs          []Pair  `yaml:"pairs"`
}

// Instrument хранит метаданные инструмента
type Instrument struct {
	Ticker     string
	Figi       string
	UID        string
	Type       string
	Lot        int
	ExpiryDate *time.Time
	GO         *float64
	UpdatedAt  time.Time
}

// Dividend хранит информацию о дивидендах
type Dividend struct {
	Ticker      string
	Dividend    float64
	ExDate      time.Time
	PaymentDate time.Time
	UpdatedAt   time.Time
}

// LastPrice кэширует последнюю цену
type LastPrice struct {
	Figi      string
	Price     float64
	PriceTime time.Time
	UpdatedAt time.Time
}
