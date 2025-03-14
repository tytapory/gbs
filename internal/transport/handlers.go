package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"gbs/internal/auth"
	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/internal/repository"
	"gbs/pkg/logger"
)

func Login(w http.ResponseWriter, r *http.Request) {
	logger.Info("Login endpoint hit")
	authenticate(w, r, auth.Login)
}

func Register(w http.ResponseWriter, r *http.Request) {
	logger.Info("Register endpoint hit")
	allowRegistration := config.GetConfig().Security.AllowDirectRegistration
	if !allowRegistration {
		initiatorID, ok := r.Context().Value(userIDKey).(int)
		if !ok {
			logger.Error("Register: Failed to retrieve initiator user ID")
			errorResponse(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		allowRegistration = repository.CheckRegistrationPermissions(initiatorID)
		logger.Debug(fmt.Sprintf("Register: Registration permission check result: %v", allowRegistration))
	}
	if allowRegistration {
		logger.Info("Register: Registration allowed, proceeding with authentication")
		authenticate(w, r, auth.RegisterUser)
	} else {
		logger.Warn("Register: Registration not allowed")
		errorResponse(w, http.StatusForbidden, "Registration not allowed")
	}
}

func GetTransactionsHistory(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetTransactionsHistory endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetTransactionsHistory: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	targetUserID, err := parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetTransactionsHistory: Missing or invalid id parameter")
		errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}

	page, err := parseQueryInt(r, "page")
	if err != nil {
		logger.Error("GetTransactionsHistory: Missing or invalid page parameter")
		errorResponse(w, http.StatusBadRequest, "Missing or invalid page parameter")
		return
	}

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("GetTransactionsHistory: Unauthorized access (missing userID in context)")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit, offset := parsePage(page)
	logger.Debug(fmt.Sprintf("GetTransactionsHistory: targetUserID=%d, initiatorID=%d, limit=%d, offset=%d", targetUserID, initiatorID, limit, offset))
	history, err := repository.GetTransactionsHistory(initiatorID, targetUserID, limit, offset)
	if err != nil {
		logger.Error("GetTransactionsHistory: Failed to get transactions history: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Failed to get transactions history")
		return
	}
	logger.Info("GetTransactionsHistory: Transactions history successfully fetched")
	json.NewEncoder(w).Encode(models.TransactionResponse{Transactions: history})
}

func GetTransactionCount(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetTransactionCount endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetTransactionCount: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	targetUserID, err := parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetTransactionCount: Missing or invalid id parameter")
		errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("GetTransactionCount: Unauthorized access (missing userID in context)")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	logger.Debug(fmt.Sprintf("GetTransactionCount: targetUserID=%d, initiatorID=%d", targetUserID, initiatorID))
	amount, err := repository.GetTransactionCount(initiatorID, targetUserID)
	if err != nil {
		logger.Error("GetTransactionCount: Failed to get transaction count: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Failed to get transaction count")
		return
	}
	logger.Info("GetTransactionCount: Transaction count successfully fetched")
	json.NewEncoder(w).Encode(models.TransactionAmountResponse{Amount: amount})
}

func GetUserPermissions(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetUserPermissions endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetUserPermissions: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, err := parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetUserPermissions: Missing or invalid id parameter")
		errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}

	logger.Debug(fmt.Sprintf("GetUserPermissions: Fetching permissions for userID=%d", userID))
	permissions, err := repository.GetUserPermissions(userID)
	if err != nil {
		logger.Error("GetUserPermissions: Failed to get user permissions: " + err.Error())
		errorResponse(w, http.StatusInternalServerError, "Failed to get user permissions")
		return
	}
	logger.Info("GetUserPermissions: User permissions successfully fetched")
	json.NewEncoder(w).Encode(models.UserPermissionsResponse{Permissions: permissions})
}

func GetUserID(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetUserID endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetUserID: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	username := r.URL.Query().Get("username")
	if username == "" {
		logger.Error("GetUserID: Username is required")
		errorResponse(w, http.StatusBadRequest, "Username is required")
		return
	}
	logger.Debug("GetUserID: Fetching userID for username: " + username)
	userID, err := repository.GetUserID(username)
	if err != nil {
		logger.Error("GetUserID: User not found for username: " + username)
		errorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}
	logger.Info("GetUserID: userID successfully fetched for username: " + username)
	json.NewEncoder(w).Encode(models.IDResponse{ID: userID})
}

func GetUsername(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetUsername endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetUsername: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	targetUserID, err := parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetUsername: Missing or invalid id parameter")
		errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}
	logger.Debug(fmt.Sprintf("GetUsername: Fetching username for userID=%d", targetUserID))
	username, err := repository.GetUsername(targetUserID)
	if err != nil {
		logger.Error("GetUsername: User not found for userID: " + strconv.Itoa(targetUserID))
		errorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}
	logger.Info("GetUsername: Username successfully fetched for userID: " + strconv.Itoa(targetUserID))
	json.NewEncoder(w).Encode(models.UsernameResponse{Username: username})
}

func GetBalance(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetBalance endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetBalance: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("GetBalance: Unauthorized access (missing userID in context)")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	targetUserID, err := parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetBalance: User ID is required")
		errorResponse(w, http.StatusBadRequest, "User ID is required")
		return
	}

	logger.Debug(fmt.Sprintf("GetBalance: Fetching balances for targetUserID=%d by initiatorID=%d", targetUserID, initiatorID))
	balances, err := repository.GetBalances(initiatorID, targetUserID)
	if err != nil {
		logger.Error("GetBalance: Failed to get user balances: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Failed to get user balances")
		return
	}
	logger.Info("GetBalance: User balances successfully fetched")
	json.NewEncoder(w).Encode(models.BalanceResponse{Balances: balances})
}

func Transaction(w http.ResponseWriter, r *http.Request) {
	logger.Info("Transaction endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("Transaction: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("Transaction: Unauthorized access (missing userID in context)")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.TransactionRequest
	if err := parseJSONRequest(r, &req); err != nil {
		logger.Error("Transaction: Invalid request body: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug(fmt.Sprintf("Transaction: Processing transfer from %d to %d, currency: %s, amount: %d", req.From, req.To, req.Currency, req.Amount))
	if err := repository.TransferMoney(req.From, req.To, userID, req.Currency, req.Amount); err != nil {
		logger.Error("Transaction: Transfer failed: " + err.Error())
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("Transaction: Completed successfully")
	w.WriteHeader(http.StatusOK)
}

func PrintMoney(w http.ResponseWriter, r *http.Request) {
	logger.Info("PrintMoney endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("PrintMoney: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("PrintMoney: Unauthorized access (missing userID in context)")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.PrintMoneyRequest
	if err := parseJSONRequest(r, &req); err != nil {
		logger.Error("PrintMoney: Invalid request body: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug(fmt.Sprintf("PrintMoney: Processing for receiverID=%d, amount=%d, currency=%s", req.ReceiverID, req.Amount, req.Currency))
	if err := repository.PrintMoney(req.ReceiverID, userID, req.Amount, req.Currency); err != nil {
		logger.Error("PrintMoney: Operation failed: " + err.Error())
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("PrintMoney: Completed successfully")
	w.WriteHeader(http.StatusOK)
}

func RefreshJWT(w http.ResponseWriter, r *http.Request) {
	logger.Info("RefreshJWT endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("RefreshJWT: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	var req models.RefreshRequest
	if err := parseJSONRequest(r, &req); err != nil {
		logger.Error("RefreshJWT: Invalid request body: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug("RefreshJWT: Attempting to refresh JWT")
	token, err := auth.RefreshJWT(req.RefreshToken)
	if err != nil || token == "" {
		logger.Error("RefreshJWT: Failed to refresh token: " + err.Error())
		errorResponse(w, http.StatusUnauthorized, "Invalid token")
		return
	}
	logger.Info("RefreshJWT: Token successfully refreshed")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.RefreshResponse{Token: token})
}

func authenticate(w http.ResponseWriter, r *http.Request, authFunc func(string, string) (string, string, error)) {
	logger.Info("Authentication attempt")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("authenticate: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	var req models.AuthRequest
	if err := parseJSONRequest(r, &req); err != nil {
		logger.Error("authenticate: Invalid request body: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug("authenticate: Attempting authentication for username: " + req.Username)
	if !checkLoginAttempt(req.Username) {
		logger.Warn("authenticate: Too many login attempts for username: " + req.Username)
		errorResponse(w, http.StatusUnauthorized, "Too many login attempts, try again later")
		return
	}

	token, refreshToken, err := authFunc(req.Username, req.Password)
	if err != nil || token == "" || refreshToken == "" {
		logger.Error("authenticate: Authentication failed for username: " + req.Username + " - " + err.Error())
		registerFailedAttempt(req.Username)
		errorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	resetLoginAttempts(req.Username)
	logger.Info("authenticate: User " + req.Username + " authenticated successfully")

	resp := models.AuthResponse{
		Token:              token,
		TokenExpiry:        config.GetConfig().Security.TokenExpiry,
		RefreshToken:       refreshToken,
		RefreshTokenExpiry: config.GetConfig().Security.RefreshTokenExpiry,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func ModifyPermission(w http.ResponseWriter, r *http.Request) {
	logger.Info("ModifyPermission endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("ModifyPermission: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("ModifyPermission: Unauthorized access (missing userID in context)")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ModifyPermissionRequest
	if err := parseJSONRequest(r, &req); err != nil {
		logger.Error("ModifyPermission: Invalid request body: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug(fmt.Sprintf("ModifyPermission: Changing permission for userID=%d, permissionID=%d, enabled=%v", req.UserID, req.PermissionID, req.Enabled))
	var err error
	if req.Enabled {
		err = repository.SetPermission(userID, req.UserID, req.PermissionID)
	} else {
		err = repository.UnsetPermission(userID, req.UserID, req.PermissionID)
	}
	if err != nil {
		logger.Error("ModifyPermission: Operation failed: " + err.Error())
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("ModifyPermission: Permission modified successfully")
	w.WriteHeader(http.StatusOK)
}

func ChangePassword(w http.ResponseWriter, r *http.Request) {
	logger.Info("ChangePassword endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("ChangePassword: Invalid method " + r.Method)
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("ChangePassword: Unauthorized access (missing userID in context)")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ChangePasswordRequest
	if err := parseJSONRequest(r, &req); err != nil {
		logger.Error("ChangePassword: Invalid request body: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug(fmt.Sprintf("ChangePassword: Attempting password change for userID=%d, targetUserID=%d", userID, req.UserID))
	if err := auth.ChangePassword(userID, req.UserID, req.Password); err != nil {
		logger.Error("ChangePassword: Operation failed: " + err.Error())
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	logger.Info("ChangePassword: Password changed successfully")
	w.WriteHeader(http.StatusOK)
}
