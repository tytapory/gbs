package app

import (
	"gbs/internal/repository"
	"gbs/internal/transport"
	"gbs/pkg/logger"
)

func Run() {
	logger.InitializeLoggers("debug", "")
	repository.InitDB()
	transport.Run()
}
