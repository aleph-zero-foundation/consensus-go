package sync

// Mutex is a structure that allows nonblocking locks.
type Mutex struct {
	token chan struct{}
}

// NewMutex creates a soft mutex.
func NewMutex() *Mutex {
	m := &Mutex{make(chan struct{}, 1)}
	m.token <- struct{}{}
	return m
}

// TryAcquire returns whether the resource has been locked.
func (m *Mutex) TryAcquire() bool {
	select {
	case _, ok := <-m.token:
		return ok
	default:
		return false
	}
}

// Release unlocks the resource.
func (m *Mutex) Release() {
	m.token <- struct{}{}
}
