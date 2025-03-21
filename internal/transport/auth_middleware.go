package transport

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gbs/internal/auth"
	"gbs/pkg/logger"
)

type contextKey string

const userIDKey contextKey = "userID"

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
		if err != nil {
			logger.Debug("Unauthorized: invalid token")
			errorResponse(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		logger.Debug(fmt.Sprintf("Authenticated userID: %d", userID))

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
