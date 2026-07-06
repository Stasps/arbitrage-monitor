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
func NewUpdater(apiService *api.Service, database *db.DB, calc *calculator.Calculator, interval int) *Updater {
	return &Updater{
		apiService: apiService,
		db:         database,
		calc:       calc,
		interval:   time.Duration(interval) * time.Second,
	}
}

// Start запускает бесконечный цикл обновления для пары
func (u *Updater) Start(pair models.Pair) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
		// Защита от паники в горутине
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Восстановление после паники в updater для %s: %v", pair.ID, r)
				}
			}()
			u.update(pair)
		}()
		<-ticker.C
	}
}

// update выполняет одно обновление данных для пары
func (u *Updater) update(pair models.Pair) {
	// Акцию ищем по тикеру
	stockInstr, err := u.apiService.GetOrFetchInstrumentByTicker(pair.StockTicker, false)
	if err != nil {
		log.Printf("Ошибка получения акции %s: %v", pair.StockTicker, err)
		return
	}
	if stockInstr == nil {
		log.Printf("Акция %s не найдена", pair.StockTicker)
		return
	}

	// Фьючерс ищем по UID или тикеру
	var futureInstr *models.Instrument
	if pair.FutureUID != "" {
		futureInstr, err = u.apiService.GetOrFetchInstrumentByUID(pair.FutureUID)
	} else {
		futureInstr, err = u.apiService.GetOrFetchInstrumentByTicker(pair.FutureTicker, true)
	}
	if err != nil {
		log.Printf("Ошибка получения фьючерса %s: %v", pair.FutureTicker, err)
		return
	}
	if futureInstr == nil {
		log.Printf("Фьючерс %s не найден", pair.FutureTicker)
		return
	}

	// Проверяем наличие FIGI
	if stockInstr.Figi == "" {
		log.Printf("FIGI для акции %s не загружен", pair.StockTicker)
		return
	}
	if futureInstr.Figi == "" {
		log.Printf("FIGI для фьючерса %s не загружен", pair.FutureTicker)
		return
	}

	// Проверяем наличие даты экспирации (для фьючерсов)
	if futureInstr.ExpiryDate == nil {
		log.Printf("Дата экспирации для фьючерса %s не загружена", pair.FutureTicker)
		return
	}

	// Проверяем лотность (множитель)
	if futureInstr.Lot == 0 {
		log.Printf("Множитель для фьючерса %s не загружен (0), укажите вручную в БД", pair.FutureTicker)
		return
	}

	// Получаем цены
	prices, err := u.apiService.GetLastPrices([]string{stockInstr.Figi, futureInstr.Figi})
	if err != nil {
		log.Printf("API недоступно, использую кэш: %v", err)
		stockPrice, _ := u.db.GetLastPrice(stockInstr.Figi)
		futurePrice, _ := u.db.GetLastPrice(futureInstr.Figi)
		if stockPrice == nil || futurePrice == nil {
			return
		}
		u.processData(stockInstr, futureInstr, stockPrice.Price, futurePrice.Price)
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

	u.processData(stockInstr, futureInstr, stockPrice, futurePrice)
}

// processData выполняет расчёты и выводит результат в лог
func (u *Updater) processData(stockInstr, futureInstr *models.Instrument, stockPrice, futurePrice float64) {
	// Защита от nil (на случай, если проверки в update пропустили)
	if futureInstr == nil || futureInstr.ExpiryDate == nil || futureInstr.Figi == "" {
		log.Printf("Пропуск расчёта для %s: неполные данные фьючерса", stockInstr.Ticker)
		return
	}

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
		LotFuture:           futureInstr.Lot, // теперь это множитель (basic_asset_size)
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
