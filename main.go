package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// HTTP接続をWebSocket接続にアップグレードするための設定
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 開発時はすべてのオリジンを許可（本番時は適切なドメインに制限）
		return true
	},
}

// 接続中のクライアントを管理する構造体（簡易版）
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Room struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.Mutex
}

func newRoom() *Room {
	return &Room{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (r *Room) run() {
	for {
		select {
		case client := <-r.register:
			r.mu.Lock()
			r.clients[client] = true
			r.mu.Unlock()
			log.Println("New client connected")
		case client := <-r.unregister:
			r.mu.Lock()
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				close(client.send)
				log.Println("Client disconnected")
			}
			r.mu.Unlock()
		case message := <-r.broadcast:
			r.mu.Lock()
			for client := range r.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(r.clients, client)
				}
			}
			r.mu.Unlock()
		}
	}
}

func serveWs(room *Room, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	client := &Client{conn: conn, send: make(chan []byte, 256)}
	room.register <- client

	// クライアントからの受信ループ
	go func() {
		defer func() {
			room.unregister <- client
			conn.Close()
		}()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			// 受信した操作入力などをルーム全体にブロードキャスト
			room.broadcast <- message
		}
	}()

	// クライアントへの送信ループ
	go func() {
		defer conn.Close()
		for message := range client.send {
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				break
			}
		}
	}()
}

func main() {
	room := newRoom()
	go room.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(room, w, r)
	})

	log.Println("MOBA WebSocket server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe error:", err)
	}
}
