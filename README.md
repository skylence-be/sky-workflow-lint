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

**Exit codes:** `0` = no problems (or warnings only), `1` = blocking problems found, `2` = tool error.

### `bash_file` nodes

A bash node may carry its script in a separate file via `bash_file = "./script.sh"`
(relative to the `.sky` file) instead of an inline `bash = "..."`. The two are
mutually exclusive. Related lint codes:

- `SKY-WF-099` — both `bash` and `bash_file` set on the same node (blocking; also
  rejected at parse time as `SKY-WF-024`).
- `SKY-WF-100` — `bash_file` references a script that does not exist (blocking).
- `SKY-WF-101` — bash syntax / shellcheck findings in the referenced script
  (warning, non-blocking; does not affect the exit code). shellcheck is used when
  it is on `PATH`; its absence is silent.

### Declared workflow params

Workflows may declare typed run inputs in `⊕meta⊕` using `params.<name>.*` keys:

```sky
params.title.type = "string"
params.title.required = true
params.priority.type = "enum"
params.priority.enum = ["low", "high"]
params.priority.default = "low"
```

Supported types are `string`, `number`, `boolean`, and `enum`. Defaults are
validated against the declared type. `{{var}}` references that are neither
declared params nor known trigger-injected vars emit `SKY-WF-104` as a warning
during rollout. Invoke nodes are cross-checked against target workflow params
when lint runs with workflow roots.

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
