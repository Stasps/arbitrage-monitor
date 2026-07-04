package api

import (
	"time"

	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/pkg/models"

	"github.com/vodolaz095/go-investAPI/investapi"
)

// Service - сервисный слой для работы с API и БД
// Обеспечивает кэширование данных и автоматическое получение из API при отсутствии в БД
type Service struct {
	client *TinkoffClient
	db     *db.DB
}

// NewService создает новый сервис
// Принимает: client - клиент API, database - подключение к БД
// Возвращает: *Service - сервисный слой
func NewService(client *TinkoffClient, database *db.DB) *Service {
	return &Service{
		client: client,
		db:     database,
	}
}

// GetOrFetchInstrumentByFigi получает инструмент из БД или из API по FIGI
// Принимает: figi - FIGI идентификатор, isFuture - true для фьючерса, false для акции
// Возвращает: *models.Instrument - данные инструмента, error - ошибка при получении
// Алгоритм: сначала проверяет БД, при отсутствии запрашивает API и сохраняет в БД
func (s *Service) GetOrFetchInstrumentByFigi(figi string, isFuture bool) (*models.Instrument, error) {
	// Проверяем в БД
	instr, err := s.db.GetInstrumentByFigi(figi)
	if err != nil {
		return nil, err
	}
	if instr != nil {
		return instr, nil
	}

	// Если нет в БД - запрашиваем из API
	var ticker string
	var lot int
	var expiryDate *time.Time

	if isFuture {
		future, err := s.client.GetFutureInfoByFigi(figi)
		if err != nil {
			return nil, err
		}
		ticker = future.Ticker
		lot = int(future.Lot)
		t := future.ExpirationDate.AsTime()
		expiryDate = &t
	} else {
		share, err := s.client.GetShareInfoByFigi(figi)
		if err != nil {
			return nil, err
		}
		ticker = share.Ticker
		lot = int(share.Lot)
	}

	instr = &models.Instrument{
		Ticker:     ticker,
		Figi:       figi,
		Type:       map[bool]string{true: "future", false: "stock"}[isFuture],
		Lot:        lot,
		ExpiryDate: expiryDate,
		UpdatedAt:  time.Now(),
	}

	if err := s.db.SaveInstrument(instr); err != nil {
		return nil, err
	}

	return instr, nil
}

// GetOrFetchInstrumentByTicker получает инструмент из БД или из API по тикеру
// Принимает: ticker - биржевой тикер, isFuture - true для фьючерса, false для акции
// Возвращает: *models.Instrument - данные инструмента, error - ошибка при получении
// Алгоритм: сначала проверяет БД, при отсутствии запрашивает API и сохраняет в БД
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
	var figi string
	var lot int
	var expiryDate *time.Time

	if isFuture {
		future, err := s.client.GetFutureInfoByTicker(ticker)
		if err != nil {
			return nil, err
		}
		figi = future.Figi
		lot = int(future.Lot)
		t := future.ExpirationDate.AsTime()
		expiryDate = &t
	} else {
		share, err := s.client.GetShareInfoByTicker(ticker)
		if err != nil {
			return nil, err
		}
		figi = share.Figi
		lot = int(share.Lot)
	}

	instr = &models.Instrument{
		Ticker:     ticker,
		Figi:       figi,
		Type:       map[bool]string{true: "future", false: "stock"}[isFuture],
		Lot:        lot,
		ExpiryDate: expiryDate,
		UpdatedAt:  time.Now(),
	}

	if err := s.db.SaveInstrument(instr); err != nil {
		return nil, err
	}

	return instr, nil
}

// GetFutureGO получает гарантийное обеспечение для фьючерса через API
// Принимает: figi - FIGI идентификатор фьючерса
// Возвращает: float64 - ГО на один контракт в рублях, error - ошибка при запросе
func (s *Service) GetFutureGO(figi string) (float64, error) {
	return s.client.GetFutureGO(figi)
}

// GetLastPrices получает последние цены через API клиент
// Принимает: figis - список FIGI идентификаторов
// Возвращает: []*investapi.LastPrice - список последних цен, error - ошибка при запросе
func (s *Service) GetLastPrices(figis []string) ([]*investapi.LastPrice, error) {
	return s.client.GetLastPrices(figis)
}
