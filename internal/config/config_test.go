package config

import (
	"log/slog"
	"testing"
)

func TestParse(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")
	cfg, err := Parse()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("want: %s\n got:%s", slog.LevelDebug, cfg.LogLevel)
	}
}
