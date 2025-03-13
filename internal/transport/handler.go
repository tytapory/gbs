package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"gbs/internal/auth"
	"gbs/internal/models"
	"gbs/internal/repository"
	"gbs/pkg/logger"
	"net/http"
	"strconv"
	"strings"
)

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
	authenticate(w, r, auth.RegisterUser)
}

func GetTransactionsHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		invalidMethod(w, r)
		return
	}
	defer r.Body.Close()

	queryUserID := r.URL.Query().Get("user_id")
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

	page := r.URL.Query().Get("user_id")
	if page == "" {
		logger.Debug("Missing page parameter")
		errorResponse(w, http.StatusBadRequest, "Missing id parameter")
		return
	}
	pageInt, err := strconv.Atoi(page)
	if err != nil {
		logger.Debug("Invalid page parameter")
		errorResponse(w, http.StatusBadRequest, "Invalid id parameter")
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
	}
	json.NewEncoder(w).Encode(history)
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

func authenticate(w http.ResponseWriter, r *http.Request, authFunc func(string, string) (string, error)) {
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
	token, err := authFunc(requestData.Username, requestData.Password)
	if err != nil || token == "" {
		logger.Debug("Invalid credentials response: " + err.Error())
		errorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	w.WriteHeader(http.StatusOK)
	resp := models.AuthResponse{Token: token}
	err = json.NewEncoder(w).Encode(resp)
	return
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
