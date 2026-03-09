package keyvault

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// Client reads secrets from Azure Key Vault.
type Client struct {
	client *azsecrets.Client
}

// NewClient creates a Key Vault client using Managed Identity.
func NewClient(vaultName string) (*Client, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net/", vaultName)
	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("create key vault client: %w", err)
	}

	return &Client{client: client}, nil
}

// GetSecret reads a secret value by name.
func (c *Client) GetSecret(ctx context.Context, name string) (string, error) {
	resp, err := c.client.GetSecret(ctx, name, "", nil)
	if err != nil {
		return "", fmt.Errorf("get secret %q: %w", name, err)
	}
	if resp.Value == nil {
		return "", fmt.Errorf("secret %q has nil value", name)
	}
	return *resp.Value, nil
}
