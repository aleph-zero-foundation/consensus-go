package tests

import (
	"crypto/rand"

	"gitlab.com/alephledger/consensus-go/pkg/gomel"
)

type testDataSource struct {
	dataSource chan gomel.Data
	blockSize  int
	exitChan   chan struct{}
}

// NewDataSource returns a test data source sending
// to the channel random slices of data of given size.
func NewDataSource(blockSize int) *testDataSource {
	exitChan := make(chan struct{})
	dataSource := make(chan gomel.Data)
	return &testDataSource{
		dataSource,
		blockSize,
		exitChan,
	}
}

// DataSource returns gomel.DataSource object from the testDataSource.
func (tds *testDataSource) DataSource() gomel.DataSource {
	return tds.dataSource
}

// Start starts generating and sending random bytes to the DataSource channel.
func (tds *testDataSource) Start() {
	go func() {
		for {
			data := make([]byte, tds.blockSize)
			rand.Read(data)
			select {
			case tds.dataSource <- data:
			case <-tds.exitChan:
				close(tds.dataSource)
				return
			}
		}
	}()
}

// Stop stops the generation of random bytes.
func (tds *testDataSource) Stop() {
	tds.exitChan <- struct{}{}
}
