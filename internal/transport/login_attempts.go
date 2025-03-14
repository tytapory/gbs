package transport

import (
	"time"

	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/pkg/logger"
)

func checkLoginAttempt(username string) bool {
	if value, ok := failedLogins.Load(username); ok {
		if attempt, ok := value.(*models.LoginAttempt); ok {
			if time.Now().Before(attempt.BlockedUntil) {
				return false
			}
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
			logger.Fatal("Invalid lockout duration format")
		}
		attempt.BlockedUntil = time.Now().Add(duration)
	}
	failedLogins.Store(username, attempt)
}

func resetLoginAttempts(username string) {
	failedLogins.Delete(username)
}
