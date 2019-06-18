package main

import (
	"flag"
	"fmt"
	"os"

	gomel "gitlab.com/alephledger/consensus-go/pkg"
	"gitlab.com/alephledger/consensus-go/pkg/growing"
	"gitlab.com/alephledger/consensus-go/pkg/tests"
)

type posetFactory struct{}

func (posetFactory) CreatePoset(pc gomel.PosetConfig) gomel.Poset {
	return growing.NewPoset(&pc)
}

type cliOptions struct {
	posetFilename string
}

func getOptions() cliOptions {
	var result cliOptions
	flag.StringVar(&result.posetFilename, "poset", "", "a file containing the poset to analyze")
	flag.Parse()
	return result
}

// collectUnits for a given poset returns a slice containing all the units from the poset.
// It uses dfs from maximal units.
func collectUnits(p gomel.Poset) []gomel.Unit {
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
	p.MaximalUnitsPerProcess().Iterate(func(units []gomel.Unit) bool {
		for _, u := range units {
			if !seenUnits[*u.Hash()] {
				dfs(u)
			}
		}
		return true
	})
	return units
}

// popularityStats for a given poset calculates for each prime unit
// the number of levels until the unit becomes popular.
// Unpopular units are ignored. The result is sliced by the prime level.
func popularityStats(p gomel.Poset, maxLevel int) [][]int {
	result := make([][]int, maxLevel+1)
	for level := 0; level <= maxLevel; level++ {
		primes := p.PrimeUnits(level)
		primes.Iterate(func(units []gomel.Unit) bool {
			for _, u := range units {
				for up := 0; up+level <= maxLevel; up++ {
					primesAbove := p.PrimeUnits(level + up)
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

// getPrimeUnitStatsOnLevel for a given poset and a given level calculates
// (1) number of prime units on the level
// (2) number of minimal (in DAG order) prime units on the level
// (3) for each prime unit number of primes on level - 1 which are below the unit
func getPrimeUnitStatsOnLevel(p gomel.Poset, level int) levelPrimeUnitStat {
	primes := p.PrimeUnits(level)
	primesBelow := p.PrimeUnits(level - 1)
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

// getPrimeUnitsStats for a given poset caluclates primeUnitStats on each level
func getPrimeUnitsStats(p gomel.Poset, maxLevel int) []levelPrimeUnitStat {
	result := []levelPrimeUnitStat{}
	for level := 0; level <= maxLevel; level++ {
		result = append(result, getPrimeUnitStatsOnLevel(p, level))
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
	skipped int
}

// getUnitStats for a given poset calculates for each level the levelUnitStat i.e.
// (1) the number of prime units on the level
// (2) the number of regular (not prime) units on the level
// (3) the number of processes which skipped the level
func getUnitStats(p gomel.Poset, units []gomel.Unit, maxLevel int) []levelUnitStat {
	result := make([]levelUnitStat, p.NProc())
	pSeen := make([]map[int]bool, maxLevel+1)
	for level := 0; level <= maxLevel; level++ {
		pSeen[level] = make(map[int]bool)
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
		result[level].skipped = p.NProc() - len(pSeen[level])
	}
	return result
}

func main() {
	options := getOptions()
	var pf posetFactory
	poset, err := tests.CreatePosetFromTestFile(options.posetFilename, pf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading poset %s: %s\n", options.posetFilename, err.Error())
		return
	}
	units := collectUnits(poset)
	maxLevel := 0
	for _, u := range units {
		if u.Level() > maxLevel {
			maxLevel = u.Level()
		}
	}
	fmt.Printf("=========================General stats========================\n\n")
	fmt.Printf("%-12s%-10d\n%-12s%-10d\n%-12s%-10d\n\n", "Processes", poset.NProc(), "Units", len(units), "Max level", maxLevel)
	fmt.Printf("=========================Units stats========================\n\n")
	us := getUnitStats(poset, units, maxLevel)
	for level := 0; level <= maxLevel; level++ {
		fmt.Printf("%-12s%-10d\n\n\t%-12s%-10d\n\t%-12s%-10d\n\t%-12s%-10d\n\n", "level", level, "primes", us[level].primes, "regular", us[level].regular, "skipped", us[level].skipped)
	}

	fmt.Printf("=========================Primes stats========================\n\n")
	pus := getPrimeUnitsStats(poset, maxLevel)
	for level := 0; level <= maxLevel; level++ {
		fmt.Printf("%-12s%-10d\n\n\t%-12s%-10d\n\t%-12s%-10d\n\n", "level", level, "primes", pus[level].primes, "minprimes", pus[level].minPrimes)
		bs := computeBasicStats(pus[level].visibleBelow)
		fmt.Printf("\t%-12s\n\t  %-12s%-10d\n\t  %-12s%-10d\n\t  %-12s%-6.4f\n\n", "visible below", "min", bs.min, "max", bs.max, "avg", bs.avg)
	}
	fmt.Printf("=========================Popularity stats========================\n\n")
	ps := popularityStats(poset, maxLevel)
	for level := 0; level <= maxLevel; level++ {
		fmt.Printf("%-12s%-10d\n\n\t%-12s%-10d\n\t%-12s%-10d\n\n", "level", level, "popular", len(ps[level]), "unpopular", pus[level].primes-len(ps[level]))
		bs := computeBasicStats(ps[level])
		fmt.Printf("\t%-12s\n\t  %-12s%-10d\n\t  %-12s%-10d\n\t  %-12s%-6.4f\n\n", "popular after", "min", bs.min, "max", bs.max, "avg", bs.avg)
	}
}
