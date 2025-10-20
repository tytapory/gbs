package auth

import (
	"errors"
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
	RegisterUser(login, password string, initiatorID int) (models.AuthResponse, error)
	Login(login, password string) (models.AuthResponse, error)
	ChangePassword(initiatorID, userID int, password string) error
	RefreshJWT(refreshToken string) (string, error)
	GetUserIDFromJWT(tokenString string) (int, error)
}

type authServiceImplementation struct {
	repo           repository.Repository
	securityConfig config.SecurityConfig
}

func NewAuthServiceImplementation(repo repository.Repository, securityConfig config.SecurityConfig) AuthService {
	return authServiceImplementation{repo: repo, securityConfig: securityConfig}
}

func (a authServiceImplementation) RegisterUser(login, password string, initiatorID int) (models.AuthResponse, error) {
	if !a.validateUsername(login) {
		return models.AuthResponse{}, &models.UnprocessableEntityError{Message: "Username is invalid. Please select another username."}
	}
	if !a.validatePassword(password) {
		return models.AuthResponse{}, &models.UnprocessableEntityError{Message: "Password is invalid. Please select another password."}
	}
	hash, err := a.generatePasswordHash(password)
	if err != nil {
		return models.AuthResponse{}, err
	}
	userID, err := a.repo.RegisterUser(initiatorID, a.securityConfig.AllowDirectRegistration, login, hash)
	if err != nil {
		return models.AuthResponse{}, err
	}
	token, err := a.generateJWT(userID)
	if err != nil {
		return models.AuthResponse{}, err
	}
	refreshToken, err := a.generateRefreshToken(userID)
	if err != nil {
		return models.AuthResponse{}, err
	}

	return models.AuthResponse{
		Token: token, TokenExpiry: a.securityConfig.TokenExpiry, RefreshToken: refreshToken,
		RefreshTokenExpiry: a.securityConfig.RefreshTokenExpiry,
	}, nil
}

func (a authServiceImplementation) Login(login, password string) (models.AuthResponse, error) {
	id, hash, err := a.repo.GetUserIDHash(login)

	if err != nil || !a.compareHashes(hash, password) {
		var notFound *models.NotFoundError
		if err != nil && !errors.As(err, &notFound) {
			return models.AuthResponse{}, err
		}

		return models.AuthResponse{}, &models.PermissionError{Message: "Invalid username or password"}
	}

	token, err := a.generateJWT(id)
	if err != nil {
		return models.AuthResponse{}, err
	}
	refreshToken, err := a.generateRefreshToken(id)
	if err != nil {
		return models.AuthResponse{}, err
	}

	return models.AuthResponse{
		Token: token, TokenExpiry: a.securityConfig.TokenExpiry, RefreshToken: refreshToken,
		RefreshTokenExpiry: a.securityConfig.RefreshTokenExpiry,
	}, nil
}

func (a authServiceImplementation) ChangePassword(initiatorID, userID int, password string) error {
	if !a.validatePassword(password) {
		return &models.UnprocessableEntityError{Message: "Password is invalid. Please select another password."}
	}
	hash, err := a.generatePasswordHash(password)
	if err != nil {
		return &models.ServerFaultError{Message: "Unexpected error during generate password hash: " + err.Error()}
	}
	err = a.repo.ChangePassword(initiatorID, userID, hash)
	if err != nil {
		return err
	}
	err = a.repo.InvalidateRefreshTokens(userID)
	if err != nil {
		return err
	}
	return nil
}

func (a authServiceImplementation) RefreshJWT(refreshToken string) (string, error) {
	userID, err := a.repo.GetUserByRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}

	token, err := a.generateJWT(userID)
	if err != nil {
		return "", err
	}
	return token, nil
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

func (a authServiceImplementation) generateJWT(id int) (string, error) {
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

func (a authServiceImplementation) generateRefreshToken(userID int) (string, error) {
	duration, err := time.ParseDuration(a.securityConfig.RefreshTokenExpiry)
	if err != nil {
		logger.Error("Invalid refresh token lifespan " + a.securityConfig.RefreshTokenExpiry)
		return "", &models.ServerFaultError{Message: "Internal config error: invalid refresh token lifespan"}
	}
	newRefreshToken, err := a.repo.CreateRefreshToken(userID, time.Now().Add(duration))
	if err != nil {
		return "", err
	}
	return newRefreshToken, nil
}

func (a authServiceImplementation) validateUsername(username string) bool {
	pattern := `^[a-zA-Z0-9!@#$%^&*()-_=+{}[\]|:;"'<>,.?/~` + "`" + `]+$`
	matched, _ := regexp.MatchString(pattern, username)
	usernameLen := len(username)
	return matched && usernameLen >= a.securityConfig.LoginMinLength && usernameLen <= a.securityConfig.LoginMaxLength
}

func (a authServiceImplementation) validatePassword(password string) bool {
	pattern := `^[a-zA-Z0-9!@#$%^&*()-_=+{}[\]|:;"'<>,.?/~` + "`" + `]+$`
	matched, _ := regexp.MatchString(pattern, password)
	passLen := len(password)
	return matched && passLen >= a.securityConfig.PasswordMinLength && passLen <= a.securityConfig.PasswordMaxLength
}

func (a authServiceImplementation) generatePasswordHash(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func (a authServiceImplementation) compareHashes(hash string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
