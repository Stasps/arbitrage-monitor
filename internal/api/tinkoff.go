package api

import (
	"context"
	"time"

	"github.com/vodolaz095/go-investAPI/investapi"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TinkoffClient - клиент для работы с API Т-Инвестиций
type TinkoffClient struct {
	client *investapi.Client
	ctx    context.Context
}

// NewTinkoffClient создает новый клиент для работы с API
// Принимает: token - авторизационный токен Т-Инвестиций
// Возвращает: *TinkoffClient - клиент, error - ошибка при создании
func NewTinkoffClient(token string) (*TinkoffClient, error) {
	client, err := investapi.New(token)
	if err != nil {
		return nil, err
	}

	return &TinkoffClient{
		client: client,
		ctx:    context.Background(),
	}, nil
}

// Close закрывает gRPC соединение с API
// Возвращает: error - ошибка при закрытии соединения
func (c *TinkoffClient) Close() error {
	return c.client.Connection.Close()
}

// GetShareInfoByTicker получает информацию об акции по тикеру
// Принимает: ticker - биржевой тикер акции (например, "SBER")
// Возвращает: *investapi.Share - информация об акции, error - ошибка при запросе
func (c *TinkoffClient) GetShareInfoByTicker(ticker string) (*investapi.Share, error) {
	resp, err := c.client.InstrumentsServiceClient.ShareBy(c.ctx,
		&investapi.InstrumentRequest{
			IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_TICKER,
			Id:     ticker,
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.Instrument, nil
}

// GetFutureInfoByTicker получает информацию о фьючерсе по тикеру
// Принимает: ticker - биржевой тикер фьючерса (например, "SRU6")
// Возвращает: *investapi.Future - информация о фьючерсе, error - ошибка при запросе
func (c *TinkoffClient) GetFutureInfoByTicker(ticker string) (*investapi.Future, error) {
	resp, err := c.client.InstrumentsServiceClient.FutureBy(c.ctx,
		&investapi.InstrumentRequest{
			IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_TICKER,
			Id:     ticker,
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.Instrument, nil
}

// GetShareInfoByFigi получает информацию об акции по FIGI идентификатору
// Принимает: figi - FIGI идентификатор акции (например, "BBG004730N88")
// Возвращает: *investapi.Share - информация об акции, error - ошибка при запросе
func (c *TinkoffClient) GetShareInfoByFigi(figi string) (*investapi.Share, error) {
	resp, err := c.client.InstrumentsServiceClient.ShareBy(c.ctx,
		&investapi.InstrumentRequest{
			IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_FIGI,
			Id:     figi,
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.Instrument, nil
}

// GetFutureInfoByFigi получает информацию о фьючерсе по FIGI идентификатору
// Принимает: figi - FIGI идентификатор фьючерса (например, "FUTSBRF09260")
// Возвращает: *investapi.Future - информация о фьючерсе, error - ошибка при запросе
func (c *TinkoffClient) GetFutureInfoByFigi(figi string) (*investapi.Future, error) {
	resp, err := c.client.InstrumentsServiceClient.FutureBy(c.ctx,
		&investapi.InstrumentRequest{
			IdType: investapi.InstrumentIdType_INSTRUMENT_ID_TYPE_FIGI,
			Id:     figi,
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.Instrument, nil
}

// GetDividends получает список дивидендов по акции за указанный период
// Принимает: figi - FIGI идентификатор акции, from - начальная дата, to - конечная дата
// Возвращает: []*investapi.Dividend - список дивидендов, error - ошибка при запросе
func (c *TinkoffClient) GetDividends(figi string, from, to time.Time) ([]*investapi.Dividend, error) {
	resp, err := c.client.InstrumentsServiceClient.GetDividends(c.ctx,
		&investapi.GetDividendsRequest{
			Figi: figi,
			From: timestamppb.New(from),
			To:   timestamppb.New(to),
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.Dividends, nil
}

// GetLastPrices получает последние цены по списку FIGI идентификаторов
// Принимает: figis - список FIGI идентификаторов
// Возвращает: []*investapi.LastPrice - список последних цен, error - ошибка при запросе
func (c *TinkoffClient) GetLastPrices(figis []string) ([]*investapi.LastPrice, error) {
	resp, err := c.client.MarketDataServiceClient.GetLastPrices(c.ctx,
		&investapi.GetLastPricesRequest{
			Figi: figis,
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.LastPrices, nil
}

// GetFutureGO получает гарантийное обеспечение для фьючерса по FIGI
// Принимает: figi - FIGI идентификатор фьючерса
// Возвращает: float64 - ГО на один контракт в рублях, error - ошибка при запросе
func (c *TinkoffClient) GetFutureGO(figi string) (float64, error) {
	resp, err := c.client.InstrumentsServiceClient.GetFuturesMargin(c.ctx,
		&investapi.GetFuturesMarginRequest{
			Figi: figi,
		},
	)
	if err != nil {
		return 0, err
	}
	if resp.InitialMarginOnSell != nil {
		return float64(resp.InitialMarginOnSell.Units) + float64(resp.InitialMarginOnSell.Nano)/1e9, nil
	}
	return 0, nil
}
