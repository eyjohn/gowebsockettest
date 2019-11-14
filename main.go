package main

import (
	// "fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
	"math/rand"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func websocketPing(conn *websocket.Conn) {
	for {
		timestamp := time.Now().UTC().Format(time.RFC3339Nano)
		err := conn.WriteMessage(websocket.TextMessage, []byte(timestamp))
		if err != nil {
			log.Println(err)
			return
		}
		time.Sleep(time.Duration(rand.Intn(int(time.Second * 3))))
	}
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	go websocketPing(conn)
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	content, _ := ioutil.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html")
	w.Write(content)
}

func main() {
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/websocket", websocketHandler)
	http.HandleFunc("/", defaultHandler)
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}
