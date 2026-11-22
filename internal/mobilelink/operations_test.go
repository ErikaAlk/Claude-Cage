package mobilelink

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"testing"
)

func TestSnapshotPollingQuery(t *testing.T) {
	const id = "12345678-1234-4321-aaaa-123456789abc"
	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"", true}, {"after=0", true}, {"after=18446744073709551615", true},
		{"after=2&epoch=" + id, true}, {"epoch=" + id + "&after=2", true},
		{"epoch=" + id, false}, {"after=2&epoch=", false}, {"after=2&epoch=" + strings.ToUpper(id), false},
		{"after=2&epoch=12345678-1234-5321-aaaa-123456789abc", false},
		{"after=2&epoch=12345678-1234-4321-0aaa-123456789abc", false},
		{"after=-1&epoch=" + id, false}, {"after=0&after=1", false}, {"after=1&other=x", false},
		{"after=1&epoch=" + id + "&epoch=" + id, false}, {"after=%31&epoch=" + id, false},
		{"after=18446744073709551616", false},
	} {
		u := &url.URL{Path: "/api/mobile/cli/sessions/" + id, RawQuery: tc.query}
		if got := allowed("GET", u); got != tc.want {
			t.Errorf("poll query %q allowed=%v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestBrowserFrameQuery(t *testing.T) {
	const tag = "b542cd94286a2489bc2df1241b043c76"
	const id = "12345678-1234-4321-aaaa-123456789abc"
	for _, tc := range []struct {
		method, surface, operation, query string
		want                              bool
	}{
		{"GET", "browser", "frame", "after=" + tag, true},
		{"GET", "browser", "frame", "after=" + tag + "&after=" + tag, false},
		{"GET", "browser", "frame", "after=" + strings.ToUpper(tag), false},
		{"POST", "browser", "action", "after=" + tag, false},
		{"GET", "desktop", "frame", "after=" + tag, false},
	} {
		u := &url.URL{Path: "/api/mobile/" + tc.surface + "/" + id + "/" + tc.operation, RawQuery: tc.query}
		if allowed(tc.method, u) != tc.want {
			t.Errorf("unexpected result for %s %s", tc.method, u.RequestURI())
		}
	}
}

func TestSessionListMutations(t *testing.T) {
	for _, operation := range []string{"metadata", "delete", "archive", "restore", "purge"} {
		u := &url.URL{Path: "/api/mobile/cli/sessions/12345678-1234-4321-aaaa-123456789abc/" + operation}
		if !allowed("POST", u) || allowed("GET", u) || allowed("DELETE", u) {
			t.Fatal("mutation method boundary", operation)
		}
		u.RawQuery = "after=1"
		if allowed("POST", u) {
			t.Fatal("mutation query accepted", operation)
		}
	}
}

func TestArchivedSessionQuery(t *testing.T) {
	u := &url.URL{Path: "/api/mobile/cli/sessions", RawQuery: "archived=true"}
	if !allowed("GET", u) || allowed("POST", u) {
		t.Fatal("archive query method")
	}
	for _, query := range []string{"archived=false", "archived=true&other=1", "archived=true&archived=true"} {
		u.RawQuery = query
		if allowed("GET", u) {
			t.Fatal("unexpected archive query", query)
		}
	}
}

func TestTranscriptReadQuery(t *testing.T) {
	const path = "/api/mobile/cli/plugins/happiest/v1/sessions/12345678-1234-4321-aaaa-123456789abc/transcript"
	closePath := strings.TrimSuffix(path, "transcript") + "close"
	if !allowed("POST", &url.URL{Path: closePath}) || allowed("GET", &url.URL{Path: closePath}) || allowed("POST", &url.URL{Path: closePath, RawQuery: "extra=1"}) {
		t.Fatal("关闭会话的请求边界错误")
	}
	for _, action := range []string{"transcript", "close"} {
		old := &url.URL{Path: "/api/mobile/cli/sessions/12345678-1234-4321-aaaa-123456789abc/" + action}
		if allowed("GET", old) || allowed("POST", old) {
			t.Fatal("旧接口仍然可用", action)
		}
	}
	for _, query := range []string{"", "offset=0&max_bytes=128", "stream=2&offset=100&source=200-" + strings.Repeat("a", 64)} {
		u := &url.URL{Path: path, RawQuery: query}
		if !allowed("GET", u) || allowed("POST", u) {
			t.Errorf("transcript read refused: %s", query)
		}
	}
	for _, query := range []string{"offset=-1", "offset=0&offset=1", "max_bytes=0", "path=secret", "cli_id=other", "config=other", "offset=%31", "source=invalid"} {
		if allowed("GET", &url.URL{Path: path, RawQuery: query}) {
			t.Errorf("invalid transcript query accepted: %s", query)
		}
	}
}

func TestTranscriptResponseFitsTransport(t *testing.T) {
	maximum := int64(math.MaxInt64)
	chunk := TranscriptChunk{ID: strings.Repeat("a", 36), CLIID: strings.Repeat("a", 36),
		Content: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{251, 239, 190}, int(TranscriptMaxBytes()/3))),
		Offset:  maximum, NextOffset: maximum, TotalBytes: maximum, Stream: maximum, StreamCount: maximum, NextStream: &maximum,
		Source: strings.Repeat("9", 19) + "-" + strings.Repeat("a", 64)}
	encoded, err := json.Marshal(chunk)
	if err != nil {
		t.Fatal(err)
	}
	spaced := strings.ReplaceAll(strings.ReplaceAll(string(encoded), ",", ", "), ":", ": ")
	spaced = strings.ReplaceAll(spaced, "+", `\u002B`)
	if len(spaced)+len(`{"result": }`)+1 > maxResponse {
		t.Fatal("transcript exceeded response budget")
	}
}
