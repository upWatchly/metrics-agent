package agent

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds agent configuration from environment variables.
type Config struct {
	APIEndpoint      string
	APIKey           string
	DisableKeepAlive bool
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

	disableKeepAlive, _ := strconv.ParseBool(os.Getenv("UW_DISABLE_KEEP_ALIVE"))

	return &Config{
		APIEndpoint:      endpoint,
		APIKey:           apiKey,
		DisableKeepAlive: disableKeepAlive,
	}, nil
}
