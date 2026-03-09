package batch

import (
	"errors"
	"testing"
)

// --- Mock ---

type mockBatchAPI struct {
	createdJobs  []string      // job IDs passed to CreateJob
	createdPools []PoolConfig  // pool configs passed to CreateJob
	createdTasks []TaskRequest // tasks passed to AddTask
	jobErr       error
	taskErr      error
}

func (m *mockBatchAPI) CreateJob(jobID string, pool PoolConfig) error {
	m.createdJobs = append(m.createdJobs, jobID)
	m.createdPools = append(m.createdPools, pool)
	return m.jobErr
}

func (m *mockBatchAPI) AddTask(jobID string, task TaskRequest) error {
	m.createdTasks = append(m.createdTasks, task)
	return m.taskErr
}

// --- Job Parameters ---

func TestJobParametersFromPayload(t *testing.T) {
	params := JobParams{
		RepoURL:   "https://github.com/test/repo.git",
		Branch:    "refs/heads/main",
		CommitSHA: "abc123def456",
		Platform:  "WebGL",
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

// --- Submit passes PoolConfig to CreateJob ---

func TestSubmitPassesPoolConfigToCreateJob(t *testing.T) {
	mock := &mockBatchAPI{}
	pool := PoolConfig{
		VMSize:          "Standard_D4s_v3",
		ImageResourceID: "/subscriptions/xxx/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery1/images/unity-ci-image/versions/latest",
	}
	client := Client{API: mock, Pool: pool}

	params := JobParams{
		RepoURL:   "https://github.com/test/repo.git",
		Branch:    "refs/heads/main",
		CommitSHA: "abc123def456",
		Platform:  "WebGL",
	}

	err := client.Submit(params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(mock.createdPools) != 1 {
		t.Fatalf("expected 1 pool config, got %d", len(mock.createdPools))
	}
	if mock.createdPools[0].VMSize != "Standard_D4s_v3" {
		t.Fatalf("pool VM size mismatch: %s", mock.createdPools[0].VMSize)
	}
	if mock.createdPools[0].ImageResourceID == "" {
		t.Fatal("pool image resource ID should not be empty")
	}
}

// --- Submit creates job and task ---

func TestSubmitCreatesJobAndTask(t *testing.T) {
	mock := &mockBatchAPI{}
	client := Client{API: mock, Pool: PoolConfig{VMSize: "Standard_D4s_v3", ImageResourceID: "/subscriptions/xxx/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery1/images/unity-ci-image/versions/latest"}}

	params := JobParams{
		RepoURL:   "https://github.com/test/repo.git",
		Branch:    "refs/heads/main",
		CommitSHA: "abc123def456",
		Platform:  "WebGL",
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
}

// --- Batch API Error Propagation ---

func TestSubmitReturnsErrorOnJobCreationFailure(t *testing.T) {
	mock := &mockBatchAPI{jobErr: errors.New("batch unavailable")}
	client := Client{API: mock, Pool: PoolConfig{}}

	err := client.Submit(JobParams{CommitSHA: "abc123"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSubmitReturnsErrorOnTaskCreationFailure(t *testing.T) {
	mock := &mockBatchAPI{taskErr: errors.New("task rejected")}
	client := Client{API: mock, Pool: PoolConfig{}}

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
