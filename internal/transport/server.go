package transport

import (
	"fmt"
	"gbs/internal/config"
	"gbs/pkg/logger"
	"github.com/rs/cors"
	"net/http"
)

func Run() {
	addr := fmt.Sprintf(":%s", config.GetConfig().Server.Port)
	mux := http.NewServeMux()

	mux.Handle("/api/v1/login", RateLimitMiddleware(http.HandlerFunc(Login)))
	mux.Handle("/api/v1/changePassword", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(ChangePassword))))

	mux.Handle("/api/v1/refreshJWT", RateLimitMiddleware(http.HandlerFunc(RefreshJWT)))
	if config.GetConfig().Security.AllowDirectRegistration {
		mux.Handle("/api/v1/register", RateLimitMiddleware(http.HandlerFunc(Register)))
	} else {
		mux.Handle("/api/v1/register", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(Register))))
	}
	mux.Handle("/api/v1/getBalances", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(GetBalance))))
	mux.Handle("/api/v1/transaction", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(Transaction))))
	mux.Handle("/api/v1/getUserID", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(GetUserID))))
	mux.Handle("/api/v1/getUsername", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(GetUsername))))
	mux.Handle("/api/v1/getUserPermissions", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(GetUserPermissions))))
	mux.Handle("/api/v1/getTransactionCount", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(GetTransactionCount))))
	mux.Handle("/api/v1/getTransactionsHistory", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(GetTransactionsHistory))))
	mux.Handle("/api/v1/printMoney", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(PrintMoney))))
	mux.Handle("/api/v1/modifyPermission", RateLimitMiddleware(AuthMiddleware(http.HandlerFunc(ModifyPermission))))

	Init()
	logger.Info(fmt.Sprintf("Server listening on %s", addr))
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
	})
	handler := corsHandler.Handler(mux)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Error(err.Error())
	}
}
