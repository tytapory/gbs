package transport

import (
	"fmt"
	"gbs/internal/config"
	"gbs/pkg/logger"
	"net/http"
)

func Run() {
	addr := fmt.Sprintf(":%s", config.GetConfig().Server.Port)
	SetupRoutes()
	logger.Info("server listening on %s" + addr)
	logger.Error(http.ListenAndServe(addr, nil).Error())
}
