package network

// Controler represents switch functionality
type Controller interface {
	Start()
	Stop()
}

// ChannelServer waits for incomming connections and returns Channel object
type ChannelServer interface {
	Controller
	Channels() []Channel
}

// Listener accepts multiple Channels and listens for incomming synchronizations
type Listener interface {
	Controller
	ListenChannels() []Channel
}

// Syncer accepts multiple Channels and initiate outgoing synchronizations
type Syncer interface {
	Controller
	SyncChannels() []Channel
}
