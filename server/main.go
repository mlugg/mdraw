package main

import (
	"log"
	"os"
	//	"image/png"
	"github.com/gorilla/websocket"
	"golang.org/x/image/vector"
	"image"
	"net/http"
	"time"
)

var img image.Image
var rasterizer *vector.Rasterizer

var upgrader = websocket.Upgrader{
	EnableCompression: false,
}

func handler(addClient, removeClient chan *Client, recv chan Packet) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade:", err)
			return
		}
		defer conn.Close()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		send := make(chan Packet, 5)
		cl := &Client{send}

		running := true

		addClient <- cl

		defer func() {
			removeClient <- cl
		}()

		go func() {
			for {
				msgType, msg, err := conn.ReadMessage()
				if err != nil {
					log.Println(err)
					running = false
					return
				}

				if msgType == websocket.TextMessage {
					recv <- Packet{cl, msg}
				}
			}
		}()

		for running {
			select {
			case p := <-send:
				if p.Client != cl {
					conn.WriteMessage(websocket.TextMessage, p.Data)
				}
			case <-ticker.C:
				conn.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}
}

type Client struct {
	Send chan Packet
}

type Packet struct {
	Client *Client
	Data   []byte
}

func main() {
	log.SetOutput(os.Stdout)
	img = image.NewRGBA(image.Rect(0, 0, 800, 500))
	rasterizer = vector.NewRasterizer(800, 500)

	addClient := make(chan *Client, 4)
	removeClient := make(chan *Client, 4)
	recvPacket := make(chan Packet, 16)

	go func() {
		clients := make(map[*Client]struct{})
		for {
			select {
			case p := <-recvPacket:
				for cl := range clients {
					cl.Send <- p
				}
			case cl := <-addClient:
				clients[cl] = struct{}{}
			case cl := <-removeClient:
				delete(clients, cl)
			}
		}
	}()

	http.HandleFunc("/", handler(addClient, removeClient, recvPacket))
	log.Fatal(http.ListenAndServe("localhost:8081", nil))
}
