package util

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	secretsCache map[string]string
	once         sync.Once
)

func GetSecret(key string, required bool) string {
	val, ok := os.LookupEnv(key)
	if ok {
		return val
	}

	if os.Getenv("SECRETS_EXTENSION_ENABLED") == "true" {
		once.Do(func() {
			secrets, err := fetchSecretsFromExtension()
			if err != nil {
				panic(fmt.Sprintf("failed to fetch secrets: %v", err))
			}

			secretsCache = secrets
		})

		if secret, ok := secretsCache[key]; ok {
			return secret
		}
	}

	if required {
		panic(fmt.Sprintf("secret %s is not set", key))
	}

	return ""
}

func fetchSecretsFromExtension() (map[string]string, error) {
	secretName := os.Getenv("DOPPLER_SECRET_NAME")
	if secretName == "" {
		return nil, fmt.Errorf("DOPPLER_SECRET_NAME is not set")
	}

	url := fmt.Sprintf("http://localhost:2773/secretsmanager/get?secretId=%s", secretName)

	delay := 50 * time.Millisecond
	maxRetries := 3

	for range maxRetries {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Aws-Parameters-Secrets-Token", os.Getenv("AWS_SESSION_TOKEN"))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusOK {
			var secretsResponse struct {
				SecretString string `json:"SecretString"`
			}
			if err := json.Unmarshal(body, &secretsResponse); err != nil {
				return nil, err
			}

			var secrets map[string]string
			if err := json.Unmarshal([]byte(secretsResponse.SecretString), &secrets); err != nil {
				return nil, err
			}

			return secrets, nil
		} else {
			slog.Error(fmt.Sprintf("failed to fetch secrets: %s", string(body)))
		}

		if strings.Contains(string(body), "not ready to serve traffic") {
			time.Sleep(delay)
			delay *= 2
			continue
		}

		return nil, fmt.Errorf("failed to fetch secrets: %s", string(body))
	}

	return nil, fmt.Errorf("failed to fetch secrets after %d retries", maxRetries)
}
