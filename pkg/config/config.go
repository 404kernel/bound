package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	TelegramBot    string            `json:"telegram_bot"`
	TelegramChatID string            `json:"telegram_chat_id"`
	WebhookURL     string            `json:"webhook_url"`
	UpstreamAPI    string            `json:"upstream_api"`
	APIKeys        map[string]string `json:"api_keys"`
}

func DefaultConfig() Config {
	return Config{
		UpstreamAPI: "https://api.openai.com",
		APIKeys:     make(map[string]string),
	}
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".bound", "config.json")
}

func LoadConfig() Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	if cfg.APIKeys == nil {
		cfg.APIKeys = make(map[string]string)
	}
	return cfg
}

func SaveConfig(cfg Config) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
