package agent

import (
	"fmt"
	"os"
)

// Config holds agent configuration from environment variables.
type Config struct {
	APIEndpoint string
	APIKey      string
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() (*Config, error) {
	endpoint := os.Getenv("UW_API_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.upwatchly.com"
	}

	apiKey := os.Getenv("UW_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("UW_API_KEY is required")
	}

	return &Config{
		APIEndpoint: endpoint,
		APIKey:      apiKey,
	}, nil
}
