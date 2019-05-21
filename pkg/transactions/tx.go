package transactions

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

// Decode decodes bytes to transactions
func Decode(data []byte) []Tx {
	reader := bytes.NewReader(data)
	result := []Tx{}
	for reader.Len() > 0 {
		var id, amount uint32
		var issuerLen, receiverLen uint8
		binary.Read(reader, binary.LittleEndian, &id)

		binary.Read(reader, binary.LittleEndian, &issuerLen)
		issuer := make([]byte, int(issuerLen))
		reader.Read(issuer)

		binary.Read(reader, binary.LittleEndian, &receiverLen)
		receiver := make([]byte, int(receiverLen))
		reader.Read(receiver)

		binary.Read(reader, binary.LittleEndian, &amount)
		result = append(result, Tx{
			ID:       id,
			Issuer:   string(issuer),
			Receiver: string(receiver),
			Amount:   amount,
		})
	}
	return result
}

// Encode returns byte representation of given list of transactions. In the following format
// ID as uint32 (4 bytes)
// length of Issuer as uint8 (1 byte)
// Issuer
// length of Receiver as uint8 (1 byte)
// Receiver
// Amount as uint32(4 bytes)
func Encode(txs []Tx) []byte {
	var data bytes.Buffer
	for _, tx := range txs {
		binary.Write(&data, binary.LittleEndian, tx.ID)
		binary.Write(&data, binary.LittleEndian, uint8(len(tx.Issuer)))
		data.Write([]byte(tx.Issuer))
		binary.Write(&data, binary.LittleEndian, uint8(len(tx.Receiver)))
		data.Write([]byte(tx.Receiver))
		binary.Write(&data, binary.LittleEndian, tx.Amount)
	}
	return data.Bytes()
}
