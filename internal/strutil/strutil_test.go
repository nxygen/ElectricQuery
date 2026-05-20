package strutil

import "testing"

func TestIsDigits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"纯数字", "123456", true},
		{"数字开头", "123abc", false},
		{"数字结尾", "abc123", false},
		{"空字符串", "", false},
		{"单数字", "5", true},
		{"含空格", "123 456", false},
		{"含中文", "123中文", false},
		{"含特殊字符", "123@456", false},
		{"负数", "-123", false},
		{"零", "0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDigits(tt.input)
			if got != tt.want {
				t.Errorf("IsDigits(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
