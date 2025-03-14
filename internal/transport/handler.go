package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"gbs/internal/auth"
	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/internal/repository"
	"gbs/pkg/logger"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

var (
	failedLogins     = sync.Map{}
	rateLimiterCache *lru.Cache[string, *models.RateLimitInfo]
	mu               sync.Mutex
	maxRequests      = config.GetConfig().Security.RPMForIP
	timeWindow       = time.Minute
)

func cleanupExpiredAttempts() {
	now := time.Now()
	failedLogins.Range(func(key, value interface{}) bool {
		attempt := value.(*models.LoginAttempt)
		if now.After(attempt.BlockedUntil) {
			failedLogins.Delete(key)
		}
		return true
	})
}

func Init() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			cleanupExpiredAttempts()
		}
	}()
	var err error
	rateLimiterCache, err = lru.New[string, *models.RateLimitInfo](1000)
	if err != nil {
		panic(err)
	}
}

func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "Ошибка определения IP", http.StatusInternalServerError)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		entry, found := rateLimiterCache.Get(ip)
		if !found {
			entry := &models.RateLimitInfo{
				Requests:  1,
				ResetTime: time.Now().Add(timeWindow),
			}
			rateLimiterCache.Add(ip, entry)
		} else {
			if time.Now().After(entry.ResetTime) {
				entry.Requests = 1
				entry.ResetTime = time.Now().Add(timeWindow)
			} else {
				entry.Requests++
				if entry.Requests > maxRequests {
					w.Header().Set("Retry-After", time.Until(entry.ResetTime).String())
					errorResponse(w, http.StatusTooManyRequests, "too many requests")
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

func checkLoginAttempt(username string) bool {
	value, ok := failedLogins.Load(username)
	if ok {
		attempt := value.(*models.LoginAttempt)
		if time.Now().Before(attempt.BlockedUntil) {
			return false
		}
	}
	return true
}

func registerFailedAttempt(username string) {
	value, _ := failedLogins.LoadOrStore(username, &models.LoginAttempt{})
	attempt := value.(*models.LoginAttempt)
	attempt.Count++
	if attempt.Count >= config.GetConfig().Security.MaxLoginAttempts {
		duration, err := time.ParseDuration(config.GetConfig().Security.LockoutDuration)
		if err != nil {
			logger.Fatal("Can't parse lockout duration")
		}
		attempt.BlockedUntil = time.Now().Add(duration)
	}
	failedLogins.Store(username, attempt)
}

func resetLoginAttempts(username string) {
	failedLogins.Delete(username)
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger.Debug("Missing Authorization header")
			errorResponse(w, http.StatusUnauthorized, "Missing Authorization header")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := auth.GetUserIDFromJWT(tokenString)
		logger.Debug(fmt.Sprintf("Got userID %d", userID))

		if err != nil {
			logger.Debug("User unauthorized")
			errorResponse(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), "userID", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	authenticate(w, r, auth.Login)
}

func Register(w http.ResponseWriter, r *http.Request) {
	registrationAllowed := true
	if !config.GetConfig().Security.AllowDirectRegistration {
		initiatorID, ok := r.Context().Value("userID").(int)
		if !ok {
			logger.Error("Failed to get initiator user ID")
			errorResponse(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		registrationAllowed = repository.CheckRegistrationPermissions(initiatorID)
	}
	if registrationAllowed {
		authenticate(w, r, auth.RegisterUser)
	}
}

func GetTransactionsHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	queryUserID := r.URL.Query().Get("id")
	if queryUserID == "" {
		logger.Debug("Missing id parameter")
		errorResponse(w, http.StatusBadRequest, "Missing id parameter")
		return
	}
	queryUserIDInt, err := strconv.Atoi(queryUserID)
	if err != nil {
		logger.Debug("Invalid id parameter")
		errorResponse(w, http.StatusBadRequest, "Invalid id parameter")
		return
	}

	page := r.URL.Query().Get("page")
	if page == "" {
		logger.Debug("Missing page parameter")
		errorResponse(w, http.StatusBadRequest, "Missing page parameter")
		return
	}
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		logger.Debug("Invalid page parameter")
		errorResponse(w, http.StatusBadRequest, "Invalid page parameter")
		return
	}

	initiatorID, ok := r.Context().Value("userID").(int)
	if !ok {
		logger.Error("Failed to get initiator user ID")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit, offset := parsePage(pageInt)

	history, err := repository.GetTransactionsHistory(initiatorID, queryUserIDInt, limit, offset)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Failed to get transactions history")
		return
	}
	json.NewEncoder(w).Encode(models.TransactionResponse{Transactions: history})
}

func parsePage(page int) (int, int) {
	if page < 1 {
		page = 1
	}
	limit := page * 20
	offset := (page - 1) * 20
	return limit, offset
}

func GetTransactionCount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	queryUserID := r.URL.Query().Get("id")
	if queryUserID == "" {
		logger.Debug("Missing id parameter")
		errorResponse(w, http.StatusBadRequest, "Missing id parameter")
		return
	}
	queryUserIDInt, err := strconv.Atoi(queryUserID)
	if err != nil {
		logger.Debug("Invalid id parameter")
		errorResponse(w, http.StatusBadRequest, "Invalid id parameter")
		return
	}
	initiatorID, ok := r.Context().Value("userID").(int)
	if !ok {
		logger.Error("Failed to get initiator user ID")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	amount, err := repository.GetTransactionCount(initiatorID, queryUserIDInt)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Failed to get amount of transactions")
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

	queryUserID := r.URL.Query().Get("id")
	if queryUserID == "" {
		logger.Debug("Missing id parameter")
		errorResponse(w, http.StatusBadRequest, "Missing id parameter")
		return
	}
	userID, err := strconv.Atoi(queryUserID)
	if err != nil {
		logger.Debug("Invalid id parameter")
		errorResponse(w, http.StatusBadRequest, "Invalid id parameter")
		return
	}
	permissions, err := repository.GetUserPermissions(userID)
	if err != nil {
		logger.Debug("Internal server error")
		errorResponse(w, http.StatusInternalServerError, "Internal server error")
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

	queryUsername := r.URL.Query().Get("username")
	if queryUsername == "" {
		logger.Debug("Missing username in query parameters " + r.URL.RawQuery)
		errorResponse(w, http.StatusBadRequest, "Username is required")
		return
	}
	userID, err := repository.GetUserID(queryUsername)
	if err != nil {
		logger.Debug("User not found " + queryUsername)
		errorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}

	response := models.IDResponse{ID: userID}
	json.NewEncoder(w).Encode(response)
}

func GetUsername(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	queryUserID := r.URL.Query().Get("id")
	if queryUserID == "" {
		logger.Debug("Missing id in query parameters " + r.URL.RawQuery)
		errorResponse(w, http.StatusBadRequest, "ID is required")
		return
	}
	queryUserIDInt, err := strconv.Atoi(queryUserID)
	username, err := repository.GetUsername(queryUserIDInt)
	if err != nil {
		logger.Debug("User not found " + queryUserID)
		errorResponse(w, http.StatusUnauthorized, "User not found")
		return
	}

	response := models.UsernameResponse{Username: username}
	json.NewEncoder(w).Encode(response)
}

func GetBalance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	initiatorID, ok := r.Context().Value("userID").(int)
	if !ok {
		logger.Error("Failed to get initiator user ID")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	queryID := r.URL.Query().Get("id")
	if queryID == "" {
		logger.Error("Missing user ID in query parameters")
		errorResponse(w, http.StatusBadRequest, "User ID is required")
		return
	}

	targetUserID, err := strconv.Atoi(queryID)
	if err != nil {
		logger.Error("Invalid user ID in query parameter")
		errorResponse(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	balances, err := repository.GetBalances(initiatorID, targetUserID)
	if err != nil {
		logger.Error("Failed to get user balances: " + err.Error())
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	response := models.BalanceResponse{Balances: balances}
	json.NewEncoder(w).Encode(response)
}

func Transaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		logger.Error("Failed to get user ID")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var requestData models.TransactionRequest
	err := parseJSONRequest(r, &requestData)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	err = repository.TransferMoney(requestData.From, requestData.To, userID, requestData.Currency, requestData.Amount)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
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
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		logger.Error("Failed to get user ID")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var requestData models.PrintMoneyRequest
	err := parseJSONRequest(r, &requestData)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	err = repository.PrintMoney(requestData.ReceiverID, userID, requestData.Amount, requestData.Currency)
	if err != nil {
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
	var requestData models.RefreshRequest
	err := parseJSONRequest(r, &requestData)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	token, err := auth.RefreshJWT(requestData.RefreshToken)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}
	if token == "" {
		errorResponse(w, http.StatusUnauthorized, "Invalid token")
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.RefreshResponse{Token: token})
}

func authenticate(w http.ResponseWriter, r *http.Request, authFunc func(string, string) (string, string, error)) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Debug("Invalid method response")
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()
	var requestData models.AuthRequest
	err := parseJSONRequest(r, &requestData)
	if err != nil {
		logger.Debug("Invalid request body response: " + err.Error())
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if !checkLoginAttempt(requestData.Username) {
		errorResponse(w, http.StatusUnauthorized, "Too many attempts, try again later")
		return
	}
	token, refreshToken, err := authFunc(requestData.Username, requestData.Password)
	if err != nil || token == "" || refreshToken == "" {
		registerFailedAttempt(requestData.Username)
		logger.Debug("Invalid credentials response: " + err.Error())
		errorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	resetLoginAttempts(requestData.Username)
	w.WriteHeader(http.StatusOK)
	resp := models.AuthResponse{Token: token, TokenExpiry: config.GetConfig().Security.TokenExpiry, RefreshToken: refreshToken, RefreshTokenExpiry: config.GetConfig().Security.RefreshTokenExpiry}
	err = json.NewEncoder(w).Encode(resp)
	return
}

func ModifyPermission(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		logger.Error("Failed to get user ID")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var requestData models.ModifyPermissionRequest
	err := parseJSONRequest(r, &requestData)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if requestData.Enabled {
		err = repository.SetPermission(userID, requestData.UserID, requestData.PermissionID)
	} else {
		err = repository.UnsetPermission(userID, requestData.UserID, requestData.PermissionID)
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
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		logger.Error("Failed to get user ID")
		errorResponse(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var requestData models.ChangePasswordRequest
	err := parseJSONRequest(r, &requestData)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	err = auth.ChangePassword(userID, requestData.UserID, requestData.Password)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func errorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	resp := models.ErrorResponse{Message: message}
	json.NewEncoder(w).Encode(resp)
}

func invalidMethod(w http.ResponseWriter, r *http.Request) {
	errorResponse(w, http.StatusMethodNotAllowed, "Invalid method "+r.Method)
}

func parseJSONRequest(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
