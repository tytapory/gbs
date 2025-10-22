package app

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"gbs/internal/auth"
	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/internal/repository"
	"gbs/internal/transport"
	"gbs/internal/usecases"
	"gbs/pkg/logger"
)

func Run() {
	logger.InitializeLoggers("debug", "")
	cfg := config.NewConfigProviderImplementation().Get()
	repo, err := repository.NewRepositoryImplementation(cfg.Database)
	if err != nil {
		logger.Fatal(err.Error())
	}
	authService := auth.NewAuthServiceImplementation(repo, cfg.Security)

	q := repo.NewSingleQuery()
	initialized, err := repo.DoesDefaultUsersInitialized(q)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to check initialization status: %s", err.Error()))
	}

	if !initialized {
		admPassword := generatePassword(16)
		admUsername := "adm"
		admHash, err := authService.GeneratePasswordHash(admPassword)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to generate hash for adm: %s", err.Error()))
		}

		admUUID, err := repo.RegisterUser(q, admUsername, admHash)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to create user '%s': %s", admUsername, err.Error()))
		}

		logger.Info(">>> Default password for adm: " + admPassword + " <<<")

		adminPermID := models.Administrator
		err = repo.SetPermission(q, admUUID, adminPermID)
		if err != nil {
			logger.Fatal(
				fmt.Sprintf(
					"Failed to grant permission '%d' (Administrator) to user '%s': %s", adminPermID, admUsername,
					err.Error(),
				),
			)
		}

		logger.Info(fmt.Sprintf("Permission '%d' (Administrator) granted to user '%s'.", adminPermID, admUsername))

		feesPassword := generatePassword(16)
		feesUsername := "fees"
		feesHash, err := authService.GeneratePasswordHash(feesPassword)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to generate hash for fees: %s", err.Error()))
		}
		_, err = repo.RegisterUser(q, feesUsername, feesHash)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to create user '%s': %s", feesUsername, err.Error()))
		}

		logger.Info(">>> Default password for fees: " + feesPassword + " <<<")
	} else {
		logger.Info("Default users already initialized, skipping.")
	}

	useCases, err := usecases.NewUseCasesImplementation(repo, authService, cfg.Security, cfg.Core)
	if err != nil {
		logger.Fatal(err.Error())
	}

	rateLimiter := transport.NewRateLimiterImplementation(cfg.Security)
	v1Handlers := transport.NewV1HandlersImplementation(useCases, rateLimiter)

	transport.Run(v1Handlers, cfg.Server, cfg.Security)
}

func generatePassword(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		randByte, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic(fmt.Sprintf("Failed to generate random byte for password: %v", err))
		}
		b[i] = charset[randByte.Int64()]
	}
	return string(b)
}
