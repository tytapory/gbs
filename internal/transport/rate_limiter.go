package transport

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"gbs/internal/config"
	"gbs/internal/models"
	lru "github.com/hashicorp/golang-lru/v2"
)

var (
	failedLogins     = sync.Map{} // map[string]*models.LoginAttempt
	rateLimiterCache *lru.Cache[string, *models.RateLimitInfo]
	rateLimiterMu    sync.Mutex
	maxRequests      = config.GetConfig().Security.RPMForIP
	timeWindow       = time.Minute
)

func cleanupExpiredAttempts() {
	now := time.Now()
	failedLogins.Range(func(key, value interface{}) bool {
		if attempt, ok := value.(*models.LoginAttempt); ok {
			if now.After(attempt.BlockedUntil) {
				failedLogins.Delete(key)
			}
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
			errorResponse(w, http.StatusInternalServerError, "Failed to determine IP")
			return
		}

		rateLimiterMu.Lock()
		defer rateLimiterMu.Unlock()

		now := time.Now()
		entry, found := rateLimiterCache.Get(ip)
		if !found {
			entry = &models.RateLimitInfo{
				Requests:  1,
				ResetTime: now.Add(timeWindow),
			}
			rateLimiterCache.Add(ip, entry)
		} else {
			if now.After(entry.ResetTime) {
				entry.Requests = 1
				entry.ResetTime = now.Add(timeWindow)
			} else {
				entry.Requests++
				if entry.Requests > maxRequests {
					w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(entry.ResetTime).Seconds())))
					errorResponse(w, http.StatusTooManyRequests, "Too many requests")
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
