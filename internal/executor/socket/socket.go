package socket

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// ClientInfo holds metadata for a connected WebSocket client.
type ClientInfo struct {
	username string
	roomID   string
}

// Message is the envelope for all WebSocket messages.
//
// Type values:
//   - "delta"        — an Ace editor delta (payload = JSON-encoded delta object)
//   - "full_sync"    — full document content sent to a new joiner (payload = document text)
//   - "request_sync" — sent by a new joiner to ask existing clients for full_sync
//   - "stop"         — client is disconnecting
type Message struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
	User    string `json:"user"`
	RoomID  string `json:"roomId"`
}

var (
	clients   = make(map[*websocket.Conn]*ClientInfo)
	clientsMu sync.RWMutex
	broadcast = make(chan Message, 256)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func HandleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer ws.Close()

	username := r.URL.Query().Get("username")
	roomID := r.URL.Query().Get("room")
	if username == "" || roomID == "" {
		log.Println("Rejected connection: missing username or room query param")
		return
	}

	log.Printf("Connected: %s → room %s", username, roomID)
	clientsMu.Lock()
	clients[ws] = &ClientInfo{username: username, roomID: roomID}
	clientsMu.Unlock()

	for {
		var msg Message
		if err := ws.ReadJSON(&msg); err != nil {
			log.Printf("Disconnected: %s (%v)", username, err)
			clientsMu.Lock()
			delete(clients, ws)
			clientsMu.Unlock()
			break
		}
		// Overwrite user/room from the authenticated query params — never trust the client fields.
		msg.User = username
		msg.RoomID = roomID
		broadcast <- msg
	}
}

func HandleMessages() {
	for msg := range broadcast {
		if msg.Type == "stop" {
			clientsMu.Lock()
			for conn, info := range clients {
				if info.username == msg.User && info.roomID == msg.RoomID {
					log.Printf("Disconnecting %s from room %s", msg.User, msg.RoomID)
					conn.Close()
					delete(clients, conn)
					break
				}
			}
			clientsMu.Unlock()
			continue
		}

		// Snapshot everyone in the same room except the sender, then write
		// outside the lock so a slow write doesn't block other goroutines.
		clientsMu.RLock()
		targets := make(map[*websocket.Conn]*ClientInfo)
		for conn, info := range clients {
			if info.roomID == msg.RoomID && info.username != msg.User {
				targets[conn] = info
			}
		}
		clientsMu.RUnlock()

		for conn, info := range targets {
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("Write error to %s: %v", info.username, err)
				clientsMu.Lock()
				conn.Close()
				delete(clients, conn)
				clientsMu.Unlock()
			}
		}
	}
}
