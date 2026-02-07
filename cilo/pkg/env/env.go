package env

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sharedco/cilo/pkg/models"
)

type RenderContext struct {
	Project   string
	Env       string
	DNSSuffix string
}

// ApplyConfig executes env handling for a workspace: optional pruning + render rules.
// It never logs secret values; only paths and counts.
func ApplyConfig(workspace string, config *models.ProjectConfig, ctx RenderContext) error {
	if config == nil || config.Env == nil {
		return nil
	}

	if err := RunInitHook(workspace, config.Env.InitHook); err != nil {
		return err
	}

	if err := applyCopyPolicy(workspace, config.Env, config); err != nil {
		return err
	}

	if len(config.Env.Render) == 0 {
		return nil
	}

	for _, rule := range config.Env.Render {
		if rule.File == "" {
			continue
		}
		path := rule.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspace, path)
		}
		if err := renderFile(path, rule, ctx); err != nil {
			return err
		}
	}

	return nil
}

// RunInitHook executes a shell command in the workspace if configured.
// This can be used to pull secrets or generate env files before rendering.
func RunInitHook(workspace string, hook string) error {
	hook = strings.TrimSpace(hook)
	if hook == "" {
		return nil
	}

	cmd := exec.Command("bash", "-lc", hook)
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func applyCopyPolicy(workspace string, envCfg *models.EnvConfig, projectCfg *models.ProjectConfig) error {
	if envCfg == nil {
		return nil
	}

	mode := strings.ToLower(strings.TrimSpace(envCfg.CopyMode))
	if mode == "" {
		mode = "all"
	}

	if mode == "all" && len(envCfg.Ignore) == 0 && len(envCfg.Copy) == 0 {
		return nil
	}

	return filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".cilo" {
				return filepath.SkipDir
			}
			return nil
		}

		base := filepath.Base(path)
		if !strings.HasPrefix(base, ".env") {
			return nil
		}

		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}

		if shouldRemoveEnvFile(rel, base, envCfg, mode) {
			return os.Remove(path)
		}

		return nil
	})
}

func shouldRemoveEnvFile(rel, base string, envCfg *models.EnvConfig, mode string) bool {
	if envCfg == nil {
		return false
	}

	for _, pattern := range envCfg.Ignore {
		if matchPattern(pattern, rel, base) {
			return true
		}
	}

	if mode == "none" {
		return true
	}

	if mode == "allowlist" {
		for _, pattern := range envCfg.Copy {
			if matchPattern(pattern, rel, base) {
				return false
			}
		}
		return true
	}

	return false
}

func matchPattern(pattern, rel, base string) bool {
	if pattern == "" {
		return false
	}
	if ok, _ := filepath.Match(pattern, rel); ok {
		return true
	}
	if ok, _ := filepath.Match(pattern, base); ok {
		return true
	}
	return false
}

func renderFile(path string, rule models.EnvRenderRule, ctx RenderContext) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read env file %s: %w", path, err)
	}

	content := string(data)
	for _, rep := range rule.Replace {
		from := expandTokens(rep.From, ctx)
		to := expandTokens(rep.To, ctx)
		if from == "" {
			continue
		}
		content = strings.ReplaceAll(content, from, to)
	}

	if rule.Tokens {
		content = expandTokens(content, ctx)
	}

	return os.WriteFile(path, []byte(content), 0644)
}

func expandTokens(value string, ctx RenderContext) string {
	dnsSuffix := ctx.DNSSuffix
	if dnsSuffix == "" {
		dnsSuffix = ".test"
	}
	baseURL := fmt.Sprintf("http://%s.%s%s", ctx.Project, ctx.Env, dnsSuffix)

	value = strings.ReplaceAll(value, "${CILO_PROJECT}", ctx.Project)
	value = strings.ReplaceAll(value, "${CILO_ENV}", ctx.Env)
	value = strings.ReplaceAll(value, "${CILO_DNS_SUFFIX}", dnsSuffix)
	value = strings.ReplaceAll(value, "${CILO_BASE_URL}", baseURL)
	return value
}
