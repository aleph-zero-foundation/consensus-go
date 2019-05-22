package logging

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
)

// LogConfig describes configuration of logger
type LogConfig struct {
	// Log level: 0-debug 1-info 2-warn 3-error 4-fatal 5-panic
	Level int

	// Path to the logfile. "stdout" or "stderr" are possible too.
	Path string

	// The size of diode buffer. 0 disables the diode. Recommended big.
	DiodeBuf int

	// The smallest unit of time (recommended time.Millisecond)
	TimeUnit time.Duration
}

// InitLogger initializes the global zerolog logger based on given LogConfig.
// This function should be called once at the very beginning.
// After that all packages can just import "github.com/rs/zerolog/log" and use it:
// 		log.Info().Int("name", 12).Str("name", "value").Msg("")
func InitLogger(lc LogConfig) error {
	var (
		output io.Writer
		err    error
	)

	switch lc.Path {
	case "stdout":
		output = os.Stdout

	case "stderr":
		output = os.Stderr

	default:
		output, err = os.Create(lc.Path)
		if err != nil {
			return err
		}
	}

	// enable diode
	if lc.DiodeBuf > 0 {
		output = diode.NewWriter(output, lc.DiodeBuf, 0, func(missed int) {
			fmt.Fprintf(os.Stderr, "WARNING: Dropped %d log entries\n", missed)
		})
	}

	log.Logger = zerolog.New(output).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.Level(lc.Level))

	// short names of compulsory fields to save some space
	zerolog.TimestampFieldName = "T"
	zerolog.LevelFieldName = "L"
	zerolog.MessageFieldName = "E"

	// log the beginning of time
	genesis := time.Now()
	log.Log().Msg("genesis")

	// time logged as integer starting at 0, with the chosen unit
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.TimestampFunc = func() time.Time {
		return time.Unix(int64(time.Since(genesis)/lc.TimeUnit), 0)
	}

	// make level names single character
	zerolog.LevelFieldMarshalFunc = func(l zerolog.Level) string {
		return strconv.Itoa(int(l))
	}

	return nil
}
