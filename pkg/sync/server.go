package sync

// Server is responsible for handling a sync protocol.
type Server interface {
	// Start starts server
	Start()
	// StopIn stops handling incoming synchronizations
	StopIn()
	// StopOut stops handling outgoing synchronizations
	StopOut()
}

// NewDefaultServer runs a pool of nOut workers for outgoing part and nIn for incoming part of the given protocol
func NewDefaultServer(proto Protocol, nOut, nIn uint) Server {
	return &server{
		outPool: NewPool(nOut, proto.Out),
		inPool:  NewPool(nIn, proto.In),
	}
}

type server struct {
	outPool *Pool
	inPool  *Pool
}

func (s *server) Start() {
	s.outPool.Start()
	s.inPool.Start()
}

func (s *server) StopIn() {
	s.inPool.Stop()
}

func (s *server) StopOut() {
	s.outPool.Stop()
}
