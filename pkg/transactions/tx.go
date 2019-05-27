package transactions

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io/ioutil"
)

// Tx is a minimalistic struct for transactions
type Tx struct {
	ID       uint32
	Issuer   string
	Receiver string
	Amount   uint32
}

// Decode decodes bytes to transactions
func Decode(data []byte) ([]Tx, error) {
	reader := bytes.NewReader(data)
	result := []Tx{}

	var id, amount uint32
	var issuerLen, receiverLen uint8
	for reader.Len() > 0 {
		err := binary.Read(reader, binary.LittleEndian, &id)
		if err != nil {
			return result, err
		}

		err = binary.Read(reader, binary.LittleEndian, &issuerLen)
		if err != nil {
			return result, err
		}
		issuer := make([]byte, int(issuerLen))
		_, err = reader.Read(issuer)
		if err != nil {
			return result, err
		}

		err = binary.Read(reader, binary.LittleEndian, &receiverLen)
		if err != nil {
			return result, err
		}
		receiver := make([]byte, int(receiverLen))
		_, err = reader.Read(receiver)
		if err != nil {
			return result, err
		}

		err = binary.Read(reader, binary.LittleEndian, &amount)
		if err != nil {
			return result, err
		}
		result = append(result, Tx{
			ID:       id,
			Issuer:   string(issuer),
			Receiver: string(receiver),
			Amount:   amount,
		})
	}
	return result, nil
}

// Encode returns compressed byte representation of given list of transactions.
// In the following format
// (1) ID as uint32 (4 bytes)
// (2) length of Issuer as uint8 (1 byte)
// (3) Issuer
// (4) length of Receiver as uint8 (1 byte)
// (5) Receiver
// (6) Amount as uint32(4 bytes)
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

//Compress is compressing the data using gzip on a given level
func Compress(data []byte, level int) ([]byte, error) {
	var b bytes.Buffer
	gz, err := gzip.NewWriterLevel(&b, level)
	if err != nil {
		return nil, err
	}
	_, err = gz.Write(data)
	if err != nil {
		return nil, err
	}
	err = gz.Close()
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Decompress is decompressing given gzipped data
func Decompress(data []byte) ([]byte, error) {
	var result []byte
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	result, err = ioutil.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	err = gz.Close()
	if err != nil {
		return nil, err
	}
	return result, nil
}
