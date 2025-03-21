package auth

import (
	"fmt"
	"gbs/internal/config"
	"gbs/internal/repository"
	"gbs/pkg/logger"
	"regexp"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var RegisterUser = func(login, password string) (string, string, error) {
	if !validateUsername(login) {
		return "", "", fmt.Errorf("invalid username")
	}
	if !validatePassword(password) {
		return "", "", fmt.Errorf("invalid password")
	}
	hash, err := generatePasswordHash(password)
	if err != nil {
		return "", "", err
	}
	userID, err := repository.RegisterUser(login, hash)
	if err != nil {
		return "", "", err
	}
	token, err := generateJWT(userID)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := generateRefreshToken(userID)
	if err != nil {
		return "", "", err
	}
	return token, refreshToken, nil
}

var Login = func(login, password string) (string, string, error) {
	id, hash, err := repository.GetUserIDHash(login)
	if err != nil {
		return "", "", err
	}
	if hash == "" {
		return "", "", fmt.Errorf("Couldnt find hash for user " + login)
	}
	if !compareHashes(hash, password) {
		return "", "", fmt.Errorf("Invalid password for user " + login)
	}
	token, err := generateJWT(id)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := generateRefreshToken(id)
	if err != nil {
		return "", "", err
	}
	return token, refreshToken, nil
}

var ChangePassword = func(initiatorID, userID int, password string) error {
	if !validatePassword(password) {
		return fmt.Errorf("invalid password")
	}
	hash, err := generatePasswordHash(password)
	if err != nil {
		return fmt.Errorf("invalid password")
	}
	err = repository.ChangePassword(initiatorID, userID, hash)
	if err != nil {
		return err
	}
	err = repository.InvalidateRefreshTokens(userID)
	if err != nil {
		logger.Error("Couldnt invalidate refresh tokens")
		return fmt.Errorf("Couldnt invalidate refresh tokens")
	}
	return nil
}

var generateJWT = func(id int) (string, error) {
	tokenLifespan, err := time.ParseDuration(config.GetConfig().Security.TokenExpiry)
	if err != nil {
		logger.Fatal("Invalid token lifespan " + config.GetConfig().Security.TokenExpiry)
	}
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(tokenLifespan).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.GetConfig().Security.JwtSecret))
}

var generateRefreshToken = func(userID int) (string, error) {
	duration, err := time.ParseDuration(config.GetConfig().Security.RefreshTokenExpiry)
	if err != nil {
		logger.Fatal("Invalid refresh token expiry " + config.GetConfig().Security.RefreshTokenExpiry)
	}
	newRefreshToken, err := repository.CreateRefreshToken(userID, time.Now().Add(duration))
	if err != nil {
		return "", err
	}
	return newRefreshToken, nil
}

var RefreshJWT = func(refreshToken string) (string, error) {
	userID, err := repository.GetUserByRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}
	if userID == -1 {
		return "", fmt.Errorf("invalid refresh token")
	}
	token, err := generateJWT(userID)
	if err != nil {
		return "", err
	}
	return token, nil
}

var GetUserIDFromJWT = func(tokenString string) (int, error) {
	secret := []byte(config.GetConfig().Security.JwtSecret)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			logger.Debug("Unexpected signing method")
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		logger.Debug("Couldn't parse token")
		return 0, fmt.Errorf("invalid token: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if userIDFloat, ok := claims["user_id"].(float64); ok {
			return int(userIDFloat), nil
		}
		logger.Debug("Couldn't parse user id from token")
		return 0, fmt.Errorf("user_id not found in token")
	}

	logger.Debug("Invalid token claims")
	return 0, fmt.Errorf("invalid token claims")
}

var validateUsername = func(username string) bool {
	cfg := config.GetConfig()
	pattern := `^[a-zA-Z0-9!@#$%^&*()-_=+{}[\]|:;"'<>,.?/~` + "`" + `]+$`
	matched, _ := regexp.MatchString(pattern, username)
	usernameLen := len(username)
	return matched && usernameLen >= cfg.Security.LoginMinLength && usernameLen <= cfg.Security.LoginMaxLength
}

var validatePassword = func(password string) bool {
	cfg := config.GetConfig()
	pattern := `^[a-zA-Z0-9!@#$%^&*()-_=+{}[\]|:;"'<>,.?/~` + "`" + `]+$`
	matched, _ := regexp.MatchString(pattern, password)
	passLen := len(password)
	return matched && passLen >= cfg.Security.PasswordMinLength && passLen <= cfg.Security.PasswordMaxLength
}

var generatePasswordHash = func(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Hash generation error")
		return "", err
	}
	return string(hashedPassword), nil
}

var compareHashes = func(hash string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
