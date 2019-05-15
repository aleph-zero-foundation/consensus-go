package gomel

// Tx is a minimalistic struct for transactions
type Tx struct {
	Issuer   string
	Receiver string
	Amount   uint
}
