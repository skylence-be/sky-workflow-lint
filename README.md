# sky-workflow-lint

Standalone open-source linter for [Skylence](https://skylence.be) `.sky` workflow files.

## Install

Download a prebuilt binary from [GitHub Releases](https://github.com/skylence-be/sky-workflow-lint/releases), or build from source:

```sh
go install github.com/skylence-be/sky-workflow-lint@latest
```

## Usage

```sh
sky-workflow-lint [--format text|json|sarif] [--repo-root <path>] <file|glob>...
```

**Exit codes:** `0` = no problems, `1` = problems found, `2` = tool error.

### Examples

```sh
# Lint all workflows in the current directory
sky-workflow-lint workflows/*.sky

# JSON output (single flat array across all files)
sky-workflow-lint --format json workflows/*.sky

# SARIF for GitHub Code Scanning
sky-workflow-lint --format sarif --repo-root . workflows/*.sky
```

## GitHub Action

Use [skylence-be/sky-lint-action](https://github.com/skylence-be/sky-lint-action) to run this linter in CI.

## License

Apache-2.0. See [LICENSE](LICENSE).
