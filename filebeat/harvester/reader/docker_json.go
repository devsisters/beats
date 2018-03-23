package reader

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/elastic/beats/libbeat/common"

	"github.com/pkg/errors"
)

// DockerJSON processor renames a given field
type DockerJSON struct {
	reader Reader
	// stream filter, `all`, `stderr` or `stdout`
	stream string
	// option for concat partial docker logs
	concatPartial bool
}

type dockerLog struct {
	Timestamp string `json:"time"`
	Log       string `json:"log"`
	Stream    string `json:"stream"`
}

type crioLog struct {
	Timestamp time.Time
	Stream    string
	Log       []byte
}

// NewDockerJSON creates a new reader renaming a field
func NewDockerJSON(r Reader, stream string, concatPartial bool) *DockerJSON {
	return &DockerJSON{
		stream:        stream,
		reader:        r,
		concatPartial: concatPartial,
	}
}

// parseCRILog parses logs in CRI log format.
// CRI log format example :
// 2017-09-12T22:32:21.212861448Z stdout 2017-09-12 22:32:21.212 [INFO][88] table.go 710: Invalidating dataplane cache
func parseCRILog(message Message, msg *crioLog) (Message, bool, error) {
	log := strings.SplitN(string(message.Content), " ", 3)
	if len(log) < 3 {
		return message, false, errors.New("invalid CRI log")
	}
	ts, err := time.Parse(time.RFC3339, log[0])
	if err != nil {
		return message, false, errors.Wrap(err, "parsing CRI timestamp")
	}

	msg.Timestamp = ts
	msg.Stream = log[1]
	msg.Log = []byte(log[2])
	message.AddFields(common.MapStr{
		"stream": msg.Stream,
	})
	message.Content = msg.Log
	message.Ts = ts

	return message, false, nil
}

// parseDockerJSONLog parses logs in Docker JSON log format.
// Docker JSON log format example:
// {"log":"1:M 09 Nov 13:27:36.276 # User requested shutdown...\n","stream":"stdout"}
func parseDockerJSONLog(message Message, msg *dockerLog) (Message, bool, error) {
	dec := json.NewDecoder(bytes.NewReader(message.Content))
	if err := dec.Decode(&msg); err != nil {
		return message, false, errors.Wrap(err, "decoding docker JSON")
	}

	// Parse timestamp
	ts, err := time.Parse(time.RFC3339, msg.Timestamp)
	if err != nil {
		return message, false, errors.Wrap(err, "parsing docker timestamp")
	}

	message.AddFields(common.MapStr{
		"stream": msg.Stream,
	})
	message.Content = []byte(msg.Log)
	message.Ts = ts

	isPartial := len(msg.Log) > 2 && (msg.Log[len(msg.Log)-1] != '\n' || msg.Log[len(msg.Log)-2] == '\\')
	return message, isPartial, nil
}

// Next returns the next line.
func (p *DockerJSON) Next() (Message, error) {
	partialContent := []byte{}
	for {
		message, err := p.reader.Next()
		if err != nil {
			return message, err
		}

		var dockerLine dockerLog
		var crioLine crioLog
		var isPartial bool

		if strings.HasPrefix(string(message.Content), "{") {
			message, isPartial, err = parseDockerJSONLog(message, &dockerLine)
		} else {
			// isPartial will always return true. for now, support only json partial logs
			message, isPartial, err = parseCRILog(message, &crioLine)
		}

		if p.stream != "all" && p.stream != dockerLine.Stream && p.stream != crioLine.Stream {
			continue
		}

		if p.concatPartial {
			if isPartial {
				partialContent = append(partialContent, message.Content...)
				continue
			} else if len(partialContent) > 0 {
				message.Content = append(partialContent, message.Content...)
			}
		}
		return message, err
	}
}
