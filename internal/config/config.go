package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderMistral   ProviderType = "mistral"
	ProviderOllama    ProviderType = "ollama"
)

type Config struct {
	Provider            ProviderType `json:"provider"`
	Model               string       `json:"model"`
	APIKey              string       `json:"-"`
	BaseURL             string       `json:"base_url"`
	MaxTokens           int          `json:"max_tokens"`
	SandboxRoot         string       `json:"sandbox_root"`
	AllowedDirs         []string     `json:"allowed_dirs"`
	DenyFiles           []string     `json:"deny_files"`
	IgnoreFiles         []string     `json:"ignore_files"`
	MaxFileSize         int64        `json:"max_file_size"`
	ThinkingVerbs       []string     `json:"thinking_verbs"`
	CommandAllowlist    []string     `json:"command_allowlist"`
	CommandDenyPatterns []string     `json:"command_deny_patterns"`
	MaxToolCalls        int          `json:"max_tool_calls"`
	MaxSessionTokens    int          `json:"max_session_tokens"`
	ExecCPUSeconds      int          `json:"exec_cpu_seconds"`
	ExecMemoryMB        int          `json:"exec_memory_mb"`
	ExecMaxFileMB       int          `json:"exec_max_file_mb"`
	ScrubPII            bool         `json:"scrub_pii"`
	QuietToolOutputs    []string     `json:"quiet_tool_outputs"`
}

func DefaultConfig() *Config {
	cwd, _ := os.Getwd()
	return &Config{
		Provider:    ProviderMistral,
		Model:       "mistral-large-latest",
		BaseURL:     "",
		MaxTokens:   16384,
		SandboxRoot: cwd,
		AllowedDirs: []string{cwd},
		DenyFiles: []string{
			".env", ".env.*", "*.pem", "*.key", "id_rsa*",
			"credentials.json", "*.secret", "*.password",
		},
		MaxFileSize: 1024 * 1024,
		ThinkingVerbs: []string{
			"pondering", "scheming", "plotting",
			"cogitating", "musing", "ruminating", "brewing",
			"conjuring", "divining", "reckoning", "untangling",
		},
		CommandAllowlist:    nil,
		CommandDenyPatterns: nil,
		MaxToolCalls:        200,
		MaxSessionTokens:    1_000_000,
		ExecCPUSeconds:      30,
		ExecMemoryMB:        512,
		ExecMaxFileMB:       100,
		ScrubPII:            true,
		QuietToolOutputs:    []string{"read_file", "search_files", "exec_command"},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.APIKey = os.Getenv(APIKeyEnvVar(cfg.Provider))
	return cfg, nil
}

func (c *Config) Validate() error {
	switch c.Provider {
	case ProviderAnthropic, ProviderMistral, ProviderOllama:
	default:
		return fmt.Errorf("unknown provider: %q (must be anthropic, mistral, or ollama)", c.Provider)
	}
	if c.Model == "" {
		return fmt.Errorf("model must not be empty")
	}
	if c.SandboxRoot == "" {
		return fmt.Errorf("sandbox root must not be empty")
	}
	if len(c.AllowedDirs) == 0 {
		return fmt.Errorf("at least one allowed directory is required")
	}
	return nil
}

func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".koko", "config.json")
}

func APIKeyEnvVar(p ProviderType) string {
	switch p {
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case ProviderMistral:
		return "MISTRAL_API_KEY"
	default:
		return ""
	}
}
