// Package checker 实现宿舍电量爬取
// ASP.NET WebForms 四步表单提交：GET首页 → POST选楼栋 → POST选楼层 → POST选房间
package checker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"electricquery/internal/config"
	"electricquery/internal/logger"
	"electricquery/internal/pkg/parser"

	"golang.org/x/net/html"
)

// PowerResult 查询结果
type PowerResult struct {
	DormRoom     string  // 原始宿舍号
	RemainingKwh string  // 剩余电量
	RemainingF   float64 // 剩余金额
	WaterAmount  string  // 剩余水量（C13/C14）
	WaterF       float64
}

// DormParts 宿舍号拆解
type DormParts struct {
	Building string // 楼栋
	Floor    string // 楼层/单元
	Room     string // 房间码
}

// ParseDorm 解析宿舍号
// 支持：140328、C14-328、C13-1301电、11-0101-011101、11|1101|132水表
func ParseDorm(dormRoom string) DormParts {
	dormRoom = strings.TrimSpace(dormRoom)

	// 旧格式兼容
	if pipeParts := strings.Split(dormRoom, "|"); len(pipeParts) == 3 {
		return DormParts{Building: pipeParts[0], Floor: pipeParts[1], Room: pipeParts[2]}
	}

	parts := strings.Split(dormRoom, "-")

	switch len(parts) {
	case 3:
		building := strings.TrimPrefix(parts[0], "C")
		return DormParts{Building: building, Floor: parts[1], Room: parts[2]}

	case 2:
		building := strings.TrimPrefix(parts[0], "C")
		rest := parts[1]

		// 含电/水后缀
		if strings.Contains(rest, "电") || strings.Contains(rest, "水") {
			return DormParts{Building: building, Floor: building, Room: rest}
		}
		return parseSixDigit(building, rest)

	case 1:
		s := strings.TrimPrefix(parts[0], "C")
		if len(s) < 2 {
			return DormParts{Building: s, Floor: s, Room: s}
		}
		return parseSixDigit(s[:2], s[2:])
	}

	return DormParts{Building: dormRoom, Floor: dormRoom, Room: dormRoom}
}

// parseSixDigit 按 楼(2)+层(2)+房(2) 拆分 6 位码
func parseSixDigit(building, suffix string) DormParts {
	if len(suffix) < 2 {
		return DormParts{Building: building, Floor: "", Room: building + suffix}
	}
	room := building + suffix

	if building == "01" {
		return DormParts{Building: building, Floor: building + suffix[:2], Room: room}
	}
	return DormParts{Building: building, Floor: building + suffix[:2], Room: room}
}

// IsC13OrC14 判断是否为 C13/C14（水电同页）
func IsC13OrC14(building string) bool {
	building = strings.TrimPrefix(building, "C")
	return building == "13" || building == "14"
}

// Checker HTTP 会话
type Checker struct {
	cfg       *config.PowerCheckerSection
	timeout   time.Duration
	transport *http.Transport
}

// NewChecker 创建 Checker
func NewChecker(cfg *config.AppConfig) *Checker {
	loginURL := cfg.PowerChecker.LoginURL
	logger.Debug("NewChecker 初始化", "login_url", loginURL)
	if loginURL == "" {
		logger.Fatal("致命错误: power_checker.login_url 未配置")
	}
	return &Checker{
		cfg:     &cfg.PowerChecker,
		timeout: time.Duration(cfg.PowerChecker.TimeoutSeconds) * time.Second,
		transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// CheckPower 查询电量
func (c *Checker) CheckPower(ctx context.Context, building, floor, room string) (*PowerResult, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: c.timeout, Jar: jar, Transport: c.transport}

	loginURL := c.cfg.LoginURL
	ua := c.cfg.UserAgent

	logger.Debug("查询宿舍", "building", building, "floor", floor, "room", room)

	// Step 1: GET 首页
	html1, status1, ct1, err := doGet(ctx, client, loginURL, ua)
	if err != nil {
		return nil, fmt.Errorf("step1 GET 失败: %w", err)
	}
	logger.Debug("step1", "status", status1, "ct", ct1, "preview", truncateTo300(html1))

	// Step 2: POST 选楼栋
	html2, status2, ct2, err := postEvent(ctx, client, loginURL, ua, html1, "drlouming", map[string]string{
		"drlouming": building,
	})
	if err != nil {
		return nil, fmt.Errorf("step2 失败: %w", err)
	}
	logger.Debug("step2", "status", status2, "ct", ct2, "ablou", ExtractDropOptions(html2, "ablou"))

	// Step 3: POST 选楼层
	html3, status3, ct3, err := postEvent(ctx, client, loginURL, ua, html2, "ablou", map[string]string{
		"drlouming": building,
		"ablou":     floor,
	})
	if err != nil {
		return nil, fmt.Errorf("step3 失败: %w", err)
	}
	logger.Debug("step3", "status", status3, "ct", ct3, "drceng", ExtractDropOptions(html3, "drceng"))

	// Step 4: POST 选房间
	html4, status4, ct4, err := postEvent(ctx, client, loginURL, ua, html3, "drceng", map[string]string{
		"drlouming": building,
		"ablou":     floor,
		"drceng":    room,
	})
	if err != nil {
		return nil, fmt.Errorf("step4 失败: %w", err)
	}
	logger.Debug("step4", "status", status4, "ct", ct4)

	// 解析
	result, err := parser.ParsePowerFromHTML(html4, IsC13OrC14(building))
	if err != nil || !result.OK {
		htmlPreview := html4
		if len(htmlPreview) > 1000 {
			htmlPreview = htmlPreview[:1000]
		}
		logger.Warn("解析失败", "building", building, "floor", floor, "room", room, "html", htmlPreview)
		return nil, fmt.Errorf("解析电量失败")
	}

	logger.Debug("查询成功", "电", result.ElecStr, "水", result.Water)
	return &PowerResult{
		RemainingKwh: result.ElecStr,
		RemainingF:  result.ElecF,
		WaterAmount:  result.Water,
		WaterF:       result.WaterF,
	}, nil
}

// CheckPowerByDorm 解析宿舍号并查询
func (c *Checker) CheckPowerByDorm(ctx context.Context, dormRoom string) (*PowerResult, error) {
	parts := ParseDorm(dormRoom)
	result, err := c.CheckPower(ctx, parts.Building, parts.Floor, parts.Room)
	if err != nil {
		return nil, err
	}
	result.DormRoom = dormRoom
	return result, nil
}

// StepByStep 逐步执行，返回指定步骤的 HTML
func (c *Checker) StepByStep(ctx context.Context, building, floor, room string) (string, string, int, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: c.timeout, Jar: jar, Transport: c.transport}

	loginURL := c.cfg.LoginURL
	ua := c.cfg.UserAgent

	html1, _, _, err := doGet(ctx, client, loginURL, ua)
	if err != nil {
		return "", "", 0, err
	}

	if floor == "" && room == "" {
		html2, status, _, err := postEvent(ctx, client, loginURL, ua, html1, "drlouming", map[string]string{
			"drlouming": building,
		})
		return html1, html2, status, err
	}

	if room == "" {
		html2, _, _, err := postEvent(ctx, client, loginURL, ua, html1, "drlouming", map[string]string{
			"drlouming": building,
		})
		if err != nil {
			return "", "", 0, err
		}
		html3, status, _, err := postEvent(ctx, client, loginURL, ua, html2, "ablou", map[string]string{
			"drlouming": building,
			"ablou":     floor,
		})
		return html1, html3, status, err
	}

	_, err = c.CheckPower(ctx, building, floor, room)
	return "", "", 200, err
}

// ========================
// 内部 HTTP 工具
// ========================

func doGet(ctx context.Context, client *http.Client, rawURL, ua string) (string, int, string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", 0, "", err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, "", err
	}
	return decodeBody(body, resp.Header.Get("Content-Type")), resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

func postEvent(ctx context.Context, client *http.Client, rawURL, ua, pageHTML, eventTarget string, extra map[string]string) (string, int, string, error) {
	viewstate, vsg, ev, err := extractViewState(pageHTML)
	if err != nil {
		return "", 0, "", fmt.Errorf("提取 VIEWSTATE 失败: %w", err)
	}

	form := url.Values{}
	form.Set("__VIEWSTATE", viewstate)
	form.Set("__VIEWSTATEGENERATOR", vsg)
	if ev != "" {
		form.Set("__EVENTVALIDATION", ev)
	}
	form.Set("__EVENTTARGET", eventTarget)
	form.Set("__EVENTARGUMENT", "")
	form.Set("radio", "buyR")
	form.Set("ImageButton1.x", "10")
	form.Set("ImageButton1.y", "10")
	for k, v := range extra {
		form.Set(k, v)
	}

	req, err := http.NewRequest("POST", rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, "", err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, "", err
	}
	return decodeBody(body, resp.Header.Get("Content-Type")), resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

func decodeBody(body []byte, contentType string) string {
	result, err := parser.DecodeGB2312(body)
	if err != nil {
		logger.Warn("解码失败", "err", err)
		return string(body)
	}
	return result
}

// ExtractDropOptions 提取下拉选项
func ExtractDropOptions(pageHTML, selectID string) []string {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}
	var options []string
	var inSelect bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "select" {
			for _, a := range n.Attr {
				if a.Key == "id" && a.Val == selectID {
					inSelect = true
					break
				}
			}
		}
		if inSelect && n.Type == html.ElementNode && n.Data == "option" {
			for _, a := range n.Attr {
				if a.Key == "value" && a.Val != "" {
					options = append(options, a.Val)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return options
}

// ExtractDropOptionTexts 提取下拉选项显示文本
func ExtractDropOptionTexts(pageHTML, selectID string) []string {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}
	var texts []string
	var inSelect bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "select" {
			for _, a := range n.Attr {
				if a.Key == "id" && a.Val == selectID {
					inSelect = true
					break
				}
			}
		}
		if inSelect && n.Type == html.ElementNode && n.Data == "option" {
			for _, a := range n.Attr {
				if a.Key == "value" && a.Val != "" {
					texts = append(texts, extractTextContent(n))
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return texts
}

func extractTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.TrimSpace(n.Data)
	}
	var result strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result.WriteString(extractTextContent(c))
	}
	return strings.TrimSpace(result.String())
}

// extractViewState 提取 VIEWSTATE
func extractViewState(pageHTML string) (string, string, string, error) {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", "", "", err
	}

	var viewstate, vsg, ev string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			var name, value string
			for _, a := range n.Attr {
				if a.Key == "name" {
					name = a.Val
				} else if a.Key == "value" {
					value = a.Val
				}
			}
			switch name {
			case "__VIEWSTATE":
				viewstate = value
			case "__VIEWSTATEGENERATOR":
				vsg = value
			case "__EVENTVALIDATION":
				ev = value
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if viewstate == "" {
		return "", "", "", fmt.Errorf("未找到 __VIEWSTATE")
	}
	return viewstate, vsg, ev, nil
}

func truncateTo300(s string) string {
	if len(s) > 300 {
		return s[:300]
	}
	return s
}
