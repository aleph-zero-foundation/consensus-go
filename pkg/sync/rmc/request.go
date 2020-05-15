package rmc

const (
	msgSendData byte = iota
	msgSendProof
	msgRequestFinished
)

// request represents a request to a multicast server
type request struct {
	msgType byte
	id      uint64
	data    []byte
}

// newRequest returns a request with given parameters
func newRequest(id uint64, data []byte, msgType byte) *request {
	return &request{
		msgType: msgType,
		id:      id,
		data:    data,
	}
}
