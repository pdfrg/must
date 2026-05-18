package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// LastFMDoAuth runs the one-time Last.fm authorization flow.
// It opens the browser for the user to grant access, then fetches the session key.
// Returns the session key or an error.
func LastFMDoAuth(apiKey, sharedSecret string) (string, error) {
	if apiKey == "" || sharedSecret == "" {
		return "", fmt.Errorf("last.fm API key and secret not configured\nSet them in ~/.config/must/config.toml:\n\n  [lastfm]\n  api_key = \"your_api_key\"\n  shared_secret = \"your_shared_secret\"\n\nGet credentials at: https://www.last.fm/api/account/create")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Step 1: Get a request token
	token, err := GetLastFMAuthToken(apiKey, sharedSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get request token: %w", err)
	}
	fmt.Printf("Request token obtained: %s\n", token)

	// Step 2: Open browser for user authorization
	authURL := GetLastFMAuthURL(apiKey, token)
	fmt.Printf("Opening browser for authorization...\n")
	fmt.Printf("If the browser doesn't open, visit this URL:\n  %s\n\n", authURL)
	OpenBrowser(authURL)

	// Step 3: Poll for session key (user must authorize in browser first)
	fmt.Println("Waiting for authorization (press Ctrl+C to cancel)...")
	sessionKey, err := lastFMPollSession(client, apiKey, sharedSecret, token)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}

	fmt.Println("Authorization successful!")
	return sessionKey, nil
}

func lastFMPollSession(client *http.Client, apiKey, secret, token string) (string, error) {
	deadline := time.Now().Add(5 * time.Minute)

	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)

		params := url.Values{}
		params.Set("method", "auth.getSession")
		params.Set("api_key", apiKey)
		params.Set("token", token)

		sig := generateLastFMSig(map[string]string{
			"method":  "auth.getSession",
			"api_key": apiKey,
			"token":   token,
		}, secret)

		params.Set("api_sig", sig)
		params.Set("format", "json")

		u, _ := url.Parse(lastfmBaseURL)
		u.RawQuery = params.Encode()

		resp, err := client.Get(u.String())
		if err != nil {
			fmt.Printf("  Retrying... (%v)\n", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		var result struct {
			Session struct {
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"session"`
			Error   int    `json:"error"`
			Message string `json:"message"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		if result.Session.Key != "" {
			fmt.Printf("  Authorized as: %s\n", result.Session.Name)
			return result.Session.Key, nil
		}

		// Error 14 = token not authorized yet, keep polling
		if result.Error == 14 {
			fmt.Print(".")
			continue
		}

		// Other errors are fatal
		return "", fmt.Errorf("last.fm error %d: %s", result.Error, result.Message)
	}

	return "", fmt.Errorf("authorization timed out after 5 minutes")
}
