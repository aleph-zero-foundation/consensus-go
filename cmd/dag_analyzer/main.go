package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type dagStats struct {
	NProc                  uint16
	NUnits                 int
	Level                  StatAggregated
	NParents               StatAggregated
	PopularAfter           StatAggregated
	NParentsOnTheSameLevel StatAggregated
	IsPrime                StatAggregated
}

type stat func(dag gomel.Dag, u gomel.Unit, units []gomel.Unit, maxLevel int) int

var level stat = func(_ gomel.Dag, u gomel.Unit, _ []gomel.Unit, _ int) int {
	return u.Level()
}

var popularAfter stat = func(dag gomel.Dag, u gomel.Unit, _ []gomel.Unit, maxLevel int) int {
	level := u.Level()
	for up := 0; up+level <= maxLevel; up++ {
		primesAbove := dag.PrimeUnits(level + up)
		ok := true
		primesAbove.Iterate(func(prs []gomel.Unit) bool {
			for _, v := range prs {
				if !gomel.Above(v, u) {
					ok = false
					return false
				}
			}
			return true
		})
		if ok {
			return up
		}
	}
	return -1
}

var nParentsOnTheSameLevel stat = func(_ gomel.Dag, u gomel.Unit, _ []gomel.Unit, _ int) int {
	result := 0
	for _, v := range u.Parents() {
		if v != nil && u.Level() == v.Level() {
			result++
		}
	}
	return result
}

var isPrime stat = func(_ gomel.Dag, u gomel.Unit, _ []gomel.Unit, _ int) int {
	if gomel.Prime(u) {
		return 1
	}
	return 0
}

func computeStats(dag gomel.Dag, units []gomel.Unit, maxLevel int) *dagStats {
	return &dagStats{
		NProc:                  dag.NProc(),
		NUnits:                 len(units),
		Level:                  aggregate(level, dag, units, maxLevel),
		PopularAfter:           aggregate(popularAfter, dag, units, maxLevel),
		NParentsOnTheSameLevel: aggregate(nParentsOnTheSameLevel, dag, units, maxLevel),
		IsPrime:                aggregate(isPrime, dag, units, maxLevel),
	}
}

// StatAnalyzed represents basic statistics of slice of ints
type StatAnalyzed struct {
	Distribution map[int]int
	Min          int
	Max          int
	Avg          float64
}

// StatAggregated represents basic statistics without aggregation, aggregated by pid and aggregated by level
type StatAggregated struct {
	Overall  StatAnalyzed
	PerProc  []StatAnalyzed
	PerLevel []StatAnalyzed
}

func aggregate(stat stat, dag gomel.Dag, units []gomel.Unit, maxLevel int) StatAggregated {
	values := []int{}
	valuesPerPid := make([][]int, dag.NProc())
	valuesPerLevel := make([][]int, maxLevel+1)

	for _, u := range units {
		v := stat(dag, u, units, maxLevel)
		values = append(values, v)
		valuesPerPid[u.Creator()] = append(valuesPerPid[u.Creator()], v)
		valuesPerLevel[u.Level()] = append(valuesPerLevel[u.Level()], v)
	}

	overall := analyze(values)
	perProc := make([]StatAnalyzed, dag.NProc())
	for i := uint16(0); i < dag.NProc(); i++ {
		perProc[i] = analyze(valuesPerPid[i])
	}
	perLevel := make([]StatAnalyzed, maxLevel+1)
	for i := 0; i <= maxLevel; i++ {
		perLevel[i] = analyze(valuesPerLevel[i])
	}

	return StatAggregated{
		Overall:  overall,
		PerProc:  perProc,
		PerLevel: perLevel,
	}
}

func analyze(values []int) StatAnalyzed {
	size := len(values)
	if size == 0 {
		return StatAnalyzed{}
	}
	sum := 0
	min := values[0]
	max := values[0]
	distribution := map[int]int{}

	for _, x := range values {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
		sum += x
		distribution[x]++
	}

	return StatAnalyzed{
		Min:          min,
		Max:          max,
		Avg:          float64(sum) / float64(size),
		Distribution: distribution,
	}
}

func storeStats(writer io.Writer, stats *dagStats) error {
	return json.NewEncoder(writer).Encode(*stats)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: dag_analyzer <dag_file>\n")
		return
	}
	filename := os.Args[1]
	var df dagFactory
	dag, _, err := tests.CreateDagFromTestFile(filename, df)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading dag %s: %s\n", filename, err.Error())
		return
	}

	units := collectUnits(dag)
	maxLevel := 0
	for _, u := range units {
		if u.Level() > maxLevel {
			maxLevel = u.Level()
		}
	}
	stats := computeStats(dag, units, maxLevel)
	storeStats(os.Stdout, stats)
}

type dagFactory struct{}

func (dagFactory) CreateDag(nProc uint16) (gomel.Dag, gomel.Adder) {
	d := dag.New(nProc)
	return d, tests.NewAdder(d)
}

func collectUnits(dag gomel.Dag) []gomel.Unit {
	seenUnits := make(map[gomel.Hash]bool)
	units := []gomel.Unit{}
	var dfs func(gomel.Unit)
	dfs = func(u gomel.Unit) {
		units = append(units, u)
		seenUnits[*u.Hash()] = true
		for _, v := range u.Parents() {
			if v == nil {
				continue
			}
			if !seenUnits[*v.Hash()] {
				dfs(v)
			}
		}
	}
	dag.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			if !seenUnits[*u.Hash()] {
				dfs(u)
			}
		}
		return true
	})
	return units
}
