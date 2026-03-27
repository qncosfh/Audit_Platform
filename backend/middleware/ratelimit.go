package middleware

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 简单的内存速率限制器（带容量限制，防止内存泄漏）
type RateLimiter struct {
	requests     map[string][]time.Time
	mu           sync.RWMutex
	limit        int
	window       time.Duration
	maxEntries   int     // 最大条目限制，防止内存泄漏
	cleanupRatio float64 // 清理比例
}

var (
	// 全局限流器
	globalLimiter = NewRateLimiter(100, time.Minute, 10000) // 每分钟100次请求，最多10000个IP

	// 登录相关路由的限流器（更严格）
	authLimiter = NewRateLimiter(5, time.Minute, 1000) // 每分钟5次尝试，最多1000个IP
)

// NewRateLimiter 创建速率限制器
// limit: 每个IP在窗口期内的最大请求数
// window: 时间窗口
// maxEntries: 最大IP条目数，防止内存泄漏
func NewRateLimiter(limit int, window time.Duration, maxEntries int) *RateLimiter {
	rl := &RateLimiter{
		requests:     make(map[string][]time.Time),
		limit:        limit,
		window:       window,
		maxEntries:   maxEntries,
		cleanupRatio: 0.2, // 当达到最大条目时，清理20%的最旧条目
	}

	// 定期清理过期记录
	go func() {
		for {
			time.Sleep(window)
			rl.cleanup()
		}
	}()

	return rl
}

// cleanup 清理过期的记录，并防止内存泄漏（使用LRU策略）
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	keysToDelete := []string{}

	for key, times := range rl.requests {
		var valid []time.Time
		for _, t := range times {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			keysToDelete = append(keysToDelete, key)
		} else {
			rl.requests[key] = valid
		}
	}

	// 删除过期的键
	for _, key := range keysToDelete {
		delete(rl.requests, key)
	}

	// 如果超过最大条目数，使用LRU策略清理最旧的条目
	if len(rl.requests) > rl.maxEntries {
		// 计算需要清理的数量
		numToDelete := int(float64(rl.maxEntries) * rl.cleanupRatio)
		if numToDelete < 10 {
			numToDelete = 10 // 至少清理10个
		}

		// 收集所有条目及其最早访问时间
		type entry struct {
			key      string
			earliest time.Time
		}
		var entries []entry
		for key, times := range rl.requests {
			if len(times) > 0 {
				// 找出最早的时间
				earliest := times[0]
				for _, t := range times[1:] {
					if t.Before(earliest) {
						earliest = t
					}
				}
				entries = append(entries, entry{key: key, earliest: earliest})
			}
		}

		// 按最早时间排序（最旧的在前）
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].earliest.Before(entries[j].earliest)
		})

		// 删除最旧的条目
		for i := 0; i < numToDelete && i < len(entries); i++ {
			delete(rl.requests, entries[i].key)
		}
	}
}

// isAllowed 检查是否允许请求
func (rl *RateLimiter) isAllowed(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	times := rl.requests[key]

	// 过滤窗口期内的请求
	var valid []time.Time
	for _, t := range times {
		if now.Sub(t) < rl.window {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[key] = valid
		return false
	}

	// 检查是否超过最大条目数，如果是则拒绝新条目
	if len(rl.requests) >= rl.maxEntries {
		// 尝试清理过期条目
		valid = valid[:0]
		for _, t := range rl.requests[key] {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}
		// 如果仍然超过限制，拒绝请求
		if len(rl.requests) >= rl.maxEntries {
			return false
		}
	}

	rl.requests[key] = append(valid, now)
	return true
}

// RateLimitMiddleware 全局速率限制中间件
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取客户端标识（IP + 用户ID如果已登录）
		key := c.ClientIP()
		if userID, exists := c.Get("userID"); exists {
			// 修复：使用fmt.Sprintf正确转换uint为string
			key = key + "-" + fmt.Sprintf("%d", userID)
		}

		if !globalLimiter.isAllowed(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "请求过于频繁，请稍后再试",
				"code":  429,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthRateLimitMiddleware 认证相关路由的速率限制（更严格）
func AuthRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() + "-auth"

		if !authLimiter.isAllowed(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "登录尝试过于频繁，请稍后再试",
				"code":  429,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
