package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type MCPSetupResult struct {
	ConfigPath string
	Agent      string
	Command    string
}

func SetupMCP(agentType string) (*MCPSetupResult, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	switch strings.ToLower(agentType) {
	case "claude", "claude-code", "claudecode":
		return setupClaudeCodeMCP(exePath)
	case "opencode", "open-code":
		return setupOpenCodeMCP(exePath)
	case "all":
		result, err := setupClaudeCodeMCP(exePath)
		if err != nil {
			return nil, err
		}
		if _, err := setupOpenCodeMCP(exePath); err != nil {
			return nil, err
		}
		result.Agent = "claude-code, opencode"
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported agent type: %s (supported: claude-code, opencode, all)", agentType)
	}
}

func setupClaudeCodeMCP(exePath string) (*MCPSetupResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".claude")
	configFile := filepath.Join(configDir, "claude_desktop_config.json")

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	config := fmt.Sprintf(`{
  "mcpServers": {
    "llm-box": {
      "command": "%s",
      "args": ["--mcp-server"]
    }
  }
}`, escapeJSONString(exePath))

	existing, err := os.ReadFile(configFile)
	if err == nil && len(existing) > 0 {
		merged, mergeErr := mergeMCPConfigSafe(existing, exePath)
		if mergeErr == nil {
			config = merged
		}
	}

	if err := os.WriteFile(configFile, []byte(config), 0600); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return &MCPSetupResult{
		ConfigPath: configFile,
		Agent:      "Claude Code",
		Command:    exePath + " --mcp-server",
	}, nil
}

func setupOpenCodeMCP(exePath string) (*MCPSetupResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	var configDir string
	switch runtime.GOOS {
	case "darwin":
		configDir = filepath.Join(home, "Library", "Application Support", "opencode", "User")
	case "windows":
		configDir = filepath.Join(os.Getenv("APPDATA"), "opencode", "User")
	default:
		configDir = filepath.Join(home, ".config", "opencode", "User")
	}

	configFile := filepath.Join(configDir, "mcp.json")

	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	config := fmt.Sprintf(`{
  "mcpServers": {
    "llm-box": {
      "command": "%s",
      "args": ["--mcp-server"]
    }
  }
}`, escapeJSONString(exePath))

	existing, err := os.ReadFile(configFile)
	if err == nil && len(existing) > 0 {
		merged, mergeErr := mergeMCPConfigSafe(existing, exePath)
		if mergeErr == nil {
			config = merged
		}
	}

	if err := os.WriteFile(configFile, []byte(config), 0600); err != nil {
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	return &MCPSetupResult{
		ConfigPath: configFile,
		Agent:      "OpenCode",
		Command:    exePath + " --mcp-server",
	}, nil
}

func escapeJSONString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

type mcpConfig struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func mergeMCPConfigSafe(existing []byte, exePath string) (string, error) {
	var cfg mcpConfig
	if err := json.Unmarshal(existing, &cfg); err != nil {
		return "", fmt.Errorf("failed to parse existing config: %w", err)
	}

	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]mcpServerEntry)
	}

	if _, exists := cfg.MCPServers["llm-box"]; !exists {
		cfg.MCPServers["llm-box"] = mcpServerEntry{
			Command: exePath,
			Args:    []string{"--mcp-server"},
		}
	}

	result, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(result), nil
}

type SkillInstallResult struct {
	Agent     string
	SkillPath string
	Installed bool
}

func InstallSkills(agentType string) (*SkillInstallResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	skillsSource := filepath.Join(cwd, "skills")
	if _, err := os.Stat(skillsSource); os.IsNotExist(err) {
		skillsSource = filepath.Join(cwd, "templates")
	}

	switch strings.ToLower(agentType) {
	case "claude", "claude-code", "claudecode":
		return installClaudeCodeSkills(home, skillsSource)
	case "opencode", "open-code":
		return installOpenCodeSkills(home, skillsSource)
	case "all":
		result, err := installClaudeCodeSkills(home, skillsSource)
		if err != nil {
			return nil, err
		}
		if _, err := installOpenCodeSkills(home, skillsSource); err != nil {
			return nil, err
		}
		result.Agent = "claude-code, opencode"
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported agent type: %s (supported: claude-code, opencode, all)", agentType)
	}
}

func installClaudeCodeSkills(home, source string) (*SkillInstallResult, error) {
	skillsDir := filepath.Join(home, ".claude", "skills")
	return installSkillsToDir(skillsDir, source, "Claude Code")
}

func installOpenCodeSkills(home, source string) (*SkillInstallResult, error) {
	var configDir string
	switch runtime.GOOS {
	case "darwin":
		configDir = filepath.Join(home, "Library", "Application Support", "opencode", "User")
	case "windows":
		configDir = filepath.Join(os.Getenv("APPDATA"), "opencode", "User")
	default:
		configDir = filepath.Join(home, ".config", "opencode", "User")
	}
	skillsDir := filepath.Join(configDir, "skills")
	return installSkillsToDir(skillsDir, source, "OpenCode")
}

func installSkillsToDir(skillsDir, source, agentName string) (*SkillInstallResult, error) {
	if err := os.MkdirAll(skillsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create skills directory: %w", err)
	}

	if _, err := os.Stat(source); os.IsNotExist(err) {
		return &SkillInstallResult{
			Agent:     agentName,
			SkillPath: skillsDir,
			Installed: false,
		}, nil
	}

	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory: %w", err)
	}

	installed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			srcPath := filepath.Join(source, entry.Name())
			dstPath := filepath.Join(skillsDir, "llm-box-"+entry.Name())

			if err := copyDir(srcPath, dstPath); err != nil {
				continue
			}
			installed++
		}
	}

	return &SkillInstallResult{
		Agent:     agentName,
		SkillPath: skillsDir,
		Installed: installed > 0,
	}, nil
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

func GetBinaryPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return exePath, nil
}

func RunCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
