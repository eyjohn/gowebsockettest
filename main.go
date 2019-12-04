package main

import (
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	content, _ := ioutil.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html")
	w.Write(content)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func websocketEcho(conn *websocket.Conn) {
	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read Error:", err)
			break
		}
		log.Printf("Received Message: %s from %v", message, conn.RemoteAddr())
		err = conn.WriteMessage(mt, message)
		if err != nil {
			log.Println("Write Error:", err)
			break
		}
	}
	conn.Close()
}

func websocketRandPing(conn *websocket.Conn) {
	for {
		err := conn.WriteMessage(websocket.TextMessage, []byte("randping"))
		if err != nil {
			log.Println(err)
			return
		}
		time.Sleep(time.Duration(rand.Intn(int(time.Second * 3))))
	}
}

// The upgrader which will perform the HTTP connection upgrade to WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("Connection from %v", conn.RemoteAddr())
	go websocketEcho(conn)
	go websocketRandPing(conn)
}

func main() {
	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/websocket", websocketHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
