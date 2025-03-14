package app

import (
	"gbs/internal/auth"
	"gbs/internal/repository"
	"gbs/internal/transport"
	"gbs/pkg/logger"
)

func Run() {
	logger.InitializeLoggers("debug", "")
	repository.InitDB()
	if !repository.DoesDefaultUsersInitialized() {
		err := auth.ChangePassword(1, 1, "CHANGE_ME")
		if err != nil {
			logger.Fatal(err.Error())
		}
		err = auth.ChangePassword(1, 2, "CHANGE_ME")
		if err != nil {
			logger.Fatal(err.Error())
		}
		err = auth.ChangePassword(1, 3, "CHANGE_ME")
		if err != nil {
			logger.Fatal(err.Error())
		}
		err = auth.ChangePassword(1, 4, "CHANGE_ME")
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Error("Default users initialized (adm, fees, registration, money_printer) - default password for all - CHANGE_ME. CHANGE ALL THE PASSWORDS RIGHT NOW")
	}
	transport.Run()
}
