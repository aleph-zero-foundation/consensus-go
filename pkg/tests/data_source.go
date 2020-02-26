package tests

import (
	"crypto/rand"

	"gitlab.com/alephledger/core-go/pkg/core"
)

// TestDataSource is a data source for testing without proper data source.
type TestDataSource struct {
	blockSize int
}

// NewDataSource returns a test data source sending
// to the channel random slices of data of given size.
func NewDataSource(blockSize int) core.DataSource {
	return &TestDataSource{blockSize}
}

// GetData returns a single piece of data for a unit.
func (tds *TestDataSource) GetData() core.Data {
	data := make([]byte, tds.blockSize)
	rand.Read(data)
	return data
}
