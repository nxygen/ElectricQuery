package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"electricquery/internal/logger"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var ErrNoElectricData = errors.New("no electric data found")

// DecodeGB2312 解码 GB2312/GBK/GB18030 → UTF-8
// 服务器 Content-Type 常撒谎，依次尝试 UTF-8 → GB18030 → GBK
func DecodeGB2312(body []byte) (string, error) {
	if isValidUTF8(body) {
		return string(body), nil
	}
	if result, ok := tryDecode(body, simplifiedchinese.GB18030.NewDecoder()); ok {
		return result, nil
	}
	if result, ok := tryDecode(body, simplifiedchinese.GBK.NewDecoder()); ok {
		return result, nil
	}
	return "", fmt.Errorf("解码失败")
}

func tryDecode(body []byte, decoder transform.Transformer) (string, bool) {
	result, _, err := transform.Bytes(decoder, body)
	if err != nil {
		return "", false
	}
	if !isValidUTF8String(result) {
		return "", false
	}
	return string(result), true
}

func isValidUTF8(data []byte) bool {
	for i := 0; i < len(data); {
		b := data[i]
		if b < 0x80 {
			i++
			continue
		}
		var seqLen int
		if b >= 0xC2 && b <= 0xDF {
			seqLen = 2
		} else if b >= 0xE0 && b <= 0xEF {
			seqLen = 3
		} else if b >= 0xF0 && b <= 0xF7 {
			seqLen = 4
		} else {
			return false
		}
		if i+seqLen > len(data) {
			return false
		}
		for j := i + 1; j < i+seqLen; j++ {
			if data[j] < 0x80 || data[j] > 0xBF {
				return false
			}
		}
		i += seqLen
	}
	return true
}

func isValidUTF8String(data []byte) bool {
	return !bytes.Contains(data, []byte{0xef, 0xbf, 0xbd})
}

// QuickDecode 快速解码
func QuickDecode(body []byte) (string, bool) {
	result, err := DecodeGB2312(body)
	if err != nil {
		return string(body), false
	}
	return result, true
}

// DecodeHTTPResponse 解码 HTTP 响应
func DecodeHTTPResponse(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return DecodeGB2312(body)
}

// ParseResult 解析结果
type ParseResult struct {
	ElecStr string
	ElecF   float64
	Water   string
	WaterF  float64
	OK      bool
}

// ParsePowerFromHTML 解析电量
// 遍历 <h6>，含"水："或"水表"前缀的为水量，其余为电量
func ParsePowerFromHTML(pageHTML string, isC13OrC14 bool) (ParseResult, error) {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		logger.Warn("HTML 解析失败", "err", err)
		return ParseResult{}, err
	}

	type h6Entry struct{ title string; node *html.Node }
	var entries []h6Entry

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "h6" {
			entries = append(entries, h6Entry{title: textContent(n), node: n})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(entries) == 0 {
		logger.Warn("未找到 h6", "len", len(pageHTML))
		return ParseResult{}, ErrNoElectricData
	}

	logger.Debug("ParsePowerFromHTML", "h6", len(entries), "c13c14", isC13OrC14)

	var elecEntry, waterEntry *h6Entry
	for i := range entries {
		entry := &entries[i]
		title := strings.TrimSpace(entry.title)
		if hasWaterPrefix(title) {
			if waterEntry == nil {
				waterEntry = entry
			}
		} else if elecEntry == nil {
			elecEntry = entry
		}
	}

	if elecEntry == nil && waterEntry == nil {
		return ParseResult{}, ErrNoElectricData
	}

	var elecStr, waterStr string
	var elecF, waterF float64

	if elecEntry != nil {
		if nums := extractNumberOrangeSpans(elecEntry.node); len(nums) > 0 {
			elecStr = nums[len(nums)-1]
			fmt.Sscanf(elecStr, "%f", &elecF)
		}
	}
	if waterEntry != nil {
		if nums := extractNumberOrangeSpans(waterEntry.node); len(nums) > 0 {
			waterStr = nums[0]
			fmt.Sscanf(waterStr, "%f", &waterF)
		}
	}

	if elecStr == "" && waterStr == "" {
		return ParseResult{}, ErrNoElectricData
	}

	return ParseResult{ElecStr: elecStr, ElecF: elecF, Water: waterStr, WaterF: waterF, OK: true}, nil
}

func extractNumberOrangeSpans(n *html.Node) []string {
	var spans []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "span" {
			for _, a := range n.Attr {
				if a.Key == "class" && a.Val == "number orange" {
					spans = append(spans, strings.TrimSpace(textContent(n)))
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return spans
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}

func hasWaterPrefix(title string) bool {
	return strings.Contains(title, "水：") || strings.Contains(title, "水表")
}
