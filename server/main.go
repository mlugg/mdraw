package main

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/llgcode/draw2d/draw2dimg"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var upgrader = websocket.Upgrader{
	EnableCompression: false,
}

type Whiteboard struct {
	Image     draw.Image
	RenderCtx *draw2dimg.GraphicContext
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
	SpawnWhiteboard("foo")
	log.Fatal(http.ListenAndServe("localhost:8081", nil))
}

func SpawnWhiteboard(boardId string) {
	img := image.NewRGBA(image.Rect(0, 0, 800, 500))
	board := &Whiteboard{
		Image: img,
		//Rasterizer: vector.NewRasterizer(800, 500),
		RenderCtx: draw2dimg.NewGraphicContext(img),
	}

	board.Clear()

	addClient := make(chan *Client, 4)
	removeClient := make(chan *Client, 4)
	recvPacket := make(chan Packet, 16)

	go func() {
		clients := make(map[*Client]struct{})
		for {
			select {
			case p := <-recvPacket:
				err := board.Update(p.Data)
				if err == nil {
					for cl := range clients {
						cl.Send <- p
					}
				} else {
					log.Println("Malformed command:", err)
				}
			case cl := <-addClient:
				clients[cl] = struct{}{}
			case cl := <-removeClient:
				delete(clients, cl)
			}
		}
	}()

	// XXX TEMPORARY
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			<-ticker.C
			f, err := os.Create("output.png")
			if err != nil {
				panic(err)
			}

			png.Encode(f, board.Image)

			defer f.Close()
			println("made image")
		}
	}()

	http.HandleFunc("/draw/ws/"+boardId, handler(addClient, removeClient, recvPacket))
}

func parseColor(s string, alpha uint8) (color.RGBA, error) {
	x, err := strconv.ParseUint(s[1:], 16, 32)

	if err != nil {
		return color.RGBA{}, err
	}

	return color.RGBA{
		R: uint8((x >> 16) & 0xFF),
		G: uint8((x >> 8) & 0xFF),
		B: uint8(x & 0xFF),
		A: alpha,
	}, nil
}

func (board *Whiteboard) Update(pkt []byte) error {
	parts := strings.Split(string(pkt), " ")
	if parts == nil {
		return errors.New("Empty command given")
	}

	switch parts[0] {
	case "DRAW":
		if len(parts) != 7 {
			return errors.New("Malformed DRAW command")
		}

		color, err := parseColor(parts[1], 255)
		if err != nil {
			return err
		}

		width, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return err
		}

		x0, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return err
		}

		y0, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return err
		}

		x1, err := strconv.ParseFloat(parts[5], 64)
		if err != nil {
			return err
		}

		y1, err := strconv.ParseFloat(parts[6], 64)
		if err != nil {
			return err
		}

		board.DrawStroke(color, width, x0, y0, x1, y1)
	case "ERASE":
		if len(parts) != 6 {
			return errors.New("Malformed ERASE command")
		}

		width, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return err
		}

		x0, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return err
		}

		y0, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return err
		}

		x1, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return err
		}

		y1, err := strconv.ParseFloat(parts[5], 64)
		if err != nil {
			return err
		}

		board.DrawStroke(color.RGBA{255, 255, 255, 255}, width, x0, y0, x1, y1)
	}

	return nil
}

func (board *Whiteboard) DrawStroke(col color.RGBA, width float64, x0, y0, x1, y1 float64) {
	// TODO: clean up drawing (one path?)
	board.RenderCtx.SetFillColor(col)
	board.RenderCtx.SetStrokeColor(col)
	board.RenderCtx.SetLineWidth(width)

	board.RenderCtx.BeginPath()
	board.RenderCtx.MoveTo(x0, y0)
	board.RenderCtx.LineTo(x1, y1)
	board.RenderCtx.Close()
	board.RenderCtx.Stroke()

	board.RenderCtx.BeginPath()
	board.RenderCtx.ArcTo(x0, y0, width/2, width/2, 0, 2*math.Pi)
	board.RenderCtx.Close()
	board.RenderCtx.Fill()

	board.RenderCtx.BeginPath()
	board.RenderCtx.ArcTo(x1, y1, width/2, width/2, 0, 2*math.Pi)
	board.RenderCtx.Close()
	board.RenderCtx.Fill()
}

func (board *Whiteboard) Clear() {
	rc := board.RenderCtx
	rc.SetStrokeColor(color.RGBA{255, 255, 255, 255})
	rc.SetFillColor(color.RGBA{255, 255, 255, 255})
	rc.SetLineWidth(0)
	rc.BeginPath()
	rc.MoveTo(0, 0)
	rc.LineTo(800, 0)
	rc.LineTo(800, 600)
	rc.LineTo(0, 600)
	rc.Close()
	rc.FillStroke()
}
