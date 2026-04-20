// Package notifier 实现多渠道消息推送
// 支持：SMTP 邮件、企业微信机器人 Webhook
package notifier

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"time"

	"electricquery/internal/config"
)

// Message 通知消息结构
type Message struct {
	Subject string
	Body    string
}

// Notifier 负责向单个用户发送通知
type Notifier struct {
	smtpCfg *config.SMTPSection
}

var defaultNotifier *Notifier

// Init 初始化全局通知器
func Init(cfg *config.AppConfig) {
	defaultNotifier = &Notifier{smtpCfg: &cfg.SMTP}
}

// SendEmail 通过 SMTP 发送邮件给指定地址
// recipientEmail: 收件人邮箱
// subject, body: 邮件主题和正文
func SendEmail(recipientEmail, subject, body string) error {
	if defaultNotifier == nil {
		return fmt.Errorf("notifier 未初始化")
	}
	return defaultNotifier.sendEmail(recipientEmail, subject, body)
}

// SendWechat 向企业微信机器人 Webhook URL 发送文本消息
func SendWechat(webhookURL, subject, body string) error {
	if webhookURL == "" {
		return fmt.Errorf("企业微信 webhook URL 为空，跳过发送")
	}
	content := fmt.Sprintf("【%s】\n%s", subject, body)
	payload := map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": content},
	}
	data, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("企业微信发送失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("企业微信返回非 200 状态: %d", resp.StatusCode)
	}
	log.Printf("[notifier] 企业微信推送成功 subject=%s", subject)
	return nil
}

// SendToUser 向用户发送通知（异步，错误仅记录日志）
// email: 用户绑定的邮箱（可为空）
// webhookURL: 用户绑定的企业微信 Webhook（可为空）
func SendToUser(email, webhookURL, subject, body string) {
	if email != "" {
		if err := SendEmail(email, subject, body); err != nil {
			log.Printf("[notifier] 邮件发送失败 to=%s err=%v", email, err)
		}
	}
	if webhookURL != "" {
		if err := SendWechat(webhookURL, subject, body); err != nil {
			log.Printf("[notifier] 企业微信发送失败 err=%v", err)
		}
	}
	if email == "" && webhookURL == "" {
		log.Printf("[notifier] 用户未绑定任何通知渠道，跳过推送 subject=%s", subject)
	}
}

// SendToUserSynced 向用户发送通知（同步，错误返回）
func SendToUserSynced(webhookURL, email, subject, body string) error {
	if webhookURL != "" {
		if err := SendWechat(webhookURL, subject, body); err != nil {
			return fmt.Errorf("企业微信发送失败: %v", err)
		}
	}
	if email != "" {
		if err := SendEmail(email, subject, body); err != nil {
			return fmt.Errorf("邮件发送失败: %v", err)
		}
	}
	if webhookURL == "" && email == "" {
		return fmt.Errorf("未绑定任何通知渠道")
	}
	return nil
}

// ---- SMTP 内部实现 ----

func (n *Notifier) sendEmail(to, subject, body string) error {
	cfg := n.smtpCfg
	if !cfg.Enabled {
		log.Printf("[notifier] SMTP 未启用，跳过邮件发送")
		return nil
	}
	if cfg.Server == "" || cfg.Password == "" {
		return fmt.Errorf("SMTP 配置不完整（server 或 password 为空）")
	}

	from := fmt.Sprintf("%s <%s>", cfg.SenderName, cfg.SenderEmail)
	msg := buildMIMEMessage(from, to, subject, body)
	addr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)

	if cfg.UseSSL {
		// SSL 直连（465 端口）
		tlsCfg := &tls.Config{ServerName: cfg.Server}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("SSL 连接 SMTP 失败: %w", err)
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, cfg.Server)
		if err != nil {
			return fmt.Errorf("SMTP 客户端创建失败: %w", err)
		}
		defer c.Quit()
		auth := smtp.PlainAuth("", cfg.SenderEmail, cfg.Password, cfg.Server)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
		if err := c.Mail(cfg.SenderEmail); err != nil {
			return err
		}
		if err := c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		defer w.Close()
		_, err = w.Write([]byte(msg))
		if err != nil {
			return err
		}
	} else {
		// STARTTLS（587 端口）
		auth := smtp.PlainAuth("", cfg.SenderEmail, cfg.Password, cfg.Server)
		if err := smtp.SendMail(addr, auth, cfg.SenderEmail, []string{to}, []byte(msg)); err != nil {
			return fmt.Errorf("SMTP 发送失败: %w", err)
		}
	}

	log.Printf("[notifier] 邮件发送成功 to=%s subject=%s", to, subject)
	return nil
}

func buildMIMEMessage(from, to, subject, body string) string {
	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body,
	)
}
