package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("USERPROFILE")
	if origHome == "" {
		origHome = os.Getenv("HOME")
	}
	defer func() {
		_ = os.Setenv("USERPROFILE", origHome)
		_ = os.Setenv("HOME", origHome)
	}()

	_ = os.Setenv("USERPROFILE", tmpDir)
	_ = os.Setenv("HOME", tmpDir)

	cfg := Config{
		TelegramBot:    "12345:TESTBOT",
		TelegramChatID: "99887766",
		WebhookURL:     "https://discord.com/api/webhooks/test",
		UpstreamAPI:    "https://api.openai.com",
		APIKeys: map[string]string{
			"openai": "sk-mock-key-12345",
		},
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded := LoadConfig()
	if loaded.TelegramBot != cfg.TelegramBot {
		t.Errorf("expected bot %s, got %s", cfg.TelegramBot, loaded.TelegramBot)
	}
	if loaded.APIKeys["openai"] != "sk-mock-key-12345" {
		t.Errorf("expected key sk-mock-key-12345, got %s", loaded.APIKeys["openai"])
	}

	expectedPath := filepath.Join(tmpDir, ".bound", "config.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("expected config file to exist at %s", expectedPath)
	}
}
