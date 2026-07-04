package models

import "time"

// Pair представляет связку акции и фьючерсного контракта
// ID - уникальный идентификатор пары
// StockTicker - тикер акции (например, "SBER")
// FutureTicker - тикер фьючерсного контракта (например, "SRU6")
type Pair struct {
	ID           string `yaml:"id"`
	StockTicker  string `yaml:"stock_ticker"`
	FutureTicker string `yaml:"future_ticker"`
}

// Config хранит конфигурацию приложения
// UpdateInterval - интервал обновления цен в секундах
// Commission - комиссия за сделку в десятичном виде (0.0004 = 0.04%)
// Pairs - список пар акция-фьючерс для мониторинга
type Config struct {
	UpdateInterval int     `yaml:"update_interval"`
	Commission     float64 `yaml:"commission"`
	Pairs          []Pair  `yaml:"pairs"`
}

// Instrument хранит метаданные инструмента из API Т-Инвестиций
// Ticker - тикер инструмента на бирже
// Figi - FIGI идентификатор инструмента
// Type - тип инструмента: "stock" или "future"
// Lot - размер лота (количество акций в контракте)
// ExpiryDate - дата экспирации (только для фьючерсов)
// GO - гарантийное обеспечение (только для фьючерсов)
// UpdatedAt - временная метка последнего обновления
type Instrument struct {
	Ticker     string
	Figi       string
	Type       string // "stock" или "future"
	Lot        int
	ExpiryDate *time.Time // только для фьючерсов
	GO         *float64   // только для фьючерсов
	UpdatedAt  time.Time
}

// Dividend хранит информацию о дивидендах по акции
// Ticker - тикер акции
// Dividend - размер дивиденда на одну акцию
// ExDate - дата отсечки (дата закрытия реестра)
// PaymentDate - дата выплаты дивидендов
// UpdatedAt - временная метка последнего обновления
type Dividend struct {
	Ticker      string
	Dividend    float64
	ExDate      time.Time // дата отсечки
	PaymentDate time.Time // дата выплаты
	UpdatedAt   time.Time
}

// LastPrice кэширует последнюю известную цену из API
// Figi - FIGI инструмента
// Price - последняя цена в рублях
// PriceTime - биржевая временная метка цены
// UpdatedAt - время обновления локального кэша
type LastPrice struct {
	Figi      string
	Price     float64
	PriceTime time.Time
	UpdatedAt time.Time
}
