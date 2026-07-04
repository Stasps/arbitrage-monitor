package main

import (
	"log"

	"arbitrage-monitor/internal/config"
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

	// TODO: следующий этап - база данных
}
