package gomel

// Tx is a minimalistic struct for transactions
type Tx struct {
	ID       uint32
	Issuer   string
	Receiver string
	Amount   uint32
}
