package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/rs/zerolog"
)

type decoder struct {
	writer io.Writer
}

// NewDecoder creates a decoder that writes to the given writer.
// Decoder implements io.Writer that takes []bytes with a single
// log event in JSON and writes it in human readable form.
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
	if event, ok := (*data)[MessageFieldName]; ok && event == Genesis {
		return fmt.Sprintln("Beginning of time at ", (*data)[Genesis])
	}
	ret := ""
	if val, ok := (*data)[TimestampFieldName]; ok {
		ret += fmt.Sprintf("%8v|", val)
	}
	if val, ok := (*data)[LevelFieldName]; ok {
		i, _ := strconv.Atoi(val.(string))
		ret += fmt.Sprintf("%5v|", zerolog.Level(i))
	}
	if val, ok := (*data)[Service]; ok {
		ret += fmt.Sprintf("%s:%7v|", fieldNameDict[Service], serviceTypeDict[int(val.(float64))])
	}
	slice := make([]string, 0)
	for k := range *data {
		if k == TimestampFieldName || k == Service || k == MessageFieldName || k == LevelFieldName {
			continue
		}
		slice = append(slice, k)
	}
	sort.Strings(slice)
	for _, k := range slice {
		if f, ok := fieldNameDict[k]; ok {
			ret += fmt.Sprintf("%8s = %-8v|", f, (*data)[k])
		} else {
			ret += fmt.Sprintf("%8s = %-8v|", k, (*data)[k])
		}
	}
	if val, ok := (*data)[MessageFieldName]; ok {
		s := val.(string)
		if _, in := eventTypeDict[s]; in {
			s = eventTypeDict[s]
		}
		ret += fmt.Sprintln("  " + s)
	}
	return ret
}
