package models

import "time"

// Pair представляет связку акции и фьючерсного контракта для арбитражного мониторинга
// ID - уникальный идентификатор пары (например, "sber")
// StockTicker - тикер акции на бирже (например, "SBER")
// StockFigi - FIGI идентификатор акции (например, "BBG004730N88")
// FutureTicker - тикер фьючерсного контракта (например, "SRU6")
// FutureFigi - FIGI идентификатор фьючерсного контракта (например, "FUTSBRF09260")
// FutureLot - сколько акций в 1 фьючерсном контракте (для Сбера = 100)
type Pair struct {
	ID           string `yaml:"id"`
	StockTicker  string `yaml:"stock_ticker"`
	StockFigi    string `yaml:"stock_figi"`
	FutureTicker string `yaml:"future_ticker"`
	FutureFigi   string `yaml:"future_figi"`
	FutureLot    int    `yaml:"future_lot"`
}

// Config хранит глобальную конфигурацию приложения
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
// Type - тип инструмента: "stock" (акция) или "future" (фьючерс)
// Lot - размер лота (количество акций в одном контракте)
// ExpiryDate - дата экспирации (только для фьючерсов, для акций - nil)
// GO - гарантийное обеспечение на один контракт (только для фьючерсов)
// UpdatedAt - временная метка последнего обновления записи в БД
type Instrument struct {
	Ticker     string
	Figi       string
	Type       string // "stock" или "future"
	Lot        int
	ExpiryDate *time.Time
	GO         *float64
	UpdatedAt  time.Time
}

// Dividend хранит информацию о дивидендах по акции
// Ticker - тикер акции
// Dividend - размер дивиденда на одну акцию в рублях
// ExDate - дата отсечки (дата закрытия реестра акционеров)
// PaymentDate - дата выплаты дивидендов акционерам
// UpdatedAt - временная метка последнего обновления записи в БД
type Dividend struct {
	Ticker      string
	Dividend    float64
	ExDate      time.Time
	PaymentDate time.Time
	UpdatedAt   time.Time
}

// LastPrice кэширует последнюю известную цену из API
// Figi - FIGI идентификатор инструмента
// Price - последняя цена в рублях за единицу инструмента
// PriceTime - биржевая временная метка последней сделки
// UpdatedAt - время обновления локального кэша в БД
type LastPrice struct {
	Figi      string
	Price     float64
	PriceTime time.Time
	UpdatedAt time.Time
}
