package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/label"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var moduleTracer trace.Tracer

func populateCommonSpan(span trace.Span) {
	span.SetAttributes(
		label.String("component", "main"),
		label.String("span.kind", "server"),
	)

}

func populateHttpSpan(span trace.Span, req *http.Request) {
	populateCommonSpan(span)
	span.SetAttributes(
		label.String("http.method", req.Method),
		label.String("http.url", req.URL.String()),
		label.String("peer.address", req.RemoteAddr),
	)
}

func populateWebsocketSpan(span trace.Span, conn *websocket.Conn) {
	populateCommonSpan(span)
	span.SetAttributes(
		label.String("peer.address", conn.RemoteAddr().String()),
	)
}

func defaultHandler(w http.ResponseWriter, req *http.Request) {
	_, span := moduleTracer.Start(req.Context(), "index")
	populateHttpSpan(span, req)
	defer span.End()
	content, _ := ioutil.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html")
	w.Write(content)
}

func healthzHandler(w http.ResponseWriter, req *http.Request) {
	_, span := moduleTracer.Start(req.Context(), "healthz")
	populateHttpSpan(span, req)
	defer span.End()
	w.Write([]byte("OK"))
}

func websocketEcho(conn *websocket.Conn) {
	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read Error:", err)
			break
		}
		_, span := moduleTracer.Start(context.Background(), "message")
		populateWebsocketSpan(span, conn)
		span.SetAttribute("message", string(message))
		log.Printf("Received Message: %s from %v", message, conn.RemoteAddr())
		err = conn.WriteMessage(mt, message)
		if err != nil {
			log.Println("Write Error:", err)
			span.AddEvent(context.Background(),
				"error", label.String("message", err.Error()))
			span.End()
			span.SetAttribute("error", true)
			break
		}
		span.End()
	}
	conn.Close()
}

func websocketRandPing(conn *websocket.Conn) {
	for {
		_, span := moduleTracer.Start(context.Background(), "ping")
		populateWebsocketSpan(span, conn)
		err := conn.WriteMessage(websocket.TextMessage, []byte("randping"))
		if err != nil {
			log.Println(err)
			return
		}
		span.End()
		time.Sleep(time.Duration(rand.Intn(int(time.Second * 3))))
	}
}

// The upgrader which will perform the HTTP connection upgrade to WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func websocketHandler(w http.ResponseWriter, req *http.Request) {
	_, span := moduleTracer.Start(context.Background(), "websocket")
	populateHttpSpan(span, req)
	defer span.End()

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Println(err)
		span.SetAttribute("error", true)
		span.AddEvent(context.Background(), "error",
			label.String("message", err.Error()))
		return
	}
	log.Printf("Connection from %v", conn.RemoteAddr())
	go websocketEcho(conn)
	go websocketRandPing(conn)
}

func initTracer(service string) func() {
	// Create and install Jaeger export pipeline
	flush, err := jaeger.InstallNewPipeline(
		jaeger.WithAgentEndpoint("localhost:6831"),
		jaeger.WithProcess(jaeger.Process{
			ServiceName: service,
		}),
		jaeger.WithSDK(&sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
	)
	if err != nil {
		log.Fatal(err)
	}
	return func() {
		flush()
	}
}

func main() {
	portPtr := flag.Int("port", 8080, "port to listen for HTTP connections")
	flag.Parse()

	fn := initTracer("gowebsockettest")
	defer fn()

	moduleTracer = global.Tracer("main")

	http.HandleFunc("/", defaultHandler)
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/websocket", websocketHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *portPtr), nil))
}
