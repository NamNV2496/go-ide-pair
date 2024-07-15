package socket

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var (
	clients   = make(map[*websocket.Conn]string)
	broadcast = make(chan Message)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type Message struct {
	Text     string `json:"text"`
	User     string `json:"user"`
	Position int    `json:"position"`
}

func HandleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// Get username from query parameters
	username := r.URL.Query().Get("username")
	if username == "" {
		log.Println("No username provided")
		return
	}

	fmt.Println("add new client: ", username)
	clients[ws] = username

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			delete(clients, ws)
			break
		}
		broadcast <- msg
	}
}

func HandleMessages() {
	for {
		msg := <-broadcast
		for client, username := range clients {
			if msg.Text == "stop" {
				fmt.Println("Delete client: ", msg.User)
				client.Close()
				delete(clients, client)
				return
			}
			if username == msg.User {
				continue
			}
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
