package main

import (
	"flag"
	"fmt"
	"os"

	"gitlab.com/alephledger/consensus-go/pkg/dag"
	"gitlab.com/alephledger/consensus-go/pkg/gomel"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type dagFactory struct{}

func (dagFactory) CreateDag(dc gomel.DagConfig) gomel.Dag {
	return dag.New(uint16(len(dc.Keys)))
}

type cliOptions struct {
	dagFilename string
}

func getOptions() cliOptions {
	var result cliOptions
	flag.StringVar(&result.dagFilename, "dag", "", "a file containing the dag to analyze")
	flag.Parse()
	return result
}

// collectUnits for a given dag returns a slice containing all the units from the dag.
// It uses dfs from maximal units.
func collectUnits(dag gomel.Dag) []gomel.Unit {
	seenUnits := make(map[gomel.Hash]bool)
	units := []gomel.Unit{}
	var dfs func(gomel.Unit)
	dfs = func(u gomel.Unit) {
		units = append(units, u)
		seenUnits[*u.Hash()] = true
		for _, v := range u.Parents() {
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

// popularityStats for a given dag calculates for each prime unit
// the number of levels until the unit becomes popular.
// Unpopular units are ignored. The result is sliced by the prime level.
func popularityStats(dag gomel.Dag, maxLevel int) [][]int {
	result := make([][]int, maxLevel+1)
	for level := 0; level <= maxLevel; level++ {
		primes := dag.PrimeUnits(level)
		primes.Iterate(func(units []gomel.Unit) bool {
			for _, u := range units {
				for up := 0; up+level <= maxLevel; up++ {
					primesAbove := dag.PrimeUnits(level + up)
					ok := true
					primesAbove.Iterate(func(prs []gomel.Unit) bool {
						for _, v := range prs {
							if !u.Below(v) {
								ok = false
								return false
							}
						}
						return true
					})
					if ok {
						result[level] = append(result[level], up)
						break
					}
				}
			}
			return true
		})
	}
	return result
}

// levelPrimeUnitStat contains statistics per level
// (1) number of prime units
// (2) number of minimal (in DAG order) prime units
// (3) number of primes visible below each prime
type levelPrimeUnitStat struct {
	primes       int
	minPrimes    int
	visibleBelow []int
}

// basicStats contains basic stats of some sequence of integers
type basicStats struct {
	size int
	min  int
	max  int
	avg  float64
}

// computeBasicStats for a given slice of integers computes
// (0) length of the slice
// (1) min value
// (2) max value
// (3) avarge
func computeBasicStats(slice []int) basicStats {
	size := len(slice)
	if size == 0 {
		return basicStats{
			size: 0,
		}
	}
	sum := 0
	min := slice[0]
	max := slice[0]
	for _, x := range slice {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
		sum += x
	}

	return basicStats{
		size: size,
		min:  min,
		max:  max,
		avg:  float64(sum) / float64(size),
	}
}

// cntVisibleBelow for a given unit u, and a collection su of SlottedUnits calculates the number
// of units which are elemnts of su and are below of u.
func cntVisibleBelow(u gomel.Unit, su gomel.SlottedUnits) int {
	ans := 0
	su.Iterate(func(units []gomel.Unit) bool {
		for _, v := range units {
			if v.Below(u) {
				ans++
			}
		}
		return true
	})
	return ans
}

// getPrimeUnitStatsOnLevel for a given dag and a given level calculates
// (1) number of prime units on the level
// (2) number of minimal (in DAG order) prime units on the level
// (3) for each prime unit number of primes on level - 1 which are below the unit
func getPrimeUnitStatsOnLevel(dag gomel.Dag, level int) levelPrimeUnitStat {
	primes := dag.PrimeUnits(level)
	primesBelow := dag.PrimeUnits(level - 1)
	var lps levelPrimeUnitStat
	primes.Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			lps.primes++
			isMinimal := true
			for _, par := range u.Parents() {
				if par.Level() == u.Level() {
					isMinimal = false
					break
				}
			}
			if isMinimal {
				lps.minPrimes++
			}
			lps.visibleBelow = append(lps.visibleBelow, cntVisibleBelow(u, primesBelow))
		}
		return true
	})
	return lps
}

// getPrimeUnitsStats for a given dag caluclates primeUnitStats on each level
func getPrimeUnitsStats(dag gomel.Dag, maxLevel int) []levelPrimeUnitStat {
	result := []levelPrimeUnitStat{}
	for level := 0; level <= maxLevel; level++ {
		result = append(result, getPrimeUnitStatsOnLevel(dag, level))
	}
	return result
}

// levelUnitStat contains
// (1) the number of prime units on the level
// (2) the number of regular (not prime) units on the level
// (3) the number of processes which skipped the level
type levelUnitStat struct {
	primes  int
	regular int
	skipped uint16
}

// getUnitStats for a given dag calculates for each level the levelUnitStat i.e.
// (1) the number of prime units on the level
// (2) the number of regular (not prime) units on the level
// (3) the number of processes which skipped the level
func getUnitStats(dag gomel.Dag, units []gomel.Unit, maxLevel int) []levelUnitStat {
	result := make([]levelUnitStat, dag.NProc())
	pSeen := make([]map[uint16]bool, maxLevel+1)
	for level := 0; level <= maxLevel; level++ {
		pSeen[level] = make(map[uint16]bool)
	}
	for _, u := range units {
		if gomel.Prime(u) {
			result[u.Level()].primes++
		} else {
			result[u.Level()].regular++
		}
		pSeen[u.Level()][u.Creator()] = true
	}
	for level := 0; level <= maxLevel; level++ {
		result[level].skipped = dag.NProc() - uint16(len(pSeen[level]))
	}
	return result
}

func main() {
	options := getOptions()
	var df dagFactory
	dag, err := tests.CreateDagFromTestFile(options.dagFilename, df)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading dag %s: %s\n", options.dagFilename, err.Error())
		return
	}
	units := collectUnits(dag)
	maxLevel := 0
	for _, u := range units {
		if u.Level() > maxLevel {
			maxLevel = u.Level()
		}
	}
	fmt.Printf("=========================General stats========================\n\n")
	fmt.Printf("%-12s%-10d\n%-12s%-10d\n%-12s%-10d\n\n", "Processes", dag.NProc(), "Units", len(units), "Max level", maxLevel)
	fmt.Printf("=========================Units stats========================\n\n")
	us := getUnitStats(dag, units, maxLevel)
	for level := 0; level <= maxLevel; level++ {
		fmt.Printf("%-12s%-10d\n\n\t%-12s%-10d\n\t%-12s%-10d\n\t%-12s%-10d\n\n", "level", level, "primes", us[level].primes, "regular", us[level].regular, "skipped", us[level].skipped)
	}

	fmt.Printf("=========================Primes stats========================\n\n")
	pus := getPrimeUnitsStats(dag, maxLevel)
	for level := 0; level <= maxLevel; level++ {
		fmt.Printf("%-12s%-10d\n\n\t%-12s%-10d\n\t%-12s%-10d\n\n", "level", level, "primes", pus[level].primes, "minprimes", pus[level].minPrimes)
		bs := computeBasicStats(pus[level].visibleBelow)
		fmt.Printf("\t%-12s\n\t  %-12s%-10d\n\t  %-12s%-10d\n\t  %-12s%-6.4f\n\n", "visible below", "min", bs.min, "max", bs.max, "avg", bs.avg)
	}
	fmt.Printf("=========================Popularity stats========================\n\n")
	ps := popularityStats(dag, maxLevel)
	for level := 0; level <= maxLevel; level++ {
		fmt.Printf("%-12s%-10d\n\n\t%-12s%-10d\n\t%-12s%-10d\n\n", "level", level, "popular", len(ps[level]), "unpopular", pus[level].primes-len(ps[level]))
		bs := computeBasicStats(ps[level])
		fmt.Printf("\t%-12s\n\t  %-12s%-10d\n\t  %-12s%-10d\n\t  %-12s%-6.4f\n\n", "popular after", "min", bs.min, "max", bs.max, "avg", bs.avg)
	}
}
