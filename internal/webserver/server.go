package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"arbitrage-monitor/internal/api"
	"arbitrage-monitor/internal/config"
	"arbitrage-monitor/pkg/models"

	"github.com/gorilla/websocket"
	"github.com/vodolaz095/go-investAPI/investapi"
)

// Server — веб-сервер с WebSocket и хранением данных
type Server struct {
	upgrader         websocket.Upgrader
	clients          map[*websocket.Conn]bool
	broadcast        chan interface{} // для отправки полного массива
	register         chan *websocket.Conn
	unregister       chan *websocket.Conn
	mu               sync.RWMutex
	dataStore        map[string]interface{} // key = PairID, value = данные пары
	marginMultiplier float64
	apiClient        *api.TinkoffClient // клиент для запросов к API Т-Инвестиций
	addPairChan      chan models.Pair   // канал для передачи новых пар в main.go
	configMu         sync.Mutex         // защищает чтение/запись config.yaml
}

// NewServer создаёт новый сервер
// Принимает:
// - marginMultiplier: множитель ГО для отображения уровня риска на сайте
// - apiClient: клиент для запросов к API Т-Инвестиций (поиск фьючерсов, добавление пар)
// - addPairChan: канал для передачи новых пар в main.go (запуск мониторинга "на лету")
//
// Возвращает: *Server
func NewServer(marginMultiplier float64, apiClient *api.TinkoffClient, addPairChan chan models.Pair) *Server {
	return &Server{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients:          make(map[*websocket.Conn]bool),
		broadcast:        make(chan interface{}, 100),
		register:         make(chan *websocket.Conn),
		unregister:       make(chan *websocket.Conn),
		dataStore:        make(map[string]interface{}),
		marginMultiplier: marginMultiplier,
		apiClient:        apiClient,
		addPairChan:      addPairChan,
	}
}

// Run запускает HTTP-сервер
func (s *Server) Run(addr string) error {
	http.HandleFunc("/", s.serveHome)
	http.HandleFunc("/ws", s.handleWebSocket)
	http.HandleFunc("/api/config", s.serveConfig)
	http.HandleFunc("/search", s.serveSearchPage)
	http.HandleFunc("/api/search", s.handleAPISearch)
	http.HandleFunc("/api/add-pair", s.handleAPIAddPair)

	go s.runLoop()

	log.Printf("Веб-сервер запущен на http://%s", addr)
	return http.ListenAndServe(addr, nil)
}

// serveConfig отдаёт JSON с настройками, которые нужны клиенту для отображения
// Возвращает: {"margin_multiplier": 0.5, "risk_level": "КПУР"}
func (s *Server) serveConfig(w http.ResponseWriter, r *http.Request) {
	riskLevel := "КСУР"
	if s.marginMultiplier < 1.0 {
		riskLevel = "КПУР"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, `{"margin_multiplier": %.2f, "risk_level": "%s"}`, s.marginMultiplier, riskLevel)
}

// serveHome отдаёт HTML-страницу из файла
func (s *Server) serveHome(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "Файл web/index.html не найден", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// serveSearchPage отдаёт страницу поиска фьючерсов из файла web/search.html
func (s *Server) serveSearchPage(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("web/search.html")
	if err != nil {
		http.Error(w, "Файл web/search.html не найден", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// searchResultItem — элемент результата поиска фьючерсов для отдачи на фронтенд
type searchResultItem struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	UID    string `json:"uid"`
}

// handleAPISearch ищет фьючерсы по строке запроса через FindInstrument.
// Принимает: GET /api/search?q=<запрос>
//
// Возвращает: JSON-массив найденных фьючерсов [{"ticker":..., "name":..., "uid":...}].
func (s *Server) handleAPISearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "Пустой поисковый запрос", http.StatusBadRequest)
		return
	}

	kind := investapi.InstrumentType_INSTRUMENT_TYPE_FUTURES
	resp, err := s.apiClient.GetInstrumentsServiceClient().FindInstrument(context.Background(),
		&investapi.FindInstrumentRequest{
			Query:          query,
			InstrumentKind: &kind,
		})
	if err != nil {
		log.Printf("Ошибка поиска фьючерсов по запросу '%s': %v", query, err)
		http.Error(w, "Ошибка запроса к API: "+err.Error(), http.StatusInternalServerError)
		return
	}

	results := make([]searchResultItem, 0, len(resp.Instruments))
	for _, inst := range resp.Instruments {
		results = append(results, searchResultItem{
			Ticker: inst.Ticker,
			Name:   inst.Name,
			UID:    inst.Uid,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(results)
}

// addPairRequest — тело запроса на добавление новой пары
type addPairRequest struct {
	FutureUID    string `json:"future_uid"`
	FutureTicker string `json:"future_ticker"`
}

// handleAPIAddPair создаёт новую пару (акция + фьючерс), сохраняет её в config.yaml
// и отправляет в канал addPairChan для запуска мониторинга "на лету".
// Принимает: POST /api/add-pair с JSON {"future_uid": "...", "future_ticker": "..."}
//
// Возвращает: JSON {"status": "ok", "pair": {...}} или ошибку.
func (s *Server) handleAPIAddPair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var req addPairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Некорректный JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.FutureUID == "" {
		http.Error(w, "Не указан future_uid", http.StatusBadRequest)
		return
	}

	// Получаем информацию о фьючерсе по UID
	future, err := s.apiClient.GetFutureInfoByUID(req.FutureUID)
	if err != nil {
		log.Printf("Ошибка получения фьючерса %s: %v", req.FutureUID, err)
		http.Error(w, "Не удалось получить информацию о фьючерсе: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if future.BasicAsset == "" {
		http.Error(w, "У фьючерса не указан базовый актив", http.StatusBadRequest)
		return
	}

	// Ищем базовую акцию по названию базового актива фьючерса
	kind := investapi.InstrumentType_INSTRUMENT_TYPE_SHARE
	shareResp, err := s.apiClient.GetInstrumentsServiceClient().FindInstrument(context.Background(),
		&investapi.FindInstrumentRequest{
			Query:          future.BasicAsset,
			InstrumentKind: &kind,
		})
	if err != nil {
		log.Printf("Ошибка поиска базовой акции '%s': %v", future.BasicAsset, err)
		http.Error(w, "Ошибка поиска базовой акции: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(shareResp.Instruments) == 0 {
		http.Error(w, "Базовая акция не найдена: "+future.BasicAsset, http.StatusNotFound)
		return
	}
	stockTicker := shareResp.Instruments[0].Ticker

	// Формируем человекочитаемое имя: тикер акции + месяц.год экспирации
	expiry := future.ExpirationDate.AsTime()
	name := fmt.Sprintf("%s %02d.%02d", stockTicker, int(expiry.Month()), expiry.Year()%100)

	pair := models.Pair{
		ID:           strings.ToLower(stockTicker),
		Name:         name,
		StockTicker:  stockTicker,
		FutureTicker: req.FutureTicker,
		FutureUID:    req.FutureUID,
	}

	// Добавляем или обновляем пару в конфиге (под мьютексом — файл один)
	s.configMu.Lock()
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		s.configMu.Unlock()
		http.Error(w, "Ошибка загрузки конфига: "+err.Error(), http.StatusInternalServerError)
		return
	}

	isNew := true
	for i, p := range cfg.Pairs {
		if p.FutureUID == pair.FutureUID || p.ID == pair.ID {
			cfg.Pairs[i] = pair
			isNew = false
			break
		}
	}
	if isNew {
		cfg.Pairs = append(cfg.Pairs, pair)
	}

	if err := config.SaveConfig("config.yaml", cfg); err != nil {
		s.configMu.Unlock()
		http.Error(w, "Ошибка сохранения конфига: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.configMu.Unlock()

	// Для новой пары запускаем мониторинг "на лету" через канал в main.go
	if isNew {
		s.addPairChan <- pair
	}

	log.Printf("Пара %s (%s/%s) %s", pair.ID, pair.StockTicker, pair.FutureTicker,
		map[bool]string{true: "добавлена", false: "обновлена"}[isNew])

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"is_new": isNew,
		"pair":   pair,
	})
}

// handleWebSocket обрабатывает подключения
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Ошибка WebSocket: %v", err)
		return
	}
	s.register <- conn
}

// UpdatePair обновляет данные по паре и отправляет всем клиентам полный массив
func (s *Server) UpdatePair(pairID string, data interface{}) {
	log.Printf("Обновление данных для пары: %s", pairID)
	s.mu.Lock()
	s.dataStore[pairID] = data
	s.mu.Unlock()
	s.broadcastAll()
}

// broadcastAll собирает все данные в массив и отправляет клиентам
func (s *Server) broadcastAll() {
	s.mu.RLock()
	// Собираем ключи и сортируем
	keys := make([]string, 0, len(s.dataStore))
	for k := range s.dataStore {
		keys = append(keys, k)
	}
	s.mu.RUnlock()

	// Сортируем ключи
	sort.Strings(keys)

	// Формируем массив данных в отсортированном порядке
	s.mu.RLock()
	allData := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		allData = append(allData, s.dataStore[k])
	}
	s.mu.RUnlock()

	if len(allData) == 0 {
		return
	}

	select {
	case s.broadcast <- allData:
	default:
		log.Println("Broadcast канал переполнен, данные пропущены")
	}
}

// runLoop управляет клиентами и рассылкой
func (s *Server) runLoop() {
	for {
		select {
		case conn := <-s.register:
			s.mu.Lock()
			s.clients[conn] = true
			s.mu.Unlock()
			log.Printf("Клиент подключён: %s", conn.RemoteAddr())
			// Отправляем текущие данные новому клиенту
			s.broadcastAll()

		case conn := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[conn]; ok {
				delete(s.clients, conn)
				conn.Close()
			}
			s.mu.Unlock()
			log.Printf("Клиент отключён: %s", conn.RemoteAddr())

		case msg := <-s.broadcast:
			s.mu.Lock()
			for conn := range s.clients {
				err := conn.WriteJSON(msg)
				if err != nil {
					log.Printf("Ошибка отправки: %v", err)
					conn.Close()
					delete(s.clients, conn)
				}
			}
			s.mu.Unlock()
		}
	}
}
