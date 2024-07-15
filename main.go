package main

import (
	"log"
	"net/http"

	"github.com/namnv2496/go-ide-pair/api"
	"github.com/namnv2496/go-ide-pair/internal/executor/socket"
	python3_job_executor "github.com/namnv2496/go-ide-pair/internal/executor/worker/python3_worker"
)

func main() {
	go func() {
		// Start WebSocket server
		http.HandleFunc("/ws", socket.HandleConnections)
		log.Println("WebSocket server started on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatal("Error starting WebSocket server:", err)
		}
	}()
	go socket.HandleMessages()
	python3_job_executor.GetInstance()
	log.Println("http server started on :8080")
	api.NewServer()
}
