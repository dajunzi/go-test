package exchange

type Api struct {
	Key        string
	Secret     string
	Passphrase string
}

type Quote struct {
	Bid float64
	Ask float64
	Ts  int64 // in MicroSecond
}

type Order struct {
	Oid    string
	IsBuy  bool
	Price  float64
	Amount float64
	Filled float64
	Closed bool
}

type IExchange interface {
	GetInfo() (float64, error)
	GetOrders() ([]Order, error)
	PlaceOrder(isBuy bool, price float64, amount float64, isMaker bool) (string, error)
	CancelOrder(oid string) error
	SubscribeQuote(cb func(*Quote)) error
	SubscribeOrder(cb func(*Order)) error
	Close()
}
