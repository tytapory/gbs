package transport

import (
	"fmt"
	"net/http"

	"github.com/rs/cors"

	"gbs/internal/config"
	"gbs/pkg/logger"
)

func Run(v1Handlers V1Handlers, serverConfig config.ServerConfig, securityConfig config.SecurityConfig) {
	addr := fmt.Sprintf(":%s", serverConfig.Port)
	mux := http.NewServeMux()

	// schrödinger's handler
	if securityConfig.AllowDirectRegistration {
		mux.Handle("/api/v1/register", v1Handlers.PublicHandler(v1Handlers.Register))
	} else {
		mux.Handle("/api/v1/register", v1Handlers.ProtectedHandler(v1Handlers.Register))
	}

	// public
	mux.Handle("/api/v1/login", v1Handlers.PublicHandler(v1Handlers.Login))
	mux.Handle("/api/v1/refreshJWT", v1Handlers.PublicHandler(v1Handlers.RefreshJWT))

	// protected
	mux.Handle("/api/v1/changePassword", v1Handlers.ProtectedHandler(v1Handlers.ChangePassword))
	mux.Handle("/api/v1/getBalances", v1Handlers.ProtectedHandler(v1Handlers.GetBalance))
	mux.Handle("/api/v1/transaction", v1Handlers.ProtectedHandler(v1Handlers.Transaction))
	mux.Handle("/api/v1/getUserID", v1Handlers.ProtectedHandler(v1Handlers.GetUserID))
	mux.Handle("/api/v1/getUsername", v1Handlers.ProtectedHandler(v1Handlers.GetUsername))
	mux.Handle("/api/v1/getUserPermissions", v1Handlers.ProtectedHandler(v1Handlers.GetUserPermissions))
	mux.Handle("/api/v1/getTransactionCount", v1Handlers.ProtectedHandler(v1Handlers.GetTransactionCount))
	mux.Handle("/api/v1/getTransactionsHistory", v1Handlers.ProtectedHandler(v1Handlers.GetTransactionsHistory))
	mux.Handle("/api/v1/printMoney", v1Handlers.ProtectedHandler(v1Handlers.PrintMoney))
	mux.Handle("/api/v1/modifyPermission", v1Handlers.ProtectedHandler(v1Handlers.ModifyPermission))

	logger.Info(fmt.Sprintf("Server listening on %s", addr))
	corsHandler := cors.New(
		cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Authorization", "Content-Type"},
			AllowCredentials: false,
		},
	)
	handler := corsHandler.Handler(mux)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Error(err.Error())
	}
}
