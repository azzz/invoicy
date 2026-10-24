package config

import "testing"

func TestSettingsFromEnv(t *testing.T) {
	t.Setenv("INVOICY_DEFAULT_PROFILE", "")
	t.Setenv("INVOICY_WORKSPACE_DIR", "")
	s, err := SettingsFromEnv()
	if err != nil || s != (Settings{}) {
		t.Fatalf("defaults: %+v, %v", s, err)
	}
	t.Setenv("INVOICY_DEFAULT_PROFILE", "mario")
	t.Setenv("INVOICY_WORKSPACE_DIR", "/srv/invoices")
	s, err = SettingsFromEnv()
	if err != nil || s != (Settings{DefaultProfile: "mario", WorkspaceDir: "/srv/invoices"}) {
		t.Fatalf("got %+v, %v", s, err)
	}
	t.Setenv("INVOICY_WORKSPACE_DIR", "bad\x07")
	if _, err = SettingsFromEnv(); err == nil {
		t.Fatal("control character accepted")
	}
}
