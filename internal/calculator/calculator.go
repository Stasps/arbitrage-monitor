package calculator

import (
	"math"
	"time"
)

// Calculator - структура для расчётов
type Calculator struct {
	CommissionStock  float64
	CommissionFuture float64
}

// InputData - входные данные для расчёта
type InputData struct {
	PriceStock          float64
	PriceFuture         float64
	LotFuture           int
	Dividend            float64
	DividendPaymentDate *time.Time
	ExpiryDate          time.Time
	GO                  float64
}

// Result - результаты расчёта
type Result struct {
	PriceFuturePerShare float64
	Spread              float64
	DividendNet         float64
	SellPrice           float64
	DaysToExpiry        int
	GOPerShare          float64
	InvestedCapital     float64
	CommissionTotal     float64
	TradeProfit         float64
	ReturnPct           float64
	AnnualReturnPct     float64
}

// NewCalculator создаёт новый калькулятор
func NewCalculator(commissionStock, commissionFuture float64) *Calculator {
	return &Calculator{
		CommissionStock:  commissionStock,
		CommissionFuture: commissionFuture,
	}
}

// Calculate выполняет все расчёты
// Формула доходности: (sellPrice − priceStock − commissionTotal) / (priceStock − dividendNet + goPerShare)
// Дивиденд вычитается из инвестированного капитала, так как приходит на счёт
// через 1-2 недели после отсечки и может быть реинвестирован.
func (c *Calculator) Calculate(data InputData) Result {
	// 1. Цена 1 акции во фьючерсе
	priceFuturePerShare := data.PriceFuture / float64(data.LotFuture)

	// 2. Спред
	spread := priceFuturePerShare - data.PriceStock

	// 3. Дивиденд очищенный (только если дата выплаты <= даты экспирации)
	var dividendNet float64
	if data.Dividend > 0 && data.DividendPaymentDate != nil &&
		!data.DividendPaymentDate.After(data.ExpiryDate) {
		dividendNet = data.Dividend * 0.87 // налог 13%
	}

	// 4. Цена продажи
	sellPrice := priceFuturePerShare + dividendNet

	// 5. Дней до экспирации (календарные)
	now := time.Now()
	daysToExpiry := int(math.Ceil(data.ExpiryDate.Sub(now).Hours() / 24))
	if daysToExpiry < 0 {
		daysToExpiry = 0
	}

	// 6. ГО на 1 акцию
	goPerShare := data.GO / float64(data.LotFuture)

	// 7. Инвестированный капитал
	// Дивиденд вычитается, так как он приходит на счёт через 1-2 недели
	// и может быть реинвестирован. Для бездивидендных акций dividendNet = 0.
	investedCapital := data.PriceStock - dividendNet + goPerShare

	// 8. Комиссия (раздельная)
	commissionTotal := c.CommissionStock*data.PriceStock + c.CommissionFuture*priceFuturePerShare

	// 9. Прибыль
	tradeProfit := sellPrice - data.PriceStock - commissionTotal

	// 10. Доходность
	var returnPct float64
	if investedCapital > 0 {
		returnPct = tradeProfit / investedCapital
	}

	// 11. Годовая доходность
	var annualReturnPct float64
	if daysToExpiry > 0 && investedCapital > 0 {
		annualReturnPct = (tradeProfit / investedCapital) / float64(daysToExpiry) * 365
	}

	return Result{
		PriceFuturePerShare: priceFuturePerShare,
		Spread:              spread,
		DividendNet:         dividendNet,
		SellPrice:           sellPrice,
		DaysToExpiry:        daysToExpiry,
		GOPerShare:          goPerShare,
		InvestedCapital:     investedCapital,
		CommissionTotal:     commissionTotal,
		TradeProfit:         tradeProfit,
		ReturnPct:           returnPct,
		AnnualReturnPct:     annualReturnPct,
	}
}
