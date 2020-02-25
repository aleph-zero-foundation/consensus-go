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

	"gitlab.com/alephledger/consensus-go/pkg/config"
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
func NewLogger(conf config.Config) (zerolog.Logger, error) {
	var output io.Writer

	output, err := os.Create(conf.LogFile)
	if err != nil {
		return zerolog.Logger{}.Level(zerolog.Disabled), err
	}

	// enable decoder
	if conf.LogHuman {
		output = NewDecoder(output)
	}

	// enable diode
	if conf.LogBuffer > 0 {
		output = diode.NewWriter(output, conf.LogBuffer, 0, func(missed int) {
			fmt.Fprintf(os.Stderr, "WARNING: Dropped %d log entries\n", missed)
		})
	}

	log := zerolog.New(output).With().Timestamp().Logger().Level(zerolog.Level(conf.LogLevel))
	log.Log().Str(Genesis, genesis.Format(time.RFC1123Z)).Msg(Genesis)

	return log, nil
}
