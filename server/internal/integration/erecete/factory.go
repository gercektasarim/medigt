package erecete

import (
	"log/slog"
	"strings"
)

// NewFromConfig is the single entry point for wiring an e-Reçete client.
// Returns a real HTTPClient if all credentials are present; otherwise
// falls back to the MockClient so dev + pilot environments work without
// Bakanlık paperwork. Logs the chosen mode so ops can grep `journalctl`
// to confirm a production deployment didn't silently downgrade to mock.
func NewFromConfig(cfg HTTPConfig) Client {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	if strings.TrimSpace(cfg.BaseURL) == "" ||
		strings.TrimSpace(cfg.ClientID) == "" ||
		strings.TrimSpace(cfg.ClientSecret) == "" {
		log.Info("e-reçete: mock client (credentials missing)")
		return NewMockClient()
	}
	real, err := NewHTTPClient(cfg)
	if err != nil {
		log.Warn("e-reçete: real client construction failed, falling back to mock", "err", err)
		return NewMockClient()
	}
	log.Info("e-reçete: real HTTP client wired", "baseURL", cfg.BaseURL)
	return real
}
