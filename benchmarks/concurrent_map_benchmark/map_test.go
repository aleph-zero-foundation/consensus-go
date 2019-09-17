package map_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"sync"
	"testing"

	"github.com/akrylysov/pogreb"
	"github.com/dgraph-io/badger"
	"github.com/syndtr/goleveldb/leveldb"
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

type pogrebDB struct {
	pg *pogreb.DB
}

func (pdb *pogrebDB) load(key keyType) ([]byte, bool) {
	value, err := pdb.pg.Get(key[:])
	if err != nil || value == nil {
		return nil, false
	}
	return value, true
}

func (pdb *pogrebDB) store(key keyType, value []byte) {
	if pdb.pg.Put(key[:], value) != nil {
		panic("unable to store key/value using pogreb")
	}
}

func (pdb *pogrebDB) closeDB() error {
	return pdb.pg.Close()
}

func newPogrebDB(file string) *pogrebDB {
	pg, err := pogreb.Open(file, nil)
	if err != nil {
		return nil
	}
	return &pogrebDB{pg: pg}
}

type levelDB struct {
	levelDB *leveldb.DB
}

func (lvl *levelDB) load(key keyType) ([]byte, bool) {
	value, err := lvl.levelDB.Get(key[:], nil)
	if err != nil || value == nil {
		return nil, false
	}
	return value, true
}

func (lvl *levelDB) store(key keyType, value []byte) {
	if lvl.levelDB.Put(key[:], value, nil) != nil {
		panic("unable to store key/value using levelDB")
	}
}

func (lvl *levelDB) closeDB() error {
	return lvl.levelDB.Close()
}

func newLevelDB(file string) (*levelDB, error) {
	levelDBVal, err := leveldb.OpenFile(file, nil)
	if err != nil {
		return nil, err
	}
	return &levelDB{levelDB: levelDBVal}, nil
}

type badgerDB struct {
	badger *badger.DB
}

func (bdg *badgerDB) load(key keyType) ([]byte, bool) {
	var result []byte
	var notFound bool
	err := bdg.badger.View(func(txn *badger.Txn) error {
		value, err := txn.Get(key[:])
		if err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
			notFound = true
			return nil
		}
		// return value.Value(func(val []byte) error {
		// 	result = val
		// 	return nil
		// })
		result, err = value.ValueCopy(nil)
		return err
	})
	if err != nil {
		panic("error while loading a value in badgerDB")
	}
	return result, !notFound
}

func (bdg *badgerDB) store(key keyType, value []byte) {
	err := bdg.badger.Update(func(txn *badger.Txn) error {
		return txn.Set(key[:], value)
	})
	if err != nil {
		panic("unable to store a value using badgerDB")
	}
}

func (bdg *badgerDB) closeDB() error {
	return bdg.badger.Close()
}

func newBadgerDB(folder string) (*badgerDB, error) {
	db, err := badger.Open(badger.DefaultOptions(folder))
	if err != nil {
		return nil, err
	}
	return &badgerDB{badger: db}, nil
}

func propagateMap(storage mapUnderTest, size int, tdg testDataGenerator) {
	values := make(map[keyType]bool)
	for len(values) < size {
		index, value := tdg.getIndexAndValue()
		storage.store(index, value)
		values[index] = false
	}
}

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

func benchmarkPogrebDB(b *testing.B, readBias uint64) {
	file, err := ioutil.TempFile("", "pogrebDB")
	if err != nil {
		panic("unable to create temporary file for the pogreb database")
	}
	testMap := newPogrebDB(file.Name())
	b.ResetTimer()
	testMapPerformance(b, testMap, readBias, missReadsBias, newWrites, newOverwrites, 1)
	if err := testMap.closeDB(); err != nil {
		panic(fmt.Sprintf("unable to close pogrebDB: %s", err.Error()))
	}
	if err := file.Close(); err != nil {
		panic(fmt.Sprintf("unable to close file of pogrebDB: %s", err.Error()))
	}
}

func BenchmarkPogrebDB(b *testing.B) {
	b.Run("more reads", func(b *testing.B) {
		benchmarkPogrebDB(b, 90)
	})
	b.Run("more writes", func(b *testing.B) {
		benchmarkPogrebDB(b, 10)
	})
}

func benchmarkLevelDB(b *testing.B, readBias uint64) {
	dbFolder, err := ioutil.TempDir("", "levelDB")
	if err != nil {
		panic("unable to create temporary file for the levelDB database")
	}
	testMap, err := newLevelDB(dbFolder)
	if err != nil {
		panic(fmt.Sprintf("unable to initilize levelDB: %s", err.Error()))
	}
	b.ResetTimer()
	testMapPerformance(b, testMap, 90, missReadsBias, newWrites, newOverwrites, 1)
	if err := testMap.closeDB(); err != nil {
		panic(fmt.Sprintf("unable to close levelDB: %s", err.Error()))
	}
}

func BenchmarkLevelDB(b *testing.B) {
	b.Run("more reads", func(b *testing.B) {
		benchmarkLevelDB(b, 90)
	})
	b.Run("more writes", func(b *testing.B) {
		benchmarkLevelDB(b, 10)
	})
}

func benchmarkBadgerDB(b *testing.B, readBias uint64) {
	dbFolder, err := ioutil.TempDir("", "badgerDB")
	if err != nil {
		panic("unable to create temporary file for the badgerDB database")
	}
	testMap, err := newBadgerDB(dbFolder)
	if err != nil {
		panic(fmt.Sprintf("unable to initilize badgerDB: %s", err.Error()))
	}
	b.ResetTimer()
	testMapPerformance(b, testMap, readBias, missReadsBias, newWrites, newOverwrites, 1)
	if err := testMap.closeDB(); err != nil {
		panic(fmt.Sprintf("unable to close badgerDB: %s", err.Error()))
	}
}

func BenchmarkBadgerDB(b *testing.B) {
	b.Run("more reads", func(b *testing.B) {
		benchmarkBadgerDB(b, 90)
	})
	b.Run("more writes", func(b *testing.B) {
		benchmarkBadgerDB(b, 10)
	})
}
