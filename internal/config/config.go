package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"gbs/pkg/logger"

	"github.com/joho/godotenv"
)

var _ ConfigProvider = &configProviderImplementation{}

type ConfigProvider interface {
	Get() Config
}

type configProviderImplementation struct {
	cfg *Config
}

func NewConfigProviderImplementation() ConfigProvider {
	return &configProviderImplementation{}
}

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
	TokenExpiry             string `json:"token_expiry"`
	RefreshTokenExpiry      string `json:"refresh_token_expiry"`
	LockoutDuration         string `json:"lockout_duration"`
	JwtSecret               string
	LoginMinLength          int  `json:"login_min_length"`
	LoginMaxLength          int  `json:"login_max_length"`
	PasswordMinLength       int  `json:"password_min_length"`
	PasswordMaxLength       int  `json:"password_max_length"`
	MaxLoginAttempts        int  `json:"max_login_attempts"`
	AllowDirectRegistration bool `json:"allow_direct_registration"`
	RPMForIP                int  `json:"rpm_for_ip"`
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
	CoreFee int64 `json:"fee"`
}

var dotEnvLocation = "configs/.env"
var fileOpenFunc = os.Open

func (c *configProviderImplementation) Get() Config {
	if c.cfg == nil {
		logger.Debug("Config is not cached, caching now...")
		c.loadConfig()
		c.loadEnv()
	} else {
		logger.Debug("Returning cached config")
	}
	return *c.cfg
}

func (c *configProviderImplementation) loadConfig() {
	logger.Info("Loading config")
	err := c.loadConfigFromFile("configs/config.json")
	if err == nil {
		return
	}
	logger.Warn(fmt.Sprintf("Can't open user config, trying to open default config: %s", err.Error()))
	err = c.loadConfigFromFile("configs/default_config.json")
	if err != nil {
		logger.Fatal(fmt.Sprintf("Can't open default config: %s", err.Error()))
	}
}

func (c *configProviderImplementation) loadConfigFromFile(filename string) error {
	logger.Debug(fmt.Sprintf("Attempting to load configuration from '%s'", filename))
	file, err := fileOpenFunc(filename)
	if err != nil {
		logger.Warn(fmt.Sprintf("Could not open file '%s': %v", filename, err))
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&c.cfg); err != nil {
		logger.Error(fmt.Sprintf("Failed to decode JSON from file '%s': %s", filename, err.Error()))
		return err
	}
	logger.Debug(fmt.Sprintf("Successfully decoded configuration from '%s'", filename))
	return nil
}

func (c *configProviderImplementation) loadEnv() {
	logger.Info("Loading JWT secret key")
	jwtSecret, exists := os.LookupEnv("GBS_JWT_KEY")
	if exists {
		logger.Info("Key was found outside of .env")
		c.cfg.Security.JwtSecret = jwtSecret
		return
	}
	if _, err := os.Stat(dotEnvLocation); os.IsNotExist(err) {
		logger.Warn(fmt.Sprintf(".env file does not exist. This is okay if it's first launch: %s", err.Error()))
		logger.Info("Trying to create new .env file and new key")
		c.createEnvFile()
	}
	logger.Info("Opening .env file")
	if err := godotenv.Load(dotEnvLocation); err != nil {
		logger.Fatal(fmt.Sprintf("Can't load .env: %s", err.Error()))
	}
	jwtSecret, exists = os.LookupEnv("GBS_JWT_SECRET")
	if !exists || jwtSecret == "" {
		logger.Fatal("Can't find 'GBS_JWT_KEY'")
	}
	c.cfg.Security.JwtSecret = jwtSecret
}

func (c *configProviderImplementation) createEnvFile() {
	file, err := os.Create(dotEnvLocation)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Can't create .env file: %s", err.Error()))
	}
	defer file.Close()
	key, err := c.generateKey()
	if err != nil {
		logger.Fatal(fmt.Sprintf("Can't create key: %s", err.Error()))
	}
	fmt.Fprintf(file, "GBS_JWT_SECRET=%s\n", key)
	logger.Info(".env was successfully created")
}

func (c *configProviderImplementation) generateKey() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}
