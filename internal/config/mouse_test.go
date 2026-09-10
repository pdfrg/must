package config

import "testing"

func TestMouseSupportDisabledByDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MouseEnabled || cfg.MouseFocusOnHover {
		t.Fatalf("mouse defaults = enabled:%v focus-on-hover:%v, want false/false", cfg.MouseEnabled, cfg.MouseFocusOnHover)
	}
}
