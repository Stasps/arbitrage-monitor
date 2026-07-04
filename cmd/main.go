package main

import (
	"log"

	"arbitrage-monitor/internal/config"
	"arbitrage-monitor/internal/db"
)

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal("Ошибка загрузки конфига:", err)
	}

	log.Printf("Конфиг загружен: интервал %dс, комиссия %.2f%%",
		cfg.UpdateInterval, cfg.Commission*100)
	log.Printf("Пар в обработке: %d", len(cfg.Pairs))
	for _, p := range cfg.Pairs {
		log.Printf("  - %s: %s / %s", p.ID, p.StockTicker, p.FutureTicker)
	}

	// Инициализация базы данных
	dbPath := "arbitrage.db"
	database, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatal("Ошибка инициализации БД:", err)
	}
	defer database.Close()

	log.Printf("База данных инициализирована: %s", dbPath)

	// TODO: следующий этап - API клиент
}
