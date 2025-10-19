package transport

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"gbs/internal/config"
	"gbs/internal/models"
	"gbs/pkg/logger"
)

var _ RateLimiter = &rateLimiterImplementation{}

type RateLimiter interface {
	CheckLoginAttempt(username string) bool
	RegisterFailedLoginAttempt(username string)
	ResetLoginAttempts(username string)
	RegisterRequestForIP(ip string) time.Duration
}

type rateLimiterImplementation struct {
	securityConfig   config.SecurityConfig
	failedLogins     *sync.Map
	rateLimiterCache *lru.Cache[string, *models.RateLimitInfo]
	rateLimiterMu    *sync.RWMutex
	maxRequests      int
	timeWindow       time.Duration
}

func NewRateLimiterImplementation(securityConfig config.SecurityConfig) RateLimiter {
	rateLimiter := &rateLimiterImplementation{
		securityConfig: securityConfig, failedLogins: &sync.Map{}, rateLimiterMu: &sync.RWMutex{},
		rateLimiterCache: nil,
		maxRequests:      securityConfig.RPMForIP, timeWindow: time.Minute,
	}
	rateLimiter.initRateLimiter()

	return rateLimiter
}

func (r *rateLimiterImplementation) RegisterRequestForIP(ip string) time.Duration {
	r.rateLimiterMu.Lock()
	defer r.rateLimiterMu.Unlock()

	now := time.Now()
	entry, found := r.rateLimiterCache.Get(ip)
	if !found {
		entry = &models.RateLimitInfo{
			Requests:  1,
			ResetTime: now.Add(r.timeWindow),
		}
		r.rateLimiterCache.Add(ip, entry)
	} else {
		if now.After(entry.ResetTime) {
			entry.Requests = 1
			entry.ResetTime = now.Add(r.timeWindow)
		} else {
			entry.Requests++
			if entry.Requests > r.maxRequests {
				return time.Until(entry.ResetTime)
			}
		}
	}

	return time.Duration(0)
}

func (r *rateLimiterImplementation) CheckLoginAttempt(username string) bool {
	if value, ok := r.failedLogins.Load(username); ok {
		if attempt, ok := value.(*models.LoginAttempt); ok {
			if time.Now().Before(attempt.BlockedUntil) {
				return false
			}
		}
	}
	return true
}

func (r *rateLimiterImplementation) RegisterFailedLoginAttempt(username string) {
	value, _ := r.failedLogins.LoadOrStore(username, &models.LoginAttempt{})
	attempt := value.(*models.LoginAttempt)
	attempt.Count++
	if attempt.Count >= r.securityConfig.MaxLoginAttempts {
		duration, err := time.ParseDuration(r.securityConfig.LockoutDuration)
		if err != nil {
			logger.Fatal("Invalid lockout duration format")
		}
		attempt.BlockedUntil = time.Now().Add(duration)
	}
	r.failedLogins.Store(username, attempt)
}

func (r *rateLimiterImplementation) ResetLoginAttempts(username string) {
	r.failedLogins.Delete(username)
}

func (r *rateLimiterImplementation) cleanupExpiredAttempts() {
	now := time.Now()
	r.failedLogins.Range(
		func(key, value interface{}) bool {
			if attempt, ok := value.(*models.LoginAttempt); ok {
				if now.After(attempt.BlockedUntil) {
					r.failedLogins.Delete(key)
				}
			}
			return true
		},
	)
}

func (r *rateLimiterImplementation) initRateLimiter() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			r.cleanupExpiredAttempts()
		}
	}()

	var err error
	r.rateLimiterCache, err = lru.New[string, *models.RateLimitInfo](1000)
	if err != nil {
		panic(err)
	}
}
