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

type PowerResult struct {
	DormRoom     string
	RemainingKwh string
	RemainingF   float64
	WaterAmount  string
	WaterF       float64
}

type DormParts struct {
	Building string
	Floor    string
	Room     string
}

func ParseDorm(dormRoom string) DormParts {
	dormRoom = strings.TrimSpace(dormRoom)

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

func IsC13OrC14(building string) bool {
	building = strings.TrimPrefix(building, "C")
	return building == "13" || building == "14"
}

type Checker struct {
	cfg       *config.PowerCheckerSection
	timeout   time.Duration
	transport *http.Transport
}

func NewChecker(cfg *config.AppConfig) *Checker {
	loginURL := cfg.PowerChecker.LoginURL
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

func (c *Checker) CheckPower(ctx context.Context, building, floor, room string) (*PowerResult, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: c.timeout, Jar: jar, Transport: c.transport}

	loginURL := c.cfg.LoginURL
	ua := c.cfg.UserAgent

	logger.Debug("查询宿舍", "building", building, "floor", floor, "room", room)

	html1, status1, ct1, err := doGet(ctx, client, loginURL, ua)
	if err != nil {
		return nil, fmt.Errorf("step1 GET 失败: %w", err)
	}
	logger.Debug("step1", "status", status1, "ct", ct1, "preview", truncateTo300(html1))

	html2, status2, ct2, err := postEvent(ctx, client, loginURL, ua, html1, "drlouming", map[string]string{
		"drlouming": building,
	})
	if err != nil {
		return nil, fmt.Errorf("step2 失败: %w", err)
	}
	logger.Debug("step2", "status", status2, "ct", ct2, "ablou", ExtractDropOptions(html2, "ablou"))

	html3, status3, ct3, err := postEvent(ctx, client, loginURL, ua, html2, "ablou", map[string]string{
		"drlouming": building,
		"ablou":     floor,
	})
	if err != nil {
		return nil, fmt.Errorf("step3 失败: %w", err)
	}
	logger.Debug("step3", "status", status3, "ct", ct3, "drceng", ExtractDropOptions(html3, "drceng"))

	html4, status4, ct4, err := postEvent(ctx, client, loginURL, ua, html3, "drceng", map[string]string{
		"drlouming": building,
		"ablou":     floor,
		"drceng":    room,
	})
	if err != nil {
		return nil, fmt.Errorf("step4 失败: %w", err)
	}
	logger.Debug("step4", "status", status4, "ct", ct4)

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

func (c *Checker) CheckPowerByDorm(ctx context.Context, dormRoom string) (*PowerResult, error) {
	parts := ParseDorm(dormRoom)
	result, err := c.CheckPower(ctx, parts.Building, parts.Floor, parts.Room)
	if err != nil {
		return nil, err
	}
	result.DormRoom = dormRoom
	return result, nil
}

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

type DropOption struct {
	Value string
	Text  string
}

func ExtractDropOptionsWithText(pageHTML, selectID string) []DropOption {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}
	var options []DropOption
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
			var value string
			for _, a := range n.Attr {
				if a.Key == "value" && a.Val != "" {
					value = a.Val
					break
				}
			}
			if value != "" {
				options = append(options, DropOption{
					Value: value,
					Text:  extractTextContent(n),
				})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return options
}

func ExtractDropOptions(pageHTML, selectID string) []string {
	opts := ExtractDropOptionsWithText(pageHTML, selectID)
	if opts == nil {
		return nil
	}
	result := make([]string, len(opts))
	for i, o := range opts {
		result[i] = o.Value
	}
	return result
}

func ExtractDropOptionTexts(pageHTML, selectID string) []string {
	opts := ExtractDropOptionsWithText(pageHTML, selectID)
	if opts == nil {
		return nil
	}
	result := make([]string, len(opts))
	for i, o := range opts {
		result[i] = o.Text
	}
	return result
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
