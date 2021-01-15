package main

import (
	"errors"
	"github.com/gorilla/websocket"
	"golang.org/x/image/vector"
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
	Image      draw.Image
	Rasterizer *vector.Rasterizer
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
	board := &Whiteboard{
		Image:      image.NewNRGBA(image.Rect(0, 0, 800, 500)),
		Rasterizer: vector.NewRasterizer(800, 500),
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

func parseColor(s string, alpha uint8) (color.NRGBA, error) {
	x, err := strconv.ParseUint(s[1:], 16, 32)

	if err != nil {
		return color.NRGBA{}, err
	}

	return color.NRGBA{
		R: uint8((x >> 16) & 0xFF),
		G: uint8((x >> 8) & 0xFF),
		B: uint8(x & 0xFF),
		A: alpha,
	}, nil
}

func Atof(s string) (float32, error) {
	x, err := strconv.ParseFloat(s, 32)
	return float32(x), err
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

		width, err := Atof(parts[2])
		if err != nil {
			return err
		}

		x0, err := Atof(parts[3])
		if err != nil {
			return err
		}

		y0, err := Atof(parts[4])
		if err != nil {
			return err
		}

		x1, err := Atof(parts[5])
		if err != nil {
			return err
		}

		y1, err := Atof(parts[6])
		if err != nil {
			return err
		}

		board.DrawStroke(color, width, x0, y0, x1, y1)
	case "ERASE":
		if len(parts) != 6 {
			return errors.New("Malformed ERASE command")
		}

		width, err := Atof(parts[1])
		if err != nil {
			return err
		}

		x0, err := Atof(parts[2])
		if err != nil {
			return err
		}

		y0, err := Atof(parts[3])
		if err != nil {
			return err
		}

		x1, err := Atof(parts[4])
		if err != nil {
			return err
		}

		y1, err := Atof(parts[5])
		if err != nil {
			return err
		}

		board.DrawStroke(color.NRGBA{255, 255, 255, 255}, width, x0, y0, x1, y1)
	}

	return nil
}

func (board *Whiteboard) DrawStroke(col color.NRGBA, width float32, x0, y0, x1, y1 float32) {
	srcImg := image.NewUniform(col)
	Line(board.Rasterizer, x0, y0, x1, y1, width)
	board.Rasterizer.Draw(board.Image, board.Rasterizer.Bounds(), srcImg, image.Point{})
	board.Rasterizer.Reset(800, 500)
	Circle(board.Rasterizer, x0, y0, width/2)
	board.Rasterizer.Draw(board.Image, board.Rasterizer.Bounds(), srcImg, image.Point{})
	board.Rasterizer.Reset(800, 500)
	Circle(board.Rasterizer, x1, y1, width/2)
	board.Rasterizer.Draw(board.Image, board.Rasterizer.Bounds(), srcImg, image.Point{})
	board.Rasterizer.Reset(800, 500)
}

func Circle(z *vector.Rasterizer, x, y, r float32) {
	c := float32(0.551915024494)
	z.MoveTo(x, y+r)
	z.CubeTo(x+r*c, y+r, x+r, y+r*c, x+r, y)
	z.CubeTo(x+r, y-r*c, x+r*c, y-r, x, y-r)
	z.CubeTo(x-r*c, y-r, x-r, y-r*c, x-r, y)
	z.CubeTo(x-r, y+r*c, x-r*c, y+r, x, y+r)
	z.ClosePath()
}

func Line(z *vector.Rasterizer, x0, y0, x1, y1, width float32) {
	tx := y0 - y1
	ty := x1 - x0
	mul := width / 2 / float32(math.Hypot(float64(tx), float64(ty)))
	tx *= mul
	ty *= mul

	z.MoveTo(x0-tx, y0-ty)
	z.LineTo(x1-tx, y1-ty)
	z.LineTo(x1+tx, y1+ty)
	z.LineTo(x0+tx, y0+ty)
	z.ClosePath()
}

func (board *Whiteboard) Clear() {
	srcImg := image.NewUniform(color.NRGBA{255, 255, 255, 255})
	z := board.Rasterizer
	z.MoveTo(0, 0)
	z.LineTo(800, 0)
	z.LineTo(800, 600)
	z.LineTo(0, 600)
	z.ClosePath()
	z.Draw(board.Image, z.Bounds(), srcImg, image.Point{})
	z.Reset(800, 500)
}
