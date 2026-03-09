package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/game-ci-automation/unity-ci-function/internal/batch"
	"github.com/game-ci-automation/unity-ci-function/internal/webhook"
)

// NewHandler creates an HTTP handler for the webhook endpoint.
func NewHandler(secret string, batchClient *batch.Client, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, _ := webhook.Validate(r, secret)

		switch result.StatusCode {
		case http.StatusUnauthorized:
			logger.Println("WARN: invalid signature attempt")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid signature"})
			return

		case http.StatusBadRequest:
			logger.Println("WARN: malformed payload")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid payload"})
			return

		case http.StatusNoContent:
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// StatusCode == 0 means validation passed → submit batch job
		logger.Printf("INFO: valid push received — %s %s", result.Payload.Ref, result.Payload.CommitSHA)

		params := batch.JobParams{
			RepoURL:   result.Payload.CloneURL,
			Branch:    result.Payload.Ref,
			CommitSHA: result.Payload.CommitSHA,
		}

		if err := batchClient.Submit(params); err != nil {
			logger.Printf("ERROR: batch submission failed — %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "batch submission failed"})
			return
		}

		logger.Println("INFO: batch job submitted")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "build_submitted"})
	})
}
