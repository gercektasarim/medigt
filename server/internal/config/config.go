package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	AppEnv          string
	Port            string
	DatabaseURL     string
	RedisURL        string
	JWTSecret       string
	AllowedOrigins  []string
	FrontendOrigin  string
	ResendAPIKey    string
	ResendFromEmail string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string

	S3Bucket    string
	S3Region    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string

	LocalUploadDir     string
	LocalUploadBaseURL string

	AllowSignup         bool
	AllowedEmailDomains []string
	AllowedEmails       []string

	MedulaBaseURL        string
	MedulaUsername       string
	MedulaPassword       string
	MedulaFacilityCode   string
	MedulaDoctorCode     string
	MedulaPollInterval   time.Duration
	MedulaMaxRetries     int
	MernisEndpoint       string
	MernisCacheTTL       time.Duration
	AuditRetentionDays   int
	FieldEncryptionKey   string

	// e-Reçete (Sağlık Bakanlığı) — empty = mock client.
	EreceteBaseURL      string
	EreceteClientID     string
	EreceteClientSecret string

	// İTS (İlaç Takip Sistemi) — empty = mock client.
	ITSBaseURL      string
	ITSClientID     string
	ITSClientSecret string

	// HL7 ADT outbound peer. Empty = MockDispatcher (in-process log +
	// synthetic AA ack). When set ("host:port") an MLLPDispatcher is
	// wired and ADT messages go over TCP MLLP. Real peers: PACS, LIS,
	// regional HIE.
	HL7ADTPeerAddress string
	HL7ADTPeerTimeout time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:              os.Getenv("APP_ENV"),
		Port:                envOr("PORT", "8088"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		RedisURL:            os.Getenv("REDIS_URL"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		AllowedOrigins:      splitCSV(os.Getenv("ALLOWED_ORIGINS")),
		FrontendOrigin:      envOr("FRONTEND_ORIGIN", "http://localhost:3008"),
		ResendAPIKey:        os.Getenv("RESEND_API_KEY"),
		ResendFromEmail:     envOr("RESEND_FROM_EMAIL", "noreply@medigt.local"),
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:   os.Getenv("GOOGLE_REDIRECT_URI"),
		S3Bucket:            os.Getenv("S3_BUCKET"),
		S3Region:            envOr("S3_REGION", "eu-central-1"),
		S3Endpoint:          os.Getenv("S3_ENDPOINT"),
		S3AccessKey:         os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:         os.Getenv("S3_SECRET_KEY"),
		LocalUploadDir:      envOr("LOCAL_UPLOAD_DIR", "./data/uploads"),
		LocalUploadBaseURL:  envOr("LOCAL_UPLOAD_BASE_URL", "http://localhost:8088"),
		AllowSignup:         envOr("ALLOW_SIGNUP", "true") == "true",
		AllowedEmailDomains: splitCSV(os.Getenv("ALLOWED_EMAIL_DOMAINS")),
		AllowedEmails:       splitCSV(os.Getenv("ALLOWED_EMAILS")),
		MedulaBaseURL:       envOr("MEDULA_BASE_URL", "https://medulatest.sgk.gov.tr"),
		MedulaUsername:      os.Getenv("MEDULA_USERNAME"),
		MedulaPassword:      os.Getenv("MEDULA_PASSWORD"),
		MedulaFacilityCode:  os.Getenv("MEDULA_FACILITY_CODE"),
		MedulaDoctorCode:    os.Getenv("MEDULA_DOCTOR_CODE"),
		MedulaPollInterval:  envDuration("MEDULA_OUTBOX_POLL_INTERVAL", 5*time.Second),
		MedulaMaxRetries:    envInt("MEDULA_OUTBOX_MAX_RETRIES", 5),
		MernisEndpoint:      envOr("MERNIS_ENDPOINT", "https://tckimlik.nvi.gov.tr/Service/KPSPublicV2.asmx"),
		MernisCacheTTL:      envDuration("MERNIS_CACHE_TTL", 720*time.Hour),
		AuditRetentionDays:  envInt("AUDIT_RETENTION_DAYS", 3650),
		FieldEncryptionKey:  os.Getenv("FIELD_ENCRYPTION_KEY"),
		EreceteBaseURL:      os.Getenv("ERECETE_BASE_URL"),
		EreceteClientID:     os.Getenv("ERECETE_CLIENT_ID"),
		EreceteClientSecret: os.Getenv("ERECETE_CLIENT_SECRET"),
		ITSBaseURL:          os.Getenv("ITS_BASE_URL"),
		ITSClientID:         os.Getenv("ITS_CLIENT_ID"),
		ITSClientSecret:     os.Getenv("ITS_CLIENT_SECRET"),
		HL7ADTPeerAddress:   os.Getenv("HL7_ADT_PEER_ADDRESS"),
		HL7ADTPeerTimeout:   envDuration("HL7_ADT_PEER_TIMEOUT", 15*time.Second),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		if cfg.IsProduction() {
			return nil, errors.New("JWT_SECRET must be set in production")
		}
	}

	return cfg, nil
}

func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
