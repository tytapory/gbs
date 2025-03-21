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

// -----------------------------------------------------------------------------
// Login Endpoint
// -----------------------------------------------------------------------------
// Login godoc
// @Summary Login
// @Description Authenticate a user and return a JWT token.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.AuthRequest true "User credentials"
// @Success 200 {object} models.AuthResponse "Authentication successful"
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 401 {object} models.ErrorResponse "Invalid credentials or unauthorized"
// @Router /login [post]
func Login(w http.ResponseWriter, r *http.Request) {
	logger.Info("Login endpoint hit")
	authenticate(w, r, auth.Login)
}

// -----------------------------------------------------------------------------
// Register Endpoint
// -----------------------------------------------------------------------------
// Register godoc
// @Summary Register a new user
// @Description Register a new user with provided credentials. Registration is allowed based on configuration and permissions.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.AuthRequest true "Registration credentials"
// @Success 200 {object} models.AuthResponse "Registration successful"
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Registration not allowed"
// @Router /register [post]
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

// -----------------------------------------------------------------------------
// Get Transactions History Endpoint
// -----------------------------------------------------------------------------
// GetTransactionsHistory godoc
// @Summary Get Transactions History
// @Description Retrieve the transaction history for a specific user.
// @Tags transactions
// @Accept json
// @Produce json
// @Param id query int true "Target user ID"
// @Param page query int true "Page number for pagination"
// @Success 200 {object} models.TransactionResponse "Transactions history"
// @Failure 400 {object} models.ErrorResponse "Missing/invalid parameters or failed to get history"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /transactions/history [get]
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

// -----------------------------------------------------------------------------
// Get Transaction Count Endpoint
// -----------------------------------------------------------------------------
// GetTransactionCount godoc
// @Summary Get Transaction Count
// @Description Retrieve the count of transactions for a specific user.
// @Tags transactions
// @Accept json
// @Produce json
// @Param id query int true "Target user ID"
// @Success 200 {object} models.TransactionAmountResponse "Transaction count"
// @Failure 400 {object} models.ErrorResponse "Missing/invalid id parameter or count retrieval failed"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /transactions/count [get]
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

// -----------------------------------------------------------------------------
// Get User Permissions Endpoint
// -----------------------------------------------------------------------------
// GetUserPermissions godoc
// @Summary Get User Permissions
// @Description Retrieve permissions for a given user.
// @Tags user
// @Accept json
// @Produce json
// @Param id query int true "User ID"
// @Success 200 {object} models.UserPermissionsResponse "User permissions"
// @Failure 400 {object} models.ErrorResponse "Missing or invalid id parameter"
// @Failure 500 {object} models.ErrorResponse "Failed to get user permissions"
// @Router /user/permissions [get]
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

// -----------------------------------------------------------------------------
// Get User ID Endpoint
// -----------------------------------------------------------------------------
// GetUserID godoc
// @Summary Get User ID
// @Description Retrieve the user ID based on username.
// @Tags user
// @Accept json
// @Produce json
// @Param username query string true "Username"
// @Success 200 {object} models.IDResponse "User ID"
// @Failure 400 {object} models.ErrorResponse "Username is required"
// @Failure 401 {object} models.ErrorResponse "User not found"
// @Router /user/id [get]
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

// -----------------------------------------------------------------------------
// Get Username Endpoint
// -----------------------------------------------------------------------------
// GetUsername godoc
// @Summary Get Username
// @Description Retrieve the username for a given user ID.
// @Tags user
// @Accept json
// @Produce json
// @Param id query int true "User ID"
// @Success 200 {object} models.UsernameResponse "Username"
// @Failure 400 {object} models.ErrorResponse "Missing or invalid id parameter"
// @Failure 401 {object} models.ErrorResponse "User not found"
// @Router /user/username [get]
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

// -----------------------------------------------------------------------------
// Get Balance Endpoint
// -----------------------------------------------------------------------------
// GetBalance godoc
// @Summary Get User Balance
// @Description Retrieve the balance information for a specified user.
// @Tags user
// @Accept json
// @Produce json
// @Param id query int true "Target user ID"
// @Success 200 {object} models.BalanceResponse "User balances"
// @Failure 400 {object} models.ErrorResponse "User ID is required or failed to get balances"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /user/balance [get]
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

// -----------------------------------------------------------------------------
// Transaction Endpoint
// -----------------------------------------------------------------------------
// Transaction godoc
// @Summary Perform Transaction
// @Description Transfer money from one user to another.
// @Tags transactions
// @Accept json
// @Produce json
// @Param request body models.TransactionRequest true "Transaction details"
// @Success 200 "Transaction completed successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request body or transfer failed"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /transaction [post]
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

// -----------------------------------------------------------------------------
// Print Money Endpoint
// -----------------------------------------------------------------------------
// PrintMoney godoc
// @Summary Print Money
// @Description Print money to a user's account.
// @Tags transactions
// @Accept json
// @Produce json
// @Param request body models.PrintMoneyRequest true "Print money details"
// @Success 200 "Operation completed successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request body or operation failed"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /printmoney [post]
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

// -----------------------------------------------------------------------------
// Refresh JWT Endpoint
// -----------------------------------------------------------------------------
// RefreshJWT godoc
// @Summary Refresh JWT Token
// @Description Refresh the JWT token using a provided refresh token.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.RefreshRequest true "Refresh token"
// @Success 200 {object} models.RefreshResponse "New JWT token"
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 401 {object} models.ErrorResponse "Invalid token"
// @Router /refresh [post]
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

// -----------------------------------------------------------------------------
// Modify Permission Endpoint
// -----------------------------------------------------------------------------
// ModifyPermission godoc
// @Summary Modify User Permission
// @Description Modify a user's permission by enabling or disabling it.
// @Tags user
// @Accept json
// @Produce json
// @Param request body models.ModifyPermissionRequest true "Permission modification details"
// @Success 200 "Permission modified successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request body or operation failed"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /user/permission [post]
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

// -----------------------------------------------------------------------------
// Change Password Endpoint
// -----------------------------------------------------------------------------
// ChangePassword godoc
// @Summary Change Password
// @Description Change the password for a user.
// @Tags user
// @Accept json
// @Produce json
// @Param request body models.ChangePasswordRequest true "Password change details"
// @Success 200 "Password changed successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request body or operation failed"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Router /user/changepassword [post]
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
