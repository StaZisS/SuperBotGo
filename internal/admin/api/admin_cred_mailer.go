package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
)

const temporaryPasswordBytes = 18

type AdminCredentialMailer interface {
	SendAdminCredentials(ctx context.Context, to, password string) error
}

type SMTPMailerConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPMailer struct {
	cfg SMTPMailerConfig
}

func NewSMTPMailer(cfg SMTPMailerConfig) *SMTPMailer {
	if cfg.Host == "" || cfg.From == "" {
		return nil
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) SendAdminCredentials(ctx context.Context, to, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	subject := mime.QEncoding.Encode("utf-8", "Доступ в админку SuperBotGo")
	body := fmt.Sprintf("Вам предоставлен доступ в админку SuperBotGo.\n\nEmail: %s\nВременный пароль: %s\n\nПосле входа смените пароль в настройках админки.\n", to, password)
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", m.cfg.From, to, subject, body)

	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}

	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	if err := smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(message)); err != nil {
		return fmt.Errorf("send admin credentials email: %w", err)
	}
	return nil
}

func generateTemporaryPassword() (string, error) {
	buf := make([]byte, temporaryPasswordBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate temporary password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
