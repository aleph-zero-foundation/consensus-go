package map_test

import (
	"bytes"
	"math/rand"
	"sync"
	"testing"
)

const (
	initialElemsCount      = 1000
	numberOfRoutines       = 512
	numberOfMapOperations  = 3 * 10 * 100
	missReadsBias          = 10
	newWrites              = 90
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
	if readBias > 100 {
		panic("readBias argument should be no bigger than 100")
	}

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
		index = stored.key
		value = stored.value
	}
	return index, value
}

func newTestDataGenerator(dataGenerator, missGenerator *rand.Rand, missReadBias, newWritesBias uint64, stored []byteKeyValuePair, minDataSize, maxDataSize int) *tdgImpl {
	return &tdgImpl{
		minDataSize:   minDataSize,
		maxDataSize:   maxDataSize,
		dataGenerator: dataGenerator,
		missGenerator: missGenerator,
		missReadBias:  missReadBias,
		newWritesBias: newWritesBias}
}

func testSyncMap(
	numberOfInitialElements, goRoutinesCount int,
	testsCount, readBias, missReads, newWrites uint64,
	testMap mapUnderTest,
	b *testing.B) {

	b.StopTimer()

	dataSeedGenerator := rand.New(rand.NewSource(dataGeneratorSeed))
	missSeedGenerator := rand.New(rand.NewSource(missGeneratorSeed))

	dataGenerator := rand.New(rand.NewSource(dataSeedGenerator.Int63()))
	missGenerator := rand.New(rand.NewSource(missSeedGenerator.Int63()))
	tdg := newTestDataGenerator(dataGenerator, missGenerator, 100, 100, []byteKeyValuePair{}, 2*32, 3*32)

	propagateMap(testMap, numberOfInitialElements, tdg)

	startChannel := make(chan struct{})
	wait := sync.WaitGroup{}
	wait.Add(goRoutinesCount)

	tester := func(wait *sync.WaitGroup, count uint64, testMap mapUnderTest, startEvent chan struct{}, dataSeed, missSeed int64) {
		defer wait.Done()

		dataGenerator := rand.New(rand.NewSource(dataSeed))
		missGenerator := rand.New(rand.NewSource(missSeed))

		ts := newTestScenario()
		storage := make([]byteKeyValuePair, len(tdg.storage)+int(testsCount))
		copy(storage, tdg.storage)
		testerTdg := newTestDataGenerator(dataGenerator, missGenerator, missReads, newWrites, storage, 2*32, 3*32)

		<-startEvent

		ts.testMapAcess(testMap, readBias, testsCount, testerTdg)

	}

	for i := 0; i < goRoutinesCount; i++ {
		go tester(&wait, testsCount, testMap, startChannel, dataSeedGenerator.Int63(), missSeedGenerator.Int63())
	}

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

func testMapPerformance(b *testing.B, testMap mapUnderTest, readBias, missReads, newWrites uint64) {

	for n := 0; n < b.N; n++ {
		testSyncMap(
			initialElemsCount,
			numberOfRoutines,
			numberOfMapOperations,
			readBias,
			missReads,
			newWrites,
			testMap,
			b)
	}
}

func BenchmarkSyncedMap(b *testing.B) {
	var testMap mapUnderTest = &syncedMap{}
	b.ResetTimer()
	testMapPerformance(b, testMap, 90, missReadsBias, newWrites)
}

func BenchmarkRWMap(b *testing.B) {
	var testMap mapUnderTest = newMapWithRWMutex()
	b.ResetTimer()
	testMapPerformance(b, testMap, 90, missReadsBias, newWrites)
}

func BenchmarkMutexMap(b *testing.B) {
	var testMap mapUnderTest = newMapWithMutex()
	b.ResetTimer()
	testMapPerformance(b, testMap, 90, missReadsBias, newWrites)
}

func BenchmarkSyncedMapMoreWrites(b *testing.B) {
	var testMap mapUnderTest = &syncedMap{}
	b.ResetTimer()
	testMapPerformance(b, testMap, 10, missReadsBias, newWrites)
}

func BenchmarkRWMapMoreWrites(b *testing.B) {
	var testMap mapUnderTest = newMapWithRWMutex()
	b.ResetTimer()
	testMapPerformance(b, testMap, 10, missReadsBias, newWrites)
}

func BenchmarkMutexMapMoreWrites(b *testing.B) {
	var testMap mapUnderTest = newMapWithMutex()
	b.ResetTimer()
	testMapPerformance(b, testMap, 10, missReadsBias, newWrites)
}
