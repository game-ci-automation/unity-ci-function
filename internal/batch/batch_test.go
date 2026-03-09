package batch

import (
	"errors"
	"testing"
)

// --- Mock ---

type mockBatchAPI struct {
	createdJobs  []string      // job IDs passed to CreateJob
	createdTasks []TaskRequest // tasks passed to AddTask
	jobErr       error
	taskErr      error
}

func (m *mockBatchAPI) CreateJob(jobID, poolID string) error {
	m.createdJobs = append(m.createdJobs, jobID)
	return m.jobErr
}

func (m *mockBatchAPI) AddTask(jobID string, task TaskRequest) error {
	m.createdTasks = append(m.createdTasks, task)
	return m.taskErr
}

// --- Job Parameters ---

func TestJobParametersFromPayload(t *testing.T) {
	params := JobParams{
		RepoURL:             "https://github.com/test/repo.git",
		Branch:              "refs/heads/main",
		CommitSHA:           "abc123def456",
		Platform:            "WebGL",
		ImageGalleryName:    "gallery1",
		ImageDefinitionName: "unity-ci-image",
	}

	if params.RepoURL != "https://github.com/test/repo.git" {
		t.Fatalf("unexpected repo_url: %s", params.RepoURL)
	}
	if params.Branch != "refs/heads/main" {
		t.Fatalf("unexpected branch: %s", params.Branch)
	}
	if params.CommitSHA != "abc123def456" {
		t.Fatalf("unexpected commit: %s", params.CommitSHA)
	}
	if params.Platform != "WebGL" {
		t.Fatalf("unexpected platform: %s", params.Platform)
	}
}

// --- Job ID includes commit SHA (idempotent) ---

func TestJobIDContainsCommitSHA(t *testing.T) {
	params := JobParams{
		CommitSHA: "abc123def456",
	}

	jobID := params.JobID()
	if jobID == "" {
		t.Fatal("job ID should not be empty")
	}
	if !containsSubstring(jobID, "abc123def456") {
		t.Fatalf("job ID should contain commit SHA, got: %s", jobID)
	}
}

// --- Submit Success ---

func TestSubmitCreatesJobAndTask(t *testing.T) {
	mock := &mockBatchAPI{}
	client := Client{API: mock, PoolID: "test-pool"}

	params := JobParams{
		RepoURL:             "https://github.com/test/repo.git",
		Branch:              "refs/heads/main",
		CommitSHA:           "abc123def456",
		Platform:            "WebGL",
		ImageGalleryName:    "gallery1",
		ImageDefinitionName: "unity-ci-image",
	}

	err := client.Submit(params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(mock.createdJobs) != 1 {
		t.Fatalf("expected 1 job created, got %d", len(mock.createdJobs))
	}
	if len(mock.createdTasks) != 1 {
		t.Fatalf("expected 1 task created, got %d", len(mock.createdTasks))
	}

	task := mock.createdTasks[0]
	if task.RepoURL != "https://github.com/test/repo.git" {
		t.Fatalf("task repo_url mismatch: %s", task.RepoURL)
	}
	if task.Branch != "refs/heads/main" {
		t.Fatalf("task branch mismatch: %s", task.Branch)
	}
	if task.CommitSHA != "abc123def456" {
		t.Fatalf("task commit mismatch: %s", task.CommitSHA)
	}
	if task.Platform != "WebGL" {
		t.Fatalf("task platform mismatch: %s", task.Platform)
	}
	if task.ImageGalleryName != "gallery1" {
		t.Fatalf("task image gallery mismatch: %s", task.ImageGalleryName)
	}
	if task.ImageDefinitionName != "unity-ci-image" {
		t.Fatalf("task image definition mismatch: %s", task.ImageDefinitionName)
	}
}

// --- Batch API Error Propagation ---

func TestSubmitReturnsErrorOnJobCreationFailure(t *testing.T) {
	mock := &mockBatchAPI{jobErr: errors.New("batch unavailable")}
	client := Client{API: mock, PoolID: "test-pool"}

	err := client.Submit(JobParams{CommitSHA: "abc123"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSubmitReturnsErrorOnTaskCreationFailure(t *testing.T) {
	mock := &mockBatchAPI{taskErr: errors.New("task rejected")}
	client := Client{API: mock, PoolID: "test-pool"}

	err := client.Submit(JobParams{CommitSHA: "abc123"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- helper ---

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstringImpl(s, sub))
}

func containsSubstringImpl(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
