package logbench_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"sync"
	"time"
)

var _ = Describe("Diode performance", func() {

	var (
		dw     diode.Writer
		writer io.Writer
		file   *os.File
		err    error
		wg     sync.WaitGroup
	)

	tick := func(from, to int) {
		for i := from; i < to; i++ {
			log.Info().Int("counter", i).Str("key", "value").Msg("bench")
		}
	}

	BeforeEach(func() {
		//use integer time in milliseconds
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		genesis := time.Now()
		zerolog.TimestampFunc = func() time.Time {
			return time.Unix(int64(time.Since(genesis)/time.Millisecond), 0)
		}

		file, err = os.Create(logfile)
		Expect(err).NotTo(HaveOccurred())
		writer = file
	})

	JustBeforeEach(func() {
		log.Logger = zerolog.New(writer).With().Timestamp().Logger()
	})

	AfterEach(func() {
		file.Close()
	})

	Describe("Without diode", func() {
		Describe("Serial", func() {
			Measure("50k", func(b Benchmarker) {
				b.Time("runtime", func() {
					tick(0, 50000)
				})
			}, 10)
		})

		Describe("Parallel (5)", func() {
			Measure("50k", func(b Benchmarker) {
				b.Time("runtime", func() {
					wg.Add(5)
					for i := 0; i < 50000; i += 10000 {
						go func(from, to int) {
							tick(from, to)
							wg.Done()
						}(i, i+10000)
					}
					wg.Wait()
				})
			}, 10)
		})

		Describe("Parallel (100)", func() {
			Measure("50k", func(b Benchmarker) {
				b.Time("runtime", func() {
					wg.Add(100)
					for i := 0; i < 50000; i += 500 {
						go func(from, to int) {
							tick(from, to)
							wg.Done()
						}(i, i+500)
					}
					wg.Wait()
				})
			}, 10)
		})
	})

	Describe("With diode", func() {

		BeforeEach(func() {
			dw = diode.NewWriter(writer, 1000000, 0, func(missed int) {})
			writer = dw
			wg = sync.WaitGroup{}
		})

		AfterEach(func() {
			dw.Close()
		})

		Describe("Serial", func() {
			Measure("50k", func(b Benchmarker) {
				b.Time("runtime", func() {
					tick(0, 50000)
				})
			}, 10)
		})

		Describe("Parallel (5)", func() {
			Measure("50k", func(b Benchmarker) {
				b.Time("runtime", func() {
					wg.Add(5)
					for i := 0; i < 50000; i += 10000 {
						go func(from, to int) {
							tick(from, to)
							wg.Done()
						}(i, i+10000)
					}
					wg.Wait()
				})
			}, 10)
		})

		Describe("Parallel (100)", func() {
			Measure("50k", func(b Benchmarker) {
				b.Time("runtime", func() {
					wg.Add(100)
					for i := 0; i < 50000; i += 500 {
						go func(from, to int) {
							tick(from, to)
							wg.Done()
						}(i, i+500)
					}
					wg.Wait()
				})
			}, 10)
		})
	})
})
