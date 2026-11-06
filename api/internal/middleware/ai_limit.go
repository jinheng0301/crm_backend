package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	started time.Time
	count   int
}

// NewAILimit implements a per-client fixed-window rate limit.
func NewAILimit(perMinute int) gin.HandlerFunc {
	if perMinute < 1 {
		perMinute = 20
	}
	var mu sync.Mutex
	buckets := map[string]bucket{}
	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()
		mu.Lock()
		entry := buckets[key]
		if now.Sub(entry.started) >= time.Minute {
			entry = bucket{started: now}
		}
		entry.count++
		buckets[key] = entry
		allowed := entry.count <= perMinute
		if len(buckets) > 10000 {
			for ip, current := range buckets {
				if now.Sub(current.started) >= time.Minute {
					delete(buckets, ip)
				}
			}
		}
		mu.Unlock()
		if !allowed {
			requestID, _ := c.Get("request_id")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
				"code": "RATE_LIMITED", "message": "AI request limit reached. Please try again in a minute.", "request_id": requestID,
			}})
			return
		}
		c.Next()
	}
}
