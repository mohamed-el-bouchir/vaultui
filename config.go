package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	VaultRoot string `toml:"vault_root"`
	Editor    string `toml:"editor"`
}

func loadConfig() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appConfigDir := filepath.Join(configDir, "vaultui")
	configPath := filepath.Join(appConfigDir, "config.toml")

	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	appConfigDir := filepath.Join(configDir, "vaultui")
	if err := os.MkdirAll(appConfigDir, 0700); err != nil {
		return err
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(appConfigDir, "config.toml"), data, 0600)
}
