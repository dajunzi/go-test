package play

import (
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

func Get() {
	resp, err := http.Get("https://api.binance.com/sapi/v1/system/status")
	if err != nil {
		log.Fatal("get:", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	fmt.Println(body)
	fmt.Println(string(body))

	status, _ := jsonparser.GetInt(body, "status")
	msg, _ := jsonparser.GetString(body, "msg")
	fmt.Println(status)
	fmt.Println(msg)
}

func SimpleWs() {
	c, _, err := websocket.DefaultDialer.Dial("wss://stream.binance.com/ws/btcusdt@bookTicker", nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Fatal("read:", err)
				return
			}
			fmt.Println(string(message))
			fmt.Println(jsonparser.GetInt(message, "u"))
		}
	}()
}

func SubWs() {
	c, _, err := websocket.DefaultDialer.Dial("wss://stream.binance.com/stream", nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Fatal("read:", err)
				return
			}
			fmt.Println(string(message))
		}
	}()

	jsReq := map[string]interface{}{"method": "SUBSCRIBE", "params": []string{"btcusdt@bookTicker"}, "id": 1}
	reqErr := c.WriteJSON(jsReq)
	if reqErr != nil {
		log.Fatal("subscribe:", reqErr)
	}
}

type Quote struct {
	bid float64
	ask float64
	ts  float64
}

type Exchange interface {
	NewQuote(func(*Quote))
	Init(string)
}

func CallbackWs(cb func(*Quote)) {
	c, _, err := websocket.DefaultDialer.Dial("wss://stream.binance.com/stream", nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Fatal("read:", err)
				return
			}
			fmt.Println(string(message))
			bidStr, _ := jsonparser.GetString(message, "data", "b")
			bid, _ := strconv.ParseFloat(bidStr, 64)
			askStr, _ := jsonparser.GetString(message, "data", "a")
			ask, _ := strconv.ParseFloat(askStr, 64)
			ts, _ := jsonparser.GetFloat(message, "data", "u")
			cb(&Quote{bid, ask, ts})
		}
	}()

	jsReq := map[string]interface{}{"method": "SUBSCRIBE", "params": []string{"btcusdt@bookTicker"}, "id": 1}
	reqErr := c.WriteJSON(jsReq)
	if reqErr != nil {
		log.Fatal("subscribe:", reqErr)
	}
}

func Post() {
}

func SimpleLock() {
	a := 0
	for i := 0; i < 1000; i++ {
		go func() { a += 1 }()
	}

	time.Sleep(time.Second)
	fmt.Println(a)
}

func pprint(quote *Quote) {
	fmt.Println(*quote)
}

func describe(i interface{}) {
	fmt.Printf("(%v, %T)\n", i, i)
}

func main() {
	CallbackWs(pprint)
	time.Sleep(1000000 * time.Second)
}
