package main

import (
	"log"
	"os"
	"time"

	"arbitrage-monitor/internal/api"
	"arbitrage-monitor/internal/calculator"
	"arbitrage-monitor/internal/config"
	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/internal/updater"
	"arbitrage-monitor/pkg/models"
)

func main() {
	token := os.Getenv("TINKOFF_TOKEN")
	if token == "" {
		log.Fatal("Установите TINKOFF_TOKEN")
	}

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal("Ошибка загрузки конфига:", err)
	}

	log.Printf("Конфиг загружен: интервал %dс, комиссия %.2f%%",
		cfg.UpdateInterval, cfg.Commission*100)

	// БД
	database, err := db.NewDB("arbitrage.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// API клиент
	client, err := api.NewTinkoffClient(token)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// === ЗАГРУЗКА ДИВИДЕНДОВ ===
	log.Println("Загрузка дивидендов...")
	for _, pair := range cfg.Pairs {
		// Запрашиваем дивиденды за последний год и следующий
		divs, err := client.GetDividends(
			pair.StockFigi,
			time.Now().AddDate(-1, 0, 0),
			time.Now().AddDate(1, 0, 0),
		)
		if err != nil {
			log.Printf("Ошибка получения дивидендов для %s: %v", pair.StockTicker, err)
			continue
		}

		for _, d := range divs {
			// Правильные поля: DividendNet, PaymentDate, DeclaredDate
			div := &models.Dividend{
				Ticker:      pair.StockTicker,
				Dividend:    float64(d.DividendNet.Units) + float64(d.DividendNet.Nano)/1e9,
				ExDate:      d.DeclaredDate.AsTime(), // используем DeclaredDate как дату отсечки
				PaymentDate: d.PaymentDate.AsTime(),
				UpdatedAt:   time.Now(),
			}
			if err := database.SaveDividend(div); err != nil {
				log.Printf("Ошибка сохранения дивиденда: %v", err)
			} else {
				log.Printf("Дивиденд для %s: %.2f руб., выплата %s",
					pair.StockTicker,
					div.Dividend,
					div.PaymentDate.Format("2006-01-02"),
				)
			}
		}
	}
	log.Println("Загрузка дивидендов завершена")

	apiService := api.NewService(client, database)
	calc := calculator.NewCalculator(cfg.Commission)

	// Запускаем updater для каждой пары
	for _, pair := range cfg.Pairs {
		log.Printf("Запуск пары: %s (%s/%s)", pair.ID, pair.StockTicker, pair.FutureTicker)
		u := updater.NewUpdater(apiService, database, calc, cfg.UpdateInterval)
		go u.Start(pair)
	}

	// Бесконечное ожидание
	select {}
}
