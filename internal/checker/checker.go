// Package checker 实现宿舍电量爬取核心逻辑
// 完整还原 Python requestNum.py 的 ASP.NET WebForms 三步表单提交流程：
//   1. GET 主页，获取初始 __VIEWSTATE
//   2. POST 选楼栋 (drlouming)
//   3. POST 选楼层 (ablou)
//   4. POST 选房间 (drceng)，从结果页解析剩余电量
package checker

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"electricquery/internal/config"

	"golang.org/x/net/html"
)

// PowerResult 封装查询结果
// C13/C14 楼水电同页：电量存 RemainingKwh，水量存 WaterAmount
// 其他楼栋：电量存 RemainingKwh，WaterAmount 为空（需单独查询）
type PowerResult struct {
	DormRoom      string // 原始宿舍号（用户填写的）
	Building      string // 楼栋参数
	Floor         string // 楼层参数
	Room          string // 房间参数
	RemainingKwh  string // 剩余电量字符串（原始值，保留精度）
	RemainingF    float64
	WaterAmount   string // 剩余水量字符串（仅 C13/C14 有值）
	WaterF        float64
}

// IsC13OrC14 判断楼栋是否为 C13 或 C14（水电同页查询）
// 支持 "C13"/"C14" 或 "13"/"14" 两种格式
func IsC13OrC14(building string) bool {
	building = strings.TrimPrefix(building, "C")
	return building == "13" || building == "14"
}

// Checker 持有一个独立的 HTTP session，每次查询使用新 session 避免 VIEWSTATE 污染
type Checker struct {
	cfg     *config.PowerCheckerSection
	timeout time.Duration
}

// NewChecker 构造一个新的 Checker 实例
func NewChecker(cfg *config.AppConfig) *Checker {
	loginURL := cfg.PowerChecker.LoginURL
	log.Printf("[checker] NewChecker 初始化，login_url=[%s]", loginURL)
	if loginURL == "" {
		log.Fatalf("[checker] 致命错误: power_checker.login_url 未配置，请检查 application.conf 文件是否存在且包含正确内容")
	}
	return &Checker{
		cfg:     &cfg.PowerChecker,
		timeout: time.Duration(cfg.PowerChecker.TimeoutSeconds) * time.Second,
	}
}

// CheckPower 查询指定楼栋/楼层/房间的剩余电量
// building: 楼栋代码（如 "01", "14"）前端存储 "C14" 传入时 ParseDorm 会自动去前缀
// floor:    楼层代码（如 "03", "1401"）
// room:     房间代码（如 "140328"）
func (c *Checker) CheckPower(building, floor, room string) (*PowerResult, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: c.timeout,
		Jar:     jar,
	}

	loginURL := c.cfg.LoginURL
	ua := c.cfg.UserAgent

	log.Printf("[checker] 开始查询 building=%s floor=%s room=%s", building, floor, room)

	// Step 1: GET 首页，取初始 VIEWSTATE
	html1, status1, ct1, err := doGet(client, loginURL, ua)
	if err != nil {
		return nil, fmt.Errorf("step1 GET 失败: %w", err)
	}
	log.Printf("[checker] step1 响应: status=%d content-type=%s 前300字=[%s]", status1, ct1, trim300(html1))

	// Step 2: POST 选楼栋
	html2, status2, ct2, err := postEvent(client, loginURL, ua, html1, "drlouming", map[string]string{
		"drlouming": building,
	})
	if err != nil {
		return nil, fmt.Errorf("step2 POST(drlouming) 失败: %w", err)
	}
	log.Printf("[checker] step2 响应: status=%d content-type=%s 前300字=[%s]", status2, ct2, trim300(html2))

	// Step 3: POST 选楼层
	html3, status3, ct3, err := postEvent(client, loginURL, ua, html2, "ablou", map[string]string{
		"drlouming": building,
		"ablou":     floor,
	})
	if err != nil {
		return nil, fmt.Errorf("step3 POST(ablou) 失败: %w", err)
	}
	log.Printf("[checker] step3 响应: status=%d content-type=%s 前300字=[%s]", status3, ct3, trim300(html3))

	// Step 4: POST 选房间，获取结果页
	html4, status4, ct4, err := postEvent(client, loginURL, ua, html3, "drceng", map[string]string{
		"drlouming": building,
		"ablou":     floor,
		"drceng":    room,
	})
	if err != nil {
		return nil, fmt.Errorf("step4 POST(drceng) 失败: %w", err)
	}
	log.Printf("[checker] step4 响应: status=%d content-type=%s 前300字=[%s]", status4, ct4, trim300(html4))

	// 解析结果页中的剩余电量和水费（C13/C14 双值，其他楼栋仅电量）
	// parsePowerAndWater 返回: elec(string), elecF(float64), water(string), waterF(float64)
	_, remainingF, waterAmt, waterF, err := parsePowerAndWater(html4, IsC13OrC14(building))
	if err != nil {
		log.Printf("[checker] 解析电量失败: %v", err)
		return nil, fmt.Errorf("解析电量失败: %w", err)
	}

	// 原始字符串需要从 h6 标签中提取（parsePowerAndWater 已提取但类型不匹配）
	// 重新提取原始字符串用于日志和返回
	elecStr, _, err2 := parseH6FromHTML(html4, IsC13OrC14(building))
	if err2 != nil {
		return nil, fmt.Errorf("解析电量失败: %w", err2)
	}

	log.Printf("[checker] 查询成功 building=%s floor=%s room=%s 电=%s度 水=%s", building, floor, room, elecStr, waterAmt)

	result := &PowerResult{
		Building:      building,
		Floor:         floor,
		Room:          room,
		RemainingKwh:  elecStr,
		RemainingF:    remainingF,
		WaterAmount:   waterAmt,
		WaterF:        waterF,
	}
	return result, nil
}

// CheckPowerByDorm 解析宿舍号字符串并查询电量
// 支持格式:
//   - "楼栋-楼层-房间" 如 "01-0101-010101"
//   - "楼栋-房间"     如 "C13-1301电"（水电分房，水房时 room 含"电"/"水"后缀）
//   - "C楼栋-3位/4位房间" 如 "C14-328"（C13/C14 水电同页，3位=楼层+房间）
//   - 单个字符串      如 "010101"（6位自动解析）
func (c *Checker) CheckPowerByDorm(dormRoom string) (*PowerResult, error) {
	building, floor, room := ParseDorm(dormRoom)
	result, err := c.CheckPower(building, floor, room)
	if err != nil {
		return nil, err
	}
	result.DormRoom = dormRoom
	return result, nil
}

// ParseDorm 解析宿舍号字符串为楼栋/楼层/房间三个参数
//
// 房间号格式规则（来自前端显示值）：
//   "C14-328"   → 3位: building="14", floor="1403" (C14三层), room="140328"
//   "C14-1428"  → 4位: building="14", floor="1401" (C14一层), room="140128"
//   "C13-1301电" → 水电分房: building="13", floor="1301", room="1301电"
//   "01-0101-010101" → 三段: building="01", floor="0101", room="010101"
//   "010101"     → 纯6位: building="01", floor="0101", room="010101"
//
// 规则总结：
//   去掉 "C" 前缀后，
//   - 如果剩余部分是 3 位：首位=楼层数字（0+楼层=4位楼层如"1403"），后2位=房间号
//   - 如果剩余部分是 4 位：前2位=楼层数字（拼楼栋成4位如"1401"），后2位=房间号
//   - 如果包含"电"/"水"后缀：floor=building，room=原始值（不拆分楼层）
func ParseDorm(dormRoom string) (building, floor, room string) {
	dormRoom = strings.TrimSpace(dormRoom)
	parts := strings.Split(dormRoom, "-")

	switch len(parts) {
	case 3:
		// "01-0101-010101" → 三段直接用
		building = strings.TrimPrefix(parts[0], "C")
		floor = parts[1]
		room = parts[2]

	case 2:
		building = strings.TrimPrefix(parts[0], "C")
		rest := parts[1]

		// 含"电"/"水"后缀 → 水电分房，floor=building，room=原始
		if strings.Contains(rest, "电") || strings.Contains(rest, "水") {
			floor = building
			room = rest
			return
		}

		// 纯数字房间号（3位或4位）
		// 规则：rest 第一位 = 楼层数字 → "0"+该位 = 2位楼层；room = 后2位
		// 例: "328" → floor="1403" (楼14+"0"+3), room="140328" (floor+"28")
		// 例: "1428" → floor="1401" (楼14+"0"+1), room="140128" (floor+"28")
		floor = building + "0" + rest[:1]
		room = floor + rest[len(rest)-2:]

	case 1:
		s := strings.TrimPrefix(parts[0], "C")
		if len(s) == 6 {
			// 纯6位：前2位=楼栋, 前4位=楼层, 完整=房间
			// 例: "010101" → building="01", floor="0101", room="010101"
			building = s[0:2]
			floor = s[0:4]
			room = s
		} else if len(s) == 4 {
			// 4位：楼栋+楼层合并，如 "1403" → building="14", floor="1403"
			building = s[0:2]
			floor = s
			room = s
		} else {
			building = s
			floor = s
			room = s
		}

	default:
		building = dormRoom
		floor = dormRoom
		room = dormRoom
	}
	return
}

// ========================
//  内部 HTTP 工具函数
// ========================

func doGet(client *http.Client, rawURL, ua string) (string, int, string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", 0, "", err
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ct := resp.Header.Get("Content-Type")
	return string(body), resp.StatusCode, ct, err
}

// postEvent 提取当前页面的 VIEWSTATE，拼装表单数据，发起 POST
// 与 Python 后端 requestNum.py 完全一致的请求格式，不带 Referer 头
func postEvent(client *http.Client, rawURL, ua, pageHTML, eventTarget string, extra map[string]string) (string, int, string, error) {
	viewstate, vsg, ev, err := extractViewState(pageHTML)
	if err != nil {
		diag := pageHTML
		if len(diag) > 500 {
			diag = diag[:500]
		}
		return "", 0, "", fmt.Errorf("提取 VIEWSTATE 失败(页面前500字=[%s]): %w", diag, err)
	}

	form := url.Values{}
	form.Set("__VIEWSTATE", viewstate)
	form.Set("__VIEWSTATEGENERATOR", vsg)
	if ev != "" {
		form.Set("__EVENTVALIDATION", ev) // 初始页可能没有此字段
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
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// 注意：不设置 Referer 头，与 Python requestNum.py 行为一致

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), resp.StatusCode, resp.Header.Get("Content-Type"), err
}

// extractViewState 从 HTML 中提取 __VIEWSTATE、__VIEWSTATEGENERATOR 和 __EVENTVALIDATION 的 value
func extractViewState(pageHTML string) (viewstate, vsg, ev string, err error) {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", "", "", fmt.Errorf("HTML 解析失败: %w", err)
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			name := ""
			value := ""
			for _, a := range n.Attr {
				switch a.Key {
				case "name":
					name = a.Val
				case "value":
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

	log.Printf("[checker] 提取结果: viewstate长度=%d vsg=[%s] ev长度=%d",
		len(viewstate), vsg, len(ev))
	if viewstate == "" {
		return "", "", "", fmt.Errorf("未找到 __VIEWSTATE 字段")
	}
	return viewstate, vsg, ev, nil
}

// parsePowerAndWater 从结果页 HTML 解析电量和水费
//
// h6 标签解析规则：
//   - C13/C14 水电同页：遍历所有 h6，检查 h6 标题（第一个文本节点）是否含"水"字
//       标题含"水"（如 "C14-328水："）→ 水量
//       标题不含"水"（如 "C14-328："）→ 电量
//   - 其他楼栋：仅电量，无水量
//
// <span class="number orange"> 解析：index=0→购买量，index=1→补助，index=2→剩余
func parsePowerAndWater(pageHTML string, isC13OrC14 bool) (elec, elecF float64, water string, waterF float64, err error) {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return 0, 0, "", 0, fmt.Errorf("结果页 HTML 解析失败: %w", err)
	}

	// 收集所有 h6 节点及其标题
	type h6Entry struct {
		node  *html.Node
		title string // h6 标题部分（第一个直接文本节点）
	}
	var entries []h6Entry
	var collectH6 func(*html.Node)
	collectH6 = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "h6" {
			title := ""
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					title = strings.TrimSpace(c.Data)
					break
				}
			}
			entries = append(entries, h6Entry{node: n, title: title})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectH6(c)
		}
	}
	collectH6(doc)

	if len(entries) == 0 {
		return 0, 0, "", 0, fmt.Errorf("未找到任何 <h6> 标签，页面可能未正确返回数据")
	}

	// C13/C14 水电同页：遍历所有 h6，根据标题前缀判断水电
	if isC13OrC14 {
		for i, e := range entries {
			val, valF, parseErr := parseH6Value(e.node)
			log.Printf("[checker] h6[%d] 标题=[%s] 解析值=%s 解析错误=%v",
				i, e.title, val, parseErr)

			// 判断是否为水量：标题含"水："前缀（水电同页时水量标题是 "C14-328水："）
			isWater := strings.Contains(e.title, "水：")
			if isWater {
				// 标题含"水："前缀 → 水量
				water = val
				waterF = valF
			} else {
				// 其余为电量
				elecStr := val
				elecF = valF
				elec, _ = strconv.ParseFloat(elecStr, 64)
			}
		}
	} else {
		// 其他楼栋：取第一个 h6 作为电量
		val, valF, err := parseH6Value(entries[0].node)
		if err != nil {
			return 0, 0, "", 0, fmt.Errorf("解析电量失败: %w", err)
		}
		elecStr := val
		elecF = valF
		elec, _ = strconv.ParseFloat(elecStr, 64)
	}

	return elec, elecF, water, waterF, nil
}

// parseH6FromHTML 从 HTML 中提取电量原始字符串
// C13/C14 根据 CSS 类判断：非蓝色 = 电量 h6，其他楼栋取第一个
func parseH6FromHTML(pageHTML string, isC13OrC14 bool) (elecStr string, f float64, err error) {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", 0, err
	}
	type h6Entry struct {
		node  *html.Node
		title string
	}
	var entries []h6Entry
	var collectH6 func(*html.Node)
	collectH6 = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "h6" {
			title := ""
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					title = strings.TrimSpace(c.Data)
					break
				}
			}
			entries = append(entries, h6Entry{node: n, title: title})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectH6(c)
		}
	}
	collectH6(doc)
	if len(entries) == 0 {
		return "", 0, fmt.Errorf("未找到 <h6> 标签")
	}

	if isC13OrC14 {
		// 找非水标题（电量 meter）的 h6
		for _, e := range entries {
			if !strings.Contains(e.title, "水：") {
				return parseH6Value(e.node)
			}
		}
		return parseH6Value(entries[0].node)
	}
	return parseH6Value(entries[0].node)
}

// parseH6Value 解析单个 h6 标签中的电量/水费值
// 返回第三个 <span class="number orange"> 的文字（index=2 → 剩余值）
func parseH6Value(h6 *html.Node) (value string, fvalue float64, err error) {
	var spans []*html.Node
	var collectSpans func(*html.Node)
	collectSpans = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "span" {
			for _, a := range n.Attr {
				if a.Key == "class" && a.Val == "number orange" {
					spans = append(spans, n)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectSpans(c)
		}
	}
	collectSpans(h6)

	if len(spans) < 3 {
		return "", 0, fmt.Errorf("<span class='number orange'> 数量不足（需要≥3，实际=%d）", len(spans))
	}

	value = strings.TrimSpace(textContent(spans[2]))
	if value == "" {
		return "", 0, fmt.Errorf("第三个 span 文字内容为空")
	}
	fmt.Sscanf(value, "%f", &fvalue)
	return value, fvalue, nil
}

// textContent 递归提取节点的所有文字内容
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

// trim300 截取字符串前 300 字符（用于日志输出）
func trim300(s string) string {
	if len(s) > 300 {
		return s[:300]
	}
	return s
}
