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

// NewTinkoffClient создает новый клиент
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

// Close закрывает соединение
func (c *TinkoffClient) Close() error {
	return c.client.Connection.Close()
}

// GetShareInfo получает информацию об акции по тикеру
func (c *TinkoffClient) GetShareInfo(ticker string) (*investapi.Share, error) {
	resp, err := c.client.InstrumentsServiceClient.ShareBy(
		c.ctx,
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

// GetFutureInfo получает информацию о фьючерсе по тикеру
func (c *TinkoffClient) GetFutureInfo(ticker string) (*investapi.Future, error) {
	resp, err := c.client.InstrumentsServiceClient.FutureBy(
		c.ctx,
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

// GetDividends получает дивиденды по акции
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

// GetLastPrices получает последние цены по списку FIGI
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
