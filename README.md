# koko 🦍💻

![GitHub License](https://img.shields.io/github/license/original-flipster69/seifenkehrer)
![Go](https://img.shields.io/badge/-Go-00ADD8?logo=go&logoColor=00add8&labelColor=grey&style=flat)
![original.flipster](https://img.shields.io/badge/-original.flipster-grey?logo=data%3Aimage%2Fgif%3Bbase64%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&logoSize=auto&labelColor=grey&color=292524)

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
