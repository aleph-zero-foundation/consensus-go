package network

type Listener interface {
	Listen() (Connection, error)
}
