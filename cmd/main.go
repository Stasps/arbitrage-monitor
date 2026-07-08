package main

import (
	"log"
	"os"
	"strings"
	"time"

	"arbitrage-monitor/internal/api"
	"arbitrage-monitor/internal/calculator"
	"arbitrage-monitor/internal/config"
	"arbitrage-monitor/internal/db"
	"arbitrage-monitor/internal/updater"
	"arbitrage-monitor/internal/webserver"
	"arbitrage-monitor/pkg/models"
)

// ========== КОНСТАНТЫ ==========
const (
	// Путь к файлу с токеном (относительно корня проекта)
	TokenFile = "token"
)

// ========== ОСНОВНАЯ ФУНКЦИЯ ==========

func main() {
	// Читаем токен из файла
	token, err := readToken(TokenFile)
	if err != nil {
		log.Printf("Ошибка чтения токена из файла %s: %v", TokenFile, err)
		log.Println("Пробуем использовать переменную окружения TINKOFF_TOKEN")
		token = os.Getenv("TINKOFF_TOKEN")
	}

	if token == "" {
		log.Fatal("Токен не найден. Создайте файл 'token' в корне проекта или установите переменную окружения TINKOFF_TOKEN")
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

	apiService := api.NewService(client, database)
	calc := calculator.NewCalculator(cfg.Commission)

	// === ЗАПУСК ВЕБ-СЕРВЕРА ===
	srv := webserver.NewServer()
	go func() {
		if err := srv.Run(":8080"); err != nil {
			log.Printf("Ошибка веб-сервера: %v", err)
		}
	}()

	// =====================================================
	// ИНИЦИАЛИЗАЦИЯ ФЬЮЧЕРСОВ (получение FIGI через UID)
	// =====================================================
	log.Println("Инициализация фьючерсов (получение FIGI через UID)...")
	for _, pair := range cfg.Pairs {
		if pair.FutureUID != "" {
			instr, err := apiService.GetOrFetchInstrumentByUID(pair.FutureUID)
			if err != nil {
				log.Printf("Ошибка инициализации фьючерса %s (UID: %s): %v", pair.FutureTicker, pair.FutureUID, err)
			} else {
				log.Printf("Фьючерс %s инициализирован (FIGI: %s)", instr.Ticker, instr.Figi)
			}
		}
	}
	log.Println("Инициализация фьючерсов завершена")

	// Загрузка дивидендов (как было)
	log.Println("Загрузка дивидендов...")
	for _, pair := range cfg.Pairs {
		stockInstr, err := apiService.GetOrFetchInstrumentByTicker(pair.StockTicker, false)
		if err != nil {
			log.Printf("Ошибка получения FIGI для %s: %v", pair.StockTicker, err)
			continue
		}

		divs, err := client.GetDividends(
			stockInstr.Figi,
			time.Now().AddDate(-1, 0, 0),
			time.Now().AddDate(1, 0, 0),
		)
		if err != nil {
			log.Printf("Ошибка получения дивидендов для %s: %v", pair.StockTicker, err)
			continue
		}

		for _, d := range divs {
			div := &models.Dividend{
				Ticker:      pair.StockTicker,
				Dividend:    float64(d.DividendNet.Units) + float64(d.DividendNet.Nano)/1e9,
				ExDate:      d.LastBuyDate.AsTime(),
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

	// Запускаем updater для каждой пары
	for _, pair := range cfg.Pairs {
		log.Printf("Запуск пары: %s (%s/%s)", pair.ID, pair.StockTicker, pair.FutureTicker)
		u := updater.NewUpdater(apiService, database, calc, cfg.UpdateInterval, srv) // <-- добавлен srv
		go u.Start(pair)
	}

	// Бесконечное ожидание
	select {}
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

// readToken читает токен из файла
func readToken(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
