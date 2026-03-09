package batch

import "fmt"

// PoolConfig holds autoPool configuration for a Batch job.
type PoolConfig struct {
	VMSize          string // e.g. "Standard_D4s_v3"
	ImageResourceID string // Full Azure resource ID for the VM image
}

// BatchAPI abstracts Azure Batch API calls for testability.
type BatchAPI interface {
	CreateJob(jobID string, pool PoolConfig) error
	AddTask(jobID string, task TaskRequest) error
}

// TaskRequest holds the parameters for a Batch task.
type TaskRequest struct {
	RepoURL   string
	Branch    string
	CommitSHA string
	Platform  string
}

// JobParams holds parameters derived from webhook payload + environment.
type JobParams struct {
	RepoURL   string
	Branch    string
	CommitSHA string
	Platform  string
}

// JobID returns a deterministic job ID that includes the commit SHA.
func (p JobParams) JobID() string {
	return fmt.Sprintf("unity-build-%s", p.CommitSHA)
}

// Client submits build jobs to Azure Batch.
type Client struct {
	API  BatchAPI
	Pool PoolConfig
}

// Submit creates a Batch job with autoPool and adds a build task.
func (c *Client) Submit(params JobParams) error {
	jobID := params.JobID()

	if err := c.API.CreateJob(jobID, c.Pool); err != nil {
		return fmt.Errorf("create job: %w", err)
	}

	task := TaskRequest{
		RepoURL:   params.RepoURL,
		Branch:    params.Branch,
		CommitSHA: params.CommitSHA,
		Platform:  params.Platform,
	}

	if err := c.API.AddTask(jobID, task); err != nil {
		return fmt.Errorf("add task: %w", err)
	}

	return nil
}
