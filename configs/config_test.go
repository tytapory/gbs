package config

import (
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestGetConfig(t *testing.T) {
	originalLoadConfig := loadConfig
	originalLoadEnv := loadEnv
	defer func() {
		loadConfig = originalLoadConfig
		loadEnv = originalLoadEnv
		cfg = nil
	}()

	//caching and getting config
	loadConfig = func() {
		cfg = &Config{
			Server: ServerConfig{
				Host: "test_host",
				Port: "test_port",
			},
		}
	}
	loadEnv = func() {
		cfg.Security.JwtSecret = "test_secret"
	}
	config := GetConfig()
	assert.Equal(t, ServerConfig{Host: "test_host", Port: "test_port"}, config.Server, "config was not load properly")
	assert.Equal(t, "test_secret", config.Security.JwtSecret, "config was not load properly")

	//loading cached config
	loadConfig = func() {
		cfg = &Config{
			Server: ServerConfig{
				Host: "test_host_new",
				Port: "test_port_new",
			},
		}
	}
	loadEnv = func() {
		cfg.Security.JwtSecret = "test_secret_new"
	}
	config = GetConfig()

	//results have to be the same
	assert.Equal(t, ServerConfig{Host: "test_host", Port: "test_port"}, config.Server, "config was not cached properly")
	assert.Equal(t, "test_secret", config.Security.JwtSecret, "config was not cached properly")
}

func TestLoadConfigUserConfigExist(t *testing.T) {
	originalLoadConfigFromFile := loadConfigFromFile
	defer func() {
		loadConfigFromFile = originalLoadConfigFromFile
		cfg = nil
	}()
	loadConfigFromFile = func(filename string) error {
		if filename == "config.json" {
			cfg = &Config{
				Server: ServerConfig{
					Host: "test_host_user_config",
					Port: "test_port_user_config",
				},
			}
		} else {
			cfg = &Config{
				Server: ServerConfig{
					Host: "test_host_default_config",
					Port: "test_port_default_config",
				},
			}
		}
		return nil
	}
	loadConfig()
	assert.Equal(t, ServerConfig{Host: "test_host_user_config", Port: "test_port_user_config"}, cfg.Server, "config was not loaded properly")
}

func TestLoadConfigUserConfigDoesNotExist(t *testing.T) {
	originalLoadConfigFromFile := loadConfigFromFile
	defer func() {
		loadConfigFromFile = originalLoadConfigFromFile
		cfg = nil
	}()
	loadConfigFromFile = func(filename string) error {
		if filename == "config.json" {
			return errors.Errorf("mock error")
		} else {
			cfg = &Config{
				Server: ServerConfig{
					Host: "test_host_default_config",
					Port: "test_port_default_config",
				},
			}
		}
		return nil
	}
	loadConfig()
	assert.Equal(t, ServerConfig{Host: "test_host_default_config", Port: "test_port_default_config"}, cfg.Server, "config was not loaded properly")
}

func TestLoadConfigFromFileSuccess(t *testing.T) {
	defer func() {
		cfg = nil
	}()
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temporary config file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configContent := `{
		"server": {"host": "localhost", "port": "8080"},
		"security": {"token_expiry": "30m"}
	}`
	_, err = tmpFile.WriteString(configContent)
	if err != nil {
		t.Fatalf("failed to write to temporary config file: %v", err)
	}
	tmpFile.Close()

	cfg = nil
	err = loadConfigFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to load config from file: %v", err)
	}

	assert.Equal(t, "localhost", cfg.Server.Host, "Host was not loaded properly from file")
	assert.Equal(t, "8080", cfg.Server.Port, "Port was not loaded properly from file")
	assert.Equal(t, "30m", cfg.Security.TokenExpiry, "Token expiry was not loaded properly from file")
}

func TestLoadConfigFromFileFailure(t *testing.T) {
	err := loadConfigFromFile("non-existent-file.json")
	assert.Error(t, err, "Expected an error when loading a non-existent file")
}

func TestLoadEnvKeyExists(t *testing.T) {
	cfg = &Config{}
	defer func() {
		cfg = nil
	}()
	err := os.Setenv("GBS_JWT_KEY", "existing_key")
	if err != nil {
		t.Fatalf("failed to set environment variable: %v", err)
	}
	defer os.Unsetenv("GBS_JWT_KEY")

	loadEnv()

	assert.Equal(t, "existing_key", cfg.Security.JwtSecret, "JWT secret key was not loaded properly from environment")
}
