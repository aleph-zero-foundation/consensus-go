package map_test

import (
	"math/rand"
	"sync"
	"testing"
)

const (
	initialElemsCount      = 100000
	numberOfRoutines       = 512
	numberOfMapOperations  = 3 * 10 * 100
	missReadsBias          = 10
	newWrites              = 90
	operationGeneratorSeed = 0
	dataGeneratorSeed      = 1
	missGeneratorSeed      = 2
)

type keyValuePair struct {
	key   uint64
	value uint64
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
			testMap.load(tdg.getIndex())
		} else {
			index, value := tdg.getIndexAndValue()
			testMap.store(index, value)
		}
	}
}

type testDataGenerator interface {
	getIndex() uint64
	getIndexAndValue() (uint64, uint64)
}

type tdgImpl struct {
	dataGenerator *rand.Rand
	missGenerator *rand.Rand
	storage       []keyValuePair
	missReadBias  uint64
	newWritesBias uint64
}

func (tdg *tdgImpl) getIndex() uint64 {
	var index uint64
	if uint64(tdg.missGenerator.Intn(100)) < tdg.missReadBias {
		index = tdg.dataGenerator.Uint64()
	} else if len(tdg.storage) > 0 {
		ix := tdg.dataGenerator.Intn(len(tdg.storage))
		index = tdg.storage[ix].key
	}
	return index
}

func (tdg *tdgImpl) getIndexAndValue() (uint64, uint64) {
	var index, value uint64
	if uint64(tdg.missGenerator.Intn(100)) < tdg.newWritesBias {
		index = tdg.dataGenerator.Uint64()
		value = tdg.dataGenerator.Uint64()
	} else if len(tdg.storage) > 0 {
		ix := tdg.dataGenerator.Intn(len(tdg.storage))
		stored := tdg.storage[ix]
		index = stored.key
		value = stored.value
	}
	tdg.storage = append(tdg.storage, keyValuePair{index, value})
	return index, value
}

func newTestDataGenerator(dataGenerator, missGenerator *rand.Rand, missReadBias, newWritesBias uint64, stored []keyValuePair) *tdgImpl {
	return &tdgImpl{
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
	tdg := newTestDataGenerator(dataGenerator, missGenerator, 100, 100, []keyValuePair{})

	propagateMap(testMap, numberOfInitialElements, tdg)

	var startChannel chan struct{} = make(chan struct{})
	wait := sync.WaitGroup{}
	wait.Add(goRoutinesCount)

	tester := func(wait *sync.WaitGroup, count uint64, testMap mapUnderTest, startEvent chan struct{}, dataSeed, missSeed int64) {
		defer wait.Done()

		dataGenerator := rand.New(rand.NewSource(dataSeed))
		missGenerator := rand.New(rand.NewSource(missSeed))

		ts := newTestScenario()
		storage := make([]keyValuePair, len(tdg.storage)+int(testsCount))
		copy(storage, tdg.storage)
		testerTdg := newTestDataGenerator(dataGenerator, missGenerator, missReads, newWrites, storage)

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
	load(uint64) (uint64, bool)
	store(uint64, uint64)
}

type syncedMap struct {
	sMap sync.Map
}

func (sMap *syncedMap) load(key uint64) (uint64, bool) {
	value, ok := sMap.sMap.Load(key)
	if !ok {
		return 0, false
	}
	return value.(uint64), true
}

func (sMap *syncedMap) store(key uint64, value uint64) {
	sMap.sMap.Store(key, value)
}

type mapWithMutex struct {
	mu      sync.Mutex
	storage map[uint64]uint64
}

func (mMap *mapWithMutex) load(key uint64) (uint64, bool) {
	mMap.mu.Lock()
	defer mMap.mu.Unlock()
	value, ok := mMap.storage[key]
	if !ok {
		return 0, false
	}
	return value, ok
}

func (mMap *mapWithMutex) store(key uint64, value uint64) {
	mMap.mu.Lock()
	mMap.storage[key] = value
	mMap.mu.Unlock()
}

func newMapWithMutex() *mapWithMutex {
	return &mapWithMutex{storage: make(map[uint64]uint64)}
}

type mapWithRWMutex struct {
	mu      sync.RWMutex
	storage map[uint64]uint64
}

func (mMap *mapWithRWMutex) load(key uint64) (uint64, bool) {
	mMap.mu.RLock()
	defer mMap.mu.RUnlock()
	value, ok := mMap.storage[key]
	if !ok {
		return 0, false
	}
	return value, ok
}

func (mMap *mapWithRWMutex) store(key uint64, value uint64) {
	mMap.mu.Lock()
	mMap.storage[key] = value
	mMap.mu.Unlock()
}

func newMapWithRWMutex() *mapWithRWMutex {
	return &mapWithRWMutex{storage: make(map[uint64]uint64)}
}

func propagateMap(storage mapUnderTest, size int, tdg testDataGenerator) {
	values := make(map[uint64]uint64)
	for len(values) < size {
		index, value := tdg.getIndexAndValue()
		storage.store(index, value)
		values[index] = value
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
