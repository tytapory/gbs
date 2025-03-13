package transport

import "net/http"

func SetupRoutes() {
	http.Handle("/api/v1/login", http.HandlerFunc(Login))
	http.Handle("/api/v1/register", http.HandlerFunc(Register))
}
