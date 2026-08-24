[![Go Report Card](https://goreportcard.com/badge/github.com/github/github-mcp-server)](https://goreportcard.com/report/github.com/github/github-mcp-server)

# GitHub MCP Server

The GitHub MCP Server connects AI tools directly to GitHub's platform. This gives AI agents, assistants, and chatbots the ability to read repositories and code files, manage issues and PRs, analyze code, and automate workflows. All through natural language interactions.

### Use Cases

- Repository Management: Browse and query code, search files, analyze commits, and understand project structure across any repository you have access to.
- Issue & PR Automation: Create, update, and manage issues and pull requests. Let AI help triage bugs, review code changes, and maintain project boards.
- CI/CD & Workflow Intelligence: Monitor GitHub Actions workflow runs, analyze build failures, manage releases, and get insights into your development pipeline.
- Code Analysis: Examine security findings, review Dependabot alerts, understand code patterns, and get comprehensive insights into your codebase.
- Team Collaboration: Access discussions, manage notifications, analyze team activity, and streamline processes for your team.

Built for developers who want to connect their AI tools to GitHub context and capabilities, from simple natural language queries to complex multi-step agent workflows.

---

## Remote GitHub MCP Server

[![Install in VS Code](https://img.shields.io/badge/VS_Code-Install_Server-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D) [![Install in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install_Server-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22type%22%3A%20%22http%22%2C%22url%22%3A%20%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D&quality=insiders) [![Install in Visual Studio](https://img.shields.io/badge/Visual_Studio-Install_Server-C16FDE?style=flat-square&logo=visualstudio&logoColor=white)](https://aka.ms/vs/mcp-install?%7B%22name%22%3A%22github%22%2C%22gallery%22%3Atrue%2C%22url%22%3A%22https%3A%2F%2Fapi.githubcopilot.com%2Fmcp%2F%22%7D)

The remote GitHub MCP Server is hosted by GitHub and provides the easiest method for getting up and running. If your MCP host does not support remote MCP servers, don't worry! You can use the [local version of the GitHub MCP Server](https://github.com/github/github-mcp-server?tab=readme-ov-file#local-github-mcp-server) instead.

### Prerequisites

1. A compatible MCP host with remote server support (VS Code 1.101+, Claude Desktop, Cursor, Windsurf, etc.)
2. Any applicable [policies enabled](https://github.com/github/github-mcp-server/blob/main/docs/policies-and-governance.md)

### Install in VS Code

For quick installation, use one of the one-click install buttons above. Once you complete that flow, toggle Agent mode (located by the Copilot Chat text input) and the server will start. Make sure you're using [VS Code 1.101](https://code.visualstudio.com/updates/v1_101) or [later](https://code.visualstudio.com/updates) for remote MCP and OAuth support.

Alternatively, to manually configure VS Code, choose the appropriate JSON block from the examples below and add it to your host configuration:

<table>
<tr><th>Using OAuth</th><th>Using a GitHub PAT</th></tr>
<tr><th align=left colspan=2>VS Code (version 1.101 or greater)</th></tr>
<tr valign=top>
<td>

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/"
    }
  }
}
```

</td>
<td>

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer ${input:github_mcp_pat}"
      }
    }
  },
  "inputs": [
    {
      "type": "promptString",
      "id": "github_mcp_pat",
      "description": "GitHub Personal Access Token",
      "password": true
    }
  ]
}
```

</td>
</tr>
</table>

### Install in other MCP hosts

- **[Copilot CLI](/docs/installation-guides/install-copilot-cli.md)** - Installation guide for GitHub Copilot CLI
- **[GitHub Copilot in other IDEs](/docs/installation-guides/install-other-copilot-ides.md)** - Installation for JetBrains, Visual Studio, Eclipse, and Xcode with GitHub Copilot
- **[Claude Applications](/docs/installation-guides/install-claude.md)** - Installation guide for Claude Desktop and Claude Code CLI
- **[Codex](/docs/installation-guides/install-codex.md)** - Installation guide for OpenAI Codex
- **[Cursor](/docs/installation-guides/install-cursor.md)** - Installation guide for Cursor IDE
- **[OpenCode](/docs/installation-guides/install-opencode.md)** - Installation guide for the OpenCode terminal agent
- **[Windsurf](/docs/installation-guides/install-windsurf.md)** - Installation guide for Windsurf IDE
- **[Zed](/docs/installation-guides/install-zed.md)** - Installation guide for Zed editor
- **[Rovo Dev CLI](/docs/installation-guides/install-rovo-dev-cli.md)** - Installation guide for Rovo Dev CLI

> **Note:** Each MCP host application needs to configure a GitHub App or OAuth App to support remote access via OAuth. Any host application that supports remote MCP servers should support the remote GitHub server with PAT authentication. Configuration details and support levels vary by host. Make sure to refer to the host application's documentation for more info.

### Configuration

#### Toolset configuration

See [Remote Server Documentation](docs/remote-server.md) for full details on remote server configuration, toolsets, headers, and advanced usage. This file provides comprehensive instructions and examples for connecting, customizing, and installing the remote GitHub MCP Server in VS Code and other MCP hosts.

When no toolsets are specified, [default toolsets](#default-toolset) are used.

#### Insiders Mode

> **Try new features early!** The remote server offers an insiders version with early access to new features and experimental tools.

<table>
<tr><th>Using URL Path</th><th>Using Header</th></tr>
<tr valign=top>
<td>

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/insiders"
    }
  }
}
```

</td>
<td>

```json
{
  "servers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "X-MCP-Insiders": "true"
      }
    }
  }
}
```

</td>
</tr>
</table>

See [Remote Server Documentation](docs/remote-server.md#insiders-mode) for more details and examples, and [Insiders Features](docs/insiders-features.md) for a full list of what's available.

#### GitHub Enterprise

##### GitHub Enterprise Cloud with data residency (ghe.com)

GitHub Enterprise Cloud can also make use of the remote server.

Example for `https://octocorp.ghe.com` with GitHub PAT token:

```
{
    ...
    "github-octocorp": {
      "type": "http",
      "url": "https://copilot-api.octocorp.ghe.com/mcp",
      "headers": {
        "Authorization": "Bearer ${input:github_mcp_pat}"
      }
    },
    ...
}
```

> **Note:** When using OAuth with GitHub Enterprise with VS Code and GitHub Copilot, you also need to configure your VS Code settings to point to your GitHub Enterprise instance - see [Authenticate from VS Code](https://docs.github.com/en/enterprise-cloud@latest/copilot/how-tos/configure-personal-settings/authenticate-to-ghecom)

##### GitHub Enterprise Server

GitHub Enterprise Server does not support remote server hosting. Please refer to [GitHub Enterprise Server and Enterprise Cloud with data residency (ghe.com)](#github-enterprise-server-and-enterprise-cloud-with-data-residency-ghecom) from the local server configuration.

---

## Local GitHub MCP Server

[![Install with Docker in VS Code](https://img.shields.io/badge/VS_Code-Install_Server-0098FF?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-p%22%2C%22127.0.0.1%3A8085%3A8085%22%2C%22-e%22%2C%22GITHUB_OAUTH_CALLBACK_PORT%22%2C%22ghcr.io%2Fgithub%2Fgithub-mcp-server%22%5D%2C%22env%22%3A%7B%22GITHUB_OAUTH_CALLBACK_PORT%22%3A%228085%22%7D%7D) [![Install with Docker in VS Code Insiders](https://img.shields.io/badge/VS_Code_Insiders-Install_Server-24bfa5?style=flat-square&logo=visualstudiocode&logoColor=white)](https://insiders.vscode.dev/redirect/mcp/install?name=github&config=%7B%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-p%22%2C%22127.0.0.1%3A8085%3A8085%22%2C%22-e%22%2C%22GITHUB_OAUTH_CALLBACK_PORT%22%2C%22ghcr.io%2Fgithub%2Fgithub-mcp-server%22%5D%2C%22env%22%3A%7B%22GITHUB_OAUTH_CALLBACK_PORT%22%3A%228085%22%7D%7D&quality=insiders) [![Install with Docker in Visual Studio](https://img.shields.io/badge/Visual_Studio-Install_Server-C16FDE?style=flat-square&logo=visualstudio&logoColor=white)](https://aka.ms/vs/mcp-install?%7B%22name%22%3A%22github%22%2C%22command%22%3A%22docker%22%2C%22args%22%3A%5B%22run%22%2C%22-i%22%2C%22--rm%22%2C%22-p%22%2C%22127.0.0.1%3A8085%3A8085%22%2C%22-e%22%2C%22GITHUB_OAUTH_CALLBACK_PORT%3D8085%22%2C%22ghcr.io%2Fgithub%2Fgithub-mcp-server%22%5D%7D)

### Prerequisites

1. To run the server in a container, you will need to have [Docker](https://www.docker.com/) installed.
2. Once Docker is installed, you will also need to ensure Docker is running. The Docker image is available at `ghcr.io/github/github-mcp-server`. The image is public; if you get errors on pull, you may have an expired token and need to `docker logout ghcr.io`.
3. **Authentication.** On github.com you don't need to create anything up front — the one-click buttons above log you in with OAuth on first use (a browser-based flow; the token is kept in memory only). The Docker buttons publish a fixed callback port (`127.0.0.1:8085`) so the container's login callback is reachable. See **[Local Server OAuth Login](docs/oauth-login.md)** for how it works, headless/device-code fallback, and bringing your own OAuth or GitHub App (required for GitHub Enterprise Server and `ghe.com`).

   Prefer a token? You can still authenticate with a [GitHub Personal Access Token](https://github.com/settings/personal-access-tokens/new) by setting `GITHUB_PERSONAL_ACCESS_TOKEN` instead (it takes precedence over OAuth). The MCP server can use many of the GitHub APIs, so enable the permissions that you feel comfortable granting your AI tools (to learn more about access tokens, please check out the [documentation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)).

<details><summary><b>Handling PATs Securely</b></summary>

### Environment Variables (Recommended)

To keep your GitHub PAT secure and reusable across different MCP hosts:

1. **Store your PAT in environment variables**

   ```bash
   export GITHUB_PAT=your_token_here
   ```

   Or create a `.env` file:

   ```env
   GITHUB_PAT=your_token_here
   ```

2. **Protect your `.env` file**

   ```bash
   # Add to .gitignore to prevent accidental commits
   echo ".env" >> .gitignore
   ```

3. **Reference the token in configurations**

   ```bash
   # CLI usage
   claude mcp add github -e GITHUB_PERSONAL_ACCESS_TOKEN=$GITHUB_PAT -- docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN ghcr.io/github/github-mcp-server

   # In config files (where supported)
   "env": {
     "GITHUB_PERSONAL_ACCESS_TOKEN": "$GITHUB_PAT"
   }
   ```

> **Note**: Environment variable support varies by host app and IDE. Some applications (like Windsurf) require hardcoded tokens in config files.

### Token Security Best Practices

- **Minimum scopes**: Only grant necessary permissions
  - `repo` - Repository operations
  - `read:packages` - Docker image access
  - `read:org` - Organization team access
- **Separate tokens**: Use different PATs for different projects/environments
- **Regular rotation**: Update tokens periodically
- **Never commit**: Keep tokens out of version control
- **File permissions**: Restrict access to config files containing tokens

  ```bash
  chmod 600 ~/.your-app/config.json
  ```

</details>

### GitHub Enterprise Server and Enterprise Cloud with data residency (ghe.com)

The flag `--gh-host` and the environment variable `GITHUB_HOST` can be used to set
the hostname for GitHub Enterprise Server or GitHub Enterprise Cloud with data residency.

- For GitHub Enterprise Server, prefix the hostname with the `https://` URI scheme. HTTPS is required and enforced: non-HTTPS hosts are refused so that credentials are never sent over cleartext (the only exception is a loopback host such as `http://localhost` for local development).
- For GitHub Enterprise Cloud with data residency, use `https://YOURSUBDOMAIN.ghe.com` as the hostname.

``` json
"github": {
    "command": "docker",
    "args": [
    "run",
    "-i",
    "--rm",
    "-e",
    "GITHUB_PERSONAL_ACCESS_TOKEN",
    "-e",
    "GITHUB_HOST",
    "ghcr.io/github/github-mcp-server"
    ],
    "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}",
        "GITHUB_HOST": "https://<your GHES or ghe.com domain name>"
    }
}
```

## Installation

### Install in GitHub Copilot on VS Code

For quick installation, use one of the one-click install buttons above. Once you complete that flow, toggle Agent mode (located by the Copilot Chat text input) and the server will start.

More about using MCP server tools in VS Code's [agent mode documentation](https://code.visualstudio.com/docs/copilot/chat/mcp-servers).

Install in GitHub Copilot on other IDEs (JetBrains, Visual Studio, Eclipse, etc.)

Add one of the following JSON blocks to your IDE's MCP settings.

**Log in with OAuth (no token to create or store).** On github.com the official image already includes the app credentials, so you provide none yourself: it runs a browser-based login on first use and keeps the resulting token **in memory only**. In Docker this needs a fixed callback port published to loopback so the container's login callback is reachable:

```json
{
  "mcp": {
    "servers": {
      "github": {
        "command": "docker",
        "args": [
          "run",
          "-i",
          "--rm",
          "-p",
          "127.0.0.1:8085:8085",
          "-e",
          "GITHUB_OAUTH_CALLBACK_PORT",
          "ghcr.io/github/github-mcp-server"
        ],
        "env": {
          "GITHUB_OAUTH_CALLBACK_PORT": "8085"
        }
      }
    }
  }
}
```

See **[Local Server OAuth Login](docs/oauth-login.md)** for the native-binary flow (no fixed port needed), the headless/device-code fallback, GitHub Enterprise Server / `ghe.com`, and bringing your own OAuth or GitHub App.

For non-interactive stdio deployments, see **[GitHub App Authentication](docs/github-app-auth.md)**.

**Or authenticate with a Personal Access Token.** Set `GITHUB_PERSONAL_ACCESS_TOKEN` instead (it takes precedence over OAuth):

```json
{
  "mcp": {
    "inputs": [
      {
        "type": "promptString",
        "id": "github_token",
        "description": "GitHub Personal Access Token",
        "password": true
      }
    ],
    "servers": {
      "github": {
        "command": "docker",
        "args": [
          "run",
          "-i",
          "--rm",
          "-e",
          "GITHUB_PERSONAL_ACCESS_TOKEN",
          "ghcr.io/github/github-mcp-server"
        ],
        "env": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
        }
      }
    }
  }
}
```

Optionally, you can add a similar example (i.e. without the mcp key) to a file called `.vscode/mcp.json` in your workspace. This will allow you to share the configuration with other host applications that accept the same format.

<details>
<summary><b>Example JSON block without the MCP key included</b></summary>
<br>

```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "github_token",
      "description": "GitHub Personal Access Token",
      "password": true
    }
  ],
  "servers": {
    "github": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e",
        "GITHUB_PERSONAL_ACCESS_TOKEN",
        "ghcr.io/github/github-mcp-server"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "${input:github_token}"
      }
    }
  }
}
```

</details>

### Install in Other MCP Hosts

For other MCP host applications, please refer to our installation guides:

- **[Copilot CLI](docs/installation-guides/install-copilot-cli.md)** - Installation guide for GitHub Copilot CLI
- **[GitHub Copilot in other IDEs](/docs/installation-guides/install-other-copilot-ides.md)** - Installation for JetBrains, Visual Studio, Eclipse, and Xcode with GitHub Copilot
- **[Claude Code & Claude Desktop](docs/installation-guides/install-claude.md)** - Installation guide for Claude Code and Claude Desktop
- **[Cursor](docs/installation-guides/install-cursor.md)** - Installation guide for Cursor IDE
- **[Google Gemini CLI](docs/installation-guides/install-gemini-cli.md)** - Installation guide for Google Gemini CLI
- **[OpenCode](docs/installation-guides/install-opencode.md)** - Installation guide for the OpenCode terminal agent
- **[Windsurf](docs/installation-guides/install-windsurf.md)** - Installation guide for Windsurf IDE
- **[Zed](docs/installation-guides/install-zed.md)** - Installation guide for Zed editor

For a complete overview of all installation options, see our **[Installation Guides Index](docs/installation-guides)**.

> **Note:** Any host application that supports local MCP servers should be able to access the local GitHub MCP server. However, the specific configuration process, syntax and stability of the integration will vary by host application. While many may follow a similar format to the examples above, this is not guaranteed. Please refer to your host application's documentation for the correct MCP configuration syntax and setup process.

### Build from source

If you don't have Docker, you can use `go build` to build the binary in the
`cmd/github-mcp-server` directory, and use the `github-mcp-server stdio` command with the `GITHUB_PERSONAL_ACCESS_TOKEN` environment variable set to your token. To specify the output location of the build, use the `-o` flag. You should configure your server to use the built executable as its `command`. For example:

```JSON
{
  "mcp": {
    "servers": {
      "github": {
        "command": "/path/to/github-mcp-server",
        "args": ["stdio"],
        "env": {
          "GITHUB_PERSONAL_ACCESS_TOKEN": "<YOUR_TOKEN>"
        }
      }
    }
  }
}
```

### CLI utilities

The `github-mcp-server` binary includes a few CLI subcommands that are helpful for debugging and exploring the server.

- `github-mcp-server tool-search "<query>"` searches tools by name, description, and input parameter names. Use `--max-results` to return more matches.
Example (color output requires a TTY; use `docker run -t` (or `-it`) when running in Docker):
```bash
docker run -it --rm ghcr.io/github/github-mcp-server tool-search "issue" --max-results 5
github-mcp-server tool-search "issue" --max-results 5
```

## Tool Configuration

The GitHub MCP Server supports enabling or disabling specific groups of functionalities via the `--toolsets` flag. This allows you to control which GitHub API capabilities are available to your AI tools. Enabling only the toolsets that you need can help the LLM with tool choice and reduce the context size.

_Toolsets are not limited to Tools. Relevant MCP Resources and Prompts are also included where applicable._

When no toolsets are specified, [default toolsets](#default-toolset) are used.

> **Looking for examples?** See the [Server Configuration Guide](./docs/server-configuration.md) for common recipes like minimal setups, read-only mode, and combining tools with toolsets.

#### Specifying Toolsets

To specify toolsets you want available to the LLM, you can pass an allow-list in two ways:

1. **Using Command Line Argument**:

   ```bash
   github-mcp-server --toolsets repos,issues,pull_requests,actions,code_security
   ```

2. **Using Environment Variable**:

   ```bash
   GITHUB_TOOLSETS="repos,issues,pull_requests,actions,code_security" ./github-mcp-server
   ```

The environment variable `GITHUB_TOOLSETS` takes precedence over the command line argument if both are provided.

#### Specifying Individual Tools

You can also configure specific tools using the `--tools` flag. Tools can be used independently or combined with toolsets for fine-grained control.

1. **Using Command Line Argument**:

   ```bash
   github-mcp-server --tools get_file_contents,issue_read,create_pull_request
   ```

2. **Using Environment Variable**:

   ```bash
   GITHUB_TOOLS="get_file_contents,issue_read,create_pull_request" ./github-mcp-server
   ```

3. **Combining with Toolsets** (additive):

   ```bash
   github-mcp-server --toolsets repos,issues --tools get_gist
   ```

   This registers all tools from `repos` and `issues` toolsets, plus `get_gist`.

**Important Notes:**

- Tools and toolsets can be used together
- Read-only mode takes priority: write tools are skipped if `--read-only` is set, even if explicitly requested via `--tools`
- Tool names must match exactly (e.g., `get_file_contents`, not `getFileContents`). Invalid tool names will cause the server to fail at startup with an error message
- When tools are renamed, old names are preserved as aliases for backward compatibility. See [Tool Renaming](docs/tool-renaming.md) for details.

### Using Toolsets With Docker

When using Docker, you can pass the toolsets as environment variables:

```bash
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_TOOLSETS="repos,issues,pull_requests,actions,code_security" \
  ghcr.io/github/github-mcp-server
```

### Using Tools With Docker

When using Docker, you can pass specific tools as environment variables. You can also combine tools with toolsets:

```bash
# Tools only
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_TOOLS="get_file_contents,issue_read,create_pull_request" \
  ghcr.io/github/github-mcp-server

# Tools combined with toolsets (additive)
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_TOOLSETS="repos,issues" \
  -e GITHUB_TOOLS="get_gist" \
  ghcr.io/github/github-mcp-server
```

### Special toolsets

#### "all" toolset

The special toolset `all` can be provided to enable all available toolsets regardless of any other configuration:

```bash
./github-mcp-server --toolsets all
```

Or using the environment variable:

```bash
GITHUB_TOOLSETS="all" ./github-mcp-server
```

#### "default" toolset

The default toolset `default` is the configuration that gets passed to the server if no toolsets are specified.

The default configuration is:

- context
- repos
- issues
- pull_requests
- users

To keep the default configuration and add additional toolsets:

```bash
GITHUB_TOOLSETS="default,stargazers" ./github-mcp-server
```

### Insiders Mode

The local GitHub MCP Server offers an insiders version with early access to new features and experimental tools.

1. **Using Command Line Argument**:

   ```bash
   ./github-mcp-server --insiders
   ```

2. **Using Environment Variable**:

   ```bash
   GITHUB_INSIDERS=true ./github-mcp-server
   ```

When using Docker:

```bash
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_INSIDERS=true \
  ghcr.io/github/github-mcp-server
```

### Available Toolsets

The following sets of tools are available:

<!-- START AUTOMATED TOOLSETS -->
|     | Toolset                 | Description                                                   |
| --- | ----------------------- | ------------------------------------------------------------- |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/person-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/person-light.png"><img src="pkg/octicons/icons/person-light.png" width="20" height="20" alt="person"></picture> | `context`               | **Strongly recommended**: Tools that provide context about the current user and GitHub context you are operating in |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/workflow-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/workflow-light.png"><img src="pkg/octicons/icons/workflow-light.png" width="20" height="20" alt="workflow"></picture> | `actions` | GitHub Actions workflows and CI/CD operations |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/tools-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/tools-light.png"><img src="pkg/octicons/icons/tools-light.png" width="20" height="20" alt="tools"></picture> | `actions_admin` | GitHub Actions configuration: variables, secrets, permissions policy and per-workflow enablement |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/code-square-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/code-square-light.png"><img src="pkg/octicons/icons/code-square-light.png" width="20" height="20" alt="code-square"></picture> | `code_quality` | GitHub Code Quality related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/codescan-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/codescan-light.png"><img src="pkg/octicons/icons/codescan-light.png" width="20" height="20" alt="codescan"></picture> | `code_security` | Code security related tools, such as GitHub Code Scanning |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/copilot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/copilot-light.png"><img src="pkg/octicons/icons/copilot-light.png" width="20" height="20" alt="copilot"></picture> | `copilot` | Copilot related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/copilot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/copilot-light.png"><img src="pkg/octicons/icons/copilot-light.png" width="20" height="20" alt="copilot"></picture> | `copilot_issue_intents` | Opt-in Copilot issue assignment tools that carry intent metadata (rationale, confidence, suggestion) |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/dependabot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/dependabot-light.png"><img src="pkg/octicons/icons/dependabot-light.png" width="20" height="20" alt="dependabot"></picture> | `dependabot` | Dependabot tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/workflow-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/workflow-light.png"><img src="pkg/octicons/icons/workflow-light.png" width="20" height="20" alt="workflow"></picture> | `deployments` | Deployment environments, deployments and deployment statuses |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/comment-discussion-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/comment-discussion-light.png"><img src="pkg/octicons/icons/comment-discussion-light.png" width="20" height="20" alt="comment-discussion"></picture> | `discussions` | GitHub Discussions related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/logo-gist-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/logo-gist-light.png"><img src="pkg/octicons/icons/logo-gist-light.png" width="20" height="20" alt="logo-gist"></picture> | `gists` | GitHub Gist related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/git-branch-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/git-branch-light.png"><img src="pkg/octicons/icons/git-branch-light.png" width="20" height="20" alt="git-branch"></picture> | `git` | GitHub Git API related tools for low-level Git operations |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/issue-opened-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/issue-opened-light.png"><img src="pkg/octicons/icons/issue-opened-light.png" width="20" height="20" alt="issue-opened"></picture> | `issues` | GitHub Issues related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/tag-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/tag-light.png"><img src="pkg/octicons/icons/tag-light.png" width="20" height="20" alt="tag"></picture> | `labels` | GitHub Labels related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/bell-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/bell-light.png"><img src="pkg/octicons/icons/bell-light.png" width="20" height="20" alt="bell"></picture> | `notifications` | GitHub Notifications related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/organization-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/organization-light.png"><img src="pkg/octicons/icons/organization-light.png" width="20" height="20" alt="organization"></picture> | `orgs` | GitHub Organization related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/project-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/project-light.png"><img src="pkg/octicons/icons/project-light.png" width="20" height="20" alt="project"></picture> | `projects` | GitHub Projects related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/git-pull-request-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/git-pull-request-light.png"><img src="pkg/octicons/icons/git-pull-request-light.png" width="20" height="20" alt="git-pull-request"></picture> | `pull_requests` | GitHub Pull Request related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-lock-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-lock-light.png"><img src="pkg/octicons/icons/shield-lock-light.png" width="20" height="20" alt="shield-lock"></picture> | `repo_governance` | Repository governance: rulesets and branch protection, repository settings, collaborator and team access, and merge policy |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/repo-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/repo-light.png"><img src="pkg/octicons/icons/repo-light.png" width="20" height="20" alt="repo"></picture> | `repos` | GitHub Repository related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-lock-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-lock-light.png"><img src="pkg/octicons/icons/shield-lock-light.png" width="20" height="20" alt="shield-lock"></picture> | `secret_protection` | Secret protection related tools, such as GitHub Secret Scanning |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-light.png"><img src="pkg/octicons/icons/shield-light.png" width="20" height="20" alt="shield"></picture> | `security_advisories` | Security advisories related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/star-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/star-light.png"><img src="pkg/octicons/icons/star-light.png" width="20" height="20" alt="star"></picture> | `stargazers` | GitHub Stargazers related tools |
| <picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/people-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/people-light.png"><img src="pkg/octicons/icons/people-light.png" width="20" height="20" alt="people"></picture> | `users` | GitHub User related tools |
<!-- END AUTOMATED TOOLSETS -->

### Additional Toolsets in Remote GitHub MCP Server

| Toolset                 | Description                                                   |
| ----------------------- | ------------------------------------------------------------- |
| `copilot` | Copilot related tools (e.g. Copilot Coding Agent) |
| `copilot_spaces` | Copilot Spaces related tools |
| `github_support_docs_search` | Search docs to answer GitHub product and support questions |

## Tools

<!-- START AUTOMATED TOOLS -->
<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/workflow-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/workflow-light.png"><img src="pkg/octicons/icons/workflow-light.png" width="20" height="20" alt="workflow"></picture> Actions</summary>

- **actions_get** - Get details of GitHub Actions resources (workflows, workflow runs, jobs, and artifacts)
  - **Required OAuth Scopes**: `repo`
  - `method`: The method to execute (string, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `resource_id`: The unique identifier of the resource. This will vary based on the "method" provided, so ensure you provide the correct ID:
    - Provide a workflow ID or workflow file name (e.g. ci.yaml) for 'get_workflow' method.
    - Provide a workflow run ID for 'get_workflow_run', 'get_workflow_run_usage', and 'get_workflow_run_logs_url' methods.
    - Provide an artifact ID for 'download_workflow_run_artifact' method.
    - Provide a job ID for 'get_workflow_job' method.
     (string, required)

- **actions_list** - List GitHub Actions workflows in a repository
  - **Required OAuth Scopes**: `repo`
  - `method`: The action to perform (string, required)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (default: 1) (number, optional)
  - `per_page`: Results per page for pagination (default: 30, max: 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `resource_id`: The unique identifier of the resource. This will vary based on the "method" provided, so ensure you provide the correct ID:
    - Do not provide any resource ID for 'list_workflows' method.
    - Provide a workflow ID or workflow file name (e.g. ci.yaml) for 'list_workflow_runs' method, or omit to list all workflow runs in the repository.
    - Provide a workflow run ID for 'list_workflow_jobs' and 'list_workflow_run_artifacts' methods.
     (string, optional)
  - `workflow_jobs_filter`: Filters for workflow jobs. **ONLY** used when method is 'list_workflow_jobs' (object, optional)
  - `workflow_runs_filter`: Filters for workflow runs. **ONLY** used when method is 'list_workflow_runs' (object, optional)

- **actions_run_trigger** - Trigger GitHub Actions workflow actions
  - **Required OAuth Scopes**: `repo`
  - `inputs`: Inputs the workflow accepts. Only used for 'run_workflow' method. (object, optional)
  - `method`: The method to execute (string, required)
  - `owner`: Repository owner (string, required)
  - `ref`: The git reference for the workflow. The reference can be a branch or tag name. Required for 'run_workflow' method. (string, optional)
  - `repo`: Repository name (string, required)
  - `run_id`: The ID of the workflow run. Required for all methods except 'run_workflow'. (number, optional)
  - `workflow_id`: The workflow ID (numeric) or workflow file name (e.g., main.yml, ci.yaml). Required for 'run_workflow' method. (string, optional)

- **checks_read** - Read check runs and commit statuses
  - **Required OAuth Scopes**: `repo`
  - `app_id`: Only return check runs or suites produced by this GitHub App ID. (number, optional)
  - `check_name`: Only return check runs or suites with this name. Used by 'list_check_runs' and 'list_check_suites'. (string, optional)
  - `check_run_id`: Numeric check run ID. Required for 'get_check_run'. (number, optional)
  - `filter`: Whether to return the latest check run per name or every run, including reruns. Defaults to 'latest'. Used by 'list_check_runs'. (string, optional)
  - `method`: The method to execute:
    - list_check_runs: check runs for a ref (Checks API)
    - get_check_run: a single check run by ID
    - list_check_suites: check suites for a ref
    - get_combined_status: the rolled-up commit status for a ref (Statuses API)
    - list_statuses: individual commit statuses for a ref, most recent first (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `ref`: Commit SHA, branch name or tag name. Required for all methods except 'get_check_run'. (string, optional)
  - `repo`: Repository name (string, required)
  - `status`: Only return check runs in this status. Used by 'list_check_runs'. (string, optional)

- **commit_status_write** - Post a commit status
  - **Required OAuth Scopes**: `repo`
  - `context`: Label that distinguishes this status from other systems' statuses (e.g. 'ci/lint'). Defaults to 'default'. (string, optional)
  - `description`: Short human-readable summary of the status. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `ref`: Commit SHA, branch name or tag name to attach the status to. (string, required)
  - `repo`: Repository name (string, required)
  - `state`: The status state. (string, required)
  - `target_url`: URL the status links to from the GitHub UI, e.g. the build output. (string, optional)

- **get_job_logs** - Get GitHub Actions workflow job logs
  - **Required OAuth Scopes**: `repo`
  - `failed_only`: When true, gets logs for all failed jobs in the workflow run specified by run_id. Requires run_id to be provided. (boolean, optional)
  - `job_id`: The unique identifier of the workflow job. Required when getting logs for a single job. (number, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `return_content`: Returns actual log content instead of URLs (boolean, optional)
  - `run_id`: The unique identifier of the workflow run. Required when failed_only is true to get logs for all failed jobs in the run. (number, optional)
  - `tail_lines`: Number of lines to return from the end of the log (number, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/tools-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/tools-light.png"><img src="pkg/octicons/icons/tools-light.png" width="20" height="20" alt="tools"></picture> Actions Admin</summary>

- **actions_cache_read** - Read Actions caches
  - **Required OAuth Scopes**: `repo`
  - `key`: Only list caches with this key. Used by 'list'. (string, optional)
  - `method`: The method to execute:
    - list: caches stored for the repository
    - usage: total cache count and size for the repository (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `ref`: Only list caches for this Git ref, e.g. 'refs/heads/main'. Used by 'list'. (string, optional)
  - `repo`: Repository name (string, required)

- **actions_cache_write** - Manage Actions caches
  - **Required OAuth Scopes**: `repo`
  - `cache_id`: Cache ID. Required for 'delete_by_id'. (integer, optional)
  - `key`: Cache key. Required for 'delete_by_key'. (string, optional)
  - `method`: The operation to perform:
    - delete_by_id: delete one cache entry by its ID
    - delete_by_key: delete all cache entries matching a key, optionally scoped to a ref (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `ref`: Only delete cache entries for this Git ref, e.g. 'refs/heads/main'. Used by 'delete_by_key'. (string, optional)
  - `repo`: Repository name (string, required)

- **actions_config_read** - Read Actions configuration
  - **Required OAuth Scopes**: `repo`
  - `method`: The method to execute. Only 'get_permissions' is supported; it returns the whole configuration. (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **actions_config_write** - Change Actions configuration
  - **Required OAuth Scopes**: `repo`
  - `allowed_actions`: Which actions workflows may use: 'all', 'local_only' for actions defined in this repository, or 'selected' for the allow-list set by 'set_allowed_actions'. Used by 'set_permissions'. (string, optional)
  - `can_approve_pull_request_reviews`: Allow GitHub Actions to approve pull requests. Used by 'set_workflow_permissions'. (boolean, optional)
  - `default_workflow_permissions`: What the default GITHUB_TOKEN may do in a workflow. Used by 'set_workflow_permissions'. (string, optional)
  - `enabled`: Whether GitHub Actions runs in this repository at all. Used by 'set_permissions'. (boolean, optional)
  - `github_owned_allowed`: Allow actions published by GitHub. Used by 'set_allowed_actions'. (boolean, optional)
  - `method`: The operation to perform:
    - set_permissions: enable or disable Actions, and choose which actions may be used
    - set_allowed_actions: the allow-list applied when allowed_actions is 'selected'
    - set_workflow_permissions: what the default GITHUB_TOKEN can do
    - enable_workflow / disable_workflow: switch one workflow on or off (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `patterns_allowed`: Specific actions to allow, e.g. ['octo-org/*', 'actions/checkout@v4']. Pass an empty array to allow no third-party actions. Used by 'set_allowed_actions'. (string[], optional)
  - `repo`: Repository name (string, required)
  - `verified_allowed`: Allow actions from verified creators. Used by 'set_allowed_actions'. (boolean, optional)
  - `workflow`: Workflow file name, e.g. 'deploy.yml', or its numeric ID. Required for 'enable_workflow' and 'disable_workflow'. (string, optional)

- **actions_runner_write** - Manage Actions runners
  - **Required OAuth Scopes (any of)**: `repo`, `write:org`
  - **Accepted OAuth Scopes**: `admin:org`, `repo`, `write:org`
  - `method`: The operation to perform:
    - remove: deregister a runner (string, required)
  - `org`: Organization login. Required when scope is 'organization'. (string, optional)
  - `owner`: Repository owner. Required when scope is 'repository'. (string, optional)
  - `repo`: Repository name. Required when scope is 'repository'. (string, optional)
  - `runner_id`: Runner ID to remove. (integer, required)
  - `scope`: Where the runner is registered. Defaults to 'repository'. (string, optional)

- **actions_runners_read** - Read Actions runners
  - **Required OAuth Scopes (any of)**: `repo`, `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `repo`, `write:org`
  - `method`: The method to execute:
    - list: all self-hosted runners in scope
    - get: one runner's details (string, required)
  - `org`: Organization login. Required when scope is 'organization'. (string, optional)
  - `owner`: Repository owner. Required when scope is 'repository'. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name. Required when scope is 'repository'. (string, optional)
  - `runner_id`: Runner ID. Required for 'get'. (integer, optional)
  - `scope`: Where the runners are registered. Defaults to 'repository'. (string, optional)

- **actions_secret_write** - Write Actions secrets
  - **Required OAuth Scopes**: `repo`
  - `environment_name`: Environment name. Required when scope is 'environment'. (string, optional)
  - `method`: Operation to perform: 'create_or_update' or 'delete' (string, required)
  - `name`: Secret name, e.g. 'DEPLOY_TOKEN'. (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)
  - `scope`: Where the value lives: on the repository, or on one of its environments. Defaults to 'repository'. Organization scope is not available on this tool. (string, optional)
  - `value`: The secret value. Required for 'create_or_update'. It is encrypted to the repository's or environment's public key before it leaves this server, is never returned by any tool, and is redacted from this server's logs. (string, optional)

- **actions_secrets_read** - Read Actions secret metadata
  - **Required OAuth Scopes**: `repo`
  - `environment_name`: Environment name. Required when scope is 'environment'. (string, optional)
  - `method`: The method to execute: 'list' for every secret in scope, 'get' for one secret's metadata by name (string, required)
  - `name`: Secret name. Required for 'get'. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `scope`: Where the value lives: on the repository, or on one of its environments. Defaults to 'repository'. Organization scope is not available on this tool. (string, optional)

- **actions_variable_write** - Write Actions variables
  - **Required OAuth Scopes**: `repo`
  - `environment_name`: Environment name. Required when scope is 'environment'. (string, optional)
  - `method`: Operation to perform: 'create_or_update' or 'delete' (string, required)
  - `name`: Variable name. (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)
  - `scope`: Where the value lives: on the repository, or on one of its environments. Defaults to 'repository'. Organization scope is not available on this tool. (string, optional)
  - `value`: Variable value. Required for 'create_or_update'. (string, optional)

- **actions_variables_read** - Read Actions variables
  - **Required OAuth Scopes (any of)**: `repo`, `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `repo`, `write:org`
  - `environment_name`: Environment name. Required when scope is 'environment'. (string, optional)
  - `method`: The method to execute: 'list' for every variable in scope, 'get' for one by name (string, required)
  - `name`: Variable name. Required for 'get'. (string, optional)
  - `org`: Organization login. Defaults to owner when scope is 'organization'. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name. Not needed when scope is 'organization'. (string, optional)
  - `scope`: Where the value lives: on the repository, on one of its environments, or on the organization. Defaults to 'repository'. (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/code-square-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/code-square-light.png"><img src="pkg/octicons/icons/code-square-light.png" width="20" height="20" alt="code-square"></picture> Code Quality</summary>

- **get_code_quality_finding** - Get code quality finding
  - **Required OAuth Scopes**: `repo`
  - `findingNumber`: The number of the finding. (number, required)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/codescan-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/codescan-light.png"><img src="pkg/octicons/icons/codescan-light.png" width="20" height="20" alt="codescan"></picture> Code Security</summary>

- **get_code_scanning_alert** - Get code scanning alert
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `alertNumber`: The number of the alert. (number, required)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

- **list_code_scanning_alerts** - List code scanning alerts
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `owner`: The owner of the repository. (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `ref`: The Git reference for the results you want to list. (string, optional)
  - `repo`: The name of the repository. (string, required)
  - `severity`: Filter code scanning alerts by severity (string, optional)
  - `state`: Filter code scanning alerts by state. Defaults to open (string, optional)
  - `tool_name`: The name of the tool used for code scanning. (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/person-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/person-light.png"><img src="pkg/octicons/icons/person-light.png" width="20" height="20" alt="person"></picture> Context</summary>

- **get_me** - Get my user profile
  - No parameters required

- **get_team_members** - Get team members
  - **Required OAuth Scopes**: `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `write:org`
  - `org`: Organization login (owner) that contains the team. (string, required)
  - `team_slug`: Team slug (string, required)

- **get_teams** - Get teams
  - **Required OAuth Scopes**: `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `write:org`
  - `user`: Username to get teams for. If not provided, uses the authenticated user. (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/copilot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/copilot-light.png"><img src="pkg/octicons/icons/copilot-light.png" width="20" height="20" alt="copilot"></picture> Copilot</summary>

- **assign_copilot_to_issue** - Assign Copilot to issue
  - **Required OAuth Scopes**: `repo`
  - `base_ref`: Git reference (e.g., branch) that the agent will start its work from. If not specified, defaults to the repository's default branch (string, optional)
  - `custom_instructions`: Optional custom instructions to guide the agent beyond the issue body. Use this to provide additional context, constraints, or guidance that is not captured in the issue description (string, optional)
  - `issue_number`: Issue number (number, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **request_copilot_review** - Request Copilot review
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/copilot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/copilot-light.png"><img src="pkg/octicons/icons/copilot-light.png" width="20" height="20" alt="copilot"></picture> Copilot Issue Intents</summary>

- **assign_copilot_to_issue_with_intent** - Assign Copilot to issue with intent
  - **Required OAuth Scopes**: `repo`
  - `base_ref`: Git reference (e.g., branch) that the agent will start its work from. If not specified, defaults to the repository's default branch. Ignored when is_suggestion is true (string, optional)
  - `confidence`: How confident you are in this choice. 'HIGH' for clear signal or explicit user request, 'MEDIUM' for reasonable inference with some ambiguity, 'LOW' for best guess with limited signal. (string, required)
  - `custom_instructions`: Optional custom instructions to guide the agent beyond the issue body. Ignored when is_suggestion is true (string, optional)
  - `is_suggestion`: If true, records a pending Copilot assignment intent rather than launching the agent. Approval later supplies the launch context; base_ref and custom_instructions are ignored in this case. (boolean, required)
  - `issue_number`: Issue number (number, required)
  - `owner`: Repository owner (string, required)
  - `rationale`: One concise sentence explaining what specifically about the issue led to choosing Copilot. State the concrete signal (e.g. 'Well-scoped task with clear acceptance criteria'). (string, required)
  - `repo`: Repository name (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/dependabot-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/dependabot-light.png"><img src="pkg/octicons/icons/dependabot-light.png" width="20" height="20" alt="dependabot"></picture> Dependabot</summary>

- **get_dependabot_alert** - Get dependabot alert
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `alertNumber`: The number of the alert. (number, required)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

- **list_dependabot_alerts** - List dependabot alerts
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, optional)
  - `owner`: The owner of the repository. (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: The name of the repository. (string, required)
  - `severity`: Filter dependabot alerts by severity (string, optional)
  - `state`: Filter dependabot alerts by state. Defaults to open (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/workflow-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/workflow-light.png"><img src="pkg/octicons/icons/workflow-light.png" width="20" height="20" alt="workflow"></picture> Deployments</summary>

- **deployment_write** - Record deployments
  - **Required OAuth Scopes**: `repo`
  - `auto_inactive`: On a successful status, mark earlier deployments to the same environment inactive. Defaults to true on GitHub's side. (boolean, optional)
  - `auto_merge`: Let GitHub merge the default branch into the ref first, failing the request if that conflicts. Defaults to false, which deploys the ref exactly as it is. (boolean, optional)
  - `deployment_id`: Deployment the status belongs to. Required for 'create_deployment_status'. (number, optional)
  - `description`: Short description of the deployment or of its status. (string, optional)
  - `environment`: Environment being deployed to, e.g. 'staging'. Defaults to 'production'. (string, optional)
  - `environment_url`: Where the deployed code can be reached. This is what the environment links to once the deployment succeeds. (string, optional)
  - `log_url`: Where the deployment output can be read, e.g. the workflow run. (string, optional)
  - `method`: Operation to perform: 'create_deployment' or 'create_deployment_status' (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `production_environment`: Mark the environment as production. Defaults to GitHub's guess from the environment name. (boolean, optional)
  - `ref`: Branch, tag or commit SHA to deploy. Required for 'create_deployment'. (string, optional)
  - `repo`: Repository name (string, required)
  - `required_contexts`: Status check contexts that must pass before the deployment is created. Pass an empty array to skip the check entirely; omit it to let GitHub require every context on the ref. (string[], optional)
  - `state`: How the deployment is going. Required for 'create_deployment_status'. (string, optional)
  - `task`: What the deployment does. Defaults to 'deploy'. (string, optional)
  - `transient_environment`: Mark the environment as temporary, e.g. a per-pull-request preview that will disappear. (boolean, optional)

- **deployments_read** - Read deployments
  - **Required OAuth Scopes**: `repo`
  - `deployment_id`: Numeric deployment ID. Required for 'get_deployment' and 'list_deployment_statuses'. (number, optional)
  - `environment`: Only list deployments to this environment. (string, optional)
  - `method`: The method to execute:
    - list_deployments: deployments, newest first, optionally filtered by environment, ref or SHA
    - get_deployment: one deployment by ID
    - list_deployment_statuses: the status history of one deployment, newest first (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `ref`: Only list deployments of this branch or tag. (string, optional)
  - `repo`: Repository name (string, required)
  - `sha`: Only list deployments of this commit SHA. Use this to go from a commit to what it deployed. (string, optional)
  - `task`: Only list deployments for this task, e.g. 'deploy'. (string, optional)

- **environment_write** - Configure deployment environments
  - **Required OAuth Scopes**: `repo`
  - `can_admins_bypass`: Allow repository admins to bypass the protection rules. (boolean, optional)
  - `custom_branch_patterns`: Branch name patterns allowed to deploy, e.g. ['main', 'release/*']. Replaces the current patterns. Requires deployment_branch_policy 'custom'. (string[], optional)
  - `deployment_branch_policy`: Which branches may deploy: 'all_branches', 'protected_branches', or 'custom' to restrict to the patterns in custom_branch_patterns. (string, optional)
  - `environment_name`: Environment name, e.g. 'staging'. Created if it does not exist. (string, required)
  - `method`: Operation to perform: 'create_or_update' or 'delete' (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `prevent_self_review`: Prevent the user who triggered a deployment from approving it. (boolean, optional)
  - `repo`: Repository name (string, required)
  - `reviewers`: Users and teams that must approve a deployment. Replaces the current list; an empty array removes the review requirement. Identify each reviewer by login, team slug, or numeric id. (object[], optional)
  - `wait_timer_minutes`: Minutes to delay before a deployment to this environment can proceed. 0 removes the delay. (number, optional)

- **environments_read** - Read deployment environments
  - **Required OAuth Scopes**: `repo`
  - `environment_name`: Environment name, e.g. 'staging'. Required for 'get_environment'. (string, optional)
  - `method`: The method to execute:
    - list_environments: every environment in the repository
    - get_environment: one environment in full, including its custom branch patterns (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/comment-discussion-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/comment-discussion-light.png"><img src="pkg/octicons/icons/comment-discussion-light.png" width="20" height="20" alt="comment-discussion"></picture> Discussions</summary>

- **discussion_comment_write** - Manage discussion comments
  - **Required OAuth Scopes**: `repo`
  - `body`: Comment content (required for 'add', 'reply', and 'update' methods) (string, optional)
  - `commentNodeID`: The Node ID of the discussion comment (required for 'reply', 'update', 'delete', 'mark_answer', and 'unmark_answer' methods). For 'reply', this is the top-level comment to reply to; GitHub Discussions only support one level of nesting. (string, optional)
  - `discussionNumber`: Discussion number (required for 'add' and 'reply' methods) (number, optional)
  - `method`: Write operation to perform on a discussion comment.
    Options are:
    - 'add' - adds a new top-level comment to a discussion.
    - 'reply' - replies to a top-level discussion comment (GitHub Discussions only support one level of nesting).
    - 'update' - updates an existing discussion comment.
    - 'delete' - deletes a discussion comment.
    - 'mark_answer' - marks a discussion comment as the answer (Q&A only).
    - 'unmark_answer' - unmarks a discussion comment as the answer (Q&A only).
     (string, required)
  - `owner`: Repository owner (required for 'add' and 'reply' methods) (string, optional)
  - `repo`: Repository name (required for 'add' and 'reply' methods) (string, optional)

- **get_discussion** - Get discussion
  - **Required OAuth Scopes**: `repo`
  - `discussionNumber`: Discussion Number (number, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **get_discussion_comments** - Get discussion comments
  - **Required OAuth Scopes**: `repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, optional)
  - `discussionNumber`: Discussion Number (number, required)
  - `includeReplies`: When true, each top-level comment will include its replies nested within it (up to 100 replies per comment, which is the GitHub API maximum). Defaults to false. (boolean, optional)
  - `owner`: Repository owner (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **list_discussion_categories** - List discussion categories
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name. If not provided, discussion categories will be queried at the organisation level. (string, optional)

- **list_discussions** - List discussions
  - **Required OAuth Scopes**: `repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, optional)
  - `category`: Optional filter by discussion category ID. If provided, only discussions with this category are listed. (string, optional)
  - `direction`: Order direction. (string, optional)
  - `orderBy`: Order discussions by field. If provided, the 'direction' also needs to be provided. (string, optional)
  - `owner`: Repository owner (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name. If not provided, discussions will be queried at the organisation level. (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/logo-gist-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/logo-gist-light.png"><img src="pkg/octicons/icons/logo-gist-light.png" width="20" height="20" alt="logo-gist"></picture> Gists</summary>

- **create_gist** - Create Gist
  - **Required OAuth Scopes**: `gist`
  - `content`: Content for simple single-file gist creation (string, required)
  - `description`: Description of the gist (string, optional)
  - `filename`: Filename for simple single-file gist creation (string, required)
  - `public`: Whether the gist is public (boolean, optional)

- **get_gist** - Get Gist Content
  - `gist_id`: The ID of the gist (string, required)

- **list_gists** - List Gists
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `since`: Only gists updated after this time (ISO 8601 timestamp) (string, optional)
  - `username`: GitHub username (omit for authenticated user's gists) (string, optional)

- **update_gist** - Update Gist
  - **Required OAuth Scopes**: `gist`
  - `content`: Content for the file (string, required)
  - `description`: Updated description of the gist (string, optional)
  - `filename`: Filename to update or create (string, required)
  - `gist_id`: ID of the gist to update (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/git-branch-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/git-branch-light.png"><img src="pkg/octicons/icons/git-branch-light.png" width="20" height="20" alt="git-branch"></picture> Git</summary>

- **get_repository_tree** - Get repository tree
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (username or organization) (string, required)
  - `path_filter`: Optional path prefix to filter the tree results (e.g., 'src/' to only show files in the src directory) (string, optional)
  - `recursive`: Setting this parameter to true returns the objects or subtrees referenced by the tree. Default is false (boolean, optional)
  - `repo`: Repository name (string, required)
  - `tree_sha`: The SHA1 value or ref (branch or tag) name of the tree. Defaults to the repository's default branch (string, optional)

- **git_ref_write** - Write operations on git references
  - **Required OAuth Scopes**: `repo`
  - `force`: Allow a non-fast-forward update, discarding commits that only the old ref pointed at. Used by 'update'. (boolean, optional)
  - `method`: Operation to perform: 'create', 'update' (move an existing ref to a different commit), or 'delete' (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `ref`: Fully qualified ref name, e.g. 'refs/heads/my-branch' or 'refs/tags/v1.2.3'. (string, required)
  - `repo`: Repository name (string, required)
  - `sha`: Commit SHA the ref points at. Required for 'create' and 'update'. (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/issue-opened-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/issue-opened-light.png"><img src="pkg/octicons/icons/issue-opened-light.png" width="20" height="20" alt="issue-opened"></picture> Issues</summary>

- **add_issue_comment** - Add comment to issue or pull request
  - **Required OAuth Scopes**: `repo`
  - `body`: Comment content. Required unless reaction is provided. (string, optional)
  - `comment_id`: The numeric ID of the issue or pull request comment to react to. Use this for reactions to comments; omit it to react to the issue or pull request itself. Cannot be combined with body. (integer, optional)
  - `issue_number`: Issue or pull request number to comment on or react to. (number, required)
  - `owner`: Repository owner (string, required)
  - `reaction`: Emoji reaction to add. Required unless body is provided. (string, optional)
  - `repo`: Repository name (string, required)

- **get_label** - Get a specific label from a repository
  - **Required OAuth Scopes**: `repo`
  - `name`: Label name. (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **issue_read** - Get issue details
  - **Required OAuth Scopes**: `repo`
  - `issue_number`: The number of the issue (number, required)
  - `method`: The read operation to perform on a single issue.
    Options are:
    1. get - Get issue details. Also returns best-effort hierarchy flags (`has_parent`, `has_children`); `parent` and `sub_issues_summary` are optional relationship summaries, and `closed_by_pull_requests` summarizes the pull requests configured to close the issue as `total_count` plus up to 5 `references`.
    2. get_comments - Get issue comments.
    3. get_sub_issues - Get sub-issues (children) of the issue.
    4. get_parent - Get the parent issue, if this issue is a sub-issue of another.
    5. get_labels - Get labels assigned to the issue.
     (string, required)
  - `owner`: The owner of the repository (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: The name of the repository (string, required)

- **issue_write** - Create or update issue/pull request
  - **Required OAuth Scopes**: `repo`
  - `assignees`: Usernames to assign to this issue (string[], optional)
  - `body`: Issue body content (string, optional)
  - `duplicate_of`: Issue number that this issue is a duplicate of. Required when state_reason is 'duplicate'. (number, optional)
  - `issue_fields`: Issue field values to set or clear. Each item requires 'field_name' and exactly one of 'value', 'field_option_name', or 'delete: true'. (object[], optional)
  - `issue_number`: Issue number to update (number, optional)
  - `labels`: Labels to apply to this issue (string[], optional)
  - `method`: Write operation to perform on a single issue.
    Options are:
    - 'create' - creates a new issue.
    - 'update' - updates an existing issue.
     (string, required)
  - `milestone`: Milestone number (number, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `state`: New state (string, optional)
  - `state_reason`: Reason for the state change. Ignored unless state is changed. (string, optional)
  - `title`: Issue title (string, optional)
  - `type`: Type of this issue. For updates, pass null to remove the current type. Only use if issue types are enabled for this repository. Use list_issue_types to get valid type values for this repository or its owner organization. If the repository doesn't support issue types, omit this parameter. (string | null, optional)

- **list_issue_fields** - List issue fields
  - **Required OAuth Scopes (any of)**: `repo`, `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `repo`, `write:org`
  - `owner`: The account owner of the repository or organization. The name is not case sensitive. (string, required)
  - `repo`: The name of the repository. When provided, returns fields for this specific repository (inherited from its organization). When omitted, returns org-level fields directly. (string, optional)

- **list_issue_types** - List available issue types
  - **Required OAuth Scopes (any of)**: `repo`, `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `repo`, `write:org`
  - `owner`: The account owner of the repository or organization. (string, required)
  - `repo`: The name of the repository. When provided, returns issue types for this specific repository. When omitted, returns org-level issue types directly. (string, optional)

- **list_issues** - List issues
  - **Required OAuth Scopes**: `repo`
  - `after`: Cursor for pagination. Use the cursor from the previous response. (string, optional)
  - `direction`: Order direction. If provided, the 'orderBy' also needs to be provided. (string, optional)
  - `field_filters`: Filter by custom issue field values. Each entry takes a field_name and a value; the server looks up the field and coerces the value to its type (single-select option name, text, number, or YYYY-MM-DD date). (object[], optional)
  - `fields`: Subset of fields to return for each issue. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body' and 'field_values' in particular drops the largest per-result data. (string[], optional)
  - `labels`: Filter by labels (string[], optional)
  - `orderBy`: Order issues by field. If provided, the 'direction' also needs to be provided. (string, optional)
  - `owner`: Repository owner (string, required)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `since`: Filter by date (ISO 8601 timestamp) (string, optional)
  - `state`: Filter by state, by default both open and closed issues are returned when not provided (string, optional)

- **list_milestones** - List milestones
  - **Required OAuth Scopes**: `repo`
  - `direction`: Sort direction. Defaults to 'asc'. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `sort`: Sort field. Defaults to 'due_on'. (string, optional)
  - `state`: Filter by milestone state. Defaults to 'open'. (string, optional)

- **milestone_write** - Write operations on milestones
  - **Required OAuth Scopes**: `repo`
  - `description`: Milestone description. On 'update', pass an empty string to clear it; omit it to leave the current description unchanged. (string, optional)
  - `due_on`: Due date as YYYY-MM-DD. Interpreted as the end of that day in UTC. On 'update', pass an empty string to clear the due date; omit it to leave the current one unchanged. (string, optional)
  - `method`: Operation to perform: 'create', 'update', or 'delete' (string, required)
  - `milestone_number`: Milestone number. Identifies the milestone for 'update' and 'delete'; takes precedence over title. (number, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)
  - `state`: Milestone state. (string, optional)
  - `title`: Milestone title. Required for 'create'. For 'update' and 'delete' it identifies the milestone when milestone_number is not given; for 'update' it also renames the milestone when milestone_number is given. (string, optional)

- **search_issues** - Search issues
  - **Required OAuth Scopes**: `repo`
  - `fields`: Subset of fields to return for each issue result. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body', 'reactions', and 'labels' in particular drops the largest per-result data. (string[], optional)
  - `order`: Sort order (string, optional)
  - `owner`: Optional repository owner. If provided with repo, only issues for this repository are listed. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: The search query, as natural language. When the user gives alternative wordings, include them as plain words rather than joining them with OR. (string, required)
  - `repo`: Optional repository name. If provided with owner, only issues for this repository are listed. (string, optional)
  - `sort`: Sort field by number of matches of categories, defaults to best match (string, optional)

- **sub_issue_write** - Change sub-issue
  - **Required OAuth Scopes**: `repo`
  - `after_id`: The ID of the sub-issue to be prioritized after (either after_id OR before_id should be specified) (number, optional)
  - `before_id`: The ID of the sub-issue to be prioritized before (either after_id OR before_id should be specified) (number, optional)
  - `issue_number`: The number of the parent issue (number, required)
  - `method`: The action to perform on a single sub-issue
    Options are:
    - 'add' - add a sub-issue to a parent issue in a GitHub repository.
    - 'remove' - remove a sub-issue from a parent issue in a GitHub repository.
    - 'reprioritize' - change the order of sub-issues within a parent issue in a GitHub repository. Use either 'after_id' or 'before_id' to specify the new position.
    Writes issue hierarchy. To move a sub-issue to a new parent, use `add` with `replace_parent=true`; there is no writable parent field.
     (string, required)
  - `owner`: Repository owner (string, required)
  - `replace_parent`: When true, replaces the sub-issue's current parent issue. Use with 'add' method only. (boolean, optional)
  - `repo`: Repository name (string, required)
  - `sub_issue_id`: The ID of the sub-issue to add. ID is not the same as issue number (number, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/tag-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/tag-light.png"><img src="pkg/octicons/icons/tag-light.png" width="20" height="20" alt="tag"></picture> Labels</summary>

- **get_label** - Get a specific label from a repository
  - **Required OAuth Scopes**: `repo`
  - `name`: Label name. (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **label_write** - Write operations on repository labels
  - **Required OAuth Scopes**: `repo`
  - `color`: Label color as 6-character hex code without '#' prefix (e.g., 'f29513'). Required for 'create', optional for 'update'. (string, optional)
  - `description`: Label description text. Optional for 'create' and 'update'. (string, optional)
  - `method`: Operation to perform: 'create', 'update', or 'delete' (string, required)
  - `name`: Label name - required for all operations (string, required)
  - `new_name`: New name for the label (used only with 'update' method to rename) (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **list_label** - List labels from a repository
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (username or organization name) - required for all operations (string, required)
  - `repo`: Repository name - required for all operations (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/bell-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/bell-light.png"><img src="pkg/octicons/icons/bell-light.png" width="20" height="20" alt="bell"></picture> Notifications</summary>

- **dismiss_notification** - Dismiss notification
  - **Required OAuth Scopes**: `notifications`
  - `state`: The new state of the notification (read/done) (string, required)
  - `threadID`: The ID of the notification thread (string, required)

- **get_notification_details** - Get notification details
  - **Required OAuth Scopes**: `notifications`
  - `notificationID`: The ID of the notification (string, required)

- **list_notifications** - List notifications
  - **Required OAuth Scopes**: `notifications`
  - `before`: Only show notifications updated before the given time (ISO 8601 format) (string, optional)
  - `filter`: Filter notifications to, use default unless specified. Read notifications are ones that have already been acknowledged by the user. Participating notifications are those that the user is directly involved in, such as issues or pull requests they have commented on or created. (string, optional)
  - `owner`: Optional repository owner. If provided with repo, only notifications for this repository are listed. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Optional repository name. If provided with owner, only notifications for this repository are listed. (string, optional)
  - `since`: Only show notifications updated after the given time (ISO 8601 format) (string, optional)

- **manage_notification_subscription** - Manage notification subscription
  - **Required OAuth Scopes**: `notifications`
  - `action`: Action to perform: ignore, watch, or delete the notification subscription. (string, required)
  - `notificationID`: The ID of the notification thread. (string, required)

- **manage_repository_notification_subscription** - Manage repository notification subscription
  - **Required OAuth Scopes**: `notifications`
  - `action`: Action to perform: ignore, watch, or delete the repository notification subscription. (string, required)
  - `owner`: The account owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

- **mark_all_notifications_read** - Mark all notifications as read
  - **Required OAuth Scopes**: `notifications`
  - `lastReadAt`: Describes the last point that notifications were checked (optional). Default: Now (string, optional)
  - `owner`: Optional repository owner. If provided with repo, only notifications for this repository are marked as read. (string, optional)
  - `repo`: Optional repository name. If provided with owner, only notifications for this repository are marked as read. (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/organization-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/organization-light.png"><img src="pkg/octicons/icons/organization-light.png" width="20" height="20" alt="organization"></picture> Organizations</summary>

- **org_member_write** - Manage organization members
  - **Required OAuth Scopes**: `write:org`
  - **Accepted OAuth Scopes**: `admin:org`, `write:org`
  - `method`: The operation to perform:
    - update_membership: change a member's role
    - remove_member: remove a user from the organization entirely (string, required)
  - `org`: Organization login. (string, required)
  - `role`: Role to grant. Required for 'update_membership'. (string, optional)
  - `username`: GitHub login. (string, required)

- **org_members_read** - Read organization members
  - **Required OAuth Scopes**: `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `write:org`
  - `method`: The method to execute:
    - list_members: users who belong to the organization
    - get_membership: one user's role and status (string, required)
  - `org`: Organization login. (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `role`: Only list members holding this role. Used by 'list_members'. Defaults to 'all'. (string, optional)
  - `username`: GitHub login. Required for 'get_membership'. (string, optional)

- **search_orgs** - Search organizations
  - **Required OAuth Scopes**: `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `write:org`
  - `order`: Sort order (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Organization search query. Examples: 'microsoft', 'location:california', 'created:>=2025-01-01'. Search is automatically scoped to type:org. (string, required)
  - `sort`: Sort field by category (string, optional)

- **team_write** - Manage teams
  - **Required OAuth Scopes**: `write:org`
  - **Accepted OAuth Scopes**: `admin:org`, `write:org`
  - `description`: Team description. Used by 'create_team' and 'update_team'. (string, optional)
  - `method`: The operation to perform:
    - create_team: create a new team
    - update_team: change a team's name, description or privacy
    - delete_team: permanently delete a team
    - add_member: add a user to a team, or change their role
    - remove_member: remove a user from a team (string, required)
  - `name`: Team name. Required for 'create_team'; optional for 'update_team'. (string, optional)
  - `org`: Organization login that contains the team. (string, required)
  - `privacy`: 'secret' (visible only to org owners and team members) or 'closed' (visible to the whole org). Used by 'create_team' and 'update_team'. (string, optional)
  - `role`: Role to grant a team member. Used by 'add_member'. Defaults to 'member'. (string, optional)
  - `team_slug`: Team slug. Required for all methods except 'create_team'. (string, optional)
  - `username`: GitHub login. Required for 'add_member' and 'remove_member'. (string, optional)

- **teams_read** - Read teams
  - **Required OAuth Scopes**: `read:org`
  - **Accepted OAuth Scopes**: `admin:org`, `read:org`, `write:org`
  - `method`: The method to execute:
    - list_teams: all teams in the organization
    - get_team: one team's details by slug
    - get_membership: a user's role and status on a team
    - list_repos: repositories the team can access (string, required)
  - `org`: Organization login that contains the team. (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `team_slug`: Team slug as it appears in the team's URL. Required for all methods except 'list_teams'. (string, optional)
  - `username`: GitHub login. Required for 'get_membership'. (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/project-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/project-light.png"><img src="pkg/octicons/icons/project-light.png" width="20" height="20" alt="project"></picture> Projects</summary>

- **projects_get** - Get details of GitHub Projects resources
  - **Required OAuth Scopes**: `read:project`
  - **Accepted OAuth Scopes**: `project`, `read:project`
  - `field_id`: The field's ID. Required for 'get_project_field' method. (number, optional)
  - `field_names`: Specific list of field names to include in the response when getting a project item (e.g. ["Status", "Priority"]). Resolved server-side to field IDs — pass this instead of 'fields' when you only know the human-readable names. Mutually exclusive with 'fields' — provide one, not both. Only used for 'get_project_item' method. (string[], optional)
  - `fields`: Specific list of field IDs to include in the response when getting a project item (e.g. ["102589", "985201", "169875"]). If neither 'fields' nor 'field_names' is provided, only the title field is included. Mutually exclusive with 'field_names' — provide one, not both. Only used for 'get_project_item' method. (string[], optional)
  - `item_id`: The item's ID. Required for 'get_project_item' method. (number, optional)
  - `method`: The method to execute (string, required)
  - `owner`: The owner (user or organization login). The name is not case sensitive. (string, optional)
  - `owner_type`: Owner type (user or org). If not provided, will be automatically detected. (string, optional)
  - `project_number`: The project's number. (number, optional)
  - `status_update_id`: The node ID of the project status update. Required for 'get_project_status_update' method. (string, optional)
  - `view_id`: The node ID of the project view. Required for 'get_project_view' method. (string, optional)

- **projects_list** - List GitHub Projects resources
  - **Required OAuth Scopes**: `read:project`
  - **Accepted OAuth Scopes**: `project`, `read:project`
  - `after`: Forward pagination cursor from previous pageInfo.nextCursor. (string, optional)
  - `before`: Backward pagination cursor from previous pageInfo.prevCursor (rare). (string, optional)
  - `field_names`: Field names to include when listing project items (e.g. ["Status", "Priority"]). Resolved server-side to field IDs — pass this instead of 'fields' when you only know the human-readable names. Names that fail to resolve return a structured error. Mutually exclusive with 'fields' — provide one, not both. Only used for 'list_project_items' method. (string[], optional)
  - `fields`: Field IDs to include when listing project items (e.g. ["102589", "985201"]). CRITICAL: Always provide to get field values. Without this (and without 'field_names'), only titles returned. Mutually exclusive with 'field_names' — provide one, not both. Only used for 'list_project_items' method. (string[], optional)
  - `method`: The action to perform (string, required)
  - `owner`: The owner (user or organization login). The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (user or org). If not provided, will automatically try both. (string, optional)
  - `per_page`: Results per page (max 50) (number, optional)
  - `project_number`: The project's number. Required for 'list_project_fields', 'list_project_items', 'list_project_views', and 'list_project_status_updates' methods. (number, optional)
  - `query`: Filter/query string. For list_projects: filter by title text and state (e.g. "roadmap is:open"). For list_project_items: advanced filtering using GitHub's project filtering syntax. (string, optional)

- **projects_write** - Manage GitHub Projects
  - **Required OAuth Scopes**: `project`
  - `body`: The body of the status update (markdown). Used for 'create_project_status_update' method. (string, optional)
  - `field_name`: The name of the iteration field (e.g. 'Sprint'). Required for 'create_iteration_field' method. (string, optional)
  - `filter`: Saved view filter; omit on update to preserve it, or pass null to clear it. (string | null, optional)
  - `issue_number`: The issue number. Required for 'add_project_item' when item_type is 'issue'. Also accepted by 'update_project_item' to resolve the item by issue number (combine with item_owner and item_repo). (number, optional)
  - `item_id`: The project item ID. Required for 'delete_project_item'. For 'update_project_item', provide either item_id, or (item_owner + item_repo + issue_number) to resolve the item by issue. (number, optional)
  - `item_owner`: The owner (user or organization) of the repository containing the issue or pull request. Required for 'add_project_item' method. Also accepted by 'update_project_item' when resolving the item by issue number. (string, optional)
  - `item_repo`: The name of the repository containing the issue or pull request. Required for 'add_project_item' method. Also accepted by 'update_project_item' when resolving the item by issue number. (string, optional)
  - `item_type`: The item's type, either issue or pull_request. Required for 'add_project_item' method. (string, optional)
  - `items`: The items to update with the top-level 'updated_field'. Required for 'update_project_items'; prefer it over calling 'update_project_item' in a loop. Each entry must match exactly one reference variant: 'node_id', numeric 'item_id', or 'item_owner' + 'item_repo' + 'issue_number'. Limit: 50 items per call. (object[], optional)
  - `iteration_duration`: Duration in days for iterations of the field (e.g. 7 for weekly, 14 for bi-weekly). Required for 'create_iteration_field' method. (number, optional)
  - `iterations`: Custom iterations for 'create_iteration_field' method. Only set this when you need iterations with varying durations, breaks between them, or specific titles. Otherwise omit it: GitHub auto-creates three iterations of 'iteration_duration' days starting on 'start_date', which is the right choice for most cases. (object[], optional)
  - `layout`: View layout; required when creating a view. (string, optional)
  - `method`: The method to execute (string, required)
  - `name`: View name; required when creating a view. (string, optional)
  - `owner`: The project owner (user or organization login). The name is not case sensitive. (string, required)
  - `owner_type`: Owner type (user or org). Required for 'create_project' method. If not provided for other methods, will be automatically detected. (string, optional)
  - `project_number`: The project's number. Required for all methods except 'create_project'. (number, optional)
  - `pull_request_number`: The pull request number (use when item_type is 'pull_request' for 'add_project_item' method). Provide either issue_number or pull_request_number. (number, optional)
  - `start_date`: Start date in YYYY-MM-DD format. Used for 'create_project_status_update' and 'create_iteration_field' methods. (string, optional)
  - `status`: The status of the project. Used for 'create_project_status_update' method. (string, optional)
  - `target_date`: The target date of the status update in YYYY-MM-DD format. Used for 'create_project_status_update' method. (string, optional)
  - `title`: The project title. Required for 'create_project' method. (string, optional)
  - `updated_field`: The field/value to apply, using {"id": 123, "value": ...} or {"name": "Status", "value": ...}; null clears the field. Required for 'update_project_item' and 'update_project_items', where one top-level field/value applies to every item in a batch. For 'update_project_item' SINGLE_SELECT fields, the name form accepts option names; the ID form expects an option ID. (object, optional)
  - `view_id`: Project view node ID for update or delete; must belong to owner/project_number. (string, optional)
  - `visible_field_names`: Ordered project field names to show on create or replace on update; omit on update to preserve, or pass [] to reset. Mutually exclusive with visible_fields. Roadmap accepts only []. (string[], optional)
  - `visible_fields`: Ordered project field database IDs to show on create or replace on update; omit on update to preserve, or pass [] to reset. Mutually exclusive with visible_field_names. Roadmap accepts only []. (string[], optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/git-pull-request-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/git-pull-request-light.png"><img src="pkg/octicons/icons/git-pull-request-light.png" width="20" height="20" alt="git-pull-request"></picture> Pull Requests</summary>

- **add_comment_to_pending_review** - Add review comment to the requester's latest pending pull request review
  - **Required OAuth Scopes**: `repo`
  - `body`: The text of the review comment (string, required)
  - `line`: The line of the blob in the pull request diff that the comment applies to. For multi-line comments, the last line of the range (number, optional)
  - `owner`: Repository owner (string, required)
  - `path`: The relative path to the file that necessitates a comment (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)
  - `side`: The side of the diff to comment on. LEFT indicates the previous state, RIGHT indicates the new state (string, optional)
  - `startLine`: For multi-line comments, the first line of the range that the comment applies to (number, optional)
  - `startSide`: For multi-line comments, the starting side of the diff that the comment applies to. LEFT indicates the previous state, RIGHT indicates the new state (string, optional)
  - `subjectType`: The level at which the comment is targeted (string, required)

- **add_reply_to_pull_request_comment** - Add reply to pull request comment
  - **Required OAuth Scopes**: `repo`
  - `body`: The text of the reply. Required unless reaction is provided. (string, optional)
  - `commentId`: The numeric ID of the pull request review comment to reply or react to. Use the number from a #discussion_r... anchor, not the GraphQL thread node ID (PRRT_...). (number, required)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number. Required when body is provided. (number, optional)
  - `reaction`: Emoji reaction to add. Required unless body is provided. (string, optional)
  - `repo`: Repository name (string, required)

- **create_pull_request** - Open new pull request
  - **Required OAuth Scopes**: `repo`
  - `base`: Branch to merge into (string, required)
  - `body`: PR description (string, optional)
  - `draft`: Create as draft PR (boolean, optional)
  - `head`: Branch containing changes (string, required)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], optional)
  - `title`: PR title (string, required)

- **list_pull_requests** - List pull requests
  - **Required OAuth Scopes**: `repo`
  - `base`: Filter by base branch (string, optional)
  - `direction`: Sort direction (string, optional)
  - `fields`: Subset of fields to return for each pull request. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body' in particular drops the largest per-result data. (string[], optional)
  - `head`: Filter by head user/org and branch (string, optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `sort`: Sort by (string, optional)
  - `state`: Filter by state (string, optional)

- **merge_pull_request** - Merge pull request
  - **Required OAuth Scopes**: `repo`
  - `commit_message`: Extra detail for merge commit (string, optional)
  - `commit_title`: Title for merge commit (string, optional)
  - `merge_method`: Merge method (string, optional)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

- **pull_request_read** - Get details for a single pull request
  - **Required OAuth Scopes**: `repo`
  - `after`: Cursor for pagination, used only by the get_review_comments method. Pass the endCursor from the previous page's PageInfo to fetch the next page. (string, optional)
  - `method`: Action to specify what pull request data needs to be retrieved from GitHub. 
    Possible options: 
     1. get - Get details of a specific pull request.
     2. get_diff - Get the diff of a pull request.
     3. get_status - Get combined commit status of a head commit in a pull request.
     4. get_files - Get the list of files changed in a pull request. Use with pagination parameters to control the number of results returned.
     5. get_commits - Get the list of commits on a pull request. Use with pagination parameters to control the number of results returned.
     6. get_review_comments - Get review threads on a pull request. Each thread contains logically grouped review comments made on the same code location during pull request reviews. Returns threads with metadata (isResolved, isOutdated, isCollapsed) and their associated comments. Use cursor-based pagination (perPage, after) to control results.
     7. get_reviews - Get the reviews on a pull request. When asked for review comments, use get_review_comments method. Use with pagination parameters to control the number of results returned.
     8. get_comments - Get comments on a pull request. Use this if user doesn't specifically want review comments. Use with pagination parameters to control the number of results returned.
     9. get_check_runs - Get check runs for the head commit of a pull request. Check runs are the individual CI/CD jobs and checks that run on the PR.
     (string, required)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

- **pull_request_review_write** - Write operations (create, submit, delete) on pull request reviews
  - **Required OAuth Scopes**: `repo`
  - `body`: Review comment text (string, optional)
  - `commitID`: SHA of commit to review (string, optional)
  - `event`: Review action to perform. (string, optional)
  - `method`: The write operation to perform on pull request review. (string, required)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)
  - `threadId`: The node ID of the review thread (e.g., PRRT_kwDOxxx). Required for resolve_thread and unresolve_thread methods. Get thread IDs from pull_request_read with method get_review_comments. (string, optional)

- **search_pull_requests** - Search pull requests
  - **Required OAuth Scopes**: `repo`
  - `fields`: Subset of fields to return for each pull request result. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body', 'reactions', and 'labels' in particular drops the largest per-result data. (string[], optional)
  - `order`: Sort order (string, optional)
  - `owner`: Optional repository owner. If provided with repo, only pull requests for this repository are listed. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Search query using GitHub pull request search syntax (string, required)
  - `repo`: Optional repository name. If provided with owner, only pull requests for this repository are listed. (string, optional)
  - `sort`: Sort field by number of matches of categories, defaults to best match (string, optional)

- **update_pull_request** - Edit pull request
  - **Required OAuth Scopes**: `repo`
  - `base`: New base branch name (string, optional)
  - `body`: New description (string, optional)
  - `draft`: Mark pull request as draft (true) or ready for review (false) (boolean, optional)
  - `maintainer_can_modify`: Allow maintainer edits (boolean, optional)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number to update (number, required)
  - `repo`: Repository name (string, required)
  - `reviewers`: GitHub usernames or ORG/team-slug team reviewers to request reviews from (string[], optional)
  - `state`: New state (string, optional)
  - `title`: New title (string, optional)

- **update_pull_request_branch** - Update pull request branch
  - **Required OAuth Scopes**: `repo`
  - `expectedHeadSha`: The expected SHA of the pull request's HEAD ref (string, optional)
  - `owner`: Repository owner (string, required)
  - `pullNumber`: Pull request number (number, required)
  - `repo`: Repository name (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-lock-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-lock-light.png"><img src="pkg/octicons/icons/shield-lock-light.png" width="20" height="20" alt="shield-lock"></picture> Repo Governance</summary>

- **collaborator_write** - Manage repository access
  - **Required OAuth Scopes**: `repo`
  - `method`: The operation to perform:
    - add_user: add a collaborator, or change the role of an existing one
    - remove_user: revoke a user's direct access
    - set_team: grant a team access, or change the role it holds
    - remove_team: revoke a team's access (string, required)
  - `org`: Organization that owns the team. Defaults to the repository owner. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `permission`: Role to grant: 'pull' read, 'triage' manage issues and pull requests, 'push' write, 'maintain' manage without destructive actions, 'admin' full control. Defaults to 'push'. (string, optional)
  - `repo`: Repository name (string, required)
  - `team_slug`: Team slug as it appears in the team's URL, not its display name. Required for 'set_team' and 'remove_team'. (string, optional)
  - `username`: GitHub login. Required for 'add_user' and 'remove_user'. (string, optional)

- **collaborators_read** - Read repository access
  - **Required OAuth Scopes**: `repo`
  - `affiliation`: Which collaborators to list: 'direct' for those granted access on the repository itself, 'outside' for collaborators who are not organization members, 'all' (default) for everyone visible. (string, optional)
  - `method`: The method to execute:
    - list_collaborators: users with access, and the role each holds
    - get_permission: one user's effective permission, including access inherited from a team or from organization membership
    - list_teams: teams granted access to the repository (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `permission`: Only list collaborators holding this role. Used by 'list_collaborators'. (string, optional)
  - `repo`: Repository name (string, required)
  - `username`: GitHub login. Required for 'get_permission'. (string, optional)

- **merge_policy_read** - Read merge policy for a branch
  - **Required OAuth Scopes**: `repo`
  - `branch`: The branch a pull request would merge into. Defaults to the repository's default branch. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **repository_settings_read** - Read repository settings
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)

- **repository_settings_write** - Change repository settings
  - **Required OAuth Scopes**: `repo`
  - `allow_auto_merge`: Allow pull requests to be queued for automatic merge once requirements pass. (boolean, optional)
  - `allow_forking`: Allow the repository to be forked. (boolean, optional)
  - `allow_merge_commit`: Allow merging pull requests with a merge commit. (boolean, optional)
  - `allow_rebase_merge`: Allow rebase merging pull requests. (boolean, optional)
  - `allow_squash_merge`: Allow squash merging pull requests. (boolean, optional)
  - `allow_update_branch`: Offer to update a pull request branch that is behind its base. (boolean, optional)
  - `archived`: Archive the repository, making it read-only. Reversible by setting it back to false, but while archived no other write tool will succeed against this repository. (boolean, optional)
  - `default_branch`: Branch to use as the default. The branch must already exist. (string, optional)
  - `delete_branch_on_merge`: Delete the head branch automatically after a pull request merges. (boolean, optional)
  - `description`: Short repository description. (string, optional)
  - `has_discussions`: Enable Discussions. (boolean, optional)
  - `has_issues`: Enable the Issues tab. (boolean, optional)
  - `has_projects`: Enable the Projects tab. (boolean, optional)
  - `has_wiki`: Enable the Wiki tab. (boolean, optional)
  - `homepage`: Homepage URL. (string, optional)
  - `merge_commit_message`: Default message body for merge commits. (string, optional)
  - `merge_commit_title`: Default title for merge commits. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)
  - `squash_merge_commit_message`: Default message body for squash merge commits. (string, optional)
  - `squash_merge_commit_title`: Default title for squash merge commits. (string, optional)
  - `topics`: Repository topics. Replaces the current list; an empty array clears it. (string[], optional)
  - `web_commit_signoff_required`: Require a sign-off on web-based commits. (boolean, optional)

- **ruleset_write** - Write repository rulesets
  - **Required OAuth Scopes**: `repo`
  - `bypass_actors`: Actors allowed to bypass the ruleset. Replaces the current list when given; pass an empty array to remove every bypass. (object[], optional)
  - `enforcement`: Whether the ruleset is enforced. 'evaluate' reports violations without blocking (available on some plans). Defaults to 'active' on create. Required for 'set_enforcement'. (string, optional)
  - `exclude_refs`: Ref patterns exempted from the ruleset. Replaces the current exclude list when given. (string[], optional)
  - `include_refs`: Ref patterns the ruleset applies to, e.g. ['refs/heads/main', 'refs/heads/release/*'] or the special value '~DEFAULT_BRANCH' or '~ALL'. Replaces the current include list when given. (string[], optional)
  - `method`: Operation to perform: 'create', 'update', 'delete', or 'set_enforcement' (string, required)
  - `name`: Ruleset name. Required for 'create'. For the other methods it identifies the ruleset when ruleset_id is not given; combined with ruleset_id on 'update' it renames the ruleset. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)
  - `rules`: The rules to enforce. Only the keys you provide are changed; every other rule on the ruleset, including rules this tool does not model, is preserved. (object, optional)
  - `ruleset_id`: Numeric ruleset ID. Identifies the ruleset for 'update', 'delete' and 'set_enforcement'; takes precedence over name. (number, optional)
  - `target`: What the ruleset applies to. Defaults to 'branch' on create. (string, optional)

- **rulesets_read** - Read repository rulesets
  - **Required OAuth Scopes**: `repo`
  - `branch`: Branch name. Required for 'get_branch_rules'. (string, optional)
  - `includes_parents`: Include rulesets inherited from the organization or enterprise. Defaults to true. Used by 'list_rulesets' and 'get_ruleset'. (boolean, optional)
  - `method`: The method to execute:
    - list_rulesets: every ruleset applying to the repository
    - get_ruleset: one ruleset in full, by ruleset_id or by name
    - get_branch_rules: the rules in force on a branch, whatever ruleset they come from (string, required)
  - `name`: Ruleset name. Identifies the ruleset for 'get_ruleset' when ruleset_id is not given. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `ruleset_id`: Numeric ruleset ID. Identifies the ruleset for 'get_ruleset'; takes precedence over name. (number, optional)

- **webhook_write** - Manage webhooks
  - **Required OAuth Scopes**: `repo`
  - `active`: Whether the webhook is active. Defaults to true on create. (boolean, optional)
  - `content_type`: Payload format. Defaults to 'form'. (string, optional)
  - `delivery_id`: Delivery ID. Required for 'redeliver'. (integer, optional)
  - `events`: GitHub event names that trigger this webhook, e.g. 'push', 'pull_request'. Defaults to ['push'] on create. (string[], optional)
  - `hook_id`: Webhook ID. Required for all methods except 'create'. (integer, optional)
  - `insecure_ssl`: '0' to verify SSL certs (default), '1' to skip verification. (string, optional)
  - `method`: The operation to perform:
    - create: create a new webhook
    - update: change an existing webhook's URL, secret, content type, SSL verification, events or active state
    - delete: permanently remove a webhook
    - ping: send a ping event to test delivery
    - redeliver: resend a past delivery (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)
  - `secret`: Shared secret GitHub uses to sign payloads. Optional. Never returned by any read tool once set. (string, optional)
  - `url`: Payload delivery URL. Required for 'create'; optional for 'update'. (string, optional)

- **webhooks_read** - Read webhooks
  - **Required OAuth Scopes**: `repo`
  - `delivery_id`: Delivery ID. Required for 'get_delivery'. (integer, optional)
  - `hook_id`: Webhook ID. Required for all methods except 'list'. (integer, optional)
  - `method`: The method to execute:
    - list: all webhooks configured on the repository
    - get: one webhook's details
    - get_config: one webhook's delivery config (URL, content type, SSL verification)
    - list_deliveries: recent delivery attempts for a webhook
    - get_delivery: one delivery's status (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/repo-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/repo-light.png"><img src="pkg/octicons/icons/repo-light.png" width="20" height="20" alt="repo"></picture> Repositories</summary>

- **compare_commits** - Compare two refs
  - **Required OAuth Scopes**: `repo`
  - `base`: The ref to compare from: a commit SHA, branch name or tag name. To compare across forks, use 'owner:branch'. (string, required)
  - `detail`: How much per-file detail to include: 'none' omits the file list, 'stats' (default) lists changed files with counts, 'full_patch' includes the diff for each file. (string, optional)
  - `head`: The ref to compare to: a commit SHA, branch name or tag name. To compare across forks, use 'owner:branch'. (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **create_branch** - Create branch
  - **Required OAuth Scopes**: `repo`
  - `branch`: Name for new branch (string, required)
  - `from_branch`: Source branch (defaults to repo default) (string, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **create_or_update_file** - Create or update file
  - **Required OAuth Scopes**: `repo`
  - `allow_symlink_write`: Set true to update a symbolic link itself; content must be its new target path. (boolean, optional)
  - `branch`: Branch to create/update the file in (string, required)
  - `content`: Content of the file, exactly as it should appear once written. Do not base64-encode it; this server does that before calling the REST API. (string, required)
  - `message`: Commit message (string, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: Path where to create/update the file (string, required)
  - `repo`: Repository name (string, required)
  - `sha`: The blob SHA of the file being replaced. Required if the file already exists. (string, optional)

- **create_repository** - Create repository
  - **Required OAuth Scopes**: `repo`
  - `autoInit`: Initialize with README (boolean, optional)
  - `description`: Repository description (string, optional)
  - `name`: Repository name (string, required)
  - `organization`: Organization to create the repository in (omit to create in your personal account) (string, optional)
  - `private`: Whether the repository should be private. Defaults to true (private) when omitted. (boolean, optional)

- **delete_file** - Delete file
  - **Required OAuth Scopes**: `repo`
  - `branch`: Branch to delete the file from (string, required)
  - `message`: Commit message (string, required)
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: Path to the file to delete (string, required)
  - `repo`: Repository name (string, required)

- **delete_repository** - Delete repository
  - **Required OAuth Scopes (all required)**: `delete_repo`, `repo`
  - `owner`: Repository owner (username or organization) (string, required)
  - `repo`: Repository name (string, required)

- **fork_repository** - Fork repository
  - **Required OAuth Scopes**: `repo`
  - `organization`: Organization to fork to (string, optional)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **get_commit** - Get commit details
  - **Required OAuth Scopes**: `repo`
  - `detail`: Level of detail to include for changed files. "none" omits stats and files entirely. "stats" (default) includes per-file metadata: filename, status, and lines-of-code counts (additions, deletions, changes), with no patch content. "full_patch" additionally includes the unified diff content for each file and can be very large. (string, optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `sha`: Commit SHA, branch name, or tag name (string, required)

- **get_file_contents** - Get file or directory contents
  - **Required OAuth Scopes**: `repo`
  - `fields`: Subset of fields to return for each entry when the path is a directory. If omitted, all fields are returned. Ignored when the path is a single file. Use this to reduce response size when listing directories and you only need specific fields, e.g. just 'name' and 'type'. (string[], optional)
  - `owner`: Repository owner (username or organization) (string, required)
  - `path`: Path to file/directory (string, optional)
  - `ref`: Accepts optional git refs such as `refs/tags/{tag}`, `refs/heads/{branch}` or `refs/pull/{pr_number}/head` (string, optional)
  - `repo`: Repository name (string, required)
  - `sha`: Accepts optional commit SHA. If specified, it will be used instead of ref (string, optional)

- **get_latest_release** - Get latest release
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **get_release_by_tag** - Get a release by tag name
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `tag`: Tag name (e.g., 'v1.0.0') (string, required)

- **get_tag** - Get tag details
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)
  - `tag`: Tag name (string, required)

- **list_branches** - List branches
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **list_commits** - List commits
  - **Required OAuth Scopes**: `repo`
  - `author`: Author username or email address to filter commits by (string, optional)
  - `fields`: Subset of fields to return for each commit. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields, e.g. just 'sha' and 'html_url'. (string[], optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `path`: Only commits containing this file path will be returned (string, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)
  - `sha`: Commit SHA, branch or tag name to list commits of. If not provided, uses the default branch of the repository. If a commit SHA is provided, will list commits up to that SHA. (string, optional)
  - `since`: Only commits after this date will be returned (ISO 8601 format: YYYY-MM-DDTHH:MM:SSZ or YYYY-MM-DD) (string, optional)
  - `until`: Only commits before this date will be returned (ISO 8601 format: YYYY-MM-DDTHH:MM:SSZ or YYYY-MM-DD) (string, optional)

- **list_releases** - List releases
  - **Required OAuth Scopes**: `repo`
  - `fields`: Subset of fields to return for each release. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'body' in particular drops the largest per-release data. (string[], optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **list_repository_collaborators** - List repository collaborators
  - **Required OAuth Scopes**: `repo`
  - `affiliation`: Filter by affiliation. Can be one of: 'outside' (outside collaborators), 'direct' (all with permissions regardless of org membership), 'all' (all collaborators). Default: 'all' (string, optional)
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (default 1, min 1) (number, optional)
  - `perPage`: Results per page for pagination (default 30, min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **list_tags** - List tags
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: Repository name (string, required)

- **push_files** - Push files to repository
  - **Required OAuth Scopes**: `repo`
  - `branch`: Branch to push to (string, required)
  - `files`: Array of file objects to push, each object with path (string) and content (string) (object[], required)
  - `message`: Commit message (string, required)
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **release_write** - Write operations on releases
  - **Required OAuth Scopes**: `repo`
  - `body`: Release notes in markdown. On 'update', pass an empty string to clear them; omit it to leave the current notes unchanged. (string, optional)
  - `configuration_file_path`: Path to a release notes configuration file for 'generate_notes' (defaults to .github/release.yml). (string, optional)
  - `discussion_category_name`: Create a discussion of this category for the release. The category must already exist in the repository. (string, optional)
  - `draft`: Whether the release is a draft (unpublished). (boolean, optional)
  - `generate_release_notes`: Auto-generate release notes on 'create'. When body is also given, the generated notes are appended to it. (boolean, optional)
  - `make_latest`: Whether to mark this release as the repository's latest. 'legacy' defers to GitHub's date-and-semver heuristic. (string, optional)
  - `method`: Operation to perform: 'create', 'update', 'delete', or 'generate_notes' (string, required)
  - `name`: Release title. On 'update', pass an empty string to clear it; omit it to leave the current title unchanged. (string, optional)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `prerelease`: Whether the release is a prerelease. (boolean, optional)
  - `previous_tag_name`: Tag to generate notes against for 'generate_notes'. Defaults to the previous release's tag. (string, optional)
  - `release_id`: Numeric release ID. Identifies the release for 'update' and 'delete'; takes precedence over tag_name. (number, optional)
  - `repo`: Repository name (string, required)
  - `tag_name`: Tag the release points at. Required for 'create' and 'generate_notes'. For 'update' and 'delete' it identifies the release when release_id is not given; for 'update' it also retags the release when release_id is given. (string, optional)
  - `target_commitish`: Branch or commit SHA the tag is created from when it does not exist yet. Defaults to the repository's default branch. (string, optional)

- **search_code** - Search code
  - **Required OAuth Scopes**: `repo`
  - `fields`: Subset of fields to return for each code search result. If omitted, all fields are returned. Use this to reduce response size when you only need specific fields; omitting 'repository' and 'text_matches' in particular drops the largest per-result data. (string[], optional)
  - `order`: Sort order for results (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Search query (GitHub code search REST). Implicit AND between terms; supports `OR`, `NOT`, and `"quoted phrase"` for exact match. Qualifiers: `repo:owner/repo`, `org:`, `user:`, `language:`, `path:dir` (prefix match), `filename:exact.ext`, `extension:`, `in:file`, `in:path`, `size:`, `is:archived`, `is:fork`. Max 256 chars. Examples: `WithContext language:go org:github`; `"package main" repo:o/r`; `func extension:go path:cmd repo:o/r`; `NOT TODO language:go repo:o/r`. (string, required)
  - `sort`: Sort field ('indexed' only) (string, optional)

- **search_commits** - Search commits
  - **Required OAuth Scopes**: `repo`
  - `order`: Sort order (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Commit search query (GitHub commit search REST). Searches commit messages on the default branch only. Scope the search with `repo:owner/repo`, `org:`, or `user:` (queries without a scope qualifier match across all of GitHub and are usually not what you want). Other qualifiers: `author:`, `committer:`, `author-name:`, `committer-name:`, `author-email:`, `committer-email:`, `author-date:`, `committer-date:` (supports `>`, `<`, `>=`, `<=`, and `YYYY-MM-DD..YYYY-MM-DD` ranges), `merge:true|false`, `hash:`, `tree:`, `parent:`, `is:public`. Examples: `repo:owner/repo fix panic`; `org:github author:defunkt committer-date:>=2024-01-01`; `"refactor cache" repo:o/r`; `hash:abc1234 repo:o/r`. (string, required)
  - `sort`: Sort by author or committer date (defaults to best match) (string, optional)

- **search_repositories** - Search repositories
  - **Required OAuth Scopes**: `repo`
  - `minimal_output`: Return minimal repository information (default: true). When false, returns full GitHub API repository objects. (boolean, optional)
  - `order`: Sort order (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: Repository search query. Examples: 'machine learning in:name stars:>1000 language:python', 'topic:react', 'user:facebook'. Supports advanced search syntax for precise filtering. (string, required)
  - `sort`: Sort repositories by field, defaults to best match (string, optional)

- **tag_write** - Write operations on tags
  - **Required OAuth Scopes**: `repo`
  - `from_ref`: Commit SHA, branch name or tag name the new tag points at. Defaults to the head of the repository's default branch. Used by 'create'. (string, optional)
  - `message`: Annotation message. Supplying it creates an annotated tag object instead of a lightweight tag. Used by 'create'. (string, optional)
  - `method`: Operation to perform: 'create' or 'delete' (string, required)
  - `owner`: Repository owner (username or organization name) (string, required)
  - `repo`: Repository name (string, required)
  - `tag`: Tag name, without the 'refs/tags/' prefix (e.g. 'v1.2.3'). (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-lock-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-lock-light.png"><img src="pkg/octicons/icons/shield-lock-light.png" width="20" height="20" alt="shield-lock"></picture> Secret Protection</summary>

- **get_secret_scanning_alert** - Get secret scanning alert
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `alertNumber`: The number of the alert. (number, required)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)

- **list_secret_scanning_alerts** - List secret scanning alerts
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `owner`: The owner of the repository. (string, required)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `repo`: The name of the repository. (string, required)
  - `resolution`: Filter by resolution (string, optional)
  - `secret_type`: A comma-separated list of secret types to return. All default secret patterns are returned. To return generic patterns, pass the token name(s) in the parameter. (string, optional)
  - `state`: Filter by state (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/shield-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/shield-light.png"><img src="pkg/octicons/icons/shield-light.png" width="20" height="20" alt="shield"></picture> Security Advisories</summary>

- **get_global_security_advisory** - Get a global security advisory
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `ghsaId`: GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx). (string, required)

- **list_global_security_advisories** - List global security advisories
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `affects`: Filter advisories by affected package or version (e.g. "package1,package2@1.0.0"). (string, optional)
  - `cveId`: Filter by CVE ID. (string, optional)
  - `cwes`: Filter by Common Weakness Enumeration IDs (e.g. ["79", "284", "22"]). (string[], optional)
  - `ecosystem`: Filter by package ecosystem. (string, optional)
  - `ghsaId`: Filter by GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx). (string, optional)
  - `isWithdrawn`: Whether to only return withdrawn advisories. (boolean, optional)
  - `modified`: Filter by publish or update date or date range (ISO 8601 date or range). (string, optional)
  - `published`: Filter by publish date or date range (ISO 8601 date or range). (string, optional)
  - `severity`: Filter by severity. (string, optional)
  - `type`: Advisory type. (string, optional)
  - `updated`: Filter by update date or date range (ISO 8601 date or range). (string, optional)

- **list_org_repository_security_advisories** - List org repository security advisories
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `direction`: Sort direction. (string, optional)
  - `org`: The organization login. (string, required)
  - `sort`: Sort field. (string, optional)
  - `state`: Filter by advisory state. (string, optional)

- **list_repository_security_advisories** - List repository security advisories
  - **Required OAuth Scopes**: `security_events`
  - **Accepted OAuth Scopes**: `repo`, `security_events`
  - `direction`: Sort direction. (string, optional)
  - `owner`: The owner of the repository. (string, required)
  - `repo`: The name of the repository. (string, required)
  - `sort`: Sort field. (string, optional)
  - `state`: Filter by advisory state. (string, optional)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/star-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/star-light.png"><img src="pkg/octicons/icons/star-light.png" width="20" height="20" alt="star"></picture> Stargazers</summary>

- **list_starred_repositories** - List starred repositories
  - **Required OAuth Scopes**: `repo`
  - `direction`: The direction to sort the results by. (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `sort`: How to sort the results. Can be either 'created' (when the repository was starred) or 'updated' (when the repository was last pushed to). (string, optional)
  - `username`: Username to list starred repositories for. Defaults to the authenticated user. (string, optional)

- **star_repository** - Star repository
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

- **unstar_repository** - Unstar repository
  - **Required OAuth Scopes**: `repo`
  - `owner`: Repository owner (string, required)
  - `repo`: Repository name (string, required)

</details>

<details>

<summary><picture><source media="(prefers-color-scheme: dark)" srcset="pkg/octicons/icons/people-dark.png"><source media="(prefers-color-scheme: light)" srcset="pkg/octicons/icons/people-light.png"><img src="pkg/octicons/icons/people-light.png" width="20" height="20" alt="people"></picture> Users</summary>

- **search_users** - Search users
  - **Required OAuth Scopes**: `repo`
  - `order`: Sort order (string, optional)
  - `page`: Page number for pagination (min 1) (number, optional)
  - `perPage`: Results per page for pagination (min 1, max 100) (number, optional)
  - `query`: User search query. Examples: 'john smith', 'location:seattle', 'followers:>100'. Search is automatically scoped to type:user. (string, required)
  - `sort`: Sort users by number of followers or repositories, or when the person joined GitHub. (string, optional)

</details>
<!-- END AUTOMATED TOOLS -->

### Additional Tools in Remote GitHub MCP Server

<details>

<summary>Copilot</summary>

- **create_pull_request_with_copilot** - Perform task with GitHub Copilot coding agent
  - `owner`: Repository owner. You can guess the owner, but confirm it with the user before proceeding. (string, required)
  - `repo`: Repository name. You can guess the repository name, but confirm it with the user before proceeding. (string, required)
  - `problem_statement`: Detailed description of the task to be performed (e.g., 'Implement a feature that does X', 'Fix bug Y', etc.) (string, required)
  - `title`: Title for the pull request that will be created (string, required)
  - `base_ref`: Git reference (e.g., branch) that the agent will start its work from. If not specified, defaults to the repository's default branch (string, optional)

</details>

<details>

<summary>Copilot Spaces</summary>

- **Authentication note**
  - Fine-grained PATs are not hidden by classic PAT scope filtering, so these tools may still appear even when the token cannot use them.
  - For org-owned spaces, fine-grained PATs must be installed on the owning organization and include `organization_copilot_spaces: read`.
  - If an org-owned space contains repository-backed resources, the token must also have access to every referenced repository or the space may be treated as not found.

- **get_copilot_space** - Get Copilot Space
  - `owner`: The owner of the space. (string, required)
  - `name`: The name of the space. (string, required)

- **list_copilot_spaces** - List Copilot Spaces

</details>

<details>

<summary>GitHub Support Docs Search</summary>

- **github_support_docs_search** - Retrieve documentation relevant to answer GitHub product and support questions. Support topics include: GitHub Actions Workflows, Authentication, GitHub Support Inquiries, Pull Request Practices, Repository Maintenance, GitHub Pages, GitHub Packages, GitHub Discussions, Copilot Spaces
  - `query`: Input from the user about the question they need answered. This is the latest raw unedited user message. You should ALWAYS leave the user message as it is, you should never modify it. (string, required)

</details>

## Read-Only Mode

To run the server in read-only mode, you can use the `--read-only` flag. This will only offer read-only tools, preventing any modifications to repositories, issues, pull requests, etc.

```bash
./github-mcp-server --read-only
```

When using Docker, you can pass the read-only mode as an environment variable:

```bash
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_READ_ONLY=1 \
  ghcr.io/github/github-mcp-server
```

## Lockdown Mode

Lockdown mode limits the content that the server will surface from public repositories. When enabled, the server checks whether the author of each item has push access to the repository. Private repositories are unaffected, and collaborators keep full access to their own content.

Lockdown mode is a best-effort content filter intended to reduce the risk of prompt injection from untrusted repository content (issues, pull requests, comments, commits, etc.). It is **not** an authorization boundary: it does not change what the underlying GitHub credential can read or write, and content withheld from a filtered tool response may still be reachable through other tools or direct GitHub API access with the same credential.

As an intentional exception, content authored by a small set of trusted bot accounts (currently `github-actions[bot]` and `copilot`) is always treated as safe, regardless of push access. This avoids filtering routine automation output (e.g. CI-generated commits or comments) that would otherwise be withheld under lockdown mode.

```bash
./github-mcp-server --lockdown-mode
```

When running with Docker, set the corresponding environment variable:

```bash
docker run -i --rm \
  -e GITHUB_PERSONAL_ACCESS_TOKEN=<your-token> \
  -e GITHUB_LOCKDOWN_MODE=1 \
  ghcr.io/github/github-mcp-server
```

In HTTP mode, this flag (or `GITHUB_LOCKDOWN_MODE`) is an upper bound: the `X-MCP-Lockdown` request header can enable lockdown mode when the operator has not, but it cannot disable lockdown mode the operator has already enabled. See the [Server Configuration Guide](docs/server-configuration.md#lockdown-mode) for details.

The behavior of lockdown mode depends on the tool invoked.

Following tools will return an error when the author lacks the push access:

- `issue_read:get`
- `pull_request_read:get`
- `pull_request_read:get_diff`
- `pull_request_read:get_files`
- `pull_request_read:get_commits`

Following tools will filter out content from users lacking the push access:

- `issue_read:get_comments`
- `issue_read:get_sub_issues`
- `pull_request_read:get_comments`
- `pull_request_read:get_review_comments`
- `pull_request_read:get_reviews`

## i18n / Overriding Descriptions

The descriptions of the tools can be overridden by creating a
`github-mcp-server-config.json` file in the same directory as the binary.

The file should contain a JSON object with the tool names as keys and the new
descriptions as values. For example:

```json
{
  "TOOL_ADD_ISSUE_COMMENT_DESCRIPTION": "an alternative description",
  "TOOL_CREATE_BRANCH_DESCRIPTION": "Create a new branch in a GitHub repository"
}
```

You can create an export of the current translations by running the binary with
the `--export-translations` flag.

This flag will preserve any translations/overrides you have made, while adding
any new translations that have been added to the binary since the last time you
exported.

```sh
./github-mcp-server --export-translations
cat github-mcp-server-config.json
```

You can also use ENV vars to override the descriptions. The environment
variable names are the same as the keys in the JSON file, prefixed with
`GITHUB_MCP_` and all uppercase.

For example, to override the `TOOL_ADD_ISSUE_COMMENT_DESCRIPTION` tool, you can
set the following environment variable:

```sh
export GITHUB_MCP_TOOL_ADD_ISSUE_COMMENT_DESCRIPTION="an alternative description"
```

### Overriding Server Name and Title

The same override mechanism can be used to customize the MCP server's `name` and
`title` fields in the initialization response. This is useful when running
multiple GitHub MCP Server instances (e.g., one for github.com and one for
GitHub Enterprise Server) so that agents can distinguish between them.

| Key | Environment Variable | Default |
|-----|---------------------|---------|
| `SERVER_NAME` | `GITHUB_MCP_SERVER_NAME` | `github-mcp-server` |
| `SERVER_TITLE` | `GITHUB_MCP_SERVER_TITLE` | `GitHub MCP Server` |

For example, to configure a server instance for GitHub Enterprise Server:

```json
{
  "SERVER_NAME": "ghes-mcp-server",
  "SERVER_TITLE": "GHES MCP Server"
}
```

Or using environment variables:

```sh
export GITHUB_MCP_SERVER_NAME="ghes-mcp-server"
export GITHUB_MCP_SERVER_TITLE="GHES MCP Server"
```

## Library Usage

The exported Go API of this module should currently be considered unstable, and subject to breaking changes. In the future, we may offer stability; please file an issue if there is a use case where this would be valuable.

## Contributing

Contributions are welcome. Before opening a pull request, please read the [contributing guide](CONTRIBUTING.md) for setup, testing, linting, and documentation generation instructions.

## Support

For help using the GitHub MCP Server, see the [support guide](SUPPORT.md). If you have found a bug or want to request a feature, please search existing issues before opening a new one.

## Security

Please do not report security vulnerabilities through public issues. Follow the instructions in the [security policy](SECURITY.md) to report vulnerabilities responsibly.

## License

This project is licensed under the terms of the MIT open source license. Please refer to [MIT](./LICENSE) for the full terms.
