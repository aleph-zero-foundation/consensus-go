package sync

type mutex struct {
	token chan struct{}
}

func newMutex() *mutex {
	m := &mutex{make(chan struct{}, 1)}
	m.token <- struct{}{}
	return m
}

func (m *mutex) tryAcquire() bool {
	select {
	case _, ok := <-m.token:
		return ok
	default:
		return false
	}
}

func (m *mutex) release() {
	m.token <- struct{}{}
}
