package checker

import (
	"testing"
)

func TestParseDorm(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantB    string
		wantF    string
		wantR    string
	}{
		{"标准格式-三段", "C11-01-132", "11", "01", "132"},
		{"标准格式-二段", "C13-01132", "13", "1301", "1301132"},
		{"无C前缀-三段", "11-01-132", "11", "01", "132"},
		{"无C前缀-二段", "13-01132", "13", "1301", "1301132"},
		{"六位纯数字", "1101132", "11", "1101", "1101132"},
		{"C01六位纯数字", "0101132", "01", "0101", "0101132"},
		{"电后缀", "C11-电", "11", "11", "电"},
		{"水后缀", "C12-水", "12", "12", "水"},
		{"旧格式-管道符", "11|01|132", "11", "01", "132"},
		{"纯楼栋码", "13", "13", "", "13"},
		{"不足两位", "1", "1", "1", "1"},
		{"含空格被裁剪", "  C11-01-132  ", "11", "01", "132"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDorm(tt.input)
			if got.Building != tt.wantB || got.Floor != tt.wantF || got.Room != tt.wantR {
				t.Errorf("ParseDorm(%q) = (%s,%s,%s), want (%s,%s,%s)",
					tt.input, got.Building, got.Floor, got.Room, tt.wantB, tt.wantF, tt.wantR)
			}
		})
	}
}

func TestIsC13OrC14(t *testing.T) {
	tests := []struct {
		building string
		want     bool
	}{
		{"C13", true},
		{"C14", true},
		{"c13", false},   // 小写不匹配
		{"C11", false},
		{"13", true},
		{"14", true},
		{"1", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.building, func(t *testing.T) {
			got := IsC13OrC14(tt.building)
			if got != tt.want {
				t.Errorf("IsC13OrC14(%q) = %v, want %v", tt.building, got, tt.want)
			}
		})
	}
}

func TestExtractDropOptionsWithText(t *testing.T) {
	html := `<select id="drceng">
<option value="11|01|132">132</option>
<option value="11|01|101">101</option>
<option value="11|01|公1">公1</option>
<option value="11|02|201">201</option>
</select>`

	got := ExtractDropOptionsWithText(html, "drceng")
	if len(got) != 4 {
		t.Fatalf("expected 4 options, got %d", len(got))
	}

	// 验证选项顺序与 HTML 一致（修复 bug：两次遍历错位）
	if got[0].Value != "11|01|132" || got[0].Text != "132" {
		t.Errorf("option[0] = (%s,%s), want (11|01|132,132)", got[0].Value, got[0].Text)
	}
	if got[1].Value != "11|01|101" || got[1].Text != "101" {
		t.Errorf("option[1] = (%s,%s), want (11|01|101,101)", got[1].Value, got[1].Text)
	}
	if got[2].Value != "11|01|公1" || got[2].Text != "公1" {
		t.Errorf("option[2] = (%s,%s), want (11|01|公1,公1)", got[2].Value, got[2].Text)
	}
	if got[3].Value != "11|02|201" || got[3].Text != "201" {
		t.Errorf("option[3] = (%s,%s), want (11|02|201,201)", got[3].Value, got[3].Text)
	}

	// 不存在的 selectID
	gotEmpty := ExtractDropOptionsWithText(html, "nonexistent")
	if len(gotEmpty) != 0 {
		t.Errorf("expected 0 options for nonexistent id, got %d", len(gotEmpty))
	}
}

func TestExtractDropOptionsWithText_SingleTraversal(t *testing.T) {
	// 确保 value 和 text 来自同一次遍历，顺序一致
	html := `<select id="room">
<option value="1">一</option>
<option value="2">二</option>
<option value="3">三</option>
</select>`

	vals := ExtractDropOptions(html, "room")
	texts := ExtractDropOptionTexts(html, "room")

	// 两者顺序应一致（修复前的 bug：两次遍历顺序不一致）
	if len(vals) != len(texts) {
		t.Errorf("values len=%d vs texts len=%d, must be equal", len(vals), len(texts))
	}
	for i := range vals {
		t.Logf("values[%d]=%s texts[%d]=%s", i, vals[i], i, texts[i])
	}
}
