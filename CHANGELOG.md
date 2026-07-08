# 
# Changelog
## [0.3.2] - 2026-07-08

**Добавлено**:
- В утилите `finder` теперь выводится FIGI и текущая цена для акций (при наличии).
- Улучшена диагностика: при поиске акций сразу видно, по какому FIGI есть цена.

**Технические детали**:
- `finder.go` теперь содержит функцию `getPrice` и вывод FIGI для акций.

**Известные ограничения**:
- Для новых акций FIGI по-прежнему нужно определять вручную через `finder` и обновлять в БД.

## [0.3.1] - 2026-07-07

**Добавлено**:
- Веб-интерфейс с таблицей, обновляемой в реальном времени через WebSocket.
- HTTP-сервер на порту `:8080` (настраивается позже).
- Отображение всех пар в виде таблицы с цветовой индикацией спреда и доходности.

**Исправлено**:
- Исправлен FIGI для акции X5 (был `TCS05A108X38`, заменён на `TCS03A108X38`), что устранило панику в цикле обновления.
- Добавлена защита от паники в `updater.go` при обработке `nil` цен и пустых FIGI.
- Исправлена проблема с отображением только одной пары на сайте (теперь хранятся все данные в `dataStore`).

**Технические детали**:
- Веб-сервер использует `gorilla/websocket`.
- Данные отправляются клиентам в JSON-массиве, отсортированном по `PairID`.
- HTML-шаблон вынесен в отдельный файл `web/index.html` для удобства правок.

**Известные ограничения**:
- При отсутствии торгов (после закрытия биржи) отображается последняя цена закрытия.
- Порт сервера пока жёстко задан (8080), в будущем будет вынесен в конфиг.

## [0.3.0] - 2026-07-06

**Добавлено**:
- Полная поддержка UID для фьючерсов (FIGI получается через `GetInstrumentBy`).
- Утилита `update_futures` для автоматического обновления множителей (basic_asset_size) и ГО.
- Защита от паники в циклах обновления.
- Логирование таймаутов API и блокировок БД.

**Исправлено**:
- Ошибка 30013 при поиске фьючерсов по тикеру (переход на UID).
- Ошибка `database locked` в утилите обновления (сбор данных в слайс).
- Тип колонки `lot` в БД приведён к INTEGER.
- Ошибка сканирования пустых строк в `lot`.

**Известные ограничения**:
- Интерфейс пока консольный (логи). TUI или веб-интерфейс — в следующей версии.
- Множители и ГО обновляются только через ручной запуск `update_futures.exe`.

## [0.2.0] - 2026-07-04

### Русский

**Добавлено:**
- Расчётный модуль со всеми формулами (спред, доходность, годовая доходность)
- Цикл обновления цен с интервалом 1 секунда
- Кэширование цен в БД при недоступности API
- Получение гарантийного обеспечения (ГО) через API
- Загрузка дивидендов при старте приложения
- Поддержка нескольких пар в параллельных горутинах
- Использование FIGI для стабильной идентификации инструментов
- Поле `future_lot` в конфиге для пересчёта цены фьючерса в цену 1 акции
- BAT-файл для запуска с переменной окружения

**Исправлено:**
- Ошибка сканирования `time.Time` в SQLite
- Ошибка 30013 (инструмент не найден) через использование FIGI
- Корректный пересчёт цены фьючерса с учётом лотности

**Технические детали:**
- FIGI: SBER = BBG004730N88, SRU6 = FUTSBRF09260
- Для Сбера 1 фьючерсный контракт = 100 акций (future_lot: 100)
- Автоматическое сохранение данных инструментов при первом запросе
- При недоступности API используются кэшированные данные из БД

---

### English

**Added:**
- Calculation module with all formulas (spread, return, annual return)
- Price update loop with 1 second interval
- Price caching in DB when API is unavailable
- Guarantee obligation (GO) retrieval via API
- Dividend loading on application startup
- Support for multiple pairs in parallel goroutines
- FIGI usage for stable instrument identification
- `future_lot` field in config for futures price recalculation
- BAT file for running with environment variable

**Fixed:**
- `time.Time` scanning error in SQLite
- Error 30013 (instrument not found) by using FIGI
- Correct futures price recalculation considering lot size

**Technical details:**
- FIGI: SBER = BBG004730N88, SRU6 = FUTSBRF09260
- For Sber: 1 futures contract = 100 shares (future_lot: 100)
- Automatic instrument data saving on first request
- Cached data from DB used when API is unavailable

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