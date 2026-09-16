package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/thyfriendlyfox/lynx/internal/atomicfile"
)

// Default settings for the chat.
const (
	DefaultModel     = "claude-opus-5"
	FallbackModel    = "claude-opus-4-8"
	DefaultEffort    = "low"
	DefaultMaxTokens = 8000
)

// Provider names.
const (
	ProviderAnthropic = "anthropic"
	ProviderLocal     = "local"
)

// Config holds the chat settings of one person. It lives outside every
// shared folder, because a shared folder travels to other people.
type Config struct {
	Provider  string `json:"provider,omitempty"`
	Model     string `json:"model,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	Effort    string `json:"effort,omitempty"`
	MaxTokens int64  `json:"max_tokens,omitempty"`
}

// ConfigPath returns the file that holds the chat settings.
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lynx", "config.json"), nil
}

// LoadConfig reads the settings and applies the environment and the
// defaults. A missing file is not an error.
func LoadConfig() Config {
	var c Config
	if path, err := ConfigPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &c)
		}
	}
	if v := os.Getenv("LYNX_CHAT_PROVIDER"); v != "" {
		c.Provider = v
	}
	if v := os.Getenv("LYNX_CHAT_MODEL"); v != "" {
		c.Model = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" && c.APIKey == "" {
		c.APIKey = v
	}
	if c.Model == "" {
		c.Model = DefaultModel
	}
	if c.Effort == "" {
		c.Effort = DefaultEffort
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = DefaultMaxTokens
	}
	if c.Provider == "" {
		c.Provider = ProviderLocal
		if c.APIKey != "" {
			c.Provider = ProviderAnthropic
		}
	}
	return c
}

// SaveConfig writes the settings.
func SaveConfig(c Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, append(data, '\n'), 0o600)
}

// NewProvider returns the provider named in the settings.
func NewProvider(c Config) (Provider, error) {
	switch strings.ToLower(c.Provider) {
	case ProviderLocal:
		return &LocalProvider{}, nil
	case ProviderAnthropic:
		if c.APIKey == "" {
			return nil, ErrNoProvider
		}
		return NewAnthropicProvider(c), nil
	default:
		return nil, ErrUnknownProvider
	}
}
