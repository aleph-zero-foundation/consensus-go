package gomel

import (
	"bytes"
	"encoding/binary"
)

// Tx is a minimalistic struct for transactions
type Tx struct {
	ID       uint32
	Issuer   string
	Receiver string
	Amount   uint32
}

// DecodeTxs decodes bytes to transactions
func DecodeTxs([]byte) []Tx {
	return []Tx{}
}

// EncodeTxs returns byte representation of given list of transactions.
func EncodeTxs(txs []Tx) []byte {
	var data bytes.Buffer
	for _, tx := range txs {
		binary.Write(&data, binary.LittleEndian, tx.ID)
		data.Write([]byte(tx.Issuer))
		data.Write([]byte(tx.Receiver))
		binary.Write(&data, binary.LittleEndian, tx.Amount)
	}
	return data.Bytes()
}
