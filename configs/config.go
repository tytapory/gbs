package config

import (
	"encoding/json"
	"os"
	"time"
)

var cfg *Config

type Config struct {
	Database DatabaseConfig `json:"database"`
	Server   ServerConfig   `json:"server"`
	Logging  LoggingConfig  `json:"logging"`
	Security SecurityConfig `json:"security"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type SecurityConfig struct {
	JwtSecret   string
	TokenExpiry time.Duration `json:"token_expiry"`
}

type LoggingConfig struct {
	Level string `json:"level"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

func GetConfig() Config {
	if cfg != nil {
		LoadConfig()
	}
	return *cfg
}

func LoadConfig() {
	err := loadConfigFromFile("config.json")
	if err == nil {
		return
	}
	err = loadConfigFromFile("default-config.json")
	if err == nil {
		return
	}
	panic(err)
}

func loadConfigFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return err
	}
	return nil
}
