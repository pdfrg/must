package config

import "testing"

func TestMouseSupportEnabledByDefault(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.MouseEnabled || !cfg.MouseFocusOnHover {
		t.Fatalf("mouse defaults = enabled:%v focus-on-hover:%v, want true/true", cfg.MouseEnabled, cfg.MouseFocusOnHover)
	}
}
