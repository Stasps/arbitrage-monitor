package api

import (
	"time"

	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/pkg/models"
)

// Service - сервис для работы с API и БД
type Service struct {
	client *TinkoffClient
	db     *db.DB
}

// NewService создает новый сервис
func NewService(client *TinkoffClient, database *db.DB) *Service {
	return &Service{
		client: client,
		db:     database,
	}
}

// GetOrFetchInstrument получает инструмент из БД или из API
func (s *Service) GetOrFetchInstrument(ticker string, isFuture bool) (*models.Instrument, error) {
	// Сначала проверяем в БД
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
		future, err := s.client.GetFutureInfo(ticker)
		if err != nil {
			return nil, err
		}
		figi = future.Figi
		lot = int(future.Lot)
		t := future.ExpirationDate.AsTime()
		expiryDate = &t
	} else {
		share, err := s.client.GetShareInfo(ticker)
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

	// Сохраняем в БД
	if err := s.db.SaveInstrument(instr); err != nil {
		return nil, err
	}

	return instr, nil
}

// GetFutureGO получает ГО для фьючерса (TODO: реализовать позже)
func (s *Service) GetFutureGO(figi string) (float64, error) {
	// Временно возвращаем заглушку
	return 0, nil
}
