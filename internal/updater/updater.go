package updater

import (
	"log"
	"time"

	"arbitrage-monitor/internal/api"
	"arbitrage-monitor/internal/calculator"
	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/internal/webserver"
	"arbitrage-monitor/pkg/models"
)

type Updater struct {
	apiService *api.Service
	db         *db.DB
	calc       *calculator.Calculator
	srv        *webserver.Server
	interval   time.Duration
}

func NewUpdater(apiService *api.Service, database *db.DB, calc *calculator.Calculator, interval int, srv *webserver.Server) *Updater {
	return &Updater{
		apiService: apiService,
		db:         database,
		calc:       calc,
		srv:        srv,
		interval:   time.Duration(interval) * time.Second,
	}
}

func (u *Updater) Start(pair models.Pair) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	for {
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

func (u *Updater) update(pair models.Pair) {
	// Защита от nil зависимостей
	if u.apiService == nil || u.db == nil || u.calc == nil || u.srv == nil {
		log.Printf("[%s] Ошибка: одна из зависимостей nil", pair.ID)
		return
	}

	// 1. Получение акции
	stockInstr, err := u.apiService.GetOrFetchInstrumentByTicker(pair.StockTicker, false)
	if err != nil {
		log.Printf("[%s] Ошибка получения акции: %v", pair.ID, err)
		return
	}
	if stockInstr == nil {
		log.Printf("[%s] Акция не найдена", pair.ID)
		return
	}

	// 2. Получение фьючерса
	var futureInstr *models.Instrument
	if pair.FutureUID != "" {
		futureInstr, err = u.apiService.GetOrFetchInstrumentByUID(pair.FutureUID)
	} else {
		futureInstr, err = u.apiService.GetOrFetchInstrumentByTicker(pair.FutureTicker, true)
	}
	if err != nil {
		log.Printf("[%s] Ошибка получения фьючерса: %v", pair.ID, err)
		return
	}
	if futureInstr == nil {
		log.Printf("[%s] Фьючерс не найден", pair.ID)
		return
	}

	// 3. Проверка полей
	if stockInstr.Figi == "" {
		log.Printf("[%s] FIGI акции пуст", pair.ID)
		return
	}
	if futureInstr.Figi == "" {
		log.Printf("[%s] FIGI фьючерса пуст", pair.ID)
		return
	}
	if futureInstr.ExpiryDate == nil {
		log.Printf("[%s] Дата экспирации nil", pair.ID)
		return
	}
	if futureInstr.Lot == 0 {
		log.Printf("[%s] Множитель равен 0", pair.ID)
		return
	}

	// 4. Получение цен с защитой от паники
	var stockPrice, futurePrice float64
	var source string
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[%s] ПАНИКА при вызове GetBestPrices: %v", pair.ID, r)
			}
		}()
		stockPrice, futurePrice, source, err = u.apiService.GetBestPrices(stockInstr.Figi, stockInstr.UID, futureInstr.Figi, futureInstr.UID)
	}()
	if err != nil {
		log.Printf("[%s] Ошибка получения цен: %v", pair.ID, err)
		// пробуем кэш
		stockPriceDB, _ := u.db.GetLastPrice(stockInstr.Figi)
		futurePriceDB, _ := u.db.GetLastPrice(futureInstr.Figi)
		if stockPriceDB == nil || futurePriceDB == nil {
			log.Printf("[%s] Нет кэшированных цен", pair.ID)
			return
		}
		stockPrice = stockPriceDB.Price
		futurePrice = futurePriceDB.Price
		log.Printf("[%s] Используем кэшированные цены (источник: %s)", pair.ID, source)
		u.processData(stockInstr, futureInstr, stockPrice, futurePrice)
		return
	}

	log.Printf("[%s] Получены цены из %s: акция %.2f, фьючерс %.2f", pair.ID, source, stockPrice, futurePrice)

	// Сохраняем цены в БД
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[%s] Паника при сохранении цены для %s: %v", pair.ID, stockInstr.Figi, r)
			}
		}()
		u.db.SaveLastPrice(&models.LastPrice{
			Figi:      stockInstr.Figi,
			Price:     stockPrice,
			PriceTime: time.Now(),
			UpdatedAt: time.Now(),
		})
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[%s] Паника при сохранении цены для %s: %v", pair.ID, futureInstr.Figi, r)
			}
		}()
		u.db.SaveLastPrice(&models.LastPrice{
			Figi:      futureInstr.Figi,
			Price:     futurePrice,
			PriceTime: time.Now(),
			UpdatedAt: time.Now(),
		})
	}()

	u.processData(stockInstr, futureInstr, stockPrice, futurePrice)
}

func (u *Updater) processData(stockInstr, futureInstr *models.Instrument, stockPrice, futurePrice float64) {
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
		LotFuture:           futureInstr.Lot,
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

	pairID := stockInstr.Ticker + "/" + futureInstr.Ticker
	data := map[string]interface{}{
		"PairID":              pairID,
		"PriceStock":          stockPrice,
		"PriceFuturePerShare": result.PriceFuturePerShare,
		"Spread":              result.Spread,
		"DividendNet":         result.DividendNet,
		"SellPrice":           result.SellPrice,
		"DaysToExpiry":        result.DaysToExpiry,
		"ReturnPct":           result.ReturnPct,
		"AnnualReturnPct":     result.AnnualReturnPct,
		"GOPerShare":          result.GOPerShare,
	}
	u.srv.UpdatePair(pairID, data)
}
