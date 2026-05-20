package cache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	// Cache 全局缓存实例
	Cache *cache.Cache
)

// InitCache 初始化缓存
func InitCache() {
	// 创建一个缓存实例
	// 默认过期时间：5 分钟
	// 清理间隔：10 分钟
	Cache = cache.New(5*time.Minute, 10*time.Minute)
}

// Set 设置缓存
func Set(key string, value interface{}, expiration time.Duration) {
	Cache.Set(key, value, expiration)
}

// Get 获取缓存
func Get(key string) (interface{}, bool) {
	return Cache.Get(key)
}

// Delete 删除缓存
func Delete(key string) {
	Cache.Delete(key)
}

// Flush 清空所有缓存
func Flush() {
	Cache.Flush()
}

// GetString 获取字符串缓存
func GetString(key string) (string, bool) {
	value, found := Get(key)
	if !found {
		return "", false
	}
	str, ok := value.(string)
	return str, ok
}

// GetDormOptions 获取宿舍选项缓存
func GetDormOptions() ([]map[string]interface{}, bool) {
	value, found := Get("dorm_options")
	if !found {
		return nil, false
	}
	options, ok := value.([]map[string]interface{})
	return options, ok
}

// SetDormOptions 设置宿舍选项缓存
func SetDormOptions(options []map[string]interface{}) {
	// 缓存 1 小时
	Set("dorm_options", options, 1*time.Hour)
}

// DeleteDormOptions 删除宿舍选项缓存
func DeleteDormOptions() {
	Delete("dorm_options")
}

// GetAppConfig 获取应用配置缓存
func GetAppConfig() (map[string]interface{}, bool) {
	value, found := Get("app_config")
	if !found {
		return nil, false
	}
	config, ok := value.(map[string]interface{})
	return config, ok
}

// SetAppConfig 设置应用配置缓存
func SetAppConfig(config map[string]interface{}) {
	// 缓存 5 分钟
	Set("app_config", config, 5*time.Minute)
}

// DeleteAppConfig 删除应用配置缓存
func DeleteAppConfig() {
	Delete("app_config")
}

// GetUser 获取用户缓存
func GetUser(username string) (map[string]interface{}, bool) {
	value, found := Get("user:" + username)
	if !found {
		return nil, false
	}
	user, ok := value.(map[string]interface{})
	return user, ok
}

// SetUser 设置用户缓存
func SetUser(username string, user map[string]interface{}) {
	// 缓存 5 分钟
	Set("user:"+username, user, 5*time.Minute)
}

// DeleteUser 删除用户缓存
func DeleteUser(username string) {
	Delete("user:" + username)
}
