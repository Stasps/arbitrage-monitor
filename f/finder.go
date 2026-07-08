package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vodolaz095/go-investAPI/investapi"
)

// ============================================================
// НАСТРОЙКИ (меняйте здесь)
// ============================================================

const (
	// Queries - запрос для поиска (можно указать несколько через пробел)
	Queries = "PLZL"

	// InstrumentFilter - фильтр по типу инструмента
	// Возможные значения: "share", "bond", "etf", "futures", "currency", "option"
	InstrumentFilter = "share"

	// ExpiryFilter - фильтр по дате экспирации (только для фьючерсов)
	// Возможные значения:
	//   ""          - показывать все фьючерсы (без фильтра)
	//   "future"    - только фьючерсы с датой экспирации >= сегодня
	//   "year"      - только фьючерсы, экспирация которых в текущем году
	//   "month"     - только фьючерсы, экспирация которых в текущем месяце
	//   "quarter"   - только фьючерсы, экспирация которых в текущем квартале
	//   "3months"   - только фьючерсы, экспирация которых в ближайшие 3 месяца
	//   "2026"      - только фьючерсы с экспирацией в указанном году
	ExpiryFilter = "3months"

	// OutputMode - режим вывода:
	// "full"     - выводить все найденные инструменты (включая пропущенные по дате)
	// "filtered" - выводить только инструменты, прошедшие фильтр (без пропущенных)
	OutputMode = "filtered"

	// TokenFile - путь к файлу с токеном (относительно корня проекта)
	TokenFile = "token"

	// OutputDir - папка для сохранения результатов
	OutputDir = "f"

	// LogFile - имя файла для лога
	LogFile = "search_results.log"
)

// ============================================================
// КОД
// ============================================================

func main() {
	if err := os.MkdirAll(OutputDir, 0755); err != nil {
		log.Fatalf("Ошибка создания директории %s: %v", OutputDir, err)
	}

	logPath := filepath.Join(OutputDir, LogFile)

	token, err := readToken(TokenFile)
	if err != nil {
		log.Fatalf("Ошибка чтения токена: %v", err)
	}
	if token == "" {
		log.Fatal("Токен пустой. Проверьте файл token")
	}

	if err := runSearchAndSave(token, logPath); err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
}

func readToken(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func runSearchAndSave(token string, logPath string) error {
	file, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer file.Close()

	log.SetOutput(file)

	client, err := investapi.New(token)
	if err != nil {
		return err
	}
	defer client.Connection.Close()

	ctx := context.Background()
	queryList := strings.Fields(Queries)

	log.Println("=== НАЧАЛО ПОИСКА ===")
	log.Printf("Запросы: %v", queryList)
	if InstrumentFilter != "" {
		log.Printf("Фильтр по типу: %s", InstrumentFilter)
	}
	if ExpiryFilter != "" {
		log.Printf("Фильтр по дате экспирации: %s", ExpiryFilter)
	}
	log.Printf("Режим вывода: %s", OutputMode)

	now := time.Now()

	for _, q := range queryList {
		if q == "" {
			continue
		}
		log.Printf("\n--- Поиск по запросу: '%s' ---", q)

		resp, err := client.InstrumentsServiceClient.FindInstrument(ctx,
			&investapi.FindInstrumentRequest{Query: q})
		if err != nil {
			log.Printf("  Ошибка поиска: %v", err)
			continue
		}

		if len(resp.Instruments) == 0 {
			log.Println("  Ничего не найдено.")
			continue
		}

		filtered := filterInstruments(resp.Instruments, InstrumentFilter)
		log.Printf("  Найдено %d инструментов (отфильтровано по типу: %d)", len(resp.Instruments), len(filtered))

		for i, inst := range filtered {
			// Проверяем дату экспирации
			valid := true
			if inst.InstrumentType == "futures" && !isExpiryValid(inst.Uid, client, now) {
				valid = false
			}

			if OutputMode == "filtered" && !valid {
				continue
			}

			if !valid {
				log.Printf("    [%d] %s (%s) — ПРОПУЩЕН ПО ДАТЕ", i+1, inst.Ticker, inst.Name)
				continue
			}

			log.Printf("    [%d] ТИКЕР: %s, ИМЯ: %s, ТИП: %s, UID: %s",
				i+1, inst.Ticker, inst.Name, inst.InstrumentType, inst.Uid)

			// Для акций запрашиваем цену
			if inst.InstrumentType == "share" && inst.Figi != "" {
				price, err := getPrice(client, inst.Figi)
				if err != nil {
					log.Printf("      Ошибка получения цены: %v", err)
				} else if price > 0 {
					log.Printf("      FIGI: %s, ЦЕНА: %.2f", inst.Figi, price)
				} else {
					log.Printf("      FIGI: %s, ЦЕНА: нет данных", inst.Figi)
				}
			}

			if inst.InstrumentType == "futures" {
				figi, expiry, err := getFutureDetails(client, inst.Uid)
				if err != nil {
					log.Printf("      Ошибка получения данных: %v", err)
				} else {
					if figi != "" {
						log.Printf("      FIGI: %s", figi)
					}
					if !expiry.IsZero() {
						log.Printf("      ДАТА ЭКСПИРАЦИИ: %s", expiry.Format("2006-01-02 15:04:05"))
					}
				}
			}
		}
		log.Println("  --------------------------")
	}

	log.Println("\n=== ПОИСК ЗАВЕРШЁН ===")
	return nil
}

func getFutureDetails(client *investapi.Client, uid string) (figi string, expiry time.Time, err error) {
	ctx := context.Background()

	instResp, err := client.InstrumentsServiceClient.GetInstrumentBy(ctx,
		&investapi.InstrumentRequest{
			IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_UID,
			Id:     uid,
		})
	if err == nil && instResp.Instrument != nil {
		figi = instResp.Instrument.Figi
	}

	futureResp, err := client.InstrumentsServiceClient.FutureBy(ctx,
		&investapi.InstrumentRequest{
			IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_UID,
			Id:     uid,
		})
	if err == nil && futureResp.Instrument != nil {
		expiry = futureResp.Instrument.ExpirationDate.AsTime()
	}

	return figi, expiry, nil
}

func isExpiryValid(uid string, client *investapi.Client, now time.Time) bool {
	if ExpiryFilter == "" {
		return true
	}

	ctx := context.Background()
	futureResp, err := client.InstrumentsServiceClient.FutureBy(ctx,
		&investapi.InstrumentRequest{
			IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_UID,
			Id:     uid,
		})
	if err != nil || futureResp.Instrument == nil {
		return false
	}

	expiry := futureResp.Instrument.ExpirationDate.AsTime()
	switch ExpiryFilter {
	case "future":
		return expiry.After(now) || expiry.Equal(now)
	case "year":
		return expiry.Year() == now.Year()
	case "month":
		return expiry.Year() == now.Year() && expiry.Month() == now.Month()
	case "quarter":
		quarter := (int(now.Month())-1)/3 + 1
		expQuarter := (int(expiry.Month())-1)/3 + 1
		return expiry.Year() == now.Year() && expQuarter == quarter
	case "3months":
		return expiry.After(now) && expiry.Before(now.AddDate(0, 3, 0))
	default:
		if len(ExpiryFilter) == 4 && isDigit(ExpiryFilter) {
			filterYear := 0
			fmt.Sscanf(ExpiryFilter, "%d", &filterYear)
			return expiry.Year() == filterYear
		}
		return true
	}
}

func isDigit(s string) bool {
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func getPrice(client *investapi.Client, figi string) (float64, error) {
	ctx := context.Background()
	resp, err := client.MarketDataServiceClient.GetLastPrices(ctx,
		&investapi.GetLastPricesRequest{
			Figi: []string{figi},
		},
	)
	if err != nil {
		return 0, err
	}
	if len(resp.LastPrices) == 0 || resp.LastPrices[0].Price == nil {
		return 0, nil
	}
	p := resp.LastPrices[0]
	return float64(p.Price.Units) + float64(p.Price.Nano)/1e9, nil
}

func filterInstruments(instruments []*investapi.InstrumentShort, filterType string) []*investapi.InstrumentShort {
	if filterType == "" {
		return instruments
	}
	result := []*investapi.InstrumentShort{}
	for _, inst := range instruments {
		if inst.InstrumentType == filterType {
			result = append(result, inst)
		}
	}
	return result
}
