package map_test

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/akrylysov/pogreb"
	"github.com/dgraph-io/badger"
	"github.com/syndtr/goleveldb/leveldb"
)

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
