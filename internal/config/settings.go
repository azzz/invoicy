package config

import "os"

// Workspace subdirectories. Their names are fixed.
const (
	ProfilesDir   = "profiles"
	InvoicesDir   = "invoices"
	OutputDir     = "output"
	SignaturesDir = "signatures"
)

// Settings holds defaults taken from the environment.
type Settings struct {
	DefaultProfile string // INVOICY_DEFAULT_PROFILE
	WorkspaceDir   string // INVOICY_WORKSPACE_DIR
}

// SettingsFromEnv reads INVOICY_DEFAULT_PROFILE and INVOICY_WORKSPACE_DIR.
func SettingsFromEnv() (Settings, error) {
	s := Settings{DefaultProfile: os.Getenv("INVOICY_DEFAULT_PROFILE"), WorkspaceDir: os.Getenv("INVOICY_WORKSPACE_DIR")}
	for _, f := range []struct{ env, value string }{
		{"INVOICY_DEFAULT_PROFILE", s.DefaultProfile}, {"INVOICY_WORKSPACE_DIR", s.WorkspaceDir},
	} {
		if err := Text(f.env, f.value); err != nil {
			return Settings{}, err
		}
	}
	return s, nil
}
