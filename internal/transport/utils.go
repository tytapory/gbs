package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func parseJSONRequest(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

func parseQueryInt(r *http.Request, key string) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, fmt.Errorf("missing parameter: %s", key)
	}
	return strconv.Atoi(value)
}

func errorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
	}{Message: message})
}

func invalidMethod(w http.ResponseWriter, r *http.Request) {
	errorResponse(w, http.StatusMethodNotAllowed, "Invalid method: "+r.Method)
}

func parsePage(page int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	limit = 20
	offset = (page - 1) * 20
	return
}
