package its

import (
	"log/slog"
	"strings"
)

// NewFromConfig wires the real İTS HTTP client when credentials are
// present, otherwise returns a MockClient. Mirrors erecete.NewFromConfig
// so main.go has a uniform shape across integrations.
func NewFromConfig(cfg HTTPConfig) Client {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	if strings.TrimSpace(cfg.BaseURL) == "" ||
		strings.TrimSpace(cfg.ClientID) == "" ||
		strings.TrimSpace(cfg.ClientSecret) == "" {
		log.Info("İTS: mock client (credentials missing)")
		return NewMockClient()
	}
	real, err := NewHTTPClient(cfg)
	if err != nil {
		log.Warn("İTS: real client construction failed, falling back to mock", "err", err)
		return NewMockClient()
	}
	log.Info("İTS: real HTTP client wired", "baseURL", cfg.BaseURL)
	return real
}
