package store

import "time"

// ProviderType classifies how to handle the API connection.
type ProviderType string

const (
	TypeAnthropic        ProviderType = "anthropic"
	TypeOpenAICompatible ProviderType = "openai_compatible"
	TypeOpenAI           ProviderType = "openai"
)

// Provider represents a configured AI API provider.
type Provider struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         ProviderType      `json:"type"`
	APIKey       string            `json:"api_key"`
	BaseURL      string            `json:"base_url"`
	Models       []string          `json:"models"`
	DefaultModel string            `json:"default_model"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
	IsActive     bool              `json:"is_active"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ProvidersFile is the top-level JSON structure persisted to disk.
type ProvidersFile struct {
	Version           int        `json:"version"`
	Providers         []Provider `json:"providers"`
	ActiveProviderID  string     `json:"active_provider_id"`
}

// ProxyState records the running proxy's runtime info.
type ProxyState struct {
	PID        int    `json:"pid"`
	Port       int    `json:"port"`
	ProviderID string `json:"provider_id"`
	Running    bool   `json:"running"`
}

// ServerSettings stores server-level preferences.
type ServerSettings struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}
