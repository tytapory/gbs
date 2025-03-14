package transport

import (
	"fmt"
	"gbs/internal/auth"
	"gbs/internal/config"
	"gbs/pkg/logger"
	"net/http"
	"sync/atomic"
)

var defaultsChangedCached int32

func defaultsAreChanged() bool {
	if _, err := auth.Login("adm", "CHANGE_ME"); err == nil {
		return false
	}
	if _, err := auth.Login("fees", "CHANGE_ME"); err == nil {
		return false
	}
	if _, err := auth.Login("registration", "CHANGE_ME"); err == nil {
		return false
	}
	if _, err := auth.Login("money_printer", "CHANGE_ME"); err == nil {
		return false
	}
	return true
}

func CheckDefaultsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&defaultsChangedCached) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		if !defaultsAreChanged() {
			errorResponse(w, http.StatusForbidden, "Default passwords not changed. Change all system passwords to continue. (You only can login and change passwords)")
			return
		}
		atomic.StoreInt32(&defaultsChangedCached, 1)
		next.ServeHTTP(w, r)
	})
}

func Run() {
	addr := fmt.Sprintf(":%s", config.GetConfig().Server.Port)
	mux := http.NewServeMux()

	mux.Handle("/api/v1/login", http.HandlerFunc(Login))
	mux.Handle("/api/v1/changePassword", AuthMiddleware(http.HandlerFunc(ChangePassword)))

	if config.GetConfig().Security.AllowDirectRegistration {
		mux.Handle("/api/v1/register", CheckDefaultsMiddleware(http.HandlerFunc(Register)))
	} else {
		mux.Handle("/api/v1/register", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(Register))))
	}
	mux.Handle("/api/v1/getBalances", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(GetBalance))))
	mux.Handle("/api/v1/transaction", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(Transaction))))
	mux.Handle("/api/v1/getUserID", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(GetUserID))))
	mux.Handle("/api/v1/getUserPermissions", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(GetUserPermissions))))
	mux.Handle("/api/v1/getTransactionCount", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(GetTransactionCount))))
	mux.Handle("/api/v1/getTransactionsHistory", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(GetTransactionsHistory))))
	mux.Handle("/api/v1/printMoney", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(PrintMoney))))
	mux.Handle("/api/v1/modifyPermission", AuthMiddleware(CheckDefaultsMiddleware(http.HandlerFunc(ModifyPermission))))

	logger.Info(fmt.Sprintf("Server listening on %s", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Error(err.Error())
	}
}
