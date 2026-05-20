package notifier

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"

	"electricquery/internal/config"
	"electricquery/internal/logger"
)

var sharedHTTPClient = &http.Client{Timeout: 10 * time.Second}

type Message struct {
	Subject string
	Body    string
}

type Notifier struct {
	smtpCfg *config.SMTPSection
}

var defaultNotifier *Notifier

func Init(cfg *config.AppConfig) {
	defaultNotifier = &Notifier{smtpCfg: &cfg.SMTP}
}

func SendEmail(recipientEmail, subject, body string) error {
	if defaultNotifier == nil {
		return fmt.Errorf("notifier 未初始化")
	}
	return defaultNotifier.sendEmail(recipientEmail, subject, body)
}

func SendWechat(webhookURL, subject, body string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL 为空")
	}
	content := fmt.Sprintf("【%s】\n%s", subject, body)
	payload := map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": content},
	}
	data, _ := json.Marshal(payload)

	resp, err := sharedHTTPClient.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("发送失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("返回非 200: %d", resp.StatusCode)
	}
	logger.Info("企业微信推送成功", "subject", subject)
	return nil
}

func SendToUser(email, webhookURL, subject, body string) {
	if email != "" {
		if err := SendEmail(email, subject, body); err != nil {
			logger.Warn("邮件发送失败", "has_email", true, "err", err)
		}
	}
	if webhookURL != "" {
		if err := SendWechat(webhookURL, subject, body); err != nil {
			logger.Warn("企业微信发送失败", "err", err)
		}
	}
	if email == "" && webhookURL == "" {
		logger.Info("未绑定通知渠道", "subject", subject)
	}
}

func SendToUserSynced(webhookURL, email, subject, body string) error {
	if webhookURL != "" {
		if err := SendWechat(webhookURL, subject, body); err != nil {
			return fmt.Errorf("企业微信: %v", err)
		}
	}
	if email != "" {
		if err := SendEmail(email, subject, body); err != nil {
			return fmt.Errorf("邮件: %v", err)
		}
	}
	if webhookURL == "" && email == "" {
		return fmt.Errorf("未绑定通知渠道")
	}
	return nil
}

func (n *Notifier) sendEmail(to, subject, body string) error {
	cfg := n.smtpCfg
	if !cfg.Enabled {
		logger.Info("SMTP 未启用")
		return nil
	}
	if cfg.Server == "" || cfg.Password == "" {
		return fmt.Errorf("SMTP 配置不完整")
	}

	from := fmt.Sprintf("%s <%s>", cfg.SenderName, cfg.SenderEmail)
	msg := buildMIMEMessage(from, to, subject, body)
	addr := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)

	if cfg.UseSSL {
		tlsCfg := &tls.Config{
			ServerName: cfg.Server,
			MinVersion: tls.VersionTLS12,
		}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("SSL 连接失败: %w", err)
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, cfg.Server)
		if err != nil {
			return fmt.Errorf("客户端创建失败: %w", err)
		}
		defer c.Quit()
		auth := smtp.PlainAuth("", cfg.SenderEmail, cfg.Password, cfg.Server)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("认证失败: %w", err)
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
		return err
	}
	auth := smtp.PlainAuth("", cfg.SenderEmail, cfg.Password, cfg.Server)
	return smtp.SendMail(addr, auth, cfg.SenderEmail, []string{to}, []byte(msg))
}

func buildMIMEMessage(from, to, subject, body string) string {
	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body,
	)
}
