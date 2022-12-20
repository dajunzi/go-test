package kucoin

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/gorilla/websocket"
	"go-test/exchange"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var sideMap = map[bool]string{true: "buy", false: "sell"}

type Kucoin struct {
	con   string
	token string
	api   exchange.Api
	conns []*websocket.Conn
}

func New(con string, api exchange.Api) exchange.IExchange {
	return &Kucoin{
		con:   strings.ToUpper(strings.ReplaceAll(con, ".", "-")),
		token: strings.ToUpper(strings.Split(con, ".")[0]),
		api:   api,
	}
}

func (inst *Kucoin) Close() {
	for _, conn := range inst.conns {
		_ = conn.Close()
	}
}

func (inst *Kucoin) SubscribeQuote(cb func(*exchange.Quote)) error {
	resp, err := inst.apiCall("POST", "/api/v1/bullet-public", "")
	if err != nil {
		return err
	}
	token, _ := jsonparser.GetString(resp, "token")
	c, _, err := websocket.DefaultDialer.Dial("wss://ws-api.kucoin.com/endpoint?token="+token, nil)
	if err != nil {
		return err
	}
	inst.conns = append(inst.conns, c)

	topic := "/spotMarket/level2Depth5:" + inst.con
	go func() { // ping
		for {
			time.Sleep(20 * time.Second)
			jsReq := map[string]interface{}{"id": time.Now().UnixNano(), "type": "ping"}
			if c.WriteJSON(jsReq) != nil {
				return
			}
		}
	}()
	go func() { // receive
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}
			msgTopic, _ := jsonparser.GetString(message, "topic")
			if msgTopic == topic {
				data, _, _, _ := jsonparser.Get(message, "data")
				ts, _ := jsonparser.GetInt(data, "timestamp")
				askStr, _ := jsonparser.GetString(data, "asks", "[0]", "[0]")
				ask, _ := strconv.ParseFloat(askStr, 64)
				bidStr, _ := jsonparser.GetString(data, "bids", "[0]", "[0]")
				bid, _ := strconv.ParseFloat(bidStr, 64)
				cb(&exchange.Quote{
					Bid: bid,
					Ask: ask,
					Ts:  ts * 1e3,
				})
			}
		}
	}()

	jsReq := map[string]interface{}{
		"id":    time.Now().UnixNano(),
		"type":  "subscribe",
		"topic": topic,
	}
	return c.WriteJSON(jsReq)
}

func (inst *Kucoin) SubscribeOrder(cb func(*exchange.Order)) error {
	resp, err := inst.apiCall("POST", "/api/v1/bullet-private", "")
	if err != nil {
		return err
	}
	token, _ := jsonparser.GetString(resp, "token")
	c, _, err := websocket.DefaultDialer.Dial("wss://ws-api.kucoin.com/endpoint?token="+token, nil)
	if err != nil {
		return err
	}
	inst.conns = append(inst.conns, c)

	topic := "/spotMarket/tradeOrders"
	go func() { // ping
		for {
			time.Sleep(20 * time.Second)
			jsReq := map[string]interface{}{"id": time.Now().UnixNano(), "type": "ping"}
			if c.WriteJSON(jsReq) != nil {
				return
			}
		}
	}()
	go func() { // receive
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}
			msgTopic, _ := jsonparser.GetString(message, "topic")
			symbol, _ := jsonparser.GetString(message, "data", "symbol")
			if msgTopic == topic && symbol == inst.con {
				data, _, _, _ := jsonparser.Get(message, "data")
				oid, _ := jsonparser.GetString(data, "orderId")
				side, _ := jsonparser.GetString(data, "side")
				priceStr, _ := jsonparser.GetString(data, "price")
				price, _ := strconv.ParseFloat(priceStr, 64)
				amountStr, _ := jsonparser.GetString(data, "size")
				amount, _ := strconv.ParseFloat(amountStr, 64)
				filledStr, _ := jsonparser.GetString(data, "filledSize")
				filled, _ := strconv.ParseFloat(filledStr, 64)
				status, _ := jsonparser.GetString(data, "status")
				cb(&exchange.Order{
					Oid:    oid,
					IsBuy:  side == "buy",
					Price:  price,
					Amount: amount,
					Filled: filled,
					Closed: status == "done",
				})
			}
		}
	}()

	jsReq := map[string]interface{}{
		"id":             time.Now().UnixNano(),
		"type":           "subscribe",
		"topic":          topic,
		"privateChannel": true,
	}
	return c.WriteJSON(jsReq)
}

func (inst *Kucoin) GetInfo() (float64, error) {
	resp, err := inst.apiCall("GET", "/api/v1/accounts?type=trade", "")
	if err != nil {
		return 0, err
	}
	info := make(map[string]float64)
	_, err = jsonparser.ArrayEach(resp, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		currency, _ := jsonparser.GetString(value, "currency")
		balanceStr, _ := jsonparser.GetString(value, "balance")
		balance, _ := strconv.ParseFloat(balanceStr, 64)
		info[currency] = balance
	})
	return info[inst.token], err
}

func (inst *Kucoin) GetOrders() ([]exchange.Order, error) {
	resp, err := inst.apiCall("GET", "/api/v1/orders?status=active&symbol="+inst.con, "")
	if err != nil {
		return nil, err
	}
	var orders []exchange.Order
	_, err = jsonparser.ArrayEach(resp, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		order := exchange.Order{}
		order.Oid, _ = jsonparser.GetString(value, "id")
		side, _ := jsonparser.GetString(value, "side")
		order.IsBuy = side == "buy"
		price, _ := jsonparser.GetString(value, "price")
		order.Price, _ = strconv.ParseFloat(price, 64)
		size, _ := jsonparser.GetString(value, "size")
		order.Amount, _ = strconv.ParseFloat(size, 64)
		orders = append(orders, order)
	}, "items")
	return orders, err
}

func (inst *Kucoin) PlaceOrder(isBuy bool, price float64, amount float64, isMaker bool) (string, error) {
	body := fmt.Sprintf(
		`{"clientOid": "%d", "side": "%s", "symbol": "%s", "price": %f, "size": %f, "postOnly": %t}`,
		time.Now().UnixNano(), sideMap[isBuy], inst.con, price, amount, isMaker,
	)
	resp, err := inst.apiCall("POST", "/api/v1/orders", body)
	if err != nil {
		return "", err
	}
	return jsonparser.GetString(resp, "orderId")
}

func (inst *Kucoin) CancelOrder(oid string) error {
	_, err := inst.apiCall("DELETE", "/api/v1/orders/"+oid, "")
	return err
}

func (inst *Kucoin) apiCall(method, endpoint, data string) ([]byte, error) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signStr := ts + method + endpoint + data
	signer := hmac.New(sha256.New, []byte(inst.api.Secret))
	signer.Write([]byte(signStr))
	signature := base64.StdEncoding.EncodeToString(signer.Sum(nil))

	req, _ := http.NewRequest(method, "https://api.kucoin.com"+endpoint, bytes.NewReader([]byte(data)))
	req.Header.Set("KC-API-SIGN", signature)
	req.Header.Set("KC-API-TIMESTAMP", ts)
	req.Header.Set("KC-API-KEY", inst.api.Key)
	req.Header.Set("KC-API-PASSPHRASE", inst.api.Passphrase)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	bodyCode, err := jsonparser.GetString(body, "code")
	if err != nil {
		return nil, err
	} else if bodyCode != "200000" {
		return nil, fmt.Errorf("code error: %s", body)
	}
	bodyData, _, _, err := jsonparser.Get(body, "data")
	return bodyData, err
}
