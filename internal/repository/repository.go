package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"gbs/internal/config"
	"gbs/pkg/logger"

	_ "github.com/lib/pq"
)

var db *sql.DB

func InitDB() {
	cfg := config.GetConfig()

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.DBName, cfg.Database.SSLMode,
	)

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to open database: %s %s", dsn, err.Error()))
	}

	if err = db.Ping(); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to connect to database: %s %s", dsn, err.Error()))
	}

	logger.Info("Successfully connected to the database")
}

var GetUserIDHash = func(username string) (int, string, error) {
	var userID int
	var passwordHash string

	err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = $1", username).Scan(&userID, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn(fmt.Sprintf("User not found: %s", username))
			return 0, "", fmt.Errorf("User not found: %s", username)
		}
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return 0, "", err
	}

	return userID, passwordHash, nil
}

var RegisterUser = func(username string, passwordHash string) (int, error) {
	var userID int

	err := db.QueryRow("SELECT register_user($1, $2)", username, passwordHash).Scan(&userID)
	if err != nil {
		logger.Error(fmt.Sprintf("Database error: %s", err.Error()))
		return 0, err
	}

	if userID == 0 {
		logger.Warn(fmt.Sprintf("User already exists: %s", username))
		return 0, fmt.Errorf("User already exists: %s", username)
	}

	return userID, nil
}
