package transport

import (
	"encoding/json"
	"gbs/internal/auth"
	"gbs/internal/models"
	"gbs/pkg/logger"
	"net/http"
)

func Login(w http.ResponseWriter, r *http.Request) {
	authenticate(w, r, auth.Login)
}

func Register(w http.ResponseWriter, r *http.Request) {
	authenticate(w, r, auth.RegisterUser)
}

func authenticate(w http.ResponseWriter, r *http.Request, authFunc func(string, string) (string, error)) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		logger.Debug("Invalid method response")
		InvalidMethod(w, r)
		return
	}
	defer r.Body.Close()
	var requestData models.AuthRequest
	err := parseJSONRequest(r, &requestData)
	if err != nil {
		logger.Debug("Invalid request body response: " + err.Error())
		ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	token, err := authFunc(requestData.Username, requestData.Password)
	if err != nil || token == "" {
		logger.Debug("Invalid credentials response: " + err.Error())
		ErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}
	w.WriteHeader(http.StatusOK)
	resp := models.AuthResponse{Token: token}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		logger.Debug("Internal Server Error response: " + err.Error())
		ErrorResponse(w, http.StatusInternalServerError, "Internal Server Error")
	}
	return
}

func ErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	resp := models.ErrorResponse{Message: message}
	json.NewEncoder(w).Encode(resp)
}

func InvalidMethod(w http.ResponseWriter, r *http.Request) {
	ErrorResponse(w, http.StatusMethodNotAllowed, "Invalid method "+r.Method)
}

func parseJSONRequest(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
