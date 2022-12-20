package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"go-test/exchange"
	"go-test/exchange/kucoin"
	"log"
	"time"
)

func myKucoin() {
	a := kucoin.New(
		"btc.usdt",
		exchange.Api{Key: "6373353749e7da00018cc7d2", Secret: "219984f8-9ecd-4337-a5ba-02e7eae74c26", Passphrase: "testtest"},
	)

	fmt.Printf("%+v\n", a)

	_ = a.SubscribeQuote(func(quote *exchange.Quote) {
		fmt.Printf("%+v\n", *quote)
	})
	_ = a.SubscribeOrder(func(order *exchange.Order) {
		fmt.Printf("%+v\n", *order)
	})
	fmt.Printf("%+v\n", a)

	oid, _ := a.PlaceOrder(true, 5000, 0.01, true)
	pl, _ := a.GetOrders()
	fmt.Println(oid, pl)
	for _, item := range pl {
		_ = a.CancelOrder(item.Oid)
	}
	time.Sleep(2 * time.Second)
	a.Close()
	time.Sleep(time.Hour)
}

func myPanic(idx int) {
	a := kucoin.New(
		"btc.usdt",
		//exchange.Api{Key: "6373353749e7da00018cc7d2", Secret: "219984f8-9ecd-4337-a5ba-02e7eae74c26", Passphrase: "testtest"},
		exchange.Api{},
	)

	err := a.SubscribeQuote(func(quote *exchange.Quote) {
		fmt.Printf("%d, %+v\n", idx, *quote)
	})
	if err != nil {
		fmt.Println(err)
	}

	defer func() {
		if recover() != nil {
			a.Close()
			fmt.Println("Recovered in f")
			myPanic(idx + 1)
		}
	}()

	time.Sleep(time.Second)
	panic("panic test")
}

func myClose() {
	var c1 *websocket.Conn
	c, _, err := websocket.DefaultDialer.Dial("wss://stream.binance.com/ws/btcusdt@bookTicker", nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				return
			}
			fmt.Println(string(message))
		}
	}()

	time.Sleep(2 * time.Second)
	fmt.Println(1, c.Close())
	time.Sleep(2 * time.Second)

	if c != nil {
		fmt.Println(2, c.Close())
	}

	if c1 != nil {
		fmt.Println(3, c1, c1.Close())
	}

	time.Sleep(time.Hour)
}

func main() {
	myPanic(1)
}
