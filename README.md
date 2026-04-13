# koko

A secure, sandboxed coding assistant for the terminal. koko connects to LLM providers and gives them controlled access to your filesystem, shell, and git — all within strict security boundaries.

## Features

### Multi-Provider LLM Support

koko supports three LLM backends:

- **Anthropic** — Claude models via the Anthropic API
- **Mistral** — Mistral models via the Mistral API (or any compatible endpoint)
- **Ollama** — Local models with no API key required (default: `http://localhost:11434`)

Switch providers and models at runtime with `/model` or configure them in `~/.koko/config.json`.

### Sandbox

The sandbox is the core security boundary in koko. Every file operation the agent performs is validated against the sandbox before execution.

The sandbox enforces:

- **Directory allowlist** — The agent can only read, write, or list files within explicitly allowed directories. By default this is the current working directory. Any path that resolves outside the allowlist is rejected.
- **Symlink resolution** — Paths are resolved through symlinks before validation, preventing symlink-based escapes.
- **Denied file patterns** — Sensitive files are blocked by glob pattern regardless of directory. The defaults are:
  - `.env`, `.env.*`
  - `*.pem`, `*.key`, `id_rsa*`
  - `credentials.json`
  - `*.secret`, `*.password`
- **File size limits** — Reads and writes are capped at a configurable maximum (default: 1 MB).

Use `-sandbox /path/to/project` to set the sandbox root, or configure `sandbox_root` and `allowed_dirs` in the config file.

### Agent Tools

The agent has 14 tools available, all mediated through the sandbox:

**File Operations**
| Tool | Description |
|------|-------------|
| `read_file` | Read file contents with optional line offset and limit |
| `write_file` | Create or overwrite a file |
| `replace_in_file` | Find-and-replace requiring a unique match |
| `delete_file` | Remove a file (supports undo) |
| `rename_file` | Move or rename a file |
| `list_dir` | List directory contents with recursive tree view |
| `search_files` | Regex search across files with context lines |

**Shell**
| Tool | Description |
|------|-------------|
| `exec_command` | Run a shell command (requires user confirmation) |

**Git**
| Tool | Description |
|------|-------------|
| `git_status` | Show modified and untracked files |
| `git_diff` | Show staged or unstaged diffs |
| `git_log` | Show recent commit history |
| `git_commit` | Stage and commit changes (requires user confirmation) |
| `git_branch` | List, create, or checkout branches |

Destructive operations (`exec_command`, `git_commit`) always prompt for user approval before running.

### Project Detection

koko automatically detects the languages, frameworks, and build tools in your project by scanning for marker files (`go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`, etc.). This context is passed to the LLM to improve the relevance of its responses.

### Audit Logging

Every tool invocation is recorded to `~/.koko/audit.jsonl` with a UTC timestamp, tool name, arguments, and result. The audit log is append-only and thread-safe.

### Session Management

Conversation history can be saved to disk and resumed later:

- `/save` — Write the current session to `~/.koko/session.json`
- `/resume` — Load a previously saved session
- `/compact` — Summarize history to reclaim context window space

### Interactive REPL

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/clear` | Reset conversation history |
| `/history` | Show message count |
| `/undo` | Revert the last file change |
| `/run <cmd>` | Run a shell command directly |
| `/diff` | Show uncommitted git changes |
| `/tokens` | Display token usage |
| `/compact` | Compress history |
| `/model [name]` | Show or switch the active model |
| `/save` | Save session to disk |
| `/resume` | Restore a saved session |
| `"""` | Start/end multi-line input |

### Non-Interactive Mode

Pass `-prompt` to run a single query and exit:

```
koko -prompt "Add error handling to main.go"
```

## Installation

```
go install github.com/meeseeks/koko@latest
```

## Usage

```
koko [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `-provider` | LLM provider: `anthropic`, `mistral`, `ollama` | `mistral` |
| `-model` | Model name | `mistral-large-latest` |
| `-base-url` | API base URL (for local or custom endpoints) | Provider default |
| `-sandbox` | Sandbox root directory | Current working directory |
| `-config` | Config file path | `~/.koko/config.json` |
| `-prompt` | Single prompt for non-interactive mode | |

## Configuration

koko reads configuration from `~/.koko/config.json`:

```json
{
  "provider": "anthropic",
  "model": "claude-sonnet-4-20250514",
  "base_url": "",
  "max_tokens": 16384,
  "sandbox_root": "/home/user/projects",
  "allowed_dirs": ["/home/user/projects"],
  "deny_files": [".env", ".env.*", "*.pem", "*.key", "id_rsa*", "credentials.json", "*.secret", "*.password"],
  "max_file_size": 1048576
}
```

API keys are read from environment variables:

- `ANTHROPIC_API_KEY` for Anthropic
- `MISTRAL_API_KEY` for Mistral
- Ollama requires no key

## Data Directory

koko stores runtime data in `~/.koko/`:

| File | Purpose |
|------|---------|
| `config.json` | User configuration |
| `audit.jsonl` | Tool invocation audit log |
| `koko.log` | Application log (JSON format) |
| `session.json` | Saved conversation history |
