package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/game-ci-automation/unity-ci-function/internal/batch"
)

const testSecret = "test-webhook-secret"

func sign(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// mockBatchAPI implements batch.BatchAPI for testing.
type mockBatchAPI struct {
	submitted bool
	jobErr    error
	taskErr   error
}

func (m *mockBatchAPI) CreateJob(jobID string, pool batch.PoolConfig) error {
	m.submitted = true
	return m.jobErr
}

func (m *mockBatchAPI) AddTask(jobID string, task batch.TaskRequest) error {
	return m.taskErr
}

func newTestHandler(mock *mockBatchAPI) http.Handler {
	batchClient := &batch.Client{API: mock, Pool: batch.PoolConfig{}}
	return NewHandler(testSecret, batchClient, log.Default())
}

// --- Response Codes ---

func TestValidPushReturns202(t *testing.T) {
	mock := &mockBatchAPI{}
	h := newTestHandler(mock)

	body := `{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/test/repo.git"},"head_commit":{"id":"abc123"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "build_submitted" {
		t.Fatalf("expected body {\"status\":\"build_submitted\"}, got %s", rr.Body.String())
	}

	if !mock.submitted {
		t.Fatal("expected batch job to be submitted")
	}
}

func TestNonPushEventReturns204(t *testing.T) {
	mock := &mockBatchAPI{}
	h := newTestHandler(mock)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "ping")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %s", rr.Body.String())
	}
}

func TestInvalidPayloadReturns400(t *testing.T) {
	mock := &mockBatchAPI{}
	h := newTestHandler(mock)

	body := `{not valid json`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "invalid payload" {
		t.Fatalf("expected error 'invalid payload', got %s", rr.Body.String())
	}
}

func TestInvalidSignatureReturns401(t *testing.T) {
	mock := &mockBatchAPI{}
	h := newTestHandler(mock)

	body := `{"ref":"refs/heads/main"}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, "wrong-secret"))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "invalid signature" {
		t.Fatalf("expected error 'invalid signature', got %s", rr.Body.String())
	}
}

func TestBatchAPIFailureReturns500(t *testing.T) {
	mock := &mockBatchAPI{jobErr: errors.New("batch unavailable")}
	h := newTestHandler(mock)

	body := `{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/test/repo.git"},"head_commit":{"id":"abc123"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "batch submission failed" {
		t.Fatalf("expected error 'batch submission failed', got %s", rr.Body.String())
	}
}

// --- Orchestration Flow Order ---

func TestSignatureValidationRunsBeforeEventCheck(t *testing.T) {
	mock := &mockBatchAPI{}
	h := newTestHandler(mock)

	// Invalid signature + non-push event → should get 401, not 204
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, "wrong-secret"))
	req.Header.Set("X-GitHub-Event", "ping")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (signature check before event filter), got %d", rr.Code)
	}
}

// --- Logging ---

func TestLogsInfoOnSuccessfulSubmission(t *testing.T) {
	mock := &mockBatchAPI{}
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	batchClient := &batch.Client{API: mock, Pool: batch.PoolConfig{}}
	h := NewHandler(testSecret, batchClient, logger)

	body := `{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/test/repo.git"},"head_commit":{"id":"abc123"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "INFO") {
		t.Fatalf("expected INFO log, got: %s", logOutput)
	}
}

func TestLogsWarnOnInvalidSignature(t *testing.T) {
	mock := &mockBatchAPI{}
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	batchClient := &batch.Client{API: mock, Pool: batch.PoolConfig{}}
	h := NewHandler(testSecret, batchClient, logger)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, "wrong-secret"))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "WARN") {
		t.Fatalf("expected WARN log, got: %s", logOutput)
	}
}

func TestLogsWarnOnMalformedPayload(t *testing.T) {
	mock := &mockBatchAPI{}
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	batchClient := &batch.Client{API: mock, Pool: batch.PoolConfig{}}
	h := NewHandler(testSecret, batchClient, logger)

	body := `{not valid json`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "WARN") {
		t.Fatalf("expected WARN log, got: %s", logOutput)
	}
}

func TestLogsErrorOnBatchFailure(t *testing.T) {
	mock := &mockBatchAPI{jobErr: errors.New("batch down")}
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	batchClient := &batch.Client{API: mock, Pool: batch.PoolConfig{}}
	h := NewHandler(testSecret, batchClient, logger)

	body := `{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/test/repo.git"},"head_commit":{"id":"abc123"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/github-webhook", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign(body, testSecret))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "ERROR") {
		t.Fatalf("expected ERROR log, got: %s", logOutput)
	}
}
