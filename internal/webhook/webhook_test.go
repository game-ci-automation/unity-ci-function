package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSecret = "test-webhook-secret"

// helper: compute HMAC-SHA256 signature for a given body
func sign(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// --- Signature Verification ---

func TestValidSignature(t *testing.T) {
	body := `{"ref":"refs/heads/main"}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	result, err := Validate(req, testSecret)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.StatusCode != 0 {
		t.Fatalf("expected no rejection, got status %d", result.StatusCode)
	}
}

func TestMissingSignatureHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader("{}"))
	req.Header.Set("X-GitHub-Event", "push")
	// No X-Hub-Signature-256 header

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", result.StatusCode)
	}
}

func TestEmptySignatureHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader("{}"))
	req.Header.Set("X-Hub-Signature-256", "")
	req.Header.Set("X-GitHub-Event", "push")

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", result.StatusCode)
	}
}

func TestSignatureWithoutSHA256Prefix(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader("{}"))
	req.Header.Set("X-Hub-Signature-256", "abcdef1234567890")
	req.Header.Set("X-GitHub-Event", "push")

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", result.StatusCode)
	}
}

func TestWrongSignature(t *testing.T) {
	body := `{"ref":"refs/heads/main"}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, "wrong-secret"))
	req.Header.Set("X-GitHub-Event", "push")

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", result.StatusCode)
	}
}

func TestEmptyBodyWithValidSignature(t *testing.T) {
	// Spec edge case: empty body + valid signature passes signature verification.
	// However, for a push event, payload parsing will fail (400).
	// Use a non-push event to verify signature-only validation.
	body := ""
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "ping")

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 (signature valid, non-push event), got %d", result.StatusCode)
	}
}

// --- Event Filtering ---

func TestPushEventProceeds(t *testing.T) {
	body := `{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/test/repo.git"},"head_commit":{"id":"abc123"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	result, err := Validate(req, testSecret)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Event != "push" {
		t.Fatalf("expected event 'push', got '%s'", result.Event)
	}
}

func TestPingEventReturns204(t *testing.T) {
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "ping")

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", result.StatusCode)
	}
}

func TestOtherEventReturns204(t *testing.T) {
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "pull_request")

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", result.StatusCode)
	}
}

func TestMissingEventHeader(t *testing.T) {
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	// No X-GitHub-Event header

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", result.StatusCode)
	}
}

// --- Payload Parsing ---

func TestParsesPushPayload(t *testing.T) {
	body := `{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/test/repo.git"},"head_commit":{"id":"abc123def456"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	result, err := Validate(req, testSecret)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Payload.Ref != "refs/heads/main" {
		t.Fatalf("expected ref 'refs/heads/main', got '%s'", result.Payload.Ref)
	}
	if result.Payload.CloneURL != "https://github.com/test/repo.git" {
		t.Fatalf("expected clone_url 'https://github.com/test/repo.git', got '%s'", result.Payload.CloneURL)
	}
	if result.Payload.CommitSHA != "abc123def456" {
		t.Fatalf("expected commit 'abc123def456', got '%s'", result.Payload.CommitSHA)
	}
}

func TestMalformedJSONReturns400(t *testing.T) {
	body := `{not valid json`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	result, _ := Validate(req, testSecret)
	if result.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", result.StatusCode)
	}
}
