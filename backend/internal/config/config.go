package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds all runtime configuration for IT Operations Hub.
// Local-first defaults: everything lives under ./data relative to the executable.
type Config struct {
	Port           string
	DataDir        string
	DBPath         string
	UploadsDir     string
	BackupsDir     string
	LogsDir        string
	SessionSecret  string
	SessionTTLHrs  int
	Environment    string
	SchedulerEvery int // seconds between scheduler ticks
	AllowedOrigins []string
}

func Load() *Config {
	dataDir := getEnv("ITOPS_DATA_DIR", "./data")

	cfg := &Config{
		Port:           getEnv("ITOPS_PORT", "8080"),
		DataDir:        dataDir,
		DBPath:         filepath.Join(dataDir, "ticketing.db"),
		UploadsDir:     filepath.Join(dataDir, "uploads"),
		BackupsDir:     filepath.Join(dataDir, "backups"),
		LogsDir:        filepath.Join(dataDir, "logs"),
		SessionSecret:  getEnv("ITOPS_SESSION_SECRET", generateFallbackSecret()),
		SessionTTLHrs:  24 * 7, // 7 days
		Environment:    getEnv("ITOPS_ENV", "production"),
		SchedulerEvery: 60, // 1 minute, per spec section 32
		AllowedOrigins: getOriginsEnv("ITOPS_ALLOWED_ORIGINS"),
	}

	return cfg
}

// getOriginsEnv reads a comma-separated list of extra allowed CORS origins
// (e.g. "http://192.168.1.42:3000") on top of the always-allowed localhost
// origins, so the app can be reached from another device on the same LAN
// (a phone, another desk) without hardcoding an IP into the binary.
func getOriginsEnv(key string) []string {
	base := []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	v := os.Getenv(key)
	if v == "" {
		return base
	}
	for _, o := range strings.Split(v, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			base = append(base, o)
		}
	}
	return base
}

// EnsureDirs creates all required data directories so the app never requires
// manual setup on first run (spec section 58).
func (c *Config) EnsureDirs() error {
	dirs := []string{c.DataDir, c.UploadsDir, c.BackupsDir, c.LogsDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// generateFallbackSecret creates a random-ish secret when none is configured.
// This is only used for local-first single-user deployments; a real
// deployment should set ITOPS_SESSION_SECRET explicitly.
func generateFallbackSecret() string {
	// Deterministic per-machine fallback avoids invalidating sessions on
	// every restart if the operator hasn't set an env var. Combined with
	// httpOnly, secure-when-possible cookies this is acceptable for the
	// local-first use case described in the spec.
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "itops-local-fallback-secret-please-set-ITOPS_SESSION_SECRET"
	}
	return "itops-hub-" + host + "-v1"
}
