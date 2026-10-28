package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestScrobbleSendsMilliseconds(t *testing.T) {
	var got struct {
		id         string
		submission string
		time       int64
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		got.id = q.Get("id")
		got.submission = q.Get("submission")
		raw := q.Get("time")
		if raw == "" {
			t.Error("time parameter missing")
		} else if v, err := strconv.ParseInt(raw, 10, 64); err != nil {
			t.Errorf("time parameter %q is not an integer: %v", raw, err)
		} else {
			got.time = v
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"subsonic-response":{"status":"ok","version":"1.16.1"}}`))
	}))
	defer srv.Close()

	c, err := NewSubsonicClient(srv.URL, "u", "p", "Navidrome", "ND")
	if err != nil {
		t.Fatal(err)
	}

	startedAt := time.Unix(1700000000, 0)
	if err := c.Scrobble("song-id", true, startedAt.UnixMilli()); err != nil {
		t.Fatalf("Scrobble: %v", err)
	}

	if got.id != "song-id" {
		t.Errorf("id = %q, want %q", got.id, "song-id")
	}
	if got.submission != "true" {
		t.Errorf("submission = %q, want %q", got.submission, "true")
	}
	if want := startedAt.UnixMilli(); got.time != want {
		t.Errorf("time = %d, want %d (UnixMilli)", got.time, want)
	}
}
