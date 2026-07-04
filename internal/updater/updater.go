package updater

import (
	"log"
	"time"

	"arbitrage-monitor/internal/api"
	"arbitrage-monitor/internal/calculator"
	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/pkg/models"
)

// Updater - управляет циклом обновления данных для одной пары
type Updater struct {
	apiService *api.Service
	db         *db.DB
	calc       *calculator.Calculator
	interval   time.Duration
}

// NewUpdater создаёт новый updater
// Принимает: apiService - сервис API, database - БД, calc - калькулятор, interval - интервал обновления в секундах
// Возвращает: *Updater
func NewUpdater(apiService *api.Service, database *db.DB, calc *calculator.Calculator, interval int) *Updater {
	return &Updater{
		apiService: apiService,
		db:         database,
		calc:       calc,
		interval:   time.Duration(interval) * time.Second,
	}
}

// Start запускает бесконечный цикл обновления для пары
// Принимает: pair - структура с тикерами и FIGI
// Запускает обновление каждые interval секунд
func (u *Updater) Start(pair models.Pair) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		u.update(pair)
		<-ticker.C
	}
}

// update выполняет одно обновление данных для пары
// Принимает: pair - структура с тикерами и FIGI
// Получает инструменты, цены, выполняет расчёты и выводит результат
func (u *Updater) update(pair models.Pair) {
	stockInstr, err := u.apiService.GetOrFetchInstrumentByFigi(pair.StockFigi, false)
	if err != nil {
		log.Printf("Ошибка получения акции %s: %v", pair.StockTicker, err)
		return
	}

	futureInstr, err := u.apiService.GetOrFetchInstrumentByFigi(pair.FutureFigi, true)
	if err != nil {
		log.Printf("Ошибка получения фьючерса %s: %v", pair.FutureTicker, err)
		return
	}

	prices, err := u.apiService.GetLastPrices([]string{stockInstr.Figi, futureInstr.Figi})
	if err != nil {
		log.Printf("API недоступно, использую кэш: %v", err)
		stockPrice, _ := u.db.GetLastPrice(stockInstr.Figi)
		futurePrice, _ := u.db.GetLastPrice(futureInstr.Figi)
		if stockPrice == nil || futurePrice == nil {
			return
		}
		u.processData(stockInstr, futureInstr, stockPrice.Price, futurePrice.Price, pair.FutureLot)
		return
	}

	var stockPrice, futurePrice float64
	for _, p := range prices {
		price := float64(p.Price.Units) + float64(p.Price.Nano)/1e9
		u.db.SaveLastPrice(&models.LastPrice{
			Figi:      p.Figi,
			Price:     price,
			PriceTime: p.Time.AsTime(),
			UpdatedAt: time.Now(),
		})
		if p.Figi == stockInstr.Figi {
			stockPrice = price
		}
		if p.Figi == futureInstr.Figi {
			futurePrice = price
		}
	}

	u.processData(stockInstr, futureInstr, stockPrice, futurePrice, pair.FutureLot)
}

// processData выполняет расчёты и выводит результат в лог
// Принимает: stockInstr - инструмент акции, futureInstr - инструмент фьючерса,
//
//	stockPrice - цена акции, futurePrice - цена фьючерса
//
// Получает дивиденды и ГО, выполняет расчёт, выводит результат
func (u *Updater) processData(stockInstr, futureInstr *models.Instrument, stockPrice, futurePrice float64, futureLot int) {
	div, _ := u.db.GetDividend(stockInstr.Ticker)
	var dividend float64
	var paymentDate *time.Time
	if div != nil {
		dividend = div.Dividend
		paymentDate = &div.PaymentDate
	}

	goVal, _ := u.apiService.GetFutureGO(futureInstr.Figi)

	result := u.calc.Calculate(calculator.InputData{
		PriceStock:          stockPrice,
		PriceFuture:         futurePrice,
		LotFuture:           futureLot, // используем множитель из конфига, а не из API
		Dividend:            dividend,
		DividendPaymentDate: paymentDate,
		ExpiryDate:          *futureInstr.ExpiryDate,
		GO:                  goVal,
	})

	log.Printf("=== %s / %s ===", stockInstr.Ticker, futureInstr.Ticker)
	log.Printf("Акция: %.2f, Фьюч(акц): %.2f, Спред: %.2f", stockPrice, result.PriceFuturePerShare, result.Spread)
	log.Printf("Див.чист: %.2f, Цена прод: %.2f, Дней: %d", result.DividendNet, result.SellPrice, result.DaysToExpiry)
	log.Printf("Доходность: %.4f%%, Годовая: %.4f%%", result.ReturnPct*100, result.AnnualReturnPct*100)
	log.Printf("ГО/акц: %.2f, Инвест: %.2f", result.GOPerShare, result.InvestedCapital)
}
