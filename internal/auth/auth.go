package auth

import (
	"fmt"
	"regexp"
	"time"

	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/internal/repository"
	"gbs/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var _ AuthService = authServiceImplementation{}

type AuthService interface {
	GetUserIDFromJWT(tokenString string) (int, error)
	ValidateUsername(username string) bool
	ValidatePassword(password string) bool
	GenerateJWT(id int) (string, error)
	GeneratePasswordHash(password string) (string, error)
	CompareHashes(hash string, password string) bool
}

type authServiceImplementation struct {
	repo           repository.Repository
	securityConfig config.SecurityConfig
}

func NewAuthServiceImplementation(repo repository.Repository, securityConfig config.SecurityConfig) AuthService {
	return authServiceImplementation{repo: repo, securityConfig: securityConfig}
}

func (a authServiceImplementation) GetUserIDFromJWT(tokenString string) (int, error) {
	secret := []byte(a.securityConfig.JwtSecret)

	token, err := jwt.Parse(
		tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				logger.Debug("Unexpected signing method")
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secret, nil
		},
	)

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

func (a authServiceImplementation) GenerateJWT(id int) (string, error) {
	tokenLifespan, err := time.ParseDuration(a.securityConfig.TokenExpiry)
	if err != nil {
		logger.Error("Invalid token lifespan " + a.securityConfig.TokenExpiry)
		return "", &models.ServerFaultError{Message: "Internal config error: invalid token lifespan"}
	}
	claims := jwt.MapClaims{
		"user_id": id,
		"exp":     time.Now().Add(tokenLifespan).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.securityConfig.JwtSecret))
}

func (a authServiceImplementation) ValidateUsername(username string) bool {
	pattern := `^[a-zA-Z0-9!@#$%^&*()-_=+{}[\]|:;"'<>,.?/~` + "`" + `]+$`
	matched, _ := regexp.MatchString(pattern, username)
	usernameLen := len(username)
	return matched && usernameLen >= a.securityConfig.LoginMinLength && usernameLen <= a.securityConfig.LoginMaxLength
}

func (a authServiceImplementation) ValidatePassword(password string) bool {
	pattern := `^[a-zA-Z0-9!@#$%^&*()-_=+{}[\]|:;"'<>,.?/~` + "`" + `]+$`
	matched, _ := regexp.MatchString(pattern, password)
	passLen := len(password)
	return matched && passLen >= a.securityConfig.PasswordMinLength && passLen <= a.securityConfig.PasswordMaxLength
}

func (a authServiceImplementation) GeneratePasswordHash(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func (a authServiceImplementation) CompareHashes(hash string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
