package main

import (
	"io"
	"bytes"
	"encoding/binary"
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
	"time"
)

var upgrader = websocket.Upgrader{
	EnableCompression: false,
}

type Whiteboard struct {
	Image     draw.Image
	RenderCtx *draw2dimg.GraphicContext
}

func handler(addClient, removeClient chan *Client, recv chan InPacket) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade:", err)
			return
		}
		defer conn.Close()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		send := make(chan OutPacket, 5)
		recvDone := make(chan struct{})
		cl := &Client{send, recvDone}

		running := true

		addClient <- cl

		defer func() {
			removeClient <- cl
		}()

		go func() {
			for {
				msgType, msgRead, err := conn.NextReader()
				if err != nil {
					log.Println(err)
					running = false
					return
				}

				if msgType == websocket.BinaryMessage {
					recv <- InPacket{cl, msgRead}
					<-recvDone
				}
			}
		}()

		for running {
			select {
			case p := <-send:
				if p.IsBinary {
					conn.WriteMessage(websocket.BinaryMessage, p.Data)
				} else {
					conn.WriteMessage(websocket.TextMessage, p.Data)
				}
			case <-ticker.C:
				conn.WriteMessage(websocket.PingMessage, nil)
			}
		}
	}
}

type Client struct {
	Send     chan OutPacket
	RecvDone chan struct{}
}

type InPacket struct {
	Client *Client
	Reader io.Reader
}

type OutPacket struct {
	IsBinary bool
	Data []byte
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
		RenderCtx: draw2dimg.NewGraphicContext(img),
	}

	board.Clear()

	addClient := make(chan *Client, 4)
	removeClient := make(chan *Client, 4)
	recvPacket := make(chan InPacket, 16)

	go func() {
		clients := make(map[*Client]struct{})
		for {
			select {
			case p := <-recvPacket:
				var cmd byte
				if err := binary.Read(p.Reader, binary.BigEndian, &cmd); err != nil {
					log.Println("Error reading: ", err)
				} else if cmd == 0x01 { // REQUEST SYNC
					b := bytes.NewBuffer([]byte{0x02}); // SYNC DATA
					if err := png.Encode(b, board.Image); err != nil {
						log.Println("Failed to encode board image as PNG:", err)
					} else {
						p.Client.Send <- OutPacket{true, b.Bytes()}
					}
				} else {
					b := bytes.NewBuffer([]byte{cmd})
					r := io.TeeReader(p.Reader, b)
					if err := board.Update(cmd, r); err == nil {
						data := b.Bytes()
						for cl := range clients {
							if cl != p.Client {
								cl.Send <- OutPacket{true, data}
							}
						}
					} else {
						log.Println("Malformed command:", err)
					}
				}

				p.Client.RecvDone <- struct{}{}
			case cl := <-addClient:
				clients[cl] = struct{}{}
			case cl := <-removeClient:
				delete(clients, cl)
			}
		}
	}()

	http.HandleFunc("/draw/ws/"+boardId, handler(addClient, removeClient, recvPacket))
}

func (board *Whiteboard) Update(cmd byte, r io.Reader) error {
	switch cmd {
	case 0x00: // CLEAR
		board.Clear()

	case 0x03: // DRAW
		var pkt struct {
			X0, Y0, X1, Y1 uint16
			Width float32
			R, G, B, A uint8
		}

		if err := binary.Read(r, binary.BigEndian, &pkt); err != nil {
			return err
		}

		col := color.RGBA{pkt.R, pkt.G, pkt.B, pkt.A}

		board.DrawStroke(col, float64(pkt.Width), float64(pkt.X0), float64(pkt.Y0), float64(pkt.X1), float64(pkt.Y1))

	case 0x04: // ERASE
		var pkt struct {
			X0, Y0, X1, Y1 uint16
			Width float32
		}

		if err := binary.Read(r, binary.BigEndian, &pkt); err != nil {
			return err
		}

		col := color.RGBA{255, 255, 255, 255}

		board.DrawStroke(col, float64(pkt.Width), float64(pkt.X0), float64(pkt.Y0), float64(pkt.X1), float64(pkt.Y1))
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
