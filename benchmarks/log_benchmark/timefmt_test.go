package logbench_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"io"
	"os"
	"time"
)

var _ = Describe("Different time formats", func() {

	var (
		writer io.Writer
		file   *os.File
		err    error
	)

	tick := func() {
		for i := 0; i < 50000; i++ {
			log.Info().Int("counter", i).Str("key", "value").Msg("bench")
		}
	}

	BeforeEach(func() {
		file, err = os.Create(logfile)
		Expect(err).NotTo(HaveOccurred())
		writer = file
	})

	AfterEach(func() {
		file.Close()
	})

	JustBeforeEach(func() {
		log.Logger = zerolog.New(writer).With().Timestamp().Logger()
	})

	Describe("Default format", func() {
		Measure("50k log calls", func(b Benchmarker) {
			b.Time("runtime", tick)
		}, 10)
	})

	Describe("Unix time format", func() {
		BeforeEach(func() {
			zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		})

		Measure("50k log calls", func(b Benchmarker) {
			b.Time("runtime", tick)
		}, 10)
	})

	Describe("Custom integer (milliseconds)", func() {
		BeforeEach(func() {
			// use integer time in milliseconds
			zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
			genesis := time.Now()
			zerolog.TimestampFunc = func() time.Time {
				return time.Unix(int64(time.Since(genesis)/time.Millisecond), 0)
			}
		})

		Measure("50k log calls", func(b Benchmarker) {
			b.Time("runtime", tick)
		}, 10)
	})
})
