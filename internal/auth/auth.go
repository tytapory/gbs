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

var registerUser = func() {

}

var GenerateAuthJWT = func(login, password string) (string, error) {
	id, hash, err := repository.GetUserIDHash(login)
	if err != nil {
		return "", err
	}
	if hash == "" {
		return "", fmt.Errorf("Couldnt find hash for user " + login)
	}
	if !compareHashes(hash, password) {
		return "", fmt.Errorf("Invalid password for user " + username)
	}
	tokenLifespan, err := time.ParseDuration(config.GetConfig().Security.TokenExpiry)
	if err != nil {
		logger.Fatal("Invalid token lifespan " + config.GetConfig().Security.TokenExpiry)
	}
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(tokenLifespan).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(config.GetConfig().Security.JWTSecret)
}

var generateRefreshToken = func() {

}

var refreshJWT = func() {

}

var getUserIDFromJWT = func() {

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
	return err != nil
}
