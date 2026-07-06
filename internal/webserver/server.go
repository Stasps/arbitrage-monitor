package webserver

import (
	"log"
	"net/http"
	"os"
	"sort"
	"sync"

	"github.com/gorilla/websocket"
)

// Server — веб-сервер с WebSocket и хранением данных
type Server struct {
	upgrader   websocket.Upgrader
	clients    map[*websocket.Conn]bool
	broadcast  chan interface{} // для отправки полного массива
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
	dataStore  map[string]interface{} // key = PairID, value = данные пары
}

// NewServer создаёт новый сервер
func NewServer() *Server {
	return &Server{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan interface{}, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		dataStore:  make(map[string]interface{}),
	}
}

// Run запускает HTTP-сервер
func (s *Server) Run(addr string) error {
	http.HandleFunc("/", s.serveHome)
	http.HandleFunc("/ws", s.handleWebSocket)

	go s.runLoop()

	log.Printf("Веб-сервер запущен на http://%s", addr)
	return http.ListenAndServe(addr, nil)
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
