package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	internalbatch "github.com/game-ci-automation/unity-ci-function/internal/batch"
	"github.com/game-ci-automation/unity-ci-function/internal/handler"
	"github.com/game-ci-automation/unity-ci-function/internal/keyvault"
)

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)

	// Read environment variables (injected by Terraform)
	keyVaultName := mustEnv("KEY_VAULT_NAME")
	batchAccountURL := mustEnv("BATCH_ACCOUNT_URL")
	imageResourceID := mustEnv("IMAGE_RESOURCE_ID")
	vmSize := os.Getenv("VM_SIZE")
	if vmSize == "" {
		vmSize = "Standard_D4s_v3"
	}

	// Azure Functions Custom Handler port
	port := os.Getenv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if port == "" {
		port = "8080"
	}

	// Key Vault — read webhook secret
	kvClient, err := keyvault.NewClient(keyVaultName)
	if err != nil {
		logger.Fatalf("ERROR: failed to create Key Vault client: %v", err)
	}

	secret, err := kvClient.GetSecret(context.Background(), "WEBHOOK-SECRET")
	if err != nil {
		logger.Fatalf("ERROR: failed to read WEBHOOK-SECRET from Key Vault: %v", err)
	}

	// Batch API client
	batchAPI, err := internalbatch.NewAzureBatchAPI(batchAccountURL)
	if err != nil {
		logger.Fatalf("ERROR: failed to create Batch API client: %v", err)
	}

	batchClient := &internalbatch.Client{
		API: batchAPI,
		Pool: internalbatch.PoolConfig{
			VMSize:          vmSize,
			ImageResourceID: imageResourceID,
		},
	}

	// Wire handler
	h := handler.NewHandler(secret, batchClient, logger)

	mux := http.NewServeMux()
	mux.Handle("/api/github-webhook", h)

	logger.Printf("INFO: starting server on port %s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		logger.Fatalf("ERROR: server failed: %v", err)
	}
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("ERROR: required environment variable %s is not set", key)
	}
	return val
}
