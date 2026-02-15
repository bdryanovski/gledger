package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DataFile   string            `yaml:"data_file"`
	DateFormat string            `yaml:"date_format"`
	Currency   string            `yaml:"currency"`
	Aliases    map[string]string `yaml:"aliases"`
	Theme      ThemeConfig       `yaml:"theme"`
}

type ThemeConfig struct {
	PrimaryColor     string `yaml:"primary_color"`
	SecondaryColor   string `yaml:"secondary_color"`
	BackgroundColor  string `yaml:"background_color"`
	TextColor        string `yaml:"text_color"`
	TableBorderStyle string `yaml:"table_border_style"`
}

func DefaultConfig() *Config {
	return &Config{
		DataFile:   "~/.gledger/data.txt",
		DateFormat: "2006-01-02",
		Currency:   "USD",
		Aliases: map[string]string{
			"exp": "expenses",
			"inc": "income",
			"ast": "assets",
		},
		Theme: ThemeConfig{
			PrimaryColor:     "#00ff00",
			TableBorderStyle: "rounded",
		},
	}
}

func LoadConfig() (*Config, error) {

	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfig(), nil
	}

	configPath := home + "/.gledger/config.yaml"

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			config := DefaultConfig()
			if err := config.Save(); err != nil {
				return config, nil
			}
			return config, nil
		}
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (config *Config) Save() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDirectory := filepath.Join(home, ".gledger")
	if err := os.MkdirAll(configDirectory, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(configDirectory, "config.yaml")

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
