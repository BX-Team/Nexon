// Package config loads Nexon runtime configuration from environment variables and a .env file.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Config holds process-level settings; runtime-editable settings live in the DB.
type Config struct {
	// DataDir holds the SQLite database and any state files.
	DataDir string
	// DBPath is the SQLite file path (derived from DataDir unless overridden).
	DBPath string

	// SubListen is the subscription HTTP server address, e.g. ":8080".
	SubListen string
	// SubBaseURL is the externally reachable base for building sub links.
	SubBaseURL string

	// TrafficPollInterval in seconds for the StatsService poller.
	TrafficPollInterval int
}

// Default returns a Config with sensible defaults, overridden by NEXON_* env vars.
func Default() Config {
	loadDotEnv(".env")
	dataDir := envOr("NEXON_DATA_DIR", defaultDataDir())
	// CLI/TUI don't inherit the systemd service env; the data-dir .env lets them read the same NEXON_* config.
	loadDotEnv(filepath.Join(dataDir, ".env"))
	cfg := Config{
		DataDir:             dataDir,
		DBPath:              envOr("NEXON_DB", filepath.Join(dataDir, "nexon.db")),
		SubListen:           envOr("NEXON_SUB_LISTEN", ":8080"),
		SubBaseURL:          envOr("NEXON_SUB_BASE_URL", "http://localhost:8080"),
		TrafficPollInterval: envIntOr("NEXON_POLL_INTERVAL", 30),
	}
	return cfg
}

// loadDotEnv loads KEY=VALUE pairs from a .env file; existing env vars always win.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip optional surrounding quotes.
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, val)
			}
		}
	}
}

func defaultDataDir() string {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("ProgramData"); d != "" {
			return filepath.Join(d, "Nexon")
		}
		return "nexon-data"
	}
	return "/var/lib/nexon"
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
