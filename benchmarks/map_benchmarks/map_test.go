package map_test

import (
	"bytes"
	"math/rand"
	"sync"
	"testing"
)

const (
	initialElemsCount      = 10000
	numberOfRoutines       = 512
	numberOfMapOperations  = 3 * 10 * 100
	missReadsBias          = 10
	newWrites              = 90
	newOverwrites          = 0
	operationGeneratorSeed = 0
	dataGeneratorSeed      = 1
	missGeneratorSeed      = 2
)

type keyType [32]byte

type byteKeyValuePair struct {
	key   keyType
	value []byte
}

type testScenario struct {
	operationGenerator *rand.Rand
}

func newTestScenario() testScenario {
	return testScenario{operationGenerator: rand.New(rand.NewSource(operationGeneratorSeed))}
}

func (ts *testScenario) testMapAcess(testMap mapUnderTest, readBias uint64, count uint64, tdg testDataGenerator) {

	for i := uint64(0); i < count; i++ {
		operation := uint64(ts.operationGenerator.Intn(100))
		if operation < readBias {
			ix, expectedValue := tdg.getIndex()
			value, present := testMap.load(ix)
			if expectedValue != nil {
				if !present || bytes.Compare(expectedValue, value) != 0 {
					panic("invalid content of the tested map")
				}
			}
		} else {
			index, value := tdg.getIndexAndValue()
			testMap.store(index, value)
		}
	}
}

type testDataGenerator interface {
	getIndex() (keyType, []byte)
	getIndexAndValue() (keyType, []byte)
}

type tdgImpl struct {
	minDataSize   int
	maxDataSize   int
	dataGenerator *rand.Rand
	missGenerator *rand.Rand
	storage       []byteKeyValuePair
	missReadBias  uint64
	newWritesBias uint64
	newOvewrites  uint64
}

func generateSlice(result []byte, rand *rand.Rand) {
	rand.Read(result)
}

func generateData(minSize, maxSize int, rand *rand.Rand) []byte {
	size := minSize + rand.Intn(maxSize-minSize)
	result := make([]byte, size)
	rand.Read(result)
	return result
}

func (tdg *tdgImpl) getIndex() (keyType, []byte) {
	var index keyType
	var data []byte
	if uint64(tdg.missGenerator.Intn(100)) < tdg.missReadBias {
		generateSlice(index[:], tdg.dataGenerator)
		data = nil
	} else if len(tdg.storage) > 0 {
		ix := tdg.dataGenerator.Intn(len(tdg.storage))
		index = tdg.storage[ix].key
		data = tdg.storage[ix].value

	}
	return index, data
}

func (tdg *tdgImpl) getIndexAndValue() (keyType, []byte) {
	var index keyType
	var value []byte
	if uint64(tdg.missGenerator.Intn(100)) < tdg.newWritesBias {
		generateSlice(index[:], tdg.dataGenerator)
		value = generateData(tdg.minDataSize, tdg.maxDataSize, tdg.dataGenerator)
		tdg.storage = append(tdg.storage, byteKeyValuePair{index, value})
	} else if len(tdg.storage) > 0 {
		ix := tdg.dataGenerator.Intn(len(tdg.storage))
		stored := tdg.storage[ix]
		if uint64(tdg.missGenerator.Intn(100)) < tdg.newOvewrites {
			stored.value = generateData(tdg.minDataSize, tdg.maxDataSize, tdg.dataGenerator)
		}
		index = stored.key
		value = stored.value
	}
	return index, value
}

func newTestDataGenerator(dataGenerator, missGenerator *rand.Rand, missReadBias, newWritesBias uint64, stored []byteKeyValuePair, minDataSize, maxDataSize int, overwritesBias uint64) *tdgImpl {
	return &tdgImpl{
		minDataSize:   minDataSize,
		maxDataSize:   maxDataSize,
		dataGenerator: dataGenerator,
		missGenerator: missGenerator,
		missReadBias:  missReadBias,
		newWritesBias: newWritesBias,
		newOvewrites:  overwritesBias,
	}
}

func testSyncMap(
	numberOfInitialElements, goRoutinesCount int,
	testsCount, readBias, missReads, newWrites, newOverwrites uint64,
	testMap mapUnderTest,
	b *testing.B) {

	b.StopTimer()

	dataSeedGenerator := rand.New(rand.NewSource(dataGeneratorSeed))
	missSeedGenerator := rand.New(rand.NewSource(missGeneratorSeed))

	dataGenerator := rand.New(rand.NewSource(dataSeedGenerator.Int63()))
	missGenerator := rand.New(rand.NewSource(missSeedGenerator.Int63()))
	tdg := newTestDataGenerator(dataGenerator, missGenerator, 100, 100, []byteKeyValuePair{}, 2*32, 3*32, 0)

	propagateMap(testMap, numberOfInitialElements, tdg)

	startChannel := make(chan struct{})
	wait := sync.WaitGroup{}
	wait.Add(goRoutinesCount)
	initWait := sync.WaitGroup{}
	initWait.Add(goRoutinesCount)

	tester := func(wait *sync.WaitGroup, count uint64, testMap mapUnderTest, startEvent chan struct{}, dataSeed, missSeed int64) {
		defer wait.Done()

		dataGenerator := rand.New(rand.NewSource(dataSeed))
		missGenerator := rand.New(rand.NewSource(missSeed))

		ts := newTestScenario()
		storage := make([]byteKeyValuePair, len(tdg.storage)+int(testsCount))
		copy(storage, tdg.storage)
		testerTdg := newTestDataGenerator(dataGenerator, missGenerator, missReads, newWrites, storage, 2*32, 3*32, newOverwrites)
		initWait.Done()

		<-startEvent

		ts.testMapAcess(testMap, readBias, testsCount, testerTdg)
	}

	for i := 0; i < goRoutinesCount; i++ {
		go tester(&wait, testsCount, testMap, startChannel, dataSeedGenerator.Int63(), missSeedGenerator.Int63())
	}

	initWait.Wait()
	b.StartTimer()
	close(startChannel)
	wait.Wait()
}

type mapUnderTest interface {
	load(keyType) ([]byte, bool)
	store(keyType, []byte)
}

type syncedMap struct {
	sMap sync.Map
}

func (sMap *syncedMap) load(key keyType) ([]byte, bool) {
	value, ok := sMap.sMap.Load(key)
	if !ok {
		return nil, false
	}
	return value.([]byte), true
}

func (sMap *syncedMap) store(key keyType, value []byte) {
	sMap.sMap.Store(key, value)
}

type mapWithMutex struct {
	mu      sync.Mutex
	storage map[keyType][]byte
}

func (mMap *mapWithMutex) load(key keyType) ([]byte, bool) {
	mMap.mu.Lock()
	defer mMap.mu.Unlock()
	value, ok := mMap.storage[key]
	if !ok {
		return nil, false
	}
	return value, ok
}

func (mMap *mapWithMutex) store(key keyType, value []byte) {
	mMap.mu.Lock()
	mMap.storage[key] = value
	mMap.mu.Unlock()
}

func newMapWithMutex() *mapWithMutex {
	return &mapWithMutex{storage: make(map[keyType][]byte)}
}

type mapWithRWMutex struct {
	mu      sync.RWMutex
	storage map[keyType][]byte
}

func (mMap *mapWithRWMutex) load(key keyType) ([]byte, bool) {
	mMap.mu.RLock()
	defer mMap.mu.RUnlock()
	value, ok := mMap.storage[key]
	if !ok {
		return nil, false
	}
	return value, ok
}

func (mMap *mapWithRWMutex) store(key keyType, value []byte) {
	mMap.mu.Lock()
	mMap.storage[key] = value
	mMap.mu.Unlock()
}

func newMapWithRWMutex() *mapWithRWMutex {
	return &mapWithRWMutex{storage: make(map[keyType][]byte)}
}

func propagateMap(storage mapUnderTest, size int, tdg testDataGenerator) {
	values := make(map[keyType]bool)
	for len(values) < size {
		index, value := tdg.getIndexAndValue()
		storage.store(index, value)
		values[index] = false
	}
}

// testMapPerformance executes a benchmark against a given instance of the mapUnderTest interface.
// Parameters:
// testMap: benchmarked instance
// readBias (0-100): how often a read should be performed
// missReads (0-100): how often we are attempting to read a value that is not included in the tested map
// newWrites (0-100): how often we should create a new key
// newOverwrites (0-100): how often we should override data when writing something to testMap
// numberOfRoutines (>=1): number of spawned goroutines
func testMapPerformance(b *testing.B, testMap mapUnderTest, readBias, missReads, newWrites, newOverwrites uint64, numberOfRoutines int) {

	for n := 0; n < b.N; n++ {
		testSyncMap(
			initialElemsCount,
			numberOfRoutines,
			numberOfMapOperations,
			readBias,
			missReads,
			newWrites,
			newOverwrites,
			testMap,
			b)
	}
}

func benchmarkMap(b *testing.B, testMap mapUnderTest) {
	b.Run("more reads", func(b *testing.B) {
		b.ResetTimer()
		testMapPerformance(b, testMap, 90, missReadsBias, newWrites, newOverwrites, numberOfRoutines)
	})

	b.Run("more writes", func(b *testing.B) {
		b.ResetTimer()
		testMapPerformance(b, testMap, 10, missReadsBias, newWrites, newOverwrites, numberOfRoutines)
	})
}

func BenchmarkSyncedMap(b *testing.B) {
	var testMap mapUnderTest = &syncedMap{}
	benchmarkMap(b, testMap)
}

func BenchmarkRWMap(b *testing.B) {
	var testMap mapUnderTest = newMapWithRWMutex()
	benchmarkMap(b, testMap)
}

func BenchmarkMutexMap(b *testing.B) {
	var testMap mapUnderTest = newMapWithMutex()
	benchmarkMap(b, testMap)
}
