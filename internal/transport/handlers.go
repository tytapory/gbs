package transport

import (
	"encoding/json"
	"net/http"

	"gbs/internal/auth"
	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/internal/repository"
	"gbs/pkg/logger"
)

func Login(w http.ResponseWriter, r *http.Request) {
	authenticate(w, r, auth.Login)
}

func Register(w http.ResponseWriter, r *http.Request) {
	allowRegistration := config.GetConfig().Security.AllowDirectRegistration
	if !allowRegistration {
		initiatorID, ok := r.Context().Value(userIDKey).(int)
		if !ok {
			logger.Error("Failed to retrieve initiator user ID")
			errorResponse(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		allowRegistration = repository.CheckRegistrationPermissions(initiatorID)
	}
	if allowRegistration {
		authenticate(w, r, auth.RegisterUser)
	} else {
		errorResponse(w, http.StatusForbidden, "Registration not allowed")
	}
}

func GetTransactionsHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	targetUserID, err := parseQueryInt(r, "id")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}

	page, err := parseQueryInt(r, "page")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Missing or invalid page parameter")
		return
	}

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit, offset := parsePage(page)
	history, err := repository.GetTransactionsHistory(initiatorID, targetUserID, limit, offset)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Failed to get transactions history")
		return
	}

	json.NewEncoder(w).Encode(models.TransactionResponse{Transactions: history})
}

func GetTransactionCount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	targetUserID, err := parseQueryInt(r, "id")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	amount, err := repository.GetTransactionCount(initiatorID, targetUserID)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Failed to get transaction count")
		return
	}
	json.NewEncoder(w).Encode(models.TransactionAmountResponse{Amount: amount})
}

func GetUserPermissions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, err := parseQueryInt(r, "id")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}

	permissions, err := repository.GetUserPermissions(userID)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to get user permissions")
		return
	}
	json.NewEncoder(w).Encode(models.UserPermissionsResponse{Permissions: permissions})
}

func GetUserID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	username := r.URL.Query().Get("username")
	if username == "" {
		errorResponse(w, http.StatusBadRequest, "Username is required")
		return
	}
	userID, err := repository.GetUserID(username)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}
	json.NewEncoder(w).Encode(models.IDResponse{ID: userID})
}

func GetUsername(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	targetUserID, err := parseQueryInt(r, "id")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}
	username, err := repository.GetUsername(targetUserID)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}
	json.NewEncoder(w).Encode(models.UsernameResponse{Username: username})
}

func GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	targetUserID, err := parseQueryInt(r, "id")
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "User ID is required")
		return
	}

	balances, err := repository.GetBalances(initiatorID, targetUserID)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Failed to get user balances")
		return
	}
	json.NewEncoder(w).Encode(models.BalanceResponse{Balances: balances})
}

func Transaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.TransactionRequest
	if err := parseJSONRequest(r, &req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := repository.TransferMoney(req.From, req.To, userID, req.Currency, req.Amount); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func PrintMoney(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.PrintMoneyRequest
	if err := parseJSONRequest(r, &req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := repository.PrintMoney(req.ReceiverID, userID, req.Amount, req.Currency); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func RefreshJWT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	var req models.RefreshRequest
	if err := parseJSONRequest(r, &req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, err := auth.RefreshJWT(req.RefreshToken)
	if err != nil || token == "" {
		errorResponse(w, http.StatusUnauthorized, "Invalid token")
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.RefreshResponse{Token: token})
}

func authenticate(w http.ResponseWriter, r *http.Request, authFunc func(string, string) (string, string, error)) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	var req models.AuthRequest
	if err := parseJSONRequest(r, &req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !checkLoginAttempt(req.Username) {
		errorResponse(w, http.StatusUnauthorized, "Too many login attempts, try again later")
		return
	}

	token, refreshToken, err := authFunc(req.Username, req.Password)
	if err != nil || token == "" || refreshToken == "" {
		registerFailedAttempt(req.Username)
		errorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	resetLoginAttempts(req.Username)

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
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ModifyPermissionRequest
	if err := parseJSONRequest(r, &req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var err error
	if req.Enabled {
		err = repository.SetPermission(userID, req.UserID, req.PermissionID)
	} else {
		err = repository.UnsetPermission(userID, req.UserID, req.PermissionID)
	}
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func ChangePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ChangePasswordRequest
	if err := parseJSONRequest(r, &req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := auth.ChangePassword(userID, req.UserID, req.Password); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
