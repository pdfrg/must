package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const userAgent = "must/1.0"

// projectURL doubles as the default contact in the MusicBrainz User-Agent
// (their API policy requires application name, version, and contact info).
const projectURL = "https://github.com/pdfrg/must"

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// apiContact optionally overrides the contact in the MusicBrainz
// User-Agent (config: musicbrainz_contact).
var apiContact string

// SetAPIContact sets the contact string used in the MusicBrainz User-Agent.
func SetAPIContact(contact string) {
	apiContact = contact
}

// MusicBrainzUserAgent returns a policy-compliant User-Agent:
// application/version plus contact details.
func MusicBrainzUserAgent() string {
	contact := apiContact
	if contact == "" {
		contact = projectURL
	}
	return fmt.Sprintf("%s (%s)", userAgent, contact)
}

// MusicBrainz allows roughly one request per second per client. Overlapping
// track-change lookups otherwise burst past that and get HTTP 503s, so every
// MusicBrainz call goes through this gate.
var (
	mbMu          sync.Mutex
	mbLastCall    time.Time
	mbMinInterval = 1100 * time.Millisecond
)

func mbRateLimit() {
	mbMu.Lock()
	defer mbMu.Unlock()
	now := time.Now()
	if dt := now.Sub(mbLastCall); dt < mbMinInterval {
		time.Sleep(mbMinInterval - dt)
		now = time.Now()
	}
	mbLastCall = now
}

var apiLogger *log.Logger
var scrobbleLogger *log.Logger

func SetAPILogger(l *log.Logger) {
	apiLogger = l
}

func SetScrobbleLogger(l *log.Logger) {
	scrobbleLogger = l
}

func fetchJSON(url string, headers map[string]string) ([]byte, error) {
	// Retry transient failures (rate limiting, overloaded upstream): without
	// this a single 503 permanently poisons downstream results.
	const maxAttempts = 3
	var err error
	var body []byte
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, err = fetchJSONOnce(url, headers)
		if err == nil {
			return body, nil
		}
		var retryErr *retryableHTTPError
		if errors.As(err, &retryErr) && attempt < maxAttempts {
			wait := retryErr.retryAfter
			if wait <= 0 {
				wait = time.Duration(attempt) * time.Second
			}
			if apiLogger != nil {
				apiLogger.Printf("fetchJSON: %s -> %s, retrying in %s (attempt %d/%d)", url, retryErr.Status, wait, attempt+1, maxAttempts)
			}
			time.Sleep(wait)
			continue
		}
		return nil, err
	}
	return nil, err
}

// retryableHTTPError marks transient HTTP statuses worth retrying.
type retryableHTTPError struct {
	Status     string
	retryAfter time.Duration
}

func (e *retryableHTTPError) Error() string { return "HTTP " + e.Status }

func retryAfterFromHeader(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		if secs > 15 {
			secs = 15
		}
		return time.Duration(secs) * time.Second
	}
	return 0
}

func fetchJSONOnce(url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusTooManyRequests,
			http.StatusServiceUnavailable,
			http.StatusBadGateway,
			http.StatusGatewayTimeout:
			_, _ = io.Copy(io.Discard, resp.Body)
			return nil, &retryableHTTPError{
				Status:     resp.Status,
				retryAfter: retryAfterFromHeader(resp.Header.Get("Retry-After")),
			}
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}

func buildURL(baseURL string, params map[string]string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
