package util

import (
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	Value      interface{}
	Expiration time.Time
}

// MemoryCache 内存缓存
type MemoryCache struct {
	items map[string]*CacheItem
	mutex sync.RWMutex
	ttl   time.Duration
}

// NewMemoryCache 创建缓存实例
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items: make(map[string]*CacheItem),
		ttl:   ttl,
	}
	// 启动清理过期缓存的goroutine
	go cache.cleanup()
	return cache
}

// Get 获取缓存
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(item.Expiration) {
		return nil, false
	}

	return item.Value, true
}

// Set 设置缓存
func (c *MemoryCache) Set(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = &CacheItem{
		Value:      value,
		Expiration: time.Now().Add(c.ttl),
	}
}

// Delete 删除缓存
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.items, key)
}

// Clear 清空缓存
func (c *MemoryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]*CacheItem)
}

// cleanup 定期清理过期缓存
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mutex.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.Expiration) {
				delete(c.items, key)
			}
		}
		c.mutex.Unlock()
	}
}

// 全局缓存实例 - 5分钟过期
var GlobalCache = NewMemoryCache(5 * time.Minute)

// CacheKeys 缓存键常量
const (
	CacheKeyUserPrefix     = "user:"
	CacheKeyCodeSourceList = "code_source_list:"
	CacheKeyTaskList       = "task_list:"
	CacheKeyModelList      = "model_list:"
)

// InvalidateUserCache 使用户相关缓存失效
func InvalidateUserCache(userID uint) {
	GlobalCache.Delete(CacheKeyUserPrefix + string(rune(userID)))
	GlobalCache.Clear() // 清空所有缓存，因为用户数据变化影响很多地方
}

// InvalidateCodeSourceCache 使代码源相关缓存失效
func InvalidateCodeSourceCache(userID uint) {
	GlobalCache.Delete(CacheKeyCodeSourceList + string(rune(userID)))
}

// InvalidateTaskCache 使任务相关缓存失效
func InvalidateTaskCache(userID uint) {
	GlobalCache.Delete(CacheKeyTaskList + string(rune(userID)))
}
