package multicast

const (
	SendData byte = iota
	SendFinished
)

type Request struct {
	msgType byte
	id      uint64
	pid     uint16
	data    []byte
}

func NewRequest(id uint64, pid uint16, data []byte, msgType byte) Request {
	return Request{
		msgType: msgType,
		id:      id,
		pid:     pid,
		data:    data,
	}
}
