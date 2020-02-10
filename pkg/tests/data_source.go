package tests

import (
	"crypto/rand"

	"gitlab.com/alephledger/core-go/pkg/core"
)

// TestDataSource is a data source for testing without proper data source.
type TestDataSource struct {
	dataSource chan core.Data
	blockSize  int
	exitChan   chan struct{}
}

// NewDataSource returns a test data source sending
// to the channel random slices of data of given size.
func NewDataSource(blockSize int) *TestDataSource {
	exitChan := make(chan struct{})
	dataSource := make(chan core.Data)
	return &TestDataSource{
		dataSource,
		blockSize,
		exitChan,
	}
}

// DataSource returns gomel.DataSource object from the TestDataSource.
func (tds *TestDataSource) DataSource() core.DataSource {
	return tds.dataSource
}

// Start starts generating and sending random bytes to the DataSource channel.
func (tds *TestDataSource) Start() {
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
func (tds *TestDataSource) Stop() {
	tds.exitChan <- struct{}{}
}
