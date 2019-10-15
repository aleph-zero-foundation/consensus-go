// Package logging sets up logging for gomel.
//
// It also contains decoders for translating logfile into human readable form.
package logging

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
)

var genesis time.Time

func init() {
	// store the beginning of time
	genesis = time.Now()

	// short names of compulsory fields to save some space
	zerolog.TimestampFieldName = Time
	zerolog.LevelFieldName = Level
	zerolog.MessageFieldName = Event

	// time logged as integer starting at 0, with the chosen unit
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.TimestampFunc = func() time.Time {
		return time.Unix(int64(time.Since(genesis)/time.Millisecond), 0)
	}

	// make level names single character
	zerolog.LevelFieldMarshalFunc = func(l zerolog.Level) string {
		return strconv.Itoa(int(l))
	}

}

// NewLogger creates a new zerolog logger based on the given configuration values.
func NewLogger(path string, level, diodeBuf int, humanReadable bool) (zerolog.Logger, error) {
	var (
		output io.Writer
		err    error
	)

	switch path {
	case "stdout":
		output = os.Stdout

	case "stderr":
		output = os.Stderr

	default:
		output, err = os.Create(path)
		if err != nil {
			return zerolog.Logger{}.Level(zerolog.Disabled), err
		}
	}

	// enable decoder
	if humanReadable {
		output = NewDecoder(output)
	}

	// enable diode
	if diodeBuf > 0 {
		output = diode.NewWriter(output, diodeBuf, 0, func(missed int) {
			fmt.Fprintf(os.Stderr, "WARNING: Dropped %d log entries\n", missed)
		})
	}

	log := zerolog.New(output).With().Timestamp().Logger().Level(zerolog.Level(level))
	log.Log().Str(Genesis, genesis.Format(time.RFC1123Z)).Msg(Genesis)

	return log, nil
}
