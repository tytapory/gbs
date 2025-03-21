// @title GBS
// @version 1.0
// @description P2P REST API
// @host localhost:8080
// @BasePath /api/v1
// @schemes http

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"gbs/internal/app"
)

func main() {
	app.Run()
}
