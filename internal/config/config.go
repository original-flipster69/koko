package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Provider string

const (
	Anthropic Provider = "anthropic"
	Mistral   Provider = "mistral"
	Ollama    Provider = "ollama"
)

type ExecProfile string

const (
	Strict     ExecProfile = "strict"
	Default    ExecProfile = "default"
	Permissive ExecProfile = "permissive"
	Off        ExecProfile = "off"
)

type IgnoreMode string

const (
	Gitignore IgnoreMode = "gitignore"
	Custom    IgnoreMode = "custom"
)

type LlmConfig struct {
	Provider         Provider `toml:"provider"`
	Model            string   `toml:"model"`
	ApiKey           string   `toml:"-"`
	Url              string   `toml:"url"`
	MaxTokens        int      `toml:"max_tokens"`
	MaxSessionTokens int      `toml:"max_session_tokens"`
	Conversations    bool     `toml:"conversations"`
}

type SandboxConfig struct {
	Root                    string     `toml:"root"`
	AdditionalDirs          []string   `toml:"additional_dirs"`
	DenyFiles               []string   `toml:"deny_files"`
	MaxFileSize             int64      `toml:"max_file_size"`
	ScrubPII                bool       `toml:"scrub_pii"`
	SuppressElevatedWarning bool       `toml:"suppress_elevated_warning"`
	SuppressPrivacyWarning  bool       `toml:"suppress_privacy_warning"`
	Exec                    ExecConfig `toml:"exec"`
}

type ExecConfig struct {
	Profile ExecProfile `toml:"profile"`
	Allow   []string    `toml:"allow"`
	Deny    []string    `toml:"deny"`
}

type IgnoreConfig struct {
	Mode  IgnoreMode `toml:"mode"`
	Files []string   `toml:"files"`
}

type StyleConfig struct {
	ThinkingVerbs []string       `toml:"thinking_verbs"`
	ColorScheme   map[string]int `toml:"color_scheme"`
}

type Config struct {
	Llm     LlmConfig     `toml:"llm"`
	Sandbox SandboxConfig `toml:"sandbox"`
	Ignore  IgnoreConfig  `toml:"ignore"`
	Style   StyleConfig   `toml:"style"`
}

func defaultConf() *Config {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return &Config{
		Llm: LlmConfig{
			Provider:         Mistral,
			Model:            "mistral-large-latest",
			Url:              "",
			MaxTokens:        16384,
			MaxSessionTokens: 1_000_000,
			Conversations:    true,
		},
		Sandbox: SandboxConfig{
			Root: cwd,
			DenyFiles: []string{
				".env", ".env.*", "*.pem", "*.key", "id_rsa*",
				"credentials.json", "*.secret", "*.password",
			},
			MaxFileSize: 1024 * 1024,
			ScrubPII:    true,
			Exec: ExecConfig{
				Profile: Default,
			},
		},
		Ignore: IgnoreConfig{
			Mode: Gitignore,
		},
		Style: StyleConfig{
			ThinkingVerbs: []string{
				"banana munching", "marinating", "concocting", "brewing",
			},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := defaultConf()

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

func (c *Config) ApplyFlags(provider, model, llmUrl, sandbox string) {
	if provider != "" {
		c.Llm.Provider = Provider(provider)
	}
	if model != "" {
		c.Llm.Model = model
	}
	if llmUrl != "" {
		c.Llm.Url = llmUrl
	}
	if sandbox != "" {
		c.Sandbox.Root = sandbox
	}
}

func (c *Config) ApplyEnv() {
	envName := apiKeyEnvName(c.Llm.Provider)
	if v := os.Getenv(envName); v != "" {
		c.Llm.ApiKey = v
	}
}

func (c *Config) Validate() error {
	if err := c.Llm.Validate(); err != nil {
		return err
	}
	if err := c.Sandbox.Validate(); err != nil {
		return err
	}
	if err := c.Ignore.Validate(); err != nil {
		return err
	}
	return nil
}

func (l *LlmConfig) Validate() error {
	switch l.Provider {
	case Anthropic, Mistral, Ollama:
	default:
		return fmt.Errorf("unknown llm.provider: %q (must be anthropic, mistral, or ollama)", l.Provider)
	}
	if (l.Provider == Anthropic || l.Provider == Mistral) && l.ApiKey == "" {
		return fmt.Errorf("%s provider requires an API key (set %s)", l.Provider, apiKeyEnvName(l.Provider))
	}
	if l.Model == "" {
		return fmt.Errorf("llm.model must not be empty")
	}
	if err := checkModelProvider(l.Provider, l.Model); err != nil {
		return err
	}
	if l.MaxTokens <= 0 {
		return fmt.Errorf("llm.max_tokens must be positive (got %d)", l.MaxTokens)
	}
	if l.MaxSessionTokens < 0 {
		return fmt.Errorf("llm.max_session_tokens must be non-negative (got %d; use 0 for unlimited)", l.MaxSessionTokens)
	}
	if l.Url != "" {
		if err := validateLlmUrl(l.Url); err != nil {
			return fmt.Errorf("llm.url: %w", err)
		}
	}
	return nil
}

func validateLlmUrl(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("url must include a host")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
		return fmt.Errorf("plain http only allowed for localhost (got host %q)", host)
	default:
		return fmt.Errorf("scheme must be https or http (got %q)", u.Scheme)
	}
}

func (s *SandboxConfig) AllowedDirs() []string {
	return append([]string{s.Root}, s.AdditionalDirs...)
}

func (s *SandboxConfig) Validate() error {
	if s.Root == "" {
		return fmt.Errorf("sandbox.root must not be empty")
	}
	if !filepath.IsAbs(s.Root) {
		return fmt.Errorf("sandbox.root must be absolute (got %q)", s.Root)
	}
	for _, d := range s.AdditionalDirs {
		if !filepath.IsAbs(d) {
			return fmt.Errorf("sandbox.additional_dirs entry must be absolute (got %q)", d)
		}
	}
	if s.MaxFileSize <= 0 {
		return fmt.Errorf("sandbox.max_file_size must be positive (got %d)", s.MaxFileSize)
	}
	if err := s.Exec.Validate(); err != nil {
		return err
	}
	return nil
}

func (i *IgnoreConfig) Validate() error {
	switch i.Mode {
	case "", Gitignore, Custom:
	default:
		return fmt.Errorf("unknown ignore.mode: %q (must be gitignore or custom)", i.Mode)
	}
	return nil
}

func (e *ExecConfig) Validate() error {
	switch e.Profile {
	case "", Strict, Default, Permissive, Off:
	default:
		return fmt.Errorf("unknown sandbox.exec.profile: %q (must be strict, default, permissive, or off)", e.Profile)
	}
	return nil
}

func (e *ExecConfig) Limits() (cpuSec, memMB, fileMB int) {
	switch e.Profile {
	case Strict:
		return 30, 512, 100
	case Permissive:
		return 1800, 8192, 2000
	case Off:
		return 0, 0, 0
	default:
		return 300, 2048, 500
	}
}

func checkModelProvider(p Provider, model string) error {
	switch {
	case strings.HasPrefix(model, "Anthropic-") && p != Anthropic:
		return fmt.Errorf("model %q looks Anthropic but provider is %q", model, p)
	case (strings.HasPrefix(model, "mistral-") ||
		strings.HasPrefix(model, "codestral-") ||
		strings.HasPrefix(model, "magistral-")) && p != Mistral:
		return fmt.Errorf("model %q looks Mistral but provider is %q", model, p)
	}
	return nil
}

func apiKeyEnvName(p Provider) string {
	switch p {
	case Anthropic:
		return "CLAUDE_API_KEY"
	case Mistral:
		return "MISTRAL_API_KEY"
	}
	return ""
}

func Path(kokoDir string) string {
	return filepath.Join(kokoDir, "config.toml")
}
