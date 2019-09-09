package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	createDelayMin = 400
	createDelayMax = 600
)

var (
	syncTimeMin int
	syncTimeMax int
	nProc       uint16
	workers     int
	maxUnits    int
)

type sendRequest struct {
	unit      []byte
	height    int
	pid       uint16
	timestamp time.Time
}

func wait(a, b int) {
	d := a + rand.Intn(b-a)
	time.Sleep(time.Duration(d) * time.Millisecond)
}

func createUnit() []byte {
	size := 300 + rand.Intn(500)
	unit := make([]byte, size)
	_, err := rand.Read(unit)
	if err != nil {
		panic("createUnit")
	}
	return unit
}

func createService(requests chan<- sendRequest, wg *sync.WaitGroup) {
	defer wg.Done()
	for counter := 0; counter < maxUnits; counter++ {
		unit := createUnit()
		timestamp := time.Now()
		for i := uint16(0); i < nProc; i++ {
			requests <- sendRequest{unit, counter, i, timestamp}
		}
		fmt.Print(counter, " ")
		wait(createDelayMin, createDelayMax)
	}
	close(requests)
}

func syncService(nWorkers int, timings [][]time.Duration, requests <-chan sendRequest, wg *sync.WaitGroup) {
	for i := 0; i < nWorkers; i++ {
		go func() {
			defer wg.Done()
			for {
				request, ok := <-requests
				if !ok {
					return
				}
				timings[request.height][request.pid] = time.Since(request.timestamp)
				wait(syncTimeMin, syncTimeMax)
			}
		}()
	}

}

func main() {
	var n = flag.Int("n", 128, "number of processes")
	var w = flag.Int("w", -1, "number of workers")
	var u = flag.Int("u", 20, "number of units")
	var s = flag.Int("s", 150, "sync time")
	flag.Parse()

	nProc = uint16(*n)
	if *w == -1 {
		workers = 4 * int(nProc)
	} else {
		workers = *w
	}
	maxUnits = *u
	syncTimeMin = *s - 50
	syncTimeMax = *s + 50

	var wg sync.WaitGroup
	var timings [][]time.Duration
	timings = make([][]time.Duration, maxUnits)
	for i := 0; i < maxUnits; i++ {
		timings[i] = make([]time.Duration, nProc)
	}
	requests := make(chan sendRequest, 20*nProc)
	wg.Add(1 + workers)
	go syncService(workers, timings, requests, &wg)
	go createService(requests, &wg)
	wg.Wait()
	fmt.Println("\nUnit     Max delay     Avg delay")
	total := time.Duration(0)
	for i := 0; i < maxUnits; i++ {
		sum := time.Duration(0)
		max := time.Duration(0)
		for j := uint16(0); j < nProc; j++ {
			if timings[i][j] > max {
				max = timings[i][j]
			}
			sum += timings[i][j]
		}
		fmt.Printf("%3d     %10v   %10v\n", i, max, sum/time.Duration(nProc))
		if i > 0 {
			total += sum
		}
	}
	fmt.Printf("Global avg w/o level 0:  %10v\n", total/time.Duration(int(nProc)*(maxUnits-1)))
}
