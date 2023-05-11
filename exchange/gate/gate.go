package gate

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
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

type Gate struct {
	con    string
	token  string
	host   string
	wsHost string
	api    exchange.Api
	conns  []*websocket.Conn
}

func New(con string, api exchange.Api) exchange.IExchange {
	return &Gate{
		host:   "https://api.gateio.ws",
		wsHost: "wss://api.gateio.ws/ws/v4/",
		con:    strings.ToUpper(strings.ReplaceAll(con, ".", "_")),
		token:  strings.ToUpper(strings.Split(con, ".")[0]),
		api:    api,
	}
}

func (g Gate) GetInfo() (float64, error) {
	resp, err := g.apiCall("GET", "/api/v4/spot/accounts", "currency="+g.token, "")
	if err != nil {
		return 0, err
	}
	currency, _ := jsonparser.GetString(resp, "[0]", "currency")
	if currency != g.token {
		return 0, fmt.Errorf("currency mismatch: %s", resp)
	}
	availableStr, _ := jsonparser.GetString(resp, "[0]", "available")
	available, _ := strconv.ParseFloat(availableStr, 64)
	lockedStr, _ := jsonparser.GetString(resp, "[0]", "locked")
	locked, _ := strconv.ParseFloat(lockedStr, 64)
	return available + locked, nil
}

func (g Gate) GetOrders() ([]exchange.Order, error) {
	resp, err := g.apiCall("GET", "/api/v4/spot/orders", "currency_pair="+g.con+"&status=open&account=cross_margin", "")
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
		size, _ := jsonparser.GetString(value, "amount")
		order.Amount, _ = strconv.ParseFloat(size, 64)
		orders = append(orders, order)
	})
	return orders, err
}

func (g Gate) PlaceOrder(isBuy bool, price float64, amount float64, isMaker bool) (string, error) {
	side := "sell"
	if isBuy {
		side = "buy"
	}
	tif := "gtc"
	if isMaker {
		tif = "poc"
	}
	body := fmt.Sprintf(
		`{"currency_pair": "%s", "type": "limit", "account": "cross_margin", "side": "%s", "price": %f, "amount": %f, "time_in_force": "%s"}`,
		g.con, side, price, amount, tif,
	)
	resp, err := g.apiCall("POST", "/api/v4/spot/orders", "", body)
	if err != nil {
		return "", err
	}
	return jsonparser.GetString(resp, "id")
}

func (g Gate) CancelOrder(oid string) error {
	_, err := g.apiCall("DELETE", "/api/v4/spot/orders/"+oid, "currency_pair="+g.con+"&account=cross_margin", "")
	return err
}

func (g Gate) SubscribeQuote(cb func(*exchange.Quote)) error {
	c, _, err := websocket.DefaultDialer.Dial(g.wsHost, nil)
	if err != nil {
		return err
	}
	g.conns = append(g.conns, c)

	topic := "spot.book_ticker"
	go func() { // receive
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}
			event, _ := jsonparser.GetString(message, "event")
			channel, _ := jsonparser.GetString(message, "channel")
			if event == "update" && channel == topic {
				ts, _ := jsonparser.GetInt(message, "result", "t")
				bidStr, _ := jsonparser.GetString(message, "result", "b")
				bid, _ := strconv.ParseFloat(bidStr, 64)
				askStr, _ := jsonparser.GetString(message, "result", "a")
				ask, _ := strconv.ParseFloat(askStr, 64)
				cb(&exchange.Quote{
					Bid: bid,
					Ask: ask,
					Ts:  ts * 1e6,
				})
			}
		}
	}()
	return c.WriteJSON(map[string]interface{}{
		"time":    time.Now().Unix(),
		"channel": topic,
		"event":   "subscribe",
		"payload": []string{g.con},
	})
}

func (g Gate) SubscribeOrder(cb func(*exchange.Order)) error {
	c, _, err := websocket.DefaultDialer.Dial(g.wsHost, nil)
	if err != nil {
		return err
	}
	g.conns = append(g.conns, c)

	topic := "spot.orders"
	go func() { // receive
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}
			event, _ := jsonparser.GetString(message, "event")
			channel, _ := jsonparser.GetString(message, "channel")
			if event == "update" && channel == topic {
				_, err = jsonparser.ArrayEach(message, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
					pair, _ := jsonparser.GetString(value, "currency_pair")
					if pair == g.con {
						oid, _ := jsonparser.GetString(value, "id")
						side, _ := jsonparser.GetString(value, "side")
						priceStr, _ := jsonparser.GetString(value, "price")
						price, _ := strconv.ParseFloat(priceStr, 64)
						amountStr, _ := jsonparser.GetString(value, "amount")
						amount, _ := strconv.ParseFloat(amountStr, 64)
						leftStr, _ := jsonparser.GetString(value, "left")
						left, _ := strconv.ParseFloat(leftStr, 64)
						status, _ := jsonparser.GetString(value, "event")
						cb(&exchange.Order{
							Oid:    oid,
							IsBuy:  side == "buy",
							Price:  price,
							Amount: amount,
							Filled: amount - left,
							Closed: status == "finish",
						})
					}
				}, "result")
			}
		}
	}()

	ts := time.Now().Unix()
	msg := fmt.Sprintf("channel=%s&event=%s&time=%d", topic, "subscribe", ts)
	mac := hmac.New(sha512.New, []byte(g.api.Secret))
	mac.Write([]byte(msg))
	sign := hex.EncodeToString(mac.Sum(nil))
	return c.WriteJSON(map[string]interface{}{
		"time":    ts,
		"channel": topic,
		"event":   "subscribe",
		"payload": []string{g.con},
		"auth": map[string]interface{}{
			"method": "api_key",
			"KEY":    g.api.Key,
			"SIGN":   sign,
		},
	})
}

func (g Gate) Close() {
	for _, conn := range g.conns {
		_ = conn.Close()
	}
}

func (g Gate) apiCall(method, endpoint, query string, payload string) ([]byte, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	hash := sha512.New()
	hash.Write([]byte(payload))
	hashedPayload := hex.EncodeToString(hash.Sum(nil))

	msg := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", method, endpoint, query, hashedPayload, ts)
	mac := hmac.New(sha512.New, []byte(g.api.Secret))
	mac.Write([]byte(msg))
	sign := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(method, g.host+endpoint+"?"+query, bytes.NewReader([]byte(payload)))
	req.Header.Set("KEY", g.api.Key)
	req.Header.Set("Timestamp", ts)
	req.Header.Set("SIGN", sign)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
