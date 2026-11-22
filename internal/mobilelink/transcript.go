package mobilelink

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type TranscriptQuery struct {
	Stream   int64
	Offset   int64
	MaxBytes int64
	Source   string
}

type TranscriptChunk struct {
	ID          string `json:"id"`
	CLIID       string `json:"cli_id"`
	Content     string `json:"content"`
	Offset      int64  `json:"offset"`
	NextOffset  int64  `json:"next_offset"`
	TotalBytes  int64  `json:"total_bytes"`
	EOF         bool   `json:"eof"`
	Source      string `json:"source"`
	Stream      int64  `json:"stream"`
	StreamCount int64  `json:"stream_count"`
	NextStream  *int64 `json:"next_stream,omitempty"`
	Error       string `json:"error,omitempty"`
}

var transcriptSource = regexp.MustCompile(`^(0|[1-9][0-9]*)-[0-9a-f]{64}$`)

// 从既有 4 MiB 响应上限扣除元数据、Python JSON 空格、Windows 包装和换行，
// 再计入 Base64 编码和 Windows JSON 对 '+' 的六字节转义开销。
func TranscriptMaxBytes() int64 {
	maximum := int64(math.MaxInt64)
	wire, _ := json.Marshal(TranscriptChunk{ID: strings.Repeat("a", 36), CLIID: strings.Repeat("a", 36),
		Offset: maximum, NextOffset: maximum, TotalBytes: maximum, Source: strings.Repeat("9", 19) + "-" + strings.Repeat("a", 64),
		Stream: maximum, StreamCount: maximum, NextStream: &maximum})
	overhead := len(wire) + bytes.Count(wire, []byte(":")) + bytes.Count(wire, []byte(",")) + len(`{"result": }`) + 1
	return int64((maxResponse - overhead) / (4 * len(`\u002B`)) * 3)
}

func ParseTranscriptQuery(raw string) (TranscriptQuery, error) {
	query := TranscriptQuery{MaxBytes: TranscriptMaxBytes()}
	if raw == "" {
		return query, nil
	}
	seen := map[string]bool{}
	for _, field := range strings.Split(raw, "&") {
		key, value, found := strings.Cut(field, "=")
		if !found || seen[key] || value == "" {
			return query, errors.New("invalid transcript query")
		}
		seen[key] = true
		if key == "source" {
			if !transcriptSource.MatchString(value) {
				return query, errors.New("invalid transcript source")
			}
			size, _, _ := strings.Cut(value, "-")
			if _, err := strconv.ParseInt(size, 10, 64); err != nil {
				return query, errors.New("invalid transcript source size")
			}
			query.Source = value
			continue
		}
		for _, digit := range value {
			if digit < '0' || digit > '9' {
				return query, errors.New("invalid transcript number")
			}
		}
		number, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return query, err
		}
		switch key {
		case "stream":
			query.Stream = number
		case "offset":
			query.Offset = number
		case "max_bytes":
			if number == 0 {
				return query, errors.New("invalid transcript size")
			}
			query.MaxBytes = min(number, TranscriptMaxBytes())
		default:
			return query, errors.New("invalid transcript field")
		}
	}
	return query, nil
}

func transcriptAllowed(method string, u *url.URL) bool {
	_, err := ParseTranscriptQuery(u.RawQuery)
	return method == "GET" && err == nil
}
