package main

import (
	"image/color"
	"log"
	"time"
	"net/http"
	"golang.org/x/net/websocket"
	"sync"
	"fmt"
	"flag"
)

type LEDStripe struct {
	LEDS []color.RGBA
}

func NewLEDStripe(count int) *LEDStripe {
	stripe := &LEDStripe{
		LEDS: make([]color.RGBA, count),
	}

	return stripe
}

type LEDRGB struct {
	R uint8
	G uint8
	B uint8
}

func (s *LEDStripe) Render() []LEDRGB {
	output := make([]LEDRGB, len(s.LEDS))

	for i, l := range s.LEDS {
		output[i] = LEDRGB{
			R: l.R,
			G: l.G,
			B: l.B,
		}
	}

	return output
}

func CreateEchoServer(broadcaster *WebsocketBroadcaster) websocket.Handler {
	return func (ws *websocket.Conn) {
		broadcaster.Add(ws)

		for {
			var msg string
			if err := websocket.Message.Receive(ws, &msg); err != nil {
				fmt.Println("Recv error: ", err.Error())
				break
			}
		}
		broadcaster.Remove(ws)
	}
}



type WebsocketBroadcaster struct {
	sync.RWMutex
	clients []*websocket.Conn

}

func (b *WebsocketBroadcaster) Add(ws *websocket.Conn) {
	b.Lock()
	defer b.Unlock()

	b.clients = append(b.clients, ws)
}

func (b *WebsocketBroadcaster) Remove(ws *websocket.Conn) {
	b.Lock()
	defer b.Unlock()

	for i, c := range b.clients {
		if c == ws {
			b.clients = append(b.clients[:i], b.clients[i+1:]...)
			return
		}
	}
}

func (b *WebsocketBroadcaster) Broadcast(l []LEDRGB) {

	type StripeJSONData [3]uint8

	type StripeJSON struct {
		Values []StripeJSONData `json:"values"`
	}


	data := StripeJSON{
		Values: make([]StripeJSONData, len(l)),
	}

	for i, led := range l {
		s := StripeJSONData{
			led.R, led.G, led.B,
		}
		data.Values[i] = s
	}

	for _, c := range b.clients {
		if err := websocket.JSON.Send(c, &data); err != nil {
			log.Println(err.Error())
		}
	}
}

func main() {

	broker := flag.String("broker", "tcp://whisky.w17.io:1883", "The broker URI. ex: tcp://whisky.w17.io:1883")
	id := flag.String("id", "super-lightsd", "The ClientID (optional)")


	stripe := NewLEDStripe(1000)

	fps := 60

	pipeline := []Operation{
		NewRainbow(),
		NewRaindrop(),
		NewRotation(60.0),
	}

	NewMqttConnection(*broker, *id, pipeline)

	bc := WebsocketBroadcaster{}

	go func() {
		http.Handle("/", websocket.Handler(CreateEchoServer(&bc)))
		err := http.ListenAndServe(":9000", nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()

	for {
		s := time.Now()
		for i := range pipeline {
			log.Printf("%v", i)
			pipeline[i].Render(stripe)
		}

		l := stripe.Render()

		log.Printf("Frame: %v", len(l))

		elapsed := time.Now().Sub(s)

		bc.Broadcast(l)
		interval := time.Second / time.Duration(fps)

		diff := interval - elapsed
		time.Sleep(diff)
	}
}