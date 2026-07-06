package updater

import (
	"log"
	"time"

	"arbitrage-monitor/internal/api"
	"arbitrage-monitor/internal/calculator"
	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/internal/webserver"
	"arbitrage-monitor/pkg/models"

	"github.com/vodolaz095/go-investAPI/investapi"
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
	var prices []*investapi.LastPrice
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[%s] ПАНИКА при вызове GetLastPrices: %v", pair.ID, r)
			}
		}()
		prices, err = u.apiService.GetLastPrices([]string{stockInstr.Figi, futureInstr.Figi})
	}()
	if err != nil {
		log.Printf("[%s] Ошибка получения цен: %v", pair.ID, err)
		// пробуем кэш
		stockPrice, _ := u.db.GetLastPrice(stockInstr.Figi)
		futurePrice, _ := u.db.GetLastPrice(futureInstr.Figi)
		if stockPrice == nil || futurePrice == nil {
			log.Printf("[%s] Нет кэшированных цен", pair.ID)
			return
		}
		u.processData(stockInstr, futureInstr, stockPrice.Price, futurePrice.Price)
		return
	}

	// Проверка длины
	if len(prices) != 2 {
		log.Printf("[%s] Ожидалось 2 цены, получено %d", pair.ID, len(prices))
		return
	}

	var stockPrice, futurePrice float64
	foundStock, foundFuture := false, false
	for idx, p := range prices {
		// Защита от nil элемента
		if p == nil {
			log.Printf("[%s] Цена [%d] равна nil", pair.ID, idx)
			continue
		}
		// Защита от пустого FIGI
		if p.Figi == "" {
			log.Printf("[%s] Цена [%d] имеет пустой FIGI", pair.ID, idx)
			continue
		}
		price := float64(p.Price.Units) + float64(p.Price.Nano)/1e9
		// Сохраняем в БД с защитой
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[%s] Паника при сохранении цены для %s: %v", pair.ID, p.Figi, r)
				}
			}()
			u.db.SaveLastPrice(&models.LastPrice{
				Figi:      p.Figi,
				Price:     price,
				PriceTime: p.Time.AsTime(),
				UpdatedAt: time.Now(),
			})
		}()
		if p.Figi == stockInstr.Figi {
			stockPrice = price
			foundStock = true
		}
		if p.Figi == futureInstr.Figi {
			futurePrice = price
			foundFuture = true
		}
	}
	if !foundStock || !foundFuture {
		log.Printf("[%s] Не найдены цены (stock=%v, future=%v)", pair.ID, foundStock, foundFuture)
		return
	}

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
