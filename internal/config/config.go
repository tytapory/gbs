package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gbs/pkg/logger"
	"os"

	"github.com/joho/godotenv"
)

var cfg *Config

type Config struct {
	Database DatabaseConfig `json:"database"`
	Server   ServerConfig   `json:"server"`
	Logging  LoggingConfig  `json:"logging"`
	Security SecurityConfig `json:"security"`
	Core     CoreConfig     `json:"core"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type SecurityConfig struct {
	TokenExpiry        string `json:"token_expiry"`
	RefreshTokenExpiry string `json:"refresh_token_expiry"`
	LockoutDuration    string `json:"lockout_duration"`
	JwtSecret          string
	LoginMinLength     int `json:"login_min_length"`
	LoginMaxLength     int `json:"login_max_length"`
	PasswordMinLength  int `json:"password_min_length"`
	PasswordMaxLength  int `json:"password_max_length"`
	MaxLoginAttempts   int `json:"max_login_attempts"`
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

type CoreConfig struct {
	CoreFee int `json:"fee"`
}

var dotEnvLocation = "../../configs/.env"
var fileOpenFunc = os.Open

func GetConfig() Config {
	if cfg == nil {
		logger.Debug("Config is not cached, caching now...")
		loadConfig()
		loadEnv()
	} else {
		logger.Debug("Returning cached config")
	}
	return *cfg
}

var loadConfig = func() {
	logger.Info("Loading config")
	err := loadConfigFromFile("../../config/config.json")
	if err == nil {
		return
	}
	logger.Warn(fmt.Sprintf("Can't open user config, trying to open default config: %s", err.Error()))
	err = loadConfigFromFile("../../config/default-config.json")
	if err == nil {
		return
	}
	logger.Fatal(fmt.Sprintf("Can't open default config: %s", err.Error()))
}

var loadConfigFromFile = func(filename string) error {
	logger.Debug(fmt.Sprintf("Attempting to load configuration from '%s'", filename))
	file, err := fileOpenFunc(filename)
	if err != nil {
		logger.Warn(fmt.Sprintf("Could not open file '%s': %v", filename, err))
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		logger.Error(fmt.Sprintf("Failed to decode JSON from file '%s': %s", filename, err.Error()))
		return err
	}
	logger.Debug(fmt.Sprintf("Successfully decoded configuration from '%s'", filename))
	return nil
}

var loadEnv = func() {
	logger.Info("Loading JWT secret key")
	jwtSecret, exists := os.LookupEnv("GBS_JWT_KEY")
	if exists {
		logger.Info("Key was found outside of .env")
		cfg.Security.JwtSecret = jwtSecret
		return
	}
	if _, err := os.Stat(dotEnvLocation); os.IsNotExist(err) {
		logger.Warn(fmt.Sprintf(".env file does not exist. This is okay if it's first launch: %s", err.Error()))
		logger.Info("Trying to create new .env file and new key")
		createEnvFile()
	}
	logger.Info("Opening .env file")
	if err := godotenv.Load(dotEnvLocation); err != nil {
		logger.Fatal(fmt.Sprintf("Can't load .env: %s", err.Error()))
	}
	jwtSecret, exists = os.LookupEnv("GBS_JWT_SECRET")
	if !exists || jwtSecret == "" {
		logger.Fatal("Can't find 'GBS_JWT_KEY'")
	}
	cfg.Security.JwtSecret = jwtSecret
}

var createEnvFile = func() {
	file, err := os.Create(dotEnvLocation)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Can't create .env file: %s", err.Error()))
	}
	defer file.Close()
	key, err := generateKey()
	if err != nil {
		logger.Fatal(fmt.Sprintf("Can't create key: %s", err.Error()))
	}
	fmt.Fprintf(file, "GBS_JWT_SECRET=%s\n", key)
	logger.Info(".env was successfully created")
}

func generateKey() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}
