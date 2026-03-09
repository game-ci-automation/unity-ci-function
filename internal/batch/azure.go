package batch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// AzureBatchAPI implements BatchAPI using Azure Batch REST API.
type AzureBatchAPI struct {
	accountURL string
	cred       *azidentity.DefaultAzureCredential
}

// NewAzureBatchAPI creates a Batch API client using Managed Identity.
func NewAzureBatchAPI(accountName, region string) (*AzureBatchAPI, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}

	accountURL := fmt.Sprintf("https://%s.%s.batch.azure.com", accountName, region)

	return &AzureBatchAPI{
		accountURL: accountURL,
		cred:       cred,
	}, nil
}

func (a *AzureBatchAPI) getToken(ctx context.Context) (string, error) {
	token, err := a.cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://batch.core.windows.net/.default"},
	})
	if err != nil {
		return "", fmt.Errorf("get batch token: %w", err)
	}
	return token.Token, nil
}

// CreateJob creates a Batch job under the specified pool.
func (a *AzureBatchAPI) CreateJob(jobID, poolID string) error {
	ctx := context.Background()
	token, err := a.getToken(ctx)
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"id": jobID,
		"poolInfo": map[string]string{
			"poolId": poolID,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal job request: %w", err)
	}

	url := fmt.Sprintf("%s/jobs?api-version=2024-07-01.20.0", a.accountURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create job request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; odata=minimalmetadata")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send job request: %w", err)
	}
	defer resp.Body.Close()

	// 201 Created or 409 Conflict (job already exists — idempotent)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create job failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// AddTask adds a build task to an existing Batch job.
func (a *AzureBatchAPI) AddTask(jobID string, task TaskRequest) error {
	ctx := context.Background()
	token, err := a.getToken(ctx)
	if err != nil {
		return err
	}

	commandLine := fmt.Sprintf(
		"/bin/bash -c 'scripts/build.sh'",
	)

	envSettings := []map[string]string{
		{"name": "REPO_URL", "value": task.RepoURL},
		{"name": "BRANCH", "value": task.Branch},
		{"name": "COMMIT_SHA", "value": task.CommitSHA},
		{"name": "PLATFORM", "value": task.Platform},
		{"name": "IMAGE_GALLERY_NAME", "value": task.ImageGalleryName},
		{"name": "IMAGE_DEFINITION_NAME", "value": task.ImageDefinitionName},
	}

	body := map[string]interface{}{
		"id":                   fmt.Sprintf("task-%s", task.CommitSHA),
		"commandLine":          commandLine,
		"environmentSettings":  envSettings,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal task request: %w", err)
	}

	url := fmt.Sprintf("%s/jobs/%s/tasks?api-version=2024-07-01.20.0", a.accountURL, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create task request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; odata=minimalmetadata")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send task request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add task failed (%d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
