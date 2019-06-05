package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/rs/zerolog"
)

type decoder struct {
	writer io.Writer
}

// NewDecoder creates a decoder that writes to the given writer.
// Decoder implements io.Writer that takes []bytes with single
// log event in JSON and writes it in human readable form
func NewDecoder(writer io.Writer) io.Writer {
	return &decoder{writer: writer}
}

func (d *decoder) Write(p []byte) (n int, err error) {
	var data map[string]interface{}
	err = json.Unmarshal(p, &data)
	if err != nil {
		return 0, err
	}
	res := decode(&data)
	_, err = d.writer.Write([]byte(res))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func decode(data *map[string]interface{}) string {
	if event, ok := (*data)[Event]; ok && event == Genesis {
		return fmt.Sprintln("Beginning of time at ", (*data)[Genesis])
	}
	ret := ""
	if val, ok := (*data)[Time]; ok {
		ret += fmt.Sprintf("%6v|", val)
	}
	if val, ok := (*data)[Level]; ok {
		i, _ := strconv.Atoi(val.(string))
		ret += fmt.Sprintf("%5v|", zerolog.Level(i))
	}
	if val, ok := (*data)[Service]; ok {
		ret += fmt.Sprintf("%s:%7v|", fieldNameDict[Service], serviceTypeDict[int(val.(float64))])
	}
	for k, v := range *data {
		if k == Time || k == Service || k == Event || k == Level {
			continue
		}
		if f, ok := fieldNameDict[k]; ok {
			ret += fmt.Sprintf("%8s = %-6v|", f, v)
		} else {
			ret += fmt.Sprintf("%8s = %-6v|", k, v)
		}
	}
	if val, ok := (*data)[Event]; ok {
		s := val.(string)
		if _, in := eventTypeDict[s]; in {
			s = eventTypeDict[s]
		}
		ret += fmt.Sprintln("  " + s)
	}
	return ret
}
