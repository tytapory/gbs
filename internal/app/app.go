package app

import (
	"crypto/rand"
	"math/big"

	"gbs/internal/auth"
	"gbs/internal/repository"
	"gbs/internal/transport"
	"gbs/pkg/logger"
)

func Run() {
	logger.InitializeLoggers("debug", "")
	repository.InitDB()
	if !repository.DoesDefaultUsersInitialized() {
		admPassword := generatePassword(16)
		err := auth.ChangePassword(1, 1, admPassword)
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Info("#############################################")
		logger.Info("password for adm : " + admPassword)
		logger.Info("#############################################")
		feesPassword := generatePassword(16)
		err = auth.ChangePassword(1, 2, feesPassword)
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Info("#############################################")
		logger.Info("password for fees : " + feesPassword)
		logger.Info("#############################################")
		registrationPassword := generatePassword(16)
		err = auth.ChangePassword(1, 3, registrationPassword)
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Info("#############################################")
		logger.Info("password for registration : " + registrationPassword)
		logger.Info("#############################################")
		moneyPrinterPassword := generatePassword(16)
		err = auth.ChangePassword(1, 4, moneyPrinterPassword)
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Info("#############################################")
		logger.Info("password for money_printer : " + moneyPrinterPassword)
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
