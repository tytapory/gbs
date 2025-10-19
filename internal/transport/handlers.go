package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"gbs/internal/models"
	"gbs/internal/usecases"
	"gbs/pkg/logger"
)

const userIDKey string = "userID"

var _ V1Handlers = v1HandlersImplementation{}

type V1Handlers interface {
	Login(w http.ResponseWriter, r *http.Request)
	Register(w http.ResponseWriter, r *http.Request)
	GetTransactionsHistory(w http.ResponseWriter, r *http.Request)
	GetTransactionCount(w http.ResponseWriter, r *http.Request)
	GetUserPermissions(w http.ResponseWriter, r *http.Request)
	GetUserID(w http.ResponseWriter, r *http.Request)
	GetUsername(w http.ResponseWriter, r *http.Request)
	GetBalance(w http.ResponseWriter, r *http.Request)
	Transaction(w http.ResponseWriter, r *http.Request)
	PrintMoney(w http.ResponseWriter, r *http.Request)
	RefreshJWT(w http.ResponseWriter, r *http.Request)
	ModifyPermission(w http.ResponseWriter, r *http.Request)
	ChangePassword(w http.ResponseWriter, r *http.Request)
	AuthMiddleware(next http.Handler) http.Handler
	RateLimitMiddleware(next http.Handler) http.Handler
	ProtectedHandler(handler http.HandlerFunc) http.Handler
	PublicHandler(handler http.HandlerFunc) http.Handler
}

type v1HandlersImplementation struct {
	useCases    usecases.UseCases
	rateLimiter RateLimiter
}

func NewV1HandlersImplementation(useCases usecases.UseCases, rateLimiter RateLimiter) V1Handlers {
	return v1HandlersImplementation{useCases: useCases, rateLimiter: rateLimiter}
}

func (h v1HandlersImplementation) ProtectedHandler(handler http.HandlerFunc) http.Handler {
	return h.RateLimitMiddleware(h.AuthMiddleware(handler))
}

func (h v1HandlersImplementation) PublicHandler(handler http.HandlerFunc) http.Handler {
	return h.RateLimitMiddleware(handler)
}

func (h v1HandlersImplementation) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				h.errorResponse(w, http.StatusInternalServerError, "Failed to determine IP")

				return
			}

			retryAfter := h.rateLimiter.RegisterRequestForIP(ip)
			if retryAfter > 0 {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
				h.errorResponse(w, http.StatusTooManyRequests, "Too many requests, please try again later")

				return
			}

			next.ServeHTTP(w, r)
		},
	)
}

func (h v1HandlersImplementation) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Debug("Missing Authorization header")
				h.errorResponse(w, http.StatusUnauthorized, "Missing Authorization header")
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := h.useCases.GetUserIDFromJWT(tokenString)
			if err != nil {
				logger.Debug("Unauthorized: invalid token")
				h.errorResponse(w, http.StatusUnauthorized, "Unauthorized")
				return
			}
			logger.Debug(fmt.Sprintf("Authenticated userID: %d", userID))

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		},
	)
}

func (h v1HandlersImplementation) Login(w http.ResponseWriter, r *http.Request) {
	logger.Info("Login endpoint hit")

	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("Login: Invalid method " + r.Method)
		h.invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	var req models.AuthRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		logger.Error("Login: Invalid request body: " + err.Error())
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug("Login: Attempting authentication for username: " + req.Username)
	if !h.rateLimiter.CheckLoginAttempt(req.Username) {
		logger.Warn("Login: Too many login attempts for username: " + req.Username)
		h.errorResponse(w, http.StatusUnauthorized, "Too many login attempts, try again later")
		return
	}

	result, err := h.useCases.Login(req.Username, req.Password)
	if err != nil || result.Token == "" || result.RefreshToken == "" {
		logger.Error("Login: Authentication failed for username: " + req.Username + " - " + err.Error())
		h.rateLimiter.RegisterFailedLoginAttempt(req.Username)
		h.respondBasedOnErrorType(w, err)

		return
	}

	h.rateLimiter.ResetLoginAttempts(req.Username)
	logger.Info("Login: User " + req.Username + " authenticated successfully")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h v1HandlersImplementation) Register(w http.ResponseWriter, r *http.Request) {
	logger.Info("Register endpoint hit")

	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("Login: Invalid method " + r.Method)
		h.invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	var req models.AuthRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		logger.Error("Register: Invalid request body: " + err.Error())
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug("Register: Attempting authentication for username: " + req.Username)
	if !h.rateLimiter.CheckLoginAttempt(req.Username) {
		logger.Warn("Register: Too many login attempts for username: " + req.Username)
		h.errorResponse(w, http.StatusUnauthorized, "Too many login attempts, try again later")
		return
	}

	initiatorID, _ := r.Context().Value(userIDKey).(int)

	result, err := h.useCases.RegisterUser(req.Username, req.Password, initiatorID)
	if err != nil || result.Token == "" || result.RefreshToken == "" {
		logger.Error("Register: Authentication failed for username: " + req.Username + " - " + err.Error())
		h.rateLimiter.RegisterFailedLoginAttempt(req.Username)
		h.respondBasedOnErrorType(w, err)

		return
	}

	h.rateLimiter.ResetLoginAttempts(req.Username)
	logger.Info("Register: User " + req.Username + " authenticated successfully")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h v1HandlersImplementation) GetTransactionsHistory(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetTransactionsHistory endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetTransactionsHistory: Invalid method " + r.Method)
		h.invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("GetTransactionsHistory: Unauthorized access")
		h.errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	targetUserID, err := h.parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetTransactionsHistory: Missing or invalid id parameter")
		h.errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}

	page, err := h.parseQueryInt(r, "page")
	if err != nil {
		logger.Error("GetTransactionsHistory: Missing or invalid page parameter")
		h.errorResponse(w, http.StatusBadRequest, "Missing or invalid page parameter")
		return
	}

	limit, offset := h.parsePage(page)
	logger.Debug(
		fmt.Sprintf(
			"GetTransactionsHistory: targetUserID=%d, initiatorID=%d, limit=%d, offset=%d", targetUserID, initiatorID,
			limit, offset,
		),
	)

	history, err := h.useCases.GetTransactionsHistory(initiatorID, targetUserID, limit, offset)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("GetTransactionsHistory: Transactions history successfully fetched")
	json.NewEncoder(w).Encode(models.TransactionResponse{Transactions: history})
}

func (h v1HandlersImplementation) GetTransactionCount(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetTransactionCount endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetTransactionCount: Invalid method " + r.Method)
		h.invalidMethod(w, r)

		return
	}
	defer r.Body.Close()

	targetUserID, err := h.parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetTransactionCount: Missing or invalid id parameter")
		h.errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")
		return
	}

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("GetTransactionCount: Unauthorized access (missing userID in context)")
		h.errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	logger.Debug(fmt.Sprintf("GetTransactionCount: targetUserID=%d, initiatorID=%d", targetUserID, initiatorID))
	amount, err := h.useCases.GetTransactionCount(initiatorID, targetUserID)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("GetTransactionCount: Transaction count successfully fetched")
	json.NewEncoder(w).Encode(models.TransactionAmountResponse{Amount: amount})
}

func (h v1HandlersImplementation) GetUserPermissions(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetUserPermissions endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetUserPermissions: Invalid method " + r.Method)
		h.invalidMethod(w, r)

		return
	}
	defer r.Body.Close()

	userID, err := h.parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetUserPermissions: Missing or invalid id parameter")
		h.errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")

		return
	}

	logger.Debug(fmt.Sprintf("GetUserPermissions: Fetching permissions for userID=%d", userID))
	permissions, err := h.useCases.GetUserPermissions(userID)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("GetUserPermissions: User permissions successfully fetched")
	json.NewEncoder(w).Encode(models.UserPermissionsResponse{Permissions: permissions})
}

func (h v1HandlersImplementation) GetUserID(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetUserID endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetUserID: Invalid method " + r.Method)
		h.invalidMethod(w, r)

		return
	}
	defer r.Body.Close()

	username := r.URL.Query().Get("username")
	if username == "" {
		logger.Error("GetUserID: Username is required")
		h.errorResponse(w, http.StatusBadRequest, "Username is required")

		return
	}

	logger.Debug("GetUserID: Fetching userID for username: " + username)
	userID, err := h.useCases.GetUserID(username)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("GetUserID: userID successfully fetched for username: " + username)
	json.NewEncoder(w).Encode(models.IDResponse{ID: userID})
}

func (h v1HandlersImplementation) GetUsername(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetUsername endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetUsername: Invalid method " + r.Method)
		h.invalidMethod(w, r)

		return
	}
	defer r.Body.Close()

	targetUserID, err := h.parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetUsername: Missing or invalid id parameter")
		h.errorResponse(w, http.StatusBadRequest, "Missing or invalid id parameter")

		return
	}

	logger.Debug(fmt.Sprintf("GetUsername: Fetching username for userID=%d", targetUserID))
	username, err := h.useCases.GetUsername(targetUserID)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("GetUsername: Username successfully fetched for userID: " + strconv.Itoa(targetUserID))
	json.NewEncoder(w).Encode(models.UsernameResponse{Username: username})
}

func (h v1HandlersImplementation) GetBalance(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetBalance endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		logger.Warn("GetBalance: Invalid method " + r.Method)
		h.invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	initiatorID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("GetBalance: Unauthorized access (missing userID in context)")
		h.errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	targetUserID, err := h.parseQueryInt(r, "id")
	if err != nil {
		logger.Error("GetBalance: User ID is required")
		h.errorResponse(w, http.StatusBadRequest, "User ID is required")
		return
	}

	logger.Debug(
		fmt.Sprintf(
			"GetBalance: Fetching balances for targetUserID=%d by initiatorID=%d", targetUserID, initiatorID,
		),
	)

	balances, err := h.useCases.GetBalances(initiatorID, targetUserID)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("GetBalance: User balances successfully fetched")
	json.NewEncoder(w).Encode(models.BalanceResponse{Balances: balances})
}

func (h v1HandlersImplementation) Transaction(w http.ResponseWriter, r *http.Request) {
	logger.Info("Transaction endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("Transaction: Invalid method " + r.Method)
		h.invalidMethod(w, r)

		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("Transaction: Unauthorized access (missing userID in context)")
		h.errorResponse(w, http.StatusUnauthorized, "Unauthorized")

		return
	}

	var req models.TransactionRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		logger.Error("Transaction: Invalid request body: " + err.Error())
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug(
		fmt.Sprintf(
			"Transaction: Processing transfer from %d to %d, currency: %s, amount: %d", req.From, req.To, req.Currency,
			req.Amount,
		),
	)
	err := h.useCases.TransferMoney(req.From, req.To, userID, req.Currency, req.Amount)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("Transaction: Completed successfully")
	w.WriteHeader(http.StatusOK)
}

func (h v1HandlersImplementation) PrintMoney(w http.ResponseWriter, r *http.Request) {
	logger.Info("PrintMoney endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("PrintMoney: Invalid method " + r.Method)
		h.invalidMethod(w, r)

		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("PrintMoney: Unauthorized access (missing userID in context)")
		h.errorResponse(w, http.StatusUnauthorized, "Unauthorized")

		return
	}

	var req models.PrintMoneyRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		logger.Error("PrintMoney: Invalid request body: " + err.Error())
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")

		return
	}

	logger.Debug(
		fmt.Sprintf(
			"PrintMoney: Processing for receiverID=%d, amount=%d, currency=%s", req.ReceiverID, req.Amount,
			req.Currency,
		),
	)
	err := h.useCases.PrintMoney(req.ReceiverID, userID, req.Amount, req.Currency)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("PrintMoney: Completed successfully")
	w.WriteHeader(http.StatusOK)
}

func (h v1HandlersImplementation) RefreshJWT(w http.ResponseWriter, r *http.Request) {
	logger.Info("RefreshJWT endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("RefreshJWT: Invalid method " + r.Method)
		h.invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	var req models.RefreshRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		logger.Error("RefreshJWT: Invalid request body: " + err.Error())
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug("RefreshJWT: Attempting to refresh JWT")
	token, err := h.useCases.RefreshJWT(req.RefreshToken)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Debug("RefreshJWT: Token successfully refreshed")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.RefreshResponse{Token: token})
}

func (h v1HandlersImplementation) ModifyPermission(w http.ResponseWriter, r *http.Request) {
	logger.Info("ModifyPermission endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("ModifyPermission: Invalid method " + r.Method)
		h.invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("ModifyPermission: Unauthorized access (missing userID in context)")
		h.errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ModifyPermissionRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		logger.Error("ModifyPermission: Invalid request body: " + err.Error())
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug(
		fmt.Sprintf(
			"ModifyPermission: Changing permission for userID=%d, permissionID=%d, enabled=%v", req.UserID,
			req.PermissionID, req.Enabled,
		),
	)

	err := h.useCases.TogglePermission(userID, req.UserID, req.PermissionID, req.Enabled)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("ModifyPermission: Permission modified successfully")
	w.WriteHeader(http.StatusOK)
}

func (h v1HandlersImplementation) ChangePassword(w http.ResponseWriter, r *http.Request) {
	logger.Info("ChangePassword endpoint hit")
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Warn("ChangePassword: Invalid method " + r.Method)
		h.invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value(userIDKey).(int)
	if !ok {
		logger.Error("ChangePassword: Unauthorized access (missing userID in context)")
		h.errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req models.ChangePasswordRequest
	if err := h.parseJSONRequest(r, &req); err != nil {
		logger.Error("ChangePassword: Invalid request body: " + err.Error())
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger.Debug(
		fmt.Sprintf(
			"ChangePassword: Attempting password change for userID=%d, targetUserID=%d", userID, req.UserID,
		),
	)
	err := h.useCases.ChangePassword(userID, req.UserID, req.Password)
	if err != nil {
		logger.Error(err.Error())
		h.respondBasedOnErrorType(w, err)

		return
	}

	logger.Info("ChangePassword: Password changed successfully")
	w.WriteHeader(http.StatusOK)
}

func (h v1HandlersImplementation) parsePage(page int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	limit = 20
	offset = (page - 1) * 20
	return
}

func (h v1HandlersImplementation) respondBasedOnErrorType(w http.ResponseWriter, err error) {
	var badReq *models.BadRequestError
	var permErr *models.PermissionError
	var notFound *models.NotFoundError
	var conflict *models.ConflictError
	var unproc *models.UnprocessableEntityError

	if err != nil {
		switch {
		case errors.As(err, &badReq):
			h.errorResponse(w, http.StatusBadRequest, badReq.Error())
		case errors.As(err, &permErr):
			h.errorResponse(w, http.StatusForbidden, permErr.Error())
		case errors.As(err, &notFound):
			h.errorResponse(w, http.StatusNotFound, notFound.Error())
		case errors.As(err, &conflict):
			h.errorResponse(w, http.StatusConflict, conflict.Error())
		case errors.As(err, &unproc):
			h.errorResponse(w, http.StatusUnprocessableEntity, unproc.Error())
		default:
			h.errorResponse(w, http.StatusInternalServerError, "Internal Server Error")
		}
	}
}

func (h v1HandlersImplementation) parseJSONRequest(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

func (h v1HandlersImplementation) parseQueryInt(r *http.Request, key string) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, fmt.Errorf("missing parameter: %s", key)
	}
	return strconv.Atoi(value)
}

func (h v1HandlersImplementation) errorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(
		struct {
			Message string `json:"message"`
		}{Message: message},
	)
}

func (h v1HandlersImplementation) invalidMethod(w http.ResponseWriter, r *http.Request) {
	h.errorResponse(w, http.StatusMethodNotAllowed, "Invalid method: "+r.Method)
}
