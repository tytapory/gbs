package transport

import (
	"fmt"
	"gbs/internal/config"
	"gbs/pkg/logger"
	"net/http"
)

func Run() {
	addr := fmt.Sprintf(":%s", config.GetConfig().Server.Port)
	mux := http.NewServeMux()

	mux.Handle("/api/v1/login", http.HandlerFunc(Login))
	if config.GetConfig().Security.AllowDirectRegistration {
		mux.Handle("/api/v1/register", http.HandlerFunc(Register))
	} else {
		mux.Handle("/api/v1/register", AuthMiddleware(http.HandlerFunc(Register)))
	}

	mux.Handle("/api/v1/getBalances", AuthMiddleware(http.HandlerFunc(GetBalance)))
	mux.Handle("/api/v1/transaction", AuthMiddleware(http.HandlerFunc(Transaction)))
	mux.Handle("/api/v1/getUserID", AuthMiddleware(http.HandlerFunc(GetUserID)))
	mux.Handle("/api/v1/getUserPermissions", AuthMiddleware(http.HandlerFunc(GetUserPermissions)))
	mux.Handle("/api/v1/getTransactionCount", AuthMiddleware(http.HandlerFunc(GetTransactionCount)))
	mux.Handle("/api/v1/getTransactionsHistory", AuthMiddleware(http.HandlerFunc(GetTransactionsHistory)))
	mux.Handle("/api/v1/printMoney", AuthMiddleware(http.HandlerFunc(PrintMoney)))
	mux.Handle("/api/v1/modifyPermission", AuthMiddleware(http.HandlerFunc(ModifyPermission)))
	mux.Handle("/api/v1/changePassword", AuthMiddleware(http.HandlerFunc(ChangePassword)))

	logger.Info("server listening on %s" + addr)
	logger.Error(http.ListenAndServe(addr, mux).Error())
}
