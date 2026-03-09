package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// PushPayload holds the parsed fields from a GitHub push event.
type PushPayload struct {
	Ref       string // e.g. "refs/heads/main"
	CloneURL  string // e.g. "https://github.com/owner/repo.git"
	CommitSHA string // e.g. "abc123..."
}

// Result is returned by Validate.
// StatusCode == 0 means validation passed (proceed to batch submission).
// StatusCode != 0 means the request should be rejected with that HTTP status.
type Result struct {
	StatusCode int
	Event      string
	Payload    PushPayload
}

// rawPushPayload mirrors the GitHub push event JSON structure.
type rawPushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	HeadCommit struct {
		ID string `json:"id"`
	} `json:"head_commit"`
}

// Validate verifies the webhook signature, filters the event, and parses the payload.
func Validate(r *http.Request, secret string) (Result, error) {
	// 1. Read raw body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return Result{StatusCode: http.StatusBadRequest}, nil
	}

	// 2. Signature verification
	sigHeader := r.Header.Get("X-Hub-Signature-256")
	if sigHeader == "" || !strings.HasPrefix(sigHeader, "sha256=") {
		return Result{StatusCode: http.StatusUnauthorized}, nil
	}

	sigHex := strings.TrimPrefix(sigHeader, "sha256=")
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return Result{StatusCode: http.StatusUnauthorized}, nil
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)

	if subtle.ConstantTimeCompare(sig, expected) != 1 {
		return Result{StatusCode: http.StatusUnauthorized}, nil
	}

	// 3. Event filtering
	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		return Result{StatusCode: http.StatusNoContent, Event: event}, nil
	}

	// 4. Payload parsing
	var raw rawPushPayload
	if err := json.Unmarshal(body, &raw); err != nil {
		return Result{StatusCode: http.StatusBadRequest}, nil
	}

	return Result{
		Event: "push",
		Payload: PushPayload{
			Ref:       raw.Ref,
			CloneURL:  raw.Repository.CloneURL,
			CommitSHA: raw.HeadCommit.ID,
		},
	}, nil
}
