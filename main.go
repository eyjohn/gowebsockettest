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
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
)

func populateCommonSpan(span opentracing.Span) {
	span.SetTag("component", "main")
	span.SetTag("span.kind", "server")

}

func populateHttpSpan(span opentracing.Span, req *http.Request) {
	populateCommonSpan(span)
	span.SetTag("http.method", req.Method)
	span.SetTag("http.url", req.URL.String())
	span.SetTag("peer.address", req.RemoteAddr)
}

func populateWebsocketSpan(span opentracing.Span, conn *websocket.Conn) {
	populateCommonSpan(span)
	span.SetTag("peer.address", conn.RemoteAddr())
}

func defaultHandler(w http.ResponseWriter, req *http.Request) {
	span := opentracing.StartSpan("index")
	populateHttpSpan(span, req)
	defer span.Finish()
	content, _ := ioutil.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html")
	w.Write(content)
}

func healthzHandler(w http.ResponseWriter, req *http.Request) {
	span := opentracing.StartSpan("healthz")
	populateHttpSpan(span, req)
	defer span.Finish()
	w.Write([]byte("OK"))
}

func websocketEcho(conn *websocket.Conn) {
	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read Error:", err)
			break
		}
		span := opentracing.StartSpan("message")
		populateWebsocketSpan(span, conn)
		span.SetTag("message", string(message))
		log.Printf("Received Message: %s from %v", message, conn.RemoteAddr())
		err = conn.WriteMessage(mt, message)
		if err != nil {
			log.Println("Write Error:", err)
			span.LogKV(
				"event", "error",
				"message", err)
			span.Finish()
			break
		}
		span.Finish()
	}
	conn.Close()
}

func websocketRandPing(conn *websocket.Conn) {
	for {
		span := opentracing.StartSpan("ping")
		populateWebsocketSpan(span, conn)
		err := conn.WriteMessage(websocket.TextMessage, []byte("randping"))
		if err != nil {
			log.Println(err)
			return
		}
		span.Finish()
		time.Sleep(time.Duration(rand.Intn(int(time.Second * 3))))
	}
}

// The upgrader which will perform the HTTP connection upgrade to WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func websocketHandler(w http.ResponseWriter, req *http.Request) {
	span := opentracing.StartSpan("websocket")
	populateHttpSpan(span, req)
	defer span.Finish()

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Println(err)
		span.SetTag("error", true)
		span.LogKV(
			"event", "error",
			"message", err)
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

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *portPtr), nil))
}
