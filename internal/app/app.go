package app

import (
	"crypto/rand"
	"gbs/internal/auth"
	"gbs/internal/repository"
	"gbs/internal/transport"
	"gbs/pkg/logger"
	"math/big"
)

func Run() {
	logger.InitializeLoggers("debug", "")
	repository.InitDB()
	if !repository.DoesDefaultUsersInitialized() {
		password1 := generatePassword(16)
		err := auth.ChangePassword(1, 1, password1)
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Info("#############################################")
		logger.Info("password for adm : " + password1)
		logger.Info("#############################################")
		password2 := generatePassword(16)
		err = auth.ChangePassword(1, 2, password2)
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Info("#############################################")
		logger.Info("password for fees : " + password2)
		logger.Info("#############################################")
		password3 := generatePassword(16)
		err = auth.ChangePassword(1, 3, password3)
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Info("#############################################")
		logger.Info("password for registration : " + password3)
		logger.Info("#############################################")
		password4 := generatePassword(16)
		err = auth.ChangePassword(1, 4, password4)
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Info("#############################################")
		logger.Info("password for money_printer : " + password4)
		logger.Info("#############################################")
		logger.Info("Default users initialized (adm, fees, registration, money_printer). Change those passwords ASAP")
	}
	transport.Run()
}

func generatePassword(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		randByte, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic(err)
		}
		b[i] = charset[randByte.Int64()]
	}
	return string(b)
}
