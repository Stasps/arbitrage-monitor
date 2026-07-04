# Changelog
## [0.1.3] - 2026-07-04

### Русский
**Добавлено:**
- Расчётный модуль со всеми формулами (спред, доходность, годовая)
- Цикл обновления цен с интервалом 1 секунда
- Кэширование цен в БД при недоступности API
- Получение гарантийного обеспечения (ГО) через API
- Поддержка нескольких пар в параллельных горутинах
- Автоматическая конвертация MoneyValue в float64

**Технические детали:**
- Расчёт дней до экспирации по календарю
- Учёт дивидендов с налогом 13%
- Комиссия 0.04% из конфига
- При недоступности API используются кэшированные данные

---

### English
**Added:**
- Calculation module with all formulas (spread, return, annual)
- Update loop with 1 second interval
- Price caching in DB when API is unavailable
- Guarantee obligation (GO) retrieval via API
- Support for multiple pairs in parallel goroutines
- Automatic MoneyValue to float64 conversion

**Technical details:**
- Calendar days to expiry calculation
- Dividend accounting with 13% tax
- 0.04% commission from config
- Cached data used when API is unavailable

## [0.1.2] - 2026-07-04

### Русский
**Добавлено:**
- Клиент для работы с API Т-Инвестиций
- Методы получения информации об акциях и фьючерсах
- Получение дивидендов и последних цен
- Сервисный слой с автоматическим кэшированием в БД
- Обработка ошибок при работе с API

**Технические детали:**
- Используется библиотека `github.com/vodolaz095/go-investAPI`
- Поддержка gRPC через `google.golang.org/protobuf`
- При первом запросе данные сохраняются в БД
- При повторных запросах данные берутся из кэша

---

### English
**Added:**
- Tinkoff API client
- Methods for getting share and future information
- Dividend and last price retrieval
- Service layer with automatic DB caching
- API error handling

**Technical details:**
- Uses `github.com/vodolaz095/go-investAPI` library
- gRPC support via `google.golang.org/protobuf`
- Data saved to DB on first request
- Subsequent requests use cached data

## [0.1.1] - 2026-07-04

### Русский
**Добавлено:**
- Слой базы данных SQLite с функциями CRUD
- Таблицы для хранения инструментов, дивидендов и последних цен
- Автоматические миграции при инициализации
- Интеграция базы данных в main.go

**Технические детали:**
- Используется `modernc.org/sqlite` (чистый Go, без CGO)
- Таблица instruments: кэш данных по инструментам
- Таблица dividends: кэш дивидендов
- Таблица last_prices: кэш последних цен (для работы при недоступности API)

---

### English
**Added:**
- SQLite database layer with CRUD functions
- Tables for instruments, dividends, and last prices
- Automatic migrations on initialization
- Database integration in main.go

**Technical details:**
- Uses `modernc.org/sqlite` (pure Go, no CGO)
- instruments table: instrument metadata cache
- dividends table: dividend cache
- last_prices table: last price cache (for API offline mode)

## [0.1.0] - 2026-07-04

### Русский
**Добавлено:**
- Структура проекта с модульной архитектурой
- Управление конфигурацией (YAML)
- Модели данных для инструментов, дивидендов, цен и пар
- Базовый main.go с загрузкой и логированием конфига
- Пример конфигурационного файла

**Технические детали:**
- Инициализированы Go модули
- Парсинг YAML через `gopkg.in/yaml.v3`
- Поддержка множества пар акция-фьючерс

**Следующие шаги:**
- Слой базы данных SQLite
- Интеграция с API Т-Инвестиций
- Кэширование цен и цикл обновления

---

### English
**Added:**
- Project structure with modular architecture
- Configuration management (YAML)
- Data models for instruments, dividends, prices, and pairs
- Basic main.go with config loading and validation
- Example configuration file

**Technical details:**
- Go modules initialized
- YAML parsing via `gopkg.in/yaml.v3`
- Support for multiple stock-future pairs

**Next steps:**
- SQLite database layer
- Tinkoff API integration
- Price caching and update loop