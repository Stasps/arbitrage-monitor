package db

import (
	"database/sql"
	"sync"
	"time"

	"arbitrage-monitor/pkg/models"

	_ "modernc.org/sqlite"
)

// DB структура для работы с базой данных
type DB struct {
	conn  *sql.DB
	mutex sync.Mutex
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

// migrate создает необходимые таблицы и выполняет миграции
func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS instruments (
			ticker TEXT PRIMARY KEY,
			figi TEXT,
			uid TEXT,
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

	// Проверяем, существует ли колонка uid
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('instruments') WHERE name = 'uid';`).Scan(&count)
	if err != nil {
		return err
	}

	// Если колонки нет — добавляем
	if count == 0 {
		_, err := db.conn.Exec(`ALTER TABLE instruments ADD COLUMN uid TEXT;`)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetInstrument получает инструмент из БД по тикеру
func (db *DB) GetInstrument(ticker string) (*models.Instrument, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	query := `SELECT ticker, figi, uid, instrument_type, lot, expiry_date, go, updated_at
		FROM instruments WHERE ticker = ?`

	var instr models.Instrument
	var expiryDate sql.NullString
	var goVal sql.NullFloat64
	var uid sql.NullString
	var updatedAt string

	err := db.conn.QueryRow(query, ticker).Scan(
		&instr.Ticker,
		&instr.Figi,
		&uid,
		&instr.Type,
		&instr.Lot,
		&expiryDate,
		&goVal,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if uid.Valid {
		instr.UID = uid.String
	}
	if expiryDate.Valid {
		t, _ := time.Parse(time.RFC3339, expiryDate.String)
		instr.ExpiryDate = &t
	}
	if goVal.Valid {
		instr.GO = &goVal.Float64
	}
	instr.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &instr, nil
}

// GetInstrumentByUID получает инструмент из БД по UID
func (db *DB) GetInstrumentByUID(uid string) (*models.Instrument, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	query := `SELECT ticker, figi, uid, instrument_type, lot, expiry_date, go, updated_at
		FROM instruments WHERE uid = ?`

	var instr models.Instrument
	var expiryDate sql.NullString
	var goVal sql.NullFloat64
	var updatedAt string

	err := db.conn.QueryRow(query, uid).Scan(
		&instr.Ticker,
		&instr.Figi,
		&instr.UID,
		&instr.Type,
		&instr.Lot,
		&expiryDate,
		&goVal,
		&updatedAt,
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
	instr.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &instr, nil
}

// SaveInstrument сохраняет или обновляет данные инструмента в БД
func (db *DB) SaveInstrument(instr *models.Instrument) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	query := `INSERT OR REPLACE INTO instruments 
		(ticker, figi, uid, instrument_type, lot, expiry_date, go, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

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
		instr.UID,
		instr.Type,
		instr.Lot,
		expiryDate,
		goVal,
		instr.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

// SaveDividend сохраняет или обновляет данные о дивиденде в БД
func (db *DB) SaveDividend(dividend *models.Dividend) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

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

// GetDividend получает ближайший дивиденд по акции (дата выплаты >= сегодня)
func (db *DB) GetDividend(ticker string) (*models.Dividend, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

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

// SaveLastPrice сохраняет или обновляет последнюю цену в БД
func (db *DB) SaveLastPrice(price *models.LastPrice) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

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

// GetLastPrice получает последнюю цену из БД по FIGI идентификатору
func (db *DB) GetLastPrice(figi string) (*models.LastPrice, error) {
	db.mutex.Lock()
	defer db.mutex.Unlock()

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
