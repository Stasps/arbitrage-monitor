package db

import (
	"database/sql"
	"time"

	"arbitrage-monitor/pkg/models"

	_ "modernc.org/sqlite"
)

// DB структура для работы с базой данных
type DB struct {
	conn *sql.DB
}

// NewDB создает новое подключение к БД и выполняет миграции
func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

// Close закрывает соединение с БД
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate создает необходимые таблицы
func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS instruments (
			ticker TEXT PRIMARY KEY,
			figi TEXT NOT NULL,
			instrument_type TEXT NOT NULL,
			lot INTEGER NOT NULL,
			expiry_date TEXT,
			go REAL,
			updated_at TEXT NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS dividends (
			ticker TEXT,
			dividend REAL NOT NULL,
			ex_date TEXT NOT NULL,
			payment_date TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (ticker, ex_date)
		)`,

		`CREATE TABLE IF NOT EXISTS last_prices (
			figi TEXT PRIMARY KEY,
			price REAL NOT NULL,
			price_time TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	}

	for _, q := range queries {
		if _, err := db.conn.Exec(q); err != nil {
			return err
		}
	}

	return nil
}

// SaveInstrument сохраняет или обновляет данные инструмента
func (db *DB) SaveInstrument(instr *models.Instrument) error {
	query := `INSERT OR REPLACE INTO instruments 
		(ticker, figi, instrument_type, lot, expiry_date, go, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	var expiryDate interface{}
	if instr.ExpiryDate != nil {
		expiryDate = instr.ExpiryDate.Format(time.RFC3339)
	} else {
		expiryDate = nil
	}

	var goVal interface{}
	if instr.GO != nil {
		goVal = *instr.GO
	} else {
		goVal = nil
	}

	_, err := db.conn.Exec(query,
		instr.Ticker,
		instr.Figi,
		instr.Type,
		instr.Lot,
		expiryDate,
		goVal,
		instr.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

// GetInstrument получает инструмент по тикеру
func (db *DB) GetInstrument(ticker string) (*models.Instrument, error) {
	query := `SELECT ticker, figi, instrument_type, lot, expiry_date, go, updated_at
		FROM instruments WHERE ticker = ?`

	var instr models.Instrument
	var expiryDate sql.NullString
	var goVal sql.NullFloat64

	err := db.conn.QueryRow(query, ticker).Scan(
		&instr.Ticker,
		&instr.Figi,
		&instr.Type,
		&instr.Lot,
		&expiryDate,
		&goVal,
		&instr.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if expiryDate.Valid {
		t, _ := time.Parse(time.RFC3339, expiryDate.String)
		instr.ExpiryDate = &t
	}
	if goVal.Valid {
		instr.GO = &goVal.Float64
	}

	return &instr, nil
}

// SaveDividend сохраняет данные о дивиденде
func (db *DB) SaveDividend(dividend *models.Dividend) error {
	query := `INSERT OR REPLACE INTO dividends 
		(ticker, dividend, ex_date, payment_date, updated_at)
		VALUES (?, ?, ?, ?, ?)`

	_, err := db.conn.Exec(query,
		dividend.Ticker,
		dividend.Dividend,
		dividend.ExDate.Format(time.RFC3339),
		dividend.PaymentDate.Format(time.RFC3339),
		dividend.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

// GetDividend получает ближайший дивиденд по акции (дата выплаты >= today)
func (db *DB) GetDividend(ticker string) (*models.Dividend, error) {
	query := `SELECT ticker, dividend, ex_date, payment_date, updated_at
		FROM dividends 
		WHERE ticker = ? AND payment_date >= datetime('now')
		ORDER BY payment_date ASC LIMIT 1`

	var div models.Dividend
	var exDate, paymentDate, updatedAt string

	err := db.conn.QueryRow(query, ticker).Scan(
		&div.Ticker,
		&div.Dividend,
		&exDate,
		&paymentDate,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	div.ExDate, _ = time.Parse(time.RFC3339, exDate)
	div.PaymentDate, _ = time.Parse(time.RFC3339, paymentDate)
	div.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &div, nil
}

// SaveLastPrice сохраняет последнюю цену
func (db *DB) SaveLastPrice(price *models.LastPrice) error {
	query := `INSERT OR REPLACE INTO last_prices 
		(figi, price, price_time, updated_at)
		VALUES (?, ?, ?, ?)`

	_, err := db.conn.Exec(query,
		price.Figi,
		price.Price,
		price.PriceTime.Format(time.RFC3339),
		price.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

// GetLastPrice получает последнюю цену по FIGI
func (db *DB) GetLastPrice(figi string) (*models.LastPrice, error) {
	query := `SELECT figi, price, price_time, updated_at
		FROM last_prices WHERE figi = ?`

	var price models.LastPrice
	var priceTime, updatedAt string

	err := db.conn.QueryRow(query, figi).Scan(
		&price.Figi,
		&price.Price,
		&priceTime,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	price.PriceTime, _ = time.Parse(time.RFC3339, priceTime)
	price.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &price, nil
}
