package service

import (
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"strings"
)

// Emailer abstracts code delivery so we can swap Mailhog/Resend/stdout.
type Emailer interface {
	SendLoginCode(to, code string) error
}

type SMTPEmailer struct {
	log      *slog.Logger
	host     string
	port     string
	from     string
	username string
	password string
}

// NewSMTPEmailer wires an SMTP-based emailer for local Mailhog dev.
// Env: SMTP_HOST (default: localhost), SMTP_PORT (default: 1025), SMTP_USERNAME, SMTP_PASSWORD.
func NewSMTPEmailer(log *slog.Logger, from string) *SMTPEmailer {
	return &SMTPEmailer{
		log:      log,
		host:     envOr("SMTP_HOST", "localhost"),
		port:     envOr("SMTP_PORT", "1025"),
		from:     from,
		username: os.Getenv("SMTP_USERNAME"),
		password: os.Getenv("SMTP_PASSWORD"),
	}
}

func (e *SMTPEmailer) SendLoginCode(to, code string) error {
	subject := "MediGt giriş kodu"
	body := fmt.Sprintf(
		"Merhaba,\r\n\r\nMediGt giriş kodunuz: %s\r\n\r\nKod 5 dakika geçerlidir.\r\nBu isteği siz yapmadıysanız bu e-postayı yok sayın.\r\n",
		code,
	)
	msg := buildMessage(e.from, to, subject, body)

	addr := e.host + ":" + e.port
	var auth smtp.Auth
	if e.username != "" {
		auth = smtp.PlainAuth("", e.username, e.password, e.host)
	}
	if err := smtp.SendMail(addr, auth, e.from, []string{to}, msg); err != nil {
		e.log.Error("smtp send failed", "err", err, "to", to, "addr", addr)
		return err
	}
	e.log.Info("login code sent", "to", to)
	return nil
}

// StdoutEmailer prints the code instead of sending email — used when no SMTP
// is reachable and we're not in production.
type StdoutEmailer struct {
	log *slog.Logger
}

func NewStdoutEmailer(log *slog.Logger) *StdoutEmailer { return &StdoutEmailer{log: log} }

func (e *StdoutEmailer) SendLoginCode(to, code string) error {
	e.log.Info("LOGIN CODE (stdout fallback — set SMTP_HOST or APP_ENV=production)",
		"to", to, "code", code)
	return nil
}

// ChainEmailer tries each emailer in order; first success wins.
type ChainEmailer struct {
	log    *slog.Logger
	chain  []Emailer
}

func NewChainEmailer(log *slog.Logger, chain ...Emailer) *ChainEmailer {
	return &ChainEmailer{log: log, chain: chain}
}

func (e *ChainEmailer) SendLoginCode(to, code string) error {
	var errs []error
	for _, em := range e.chain {
		if err := em.SendLoginCode(to, code); err == nil {
			return nil
		} else {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return errors.New("no emailers configured")
	}
	return errors.Join(errs...)
}

func buildMessage(from, to, subject, body string) []byte {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return []byte(sb.String())
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
