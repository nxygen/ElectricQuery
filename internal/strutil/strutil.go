// Package strutil 提供字符串处理工具函数
package strutil

// IsDigits 判断字符串是否全部由 ASCII 数字组成
func IsDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

