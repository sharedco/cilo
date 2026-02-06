package env

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cilo/cilo/pkg/models"
)

func TestApplyConfig_RenderTokensAndReplace(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "envs", "app", ".env")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "BASE_URL=http://pc.localhost\nNAME=${CILO_PROJECT}-${CILO_ENV}\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	config := &models.ProjectConfig{
		Env: &models.EnvConfig{
			CopyMode: "all",
			Render: []models.EnvRenderRule{
				{
					File:   "envs/app/.env",
					Tokens: true,
					Replace: []models.EnvReplace{
						{From: "pc.localhost", To: "pc.${CILO_ENV}.test"},
					},
				},
			},
		},
	}

	ctx := RenderContext{Project: "proj", Env: "dev", DNSSuffix: ".test"}
	if err := ApplyConfig(workspace, config, ctx); err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "http://pc.dev.test") {
		t.Fatalf("expected replaced URL, got %q", text)
	}
	if !strings.Contains(text, "NAME=proj-dev") {
		t.Fatalf("expected token expansion, got %q", text)
	}
}

func TestApplyConfig_CopyModeAllowlist(t *testing.T) {
	workspace := t.TempDir()
	keep := filepath.Join(workspace, ".env")
	remove := filepath.Join(workspace, ".env.local")
	if err := os.WriteFile(keep, []byte("A=1\n"), 0644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(remove, []byte("B=2\n"), 0644); err != nil {
		t.Fatalf("write remove: %v", err)
	}

	config := &models.ProjectConfig{
		Env: &models.EnvConfig{
			CopyMode: "allowlist",
			Copy:     []string{".env"},
		},
	}

	if err := ApplyConfig(workspace, config, RenderContext{}); err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("expected keep file: %v", err)
	}
	if _, err := os.Stat(remove); !os.IsNotExist(err) {
		t.Fatalf("expected remove file to be deleted, err=%v", err)
	}
}

func TestApplyConfig_CopyModeNone(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, ".env")
	if err := os.WriteFile(filePath, []byte("A=1\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	config := &models.ProjectConfig{
		Env: &models.EnvConfig{
			CopyMode: "none",
		},
	}

	if err := ApplyConfig(workspace, config, RenderContext{}); err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("expected env file deleted, err=%v", err)
	}
}
