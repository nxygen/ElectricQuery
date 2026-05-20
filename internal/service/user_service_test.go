package service

import "testing"

func TestIsWaterMeterType(t *testing.T) {
	tests := []struct {
		label string
		want  bool
	}{
		{"C11-电", false},
		{"C11-水", true},
		{"C12-水表", true},
		{"C13", false},
		{"", false},
		{"水电", true},  // 含"水"
		{"水表房间", true},
		{"纯电表", false},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := IsWaterMeterType(tt.label)
			if got != tt.want {
				t.Errorf("IsWaterMeterType(%q) = %v, want %v", tt.label, got, tt.want)
			}
		})
	}
}

func TestExtractDigits(t *testing.T) {
	// extractDigits 在遇到非数字后立即停止（不含分隔符）
	tests := []struct {
		input string
		want  string
	}{
		{"C11-01-132", "11"},   // 遇到 '-' 停止
		{"11|01|132", "11"},    // 遇到 '|' 停止
		{"abc123def456", "123"}, // 遇到 'd' 停止
		{"nodigits", ""},
		{"", ""},
		{"123", "123"},
		{"a1b2c3", "1"},        // 遇到 'b' 停止
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractDigits(tt.input)
			if got != tt.want {
				t.Errorf("extractDigits(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidatePasswordComplexity(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"Weak1", true},          // 不足 8 位
		{"weakpass", true},      // 无数字
		{"weak1234", true},      // 无字母
		{"WeakPass123", false},  // 满足条件
		{"Valid@1234", false},
		{"", true},
		{"LongEnough1", false},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			err := validatePasswordComplexity(tt.password)
			gotErr := err != nil
			if gotErr != tt.wantErr {
				t.Errorf("validatePasswordComplexity(%q) err=%v, wantErr=%v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"user@example.com", false},
		{"", false},            // 空邮箱跳过
		{"invalid", true},
		{"@example.com", true},
		{"user@", true},
		{"user@.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := validateEmail(tt.email)
			gotErr := err != nil
			if gotErr != tt.wantErr {
				t.Errorf("validateEmail(%q) err=%v, wantErr=%v", tt.email, err, tt.wantErr)
			}
		})
	}
}

// ToWebValue / buildWeeklyReportBody 依赖 model.DB，需集成测试环境，暂跳过
