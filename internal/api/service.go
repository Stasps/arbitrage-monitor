package api

import (
	"fmt"
	"log"
	"time"

	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/pkg/models"

	"github.com/vodolaz095/go-investAPI/investapi"
)

// Service - сервисный слой для работы с API и БД
// Обеспечивает кэширование данных и автоматическое получение из API при отсутствии в БД
type Service struct {
	client           *TinkoffClient
	db               *db.DB
	marginMultiplier float64
}

// NewService создает новый сервис
// Принимает: client - клиент API, database - подключение к БД,
// marginMultiplier - множитель ГО (0.5 для КПУР, 1.0 для КСУР)
//
// Возвращает: *Service - сервисный слой
func NewService(client *TinkoffClient, database *db.DB, marginMultiplier float64) *Service {
	return &Service{
		client:           client,
		db:               database,
		marginMultiplier: marginMultiplier,
	}
}

// GetOrFetchInstrumentByTicker получает инструмент из БД или из API по тикеру (только для акций)
// Принимает: ticker - биржевой тикер, isFuture - true для фьючерса, false для акции
// Возвращает: *models.Instrument - данные инструмента, error - ошибка при получении
// Алгоритм: сначала проверяет БД, при отсутствии запрашивает API и сохраняет в БД
// Для фьючерсов рекомендуется использовать GetOrFetchInstrumentByUID
func (s *Service) GetOrFetchInstrumentByTicker(ticker string, isFuture bool) (*models.Instrument, error) {
	// Проверяем в БД
	instr, err := s.db.GetInstrument(ticker)
	if err != nil {
		return nil, err
	}
	if instr != nil {
		return instr, nil
	}

	// Если нет в БД - запрашиваем из API
	if isFuture {
		return nil, nil // для фьючерсов используем GetOrFetchInstrumentByUID
	}

	share, err := s.client.GetShareInfoByTicker(ticker)
	if err != nil {
		return nil, err
	}

	instr = &models.Instrument{
		Ticker:    ticker,
		Figi:      share.Figi,
		Type:      "stock",
		Lot:       int(share.Lot),
		UpdatedAt: time.Now(),
	}

	if err := s.db.SaveInstrument(instr); err != nil {
		return nil, err
	}

	return instr, nil
}

// GetOrFetchInstrumentByUID получает инструмент из БД или из API по UID (только для фьючерсов)
// Принимает: uid - уникальный идентификатор фьючерса
// Возвращает: *models.Instrument - данные инструмента, error - ошибка при получении
// Алгоритм:
//  1. Проверяет БД по UID
//  2. Если инструмент найден и FIGI не пустой - возвращает данные из БД
//  3. Если инструмент не найден или FIGI пустой - запрашивает API:
//     a. GetInstrumentByUID для получения FIGI и общей информации
//     b. FutureBy для получения даты экспирации и множителя (basic_asset_size)
//  4. Сохраняет в БД: FIGI, множитель (в поле Lot), дату экспирации
//  5. Получает ГО через GetFutureGO (используя полученный FIGI)
//  6. Возвращает обновлённый инструмент
func (s *Service) GetOrFetchInstrumentByUID(uid string) (*models.Instrument, error) {
	// 1. Проверяем в БД
	instr, err := s.db.GetInstrumentByUID(uid)
	if err != nil {
		return nil, err
	}

	// 2. Если инструмент найден и FIGI не пустой — возвращаем
	if instr != nil && instr.Figi != "" {
		return instr, nil
	}

	// 3. Если инструмент не найден ИЛИ FIGI пустой — запрашиваем API
	log.Printf("Запрашиваем информацию по UID: %s", uid)

	// 3a. Получаем FIGI через GetInstrumentBy
	instrument, err := s.client.GetInstrumentByUID(uid)
	if err != nil {
		return nil, err
	}
	figi := instrument.Figi
	if figi == "" {
		log.Printf("Внимание: FIGI для UID %s не найден", uid)
	}

	// 3b. Получаем данные фьючерса через FutureBy (для даты экспирации и множителя)
	future, err := s.client.GetFutureInfoByUID(uid)
	if err != nil {
		return nil, err
	}
	expiryTime := future.ExpirationDate.AsTime()
	ticker := future.Ticker

	// Определяем множитель (количество акций в одном контракте)
	// Используем basic_asset_size, если он есть, иначе fallback на lot (обычно 1)
	var multiplier int
	if future.BasicAssetSize != nil {
		multiplier = int(future.BasicAssetSize.Units)
	} else {
		multiplier = int(future.Lot)
	}
	log.Printf("Множитель для %s: %d (basic_asset_size)", ticker, multiplier)

	// 4. Если инструмент уже был в БД — обновляем
	if instr != nil {
		instr.Figi = figi
		instr.ExpiryDate = &expiryTime
		instr.Lot = multiplier // <-- сохраняем множитель в поле Lot
		instr.Ticker = ticker
		instr.UpdatedAt = time.Now()
		// Получаем ГО (если есть FIGI)
		if figi != "" {
			if goVal, err := s.client.GetFutureGO(figi); err == nil {
				instr.GO = &goVal
			} else {
				log.Printf("Ошибка получения ГО для %s: %v", ticker, err)
			}
		}
		if err := s.db.SaveInstrument(instr); err != nil {
			return nil, err
		}
		return instr, nil
	}

	// 5. Создаём новый инструмент
	instr = &models.Instrument{
		Ticker:     ticker,
		Figi:       figi,
		UID:        uid,
		Type:       "futures",
		Lot:        multiplier, // <-- сохраняем множитель
		ExpiryDate: &expiryTime,
		UpdatedAt:  time.Now(),
	}
	if figi != "" {
		if goVal, err := s.client.GetFutureGO(figi); err == nil {
			instr.GO = &goVal
		}
	}
	if err := s.db.SaveInstrument(instr); err != nil {
		return nil, err
	}
	return instr, nil
}

// GetFutureGO получает гарантийное обеспечение для фьючерса через API
// Принимает: figi - FIGI идентификатор фьючерса
// Возвращает: float64 - ГО на один контракт в рублях (с учётом уровня риска клиента), error - ошибка
// Примечание: API возвращает ГО для стандартного уровня риска (КСУР).
// Для КПУР применяется множитель margin_multiplier (обычно 0.5).
func (s *Service) GetFutureGO(figi string) (float64, error) {
	raw, err := s.client.GetFutureGO(figi)
	if err != nil {
		return 0, err
	}
	return raw * s.marginMultiplier, nil
}

// GetLastPrices получает последние цены через API клиент
// Принимает: figis - список FIGI идентификаторов
// Возвращает: []*investapi.LastPrice - список последних цен, error - ошибка при запросе
func (s *Service) GetLastPrices(figis []string) ([]*investapi.LastPrice, error) {
	return s.client.GetLastPrices(figis)
}

// GetBestPrices получает лучшие цены для акции и фьючерса
// Принимает: stockFigi, stockUid - идентификаторы акции (FIGI и UID),
// futureFigi, futureUid - идентификаторы фьючерса (FIGI и UID)
// Возвращает: stockPrice - цена акции, futurePrice - цена фьючерса,
// source - источник цены ("orderbook" или "lastprice"), error - ошибка
// Логика:
//  1. Запрашивает стакан для акции — берёт лучшую заявку на продажу (asks[0].Price)
//  2. Запрашивает стакан для фьючерса — берёт лучшую заявку на покупку (bids[0].Price)
//  3. Если хотя бы один стакан пуст или недоступен — fallback на GetLastPrices
func (s *Service) GetBestPrices(stockFigi, stockUid, futureFigi, futureUid string) (stockPrice, futurePrice float64, source string, err error) {
	// Получаем стакан для акции
	stockOrderBook, err := s.client.GetOrderBook(stockFigi)
	if err != nil {
		log.Printf("Ошибка получения стакана для акции %s: %v", stockFigi, err)
		return s.getLastPricesFallback(stockFigi, stockUid, futureFigi, futureUid)
	}

	// Получаем стакан для фьючерса
	futureOrderBook, err := s.client.GetOrderBook(futureFigi)
	if err != nil {
		log.Printf("Ошибка получения стакана для фьючерса %s: %v", futureFigi, err)
		return s.getLastPricesFallback(stockFigi, stockUid, futureFigi, futureUid)
	}

	// Проверяем, что в стакане акции есть заявки на продажу (asks)
	if len(stockOrderBook.Asks) == 0 {
		log.Printf("Предупреждение: стакан акции %s пуст (нет asks), используем GetLastPrices", stockFigi)
		return s.getLastPricesFallback(stockFigi, stockUid, futureFigi, futureUid)
	}

	// Проверяем, что в стакане фьючерса есть заявки на покупку (bids)
	if len(futureOrderBook.Bids) == 0 {
		log.Printf("Предупреждение: стакан фьючерса %s пуст (нет bids), используем GetLastPrices", futureFigi)
		return s.getLastPricesFallback(stockFigi, stockUid, futureFigi, futureUid)
	}

	// Цена акции — лучшая заявка на продажу (по ней можем купить)
	stockPrice = float64(stockOrderBook.Asks[0].Price.Units) + float64(stockOrderBook.Asks[0].Price.Nano)/1e9
	// Цена фьючерса — лучшая заявка на покупку (по ней можем продать)
	futurePrice = float64(futureOrderBook.Bids[0].Price.Units) + float64(futureOrderBook.Bids[0].Price.Nano)/1e9

	return stockPrice, futurePrice, "orderbook", nil
}

// getLastPricesFallback — вспомогательная функция для fallback на GetLastPrices
// Используется, когда стакан пуст или недоступен.
// Принимает: stockFigi, stockUid - идентификаторы акции (FIGI и UID),
// futureFigi, futureUid - идентификаторы фьючерса (FIGI и UID)
//
// Возвращает: stockPrice, futurePrice - цены, source - "lastprice", error - ошибка
// Примечание: сравнение идёт по FIGI И UID одновременно, так как FIGI deprecated
// и API может перестать его заполнять в ответе.
func (s *Service) getLastPricesFallback(stockFigi, stockUid, futureFigi, futureUid string) (stockPrice, futurePrice float64, source string, err error) {
	prices, err := s.client.GetLastPrices([]string{stockFigi, futureFigi})
	if err != nil {
		log.Printf("Ошибка получения цен через GetLastPrices: %v", err)
		return 0, 0, "", err
	}

	if len(prices) != 2 {
		log.Printf("Ожидалось 2 цены, получено %d", len(prices))
		return 0, 0, "", fmt.Errorf("ожидалось 2 цены, получено %d", len(prices))
	}

	var stockFound, futureFound bool
	for _, p := range prices {
		if p == nil {
			continue
		}
		price := float64(p.Price.Units) + float64(p.Price.Nano)/1e9

		// Сравниваем по FIGI и UID (FIGI deprecated, может быть пустым)
		if p.Figi == stockFigi || p.InstrumentUid == stockUid {
			stockPrice = price
			stockFound = true
		} else if p.Figi == futureFigi || p.InstrumentUid == futureUid {
			futurePrice = price
			futureFound = true
		}
	}

	if !stockFound || !futureFound {
		log.Printf("Не найдены цены (stock=%v, future=%v)", stockFound, futureFound)
		return 0, 0, "", fmt.Errorf("не найдены цены для stock=%v, future=%v", stockFound, futureFound)
	}

	return stockPrice, futurePrice, "lastprice", nil
}
