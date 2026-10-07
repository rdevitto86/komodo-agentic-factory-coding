// Package claude is the Claude Code mount: the only place this host's names appear.
package claude

const (
	// settingSourcesFlag excludes personal layer.
	settingSourcesFlag = "local"

	// strictMCPConfigFlag restricts MCP to project and role definitions only.
	strictMCPConfigFlag = "--strict-mcp-config"

	// claudeConfigDirEnv is removed from session environment to exclude personal config.
	claudeConfigDirEnv = "CLAUDE_CONFIG_DIR"
)
