package main

import (
	"log"
	"os"

	"arbitrage-monitor/internal/api"
	"arbitrage-monitor/internal/calculator"
	"arbitrage-monitor/internal/config"
	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/internal/updater"
)

func main() {
	token := os.Getenv("TINKOFF_TOKEN")
	if token == "" {
		log.Fatal("Установите переменную окружения TINKOFF_TOKEN")
	}

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal("Ошибка загрузки конфига:", err)
	}

	log.Printf("Конфиг загружен: интервал %dс, комиссия %.2f%%",
		cfg.UpdateInterval, cfg.Commission*100)

	// Инициализация БД
	dbPath := "arbitrage.db"
	database, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatal("Ошибка инициализации БД:", err)
	}
	defer database.Close()

	log.Printf("База данных инициализирована: %s", dbPath)

	// Инициализация API клиента
	client, err := api.NewTinkoffClient(token)
	if err != nil {
		log.Fatal("Ошибка инициализации API клиента:", err)
	}
	defer client.Close()

	apiService := api.NewService(client, database)

	// Инициализация калькулятора
	calc := calculator.NewCalculator(cfg.Commission)

	// Для каждой пары запускаем обновление (в отдельных горутинах)
	for _, pair := range cfg.Pairs {
		log.Printf("Запуск мониторинга пары: %s (%s/%s)",
			pair.ID, pair.StockTicker, pair.FutureTicker)

		u := updater.NewUpdater(apiService, database, calc, cfg.UpdateInterval)

		// Запускаем в горутине
		go func(stock, future string) {
			u.Start(stock, future)
		}(pair.StockTicker, pair.FutureTicker)
	}

	// Бесконечное ожидание
	select {}
}
