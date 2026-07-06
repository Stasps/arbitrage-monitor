package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/vodolaz095/go-investAPI/investapi"
	_ "modernc.org/sqlite"
)

// ============================================================
// НАСТРОЙКИ
// ============================================================

const (
	TokenFile = "token"
	DBFile    = "arbitrage.db"
	LogFile   = "f/update_futures.log"
	Mode      = "all" // "all" или "missing"

	APITimeout = 15 * time.Second
)

// ============================================================
// КОД
// ============================================================

func main() {
	os.MkdirAll("f", 0755)

	logFile, err := os.Create(LogFile)
	if err != nil {
		log.Fatalf("Ошибка создания лог-файла: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	log.Println("=== НАЧАЛО ОБНОВЛЕНИЯ ФЬЮЧЕРСОВ ===")

	// 1. Токен
	token, err := readToken(TokenFile)
	if err != nil {
		log.Fatalf("Ошибка чтения токена: %v", err)
	}
	if token == "" {
		log.Fatal("Токен пустой")
	}

	// 2. БД
	db, err := sql.Open("sqlite", DBFile+"?_busy_timeout=30000&_journal_mode=WAL")
	if err != nil {
		log.Fatalf("Ошибка открытия БД: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		log.Fatalf("БД не отвечает: %v", err)
	}
	log.Println("Подключение к БД установлено")

	// 3. Клиент API
	client, err := investapi.New(token)
	if err != nil {
		log.Fatalf("Ошибка создания клиента: %v", err)
	}
	defer client.Connection.Close()
	log.Println("Клиент API создан")

	// 4. Собираем все данные из БД в слайс (чтобы закрыть rows)
	rows, err := db.Query(`
		SELECT ticker, uid, figi, lot, go
		FROM instruments
		WHERE instrument_type = 'futures' AND uid != '' AND uid IS NOT NULL
	`)
	if err != nil {
		log.Fatalf("Ошибка запроса: %v", err)
	}

	type FutureRecord struct {
		Ticker string
		UID    string
		Figi   string
		Lot    int
		Go     sql.NullFloat64
	}

	var records []FutureRecord
	for rows.Next() {
		var r FutureRecord
		if err := rows.Scan(&r.Ticker, &r.UID, &r.Figi, &r.Lot, &r.Go); err != nil {
			log.Printf("Ошибка сканирования: %v", err)
			continue
		}
		records = append(records, r)
	}
	rows.Close() // <- Закрываем rows, освобождаем блокировку

	if err := rows.Err(); err != nil {
		log.Fatalf("Ошибка после итерации: %v", err)
	}

	if len(records) == 0 {
		log.Println("Нет фьючерсов для обновления.")
		return
	}

	var updatedCount, skippedCount int

	// 5. Обновляем каждую запись
	for _, r := range records {
		needUpdate := false
		switch Mode {
		case "all":
			needUpdate = true
		case "missing":
			if r.Lot == 0 || !r.Go.Valid || r.Go.Float64 == 0 {
				needUpdate = true
			}
		}
		if !needUpdate {
			log.Printf("Пропускаем %s (данные есть)", r.Ticker)
			skippedCount++
			continue
		}

		log.Printf("Обновляем %s (UID: %s)...", r.Ticker, r.UID)

		ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
		defer cancel()

		// 5a. Получаем FIGI (если нет)
		figi := r.Figi
		if figi == "" {
			log.Printf("  Запрашиваем FIGI через GetInstrumentBy...")
			instResp, err := client.InstrumentsServiceClient.GetInstrumentBy(ctx,
				&investapi.InstrumentRequest{
					IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_UID,
					Id:     r.UID,
				},
			)
			if err != nil {
				log.Printf("  Ошибка GetInstrumentBy: %v", err)
				continue
			}
			if instResp.Instrument != nil {
				figi = instResp.Instrument.Figi
				log.Printf("  Получен FIGI: %s", figi)
			}
		}

		// 5b. Получаем множитель и дату экспирации через FutureBy
		log.Printf("  Запрашиваем данные фьючерса через FutureBy...")
		futureResp, err := client.InstrumentsServiceClient.FutureBy(ctx,
			&investapi.InstrumentRequest{
				IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_UID,
				Id:     r.UID,
			},
		)
		if err != nil {
			log.Printf("  Ошибка FutureBy: %v", err)
			continue
		}
		if futureResp.Instrument == nil {
			log.Printf("  Фьючерс не найден")
			continue
		}

		future := futureResp.Instrument
		multiplier := 1
		if future.BasicAssetSize != nil {
			multiplier = int(future.BasicAssetSize.Units)
		}
		expiry := future.ExpirationDate.AsTime()
		log.Printf("  Множитель: %d, экспирация: %s", multiplier, expiry.Format("2006-01-02"))

		// 5c. Получаем GO
		var goNew float64
		if figi != "" {
			log.Printf("  Запрашиваем GO через GetFuturesMargin...")
			marginResp, err := client.InstrumentsServiceClient.GetFuturesMargin(ctx,
				&investapi.GetFuturesMarginRequest{
					Figi: figi,
				},
			)
			if err != nil {
				log.Printf("  Ошибка GetFuturesMargin: %v", err)
			} else if marginResp.InitialMarginOnSell != nil {
				goNew = float64(marginResp.InitialMarginOnSell.Units) + float64(marginResp.InitialMarginOnSell.Nano)/1e9
				log.Printf("  Получен GO: %.2f", goNew)
			}
		}

		// 5d. Обновляем БД
		log.Printf("  Обновляем запись в БД...")
		_, err = db.Exec(`
			UPDATE instruments
			SET figi = ?,
			    lot = ?,
			    expiry_date = ?,
			    go = ?,
			    updated_at = ?
			WHERE uid = ?
		`, figi, multiplier, expiry.Format(time.RFC3339), goNew, time.Now().Format(time.RFC3339), r.UID)

		if err != nil {
			log.Printf("  Ошибка обновления БД: %v", err)
			continue
		}

		log.Printf("  Успешно обновлён: lot=%d, go=%.2f", multiplier, goNew)
		updatedCount++
	}

	log.Printf("=== ОБНОВЛЕНИЕ ЗАВЕРШЕНО ===")
	log.Printf("Обновлено: %d, пропущено: %d", updatedCount, skippedCount)

	fmt.Printf("Обновление завершено. Подробности в файле: %s\n", LogFile)
	fmt.Println("Нажмите Enter для выхода...")
	fmt.Scanln()
}

func readToken(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
