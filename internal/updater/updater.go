package updater

import (
	"log"
	"time"

	"arbitrage-monitor/internal/api"
	"arbitrage-monitor/internal/calculator"
	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/pkg/models"
)

// Updater - управляет циклом обновления данных
type Updater struct {
	apiService *api.Service
	db         *db.DB
	calc       *calculator.Calculator
	interval   time.Duration
}

// NewUpdater создаёт новый updater
func NewUpdater(apiService *api.Service, database *db.DB, calc *calculator.Calculator, interval int) *Updater {
	return &Updater{
		apiService: apiService,
		db:         database,
		calc:       calc,
		interval:   time.Duration(interval) * time.Second,
	}
}

// Start запускает цикл обновления
func (u *Updater) Start(stockTicker, futureTicker string) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		u.update(stockTicker, futureTicker)
		<-ticker.C
	}
}

// update выполняет одно обновление данных
func (u *Updater) update(stockTicker, futureTicker string) {
	// 1. Получаем инструменты (из кэша или API)
	stockInstr, err := u.apiService.GetOrFetchInstrument(stockTicker, false)
	if err != nil {
		log.Printf("Ошибка получения акции %s: %v", stockTicker, err)
		return
	}

	futureInstr, err := u.apiService.GetOrFetchInstrument(futureTicker, true)
	if err != nil {
		log.Printf("Ошибка получения фьючерса %s: %v", futureTicker, err)
		return
	}

	// 2. Получаем цены
	prices, err := u.apiService.GetLastPrices([]string{stockInstr.Figi, futureInstr.Figi})
	if err != nil {
		// Используем кэшированные цены
		log.Printf("API недоступно, использую кэш: %v", err)
		stockPrice, _ := u.db.GetLastPrice(stockInstr.Figi)
		futurePrice, _ := u.db.GetLastPrice(futureInstr.Figi)
		if stockPrice == nil || futurePrice == nil {
			return
		}
		u.processData(stockInstr, futureInstr, stockPrice.Price, futurePrice.Price)
		return
	}

	// 3. Сохраняем цены в БД
	for _, p := range prices {
		u.db.SaveLastPrice(&models.LastPrice{
			Figi:      p.Figi,
			Price:     float64(p.Price.Units) + float64(p.Price.Nano)/1e9,
			PriceTime: p.Time.AsTime(),
			UpdatedAt: time.Now(),
		})
	}

	// 4. Находим цены для акции и фьючерса
	var stockPrice, futurePrice float64
	for _, p := range prices {
		if p.Figi == stockInstr.Figi {
			stockPrice = float64(p.Price.Units) + float64(p.Price.Nano)/1e9
		}
		if p.Figi == futureInstr.Figi {
			futurePrice = float64(p.Price.Units) + float64(p.Price.Nano)/1e9
		}
	}

	// 5. Обрабатываем данные
	u.processData(stockInstr, futureInstr, stockPrice, futurePrice)
}

// processData выполняет расчёты и выводит результат
func (u *Updater) processData(stockInstr, futureInstr *models.Instrument, stockPrice, futurePrice float64) {
	// Получаем дивиденд
	div, _ := u.db.GetDividend(stockInstr.Ticker)
	var dividend float64
	var paymentDate *time.Time
	if div != nil {
		dividend = div.Dividend
		paymentDate = &div.PaymentDate
	}

	// Получаем ГО
	goVal, _ := u.apiService.GetFutureGO(futureInstr.Figi)

	// Расчёт
	result := u.calc.Calculate(calculator.InputData{
		PriceStock:          stockPrice,
		PriceFuture:         futurePrice,
		LotFuture:           futureInstr.Lot,
		Dividend:            dividend,
		DividendPaymentDate: paymentDate,
		ExpiryDate:          *futureInstr.ExpiryDate,
		GO:                  goVal,
	})

	// Вывод в консоль (позже заменим на TUI)
	log.Printf("=== %s / %s ===", stockInstr.Ticker, futureInstr.Ticker)
	log.Printf("Акция: %.2f, Фьюч(акц): %.2f, Спред: %.2f", stockPrice, result.PriceFuturePerShare, result.Spread)
	log.Printf("Див.чист: %.2f, Цена прод: %.2f, Дней: %d", result.DividendNet, result.SellPrice, result.DaysToExpiry)
	log.Printf("Доходность: %.4f%%, Годовая: %.4f%%", result.ReturnPct*100, result.AnnualReturnPct*100)
	log.Printf("ГО/акц: %.2f, Инвест: %.2f", result.GOPerShare, result.InvestedCapital)
}
