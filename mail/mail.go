package mail

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type Service struct {
	config SMTPConfig
}

func NewService(config SMTPConfig) *Service {
	return &Service{config: config}
}

func (s *Service) SendPasswordResetEmail(toEmail, resetLink string) error {
	subject := "密码重置 - Image Go"
	body := fmt.Sprintf(`您好，

您收到此邮件是因为您请求重置 Image Go 账户的密码。

请点击以下链接重置密码（链接有效期 1 小时）：
%s

如果您没有请求重置密码，请忽略此邮件。

此致
Image Go 团队`, resetLink)

	return s.sendEmail(toEmail, subject, body)
}

func (s *Service) sendEmail(to, subject, body string) error {
	from := s.config.From
	if from == "" {
		from = s.config.Username
	}

	headers := map[string]string{
		"From":         from,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=utf-8",
	}

	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)

	if s.config.Port == 465 {
		return s.sendEmailTLS(addr, auth, from, to, msg.String())
	}

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg.String()))
}

func (s *Service) sendEmailTLS(addr string, auth smtp.Auth, from, to, msg string) error {
	tlsConfig := &tls.Config{
		ServerName: s.config.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS 连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP 认证失败: %w", err)
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("设置收件人失败: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %w", err)
	}

	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("关闭数据写入器失败: %w", err)
	}

	return client.Quit()
}
