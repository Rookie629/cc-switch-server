package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeSettings represents the structure Claude Code expects in settings.json.
type ClaudeSettings struct {
	Env map[string]string `json:"env"`
}

// ConfigWriter writes provider configuration into Claude Code's settings.json.
type ConfigWriter struct {
	claudeSettingsPath string
}

// NewConfigWriter creates a ConfigWriter targeting ~/.claude/settings.json.
func NewConfigWriter() *ConfigWriter {
	home, _ := os.UserHomeDir()
	return &ConfigWriter{
		claudeSettingsPath: filepath.Join(home, ".claude", "settings.json"),
	}
}

// WriteProvider writes the provider's API key and base URL into Claude Code's
// settings file. It merges with existing settings — only env keys are touched.
// Settings file is created if it doesn't exist.
func (cw *ConfigWriter) WriteProvider(apiKey, baseURL string, extraEnv map[string]string) error {
	settings := make(map[string]interface{})

	// Read existing settings if present
	if data, err := os.ReadFile(cw.claudeSettingsPath); err == nil {
		json.Unmarshal(data, &settings)
	}

	// Ensure env map exists
	envRaw, ok := settings["env"]
	if !ok {
		envRaw = make(map[string]interface{})
	}
	env, ok := envRaw.(map[string]interface{})
	if !ok {
		env = make(map[string]interface{})
	}

	// Set provider values
	env["ANTHROPIC_BASE_URL"] = baseURL
	env["ANTHROPIC_AUTH_TOKEN"] = apiKey

	// Apply extra env vars
	for k, v := range extraEnv {
		env[k] = v
	}

	settings["env"] = env

	// Ensure the .claude directory exists
	claudeDir := filepath.Dir(cw.claudeSettingsPath)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create claude config dir: %w", err)
	}

	// Write atomically
	tmpFile := cw.claudeSettingsPath + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create temp settings file: %w", err)
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(settings); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("encode settings: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpFile, cw.claudeSettingsPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("write settings file: %w", err)
	}

	return nil
}

// Backup creates a timestamped backup of the current Claude Code settings.
// Returns the backup path or empty string if no settings file exists.
func (cw *ConfigWriter) Backup() (string, error) {
	data, err := os.ReadFile(cw.claudeSettingsPath)
	if err != nil {
		return "", nil // no settings to back up
	}

	backupPath := cw.claudeSettingsPath + ".cc-switch-backup"
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return "", fmt.Errorf("backup settings: %w", err)
	}
	return backupPath, nil
}

// RestoreBackup restores the most recent backup. Returns nil if no backup exists.
func (cw *ConfigWriter) RestoreBackup() error {
	backupPath := cw.claudeSettingsPath + ".cc-switch-backup"
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return nil // no backup to restore
	}

	tmpFile := cw.claudeSettingsPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}
	if err := os.Rename(tmpFile, cw.claudeSettingsPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("restore settings: %w", err)
	}

	// Clean up backup
	os.Remove(backupPath)
	return nil
}
