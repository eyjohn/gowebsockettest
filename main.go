package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
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

func initJaeger(service string) (opentracing.Tracer, io.Closer) {
	cfg := &config.Configuration{
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &config.ReporterConfig{
			LogSpans: true,
		},
	}
	tracer, closer, err := cfg.New(service, config.Logger(jaeger.StdLogger))
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	return tracer, closer
}

func main() {
	portPtr := flag.Int("port", 8080, "port to listen for HTTP connections")
	flag.Parse()

	tracer, closer := initJaeger("gowebsockettest")
	opentracing.SetGlobalTracer(tracer)
	defer closer.Close()

	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/websocket", websocketHandler)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *portPtr), nethttp.Middleware(tracer, http.DefaultServeMux)))
}
