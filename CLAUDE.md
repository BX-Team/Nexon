# Nexon

Multi-protocol VPN control-plane for [Xray](https://github.com/XTLS/Xray-core) nodes — a single Go binary exposing a **CLI** (cobra) and an interactive **TUI** (Bubble Tea), plus a **subscription server**. No web panel. SQLite (WAL) is the single source of truth; users are generated with a full proxy bundle and selectively pushed to nodes over the Xray gRPC API.

## Architecture

Module `github.com/BX-Team/Nexon` (binary `cmd/nexon`). Everything else lives under `internal/`:

| Package         | Responsibility                                                                                     |
| --------------- | -------------------------------------------------------------------------------------------------- |
| `cli/`          | cobra commands (`user`, `node`, `group`, `clients`, `template`, `settings`, `serve`, `tui`, …). `root.go` opens the store, seeds defaults, and builds the `core.Service` shared by every command. |
| `tui/`          | Bubble Tea cockpit: one file per tab (dashboard, users, nodes, groups, clients, templates, settings) plus `form`/`format` helpers. |
| `core/`         | The service layer — **all** business logic shared by CLI, TUI and the sub server. `Service` is the façade over the store and node connectors (`user`, `node`, `sub`, `traffic`, `groups`, `templates`, `clients`, `inbound`, `admin`). |
| `store/`        | SQLite (pure-Go `modernc.org/sqlite`, no CGO): embedded ordered `migrations/`, `queries`, `models`, and `seed` (default UA→format rules / settings). Single source of truth. |
| `node/`         | `NodeConnector` over the Xray gRPC API — `grpc` (real Handler/Stats), `stub` (logging-only), `connector`, `account`. Selected via a `node.Factory`. |
| `secrets/`      | Per-user proxy bundle generation (VLESS/Trojan/SS/…) and subscription tokens. |
| `subgen/`       | Subscription format generators (`base64`, `links`, `xray`, `clash`, `singbox`) and custom `template` rendering; `BuildEndpoints` turns a user + inbounds into connectable endpoints. |
| `subserver/`    | The sub HTTP server: `detect` (User-Agent → format rules), HWID device handling, and the browser HTML dashboard. |
| `config/`       | `NEXON_*` environment configuration (`config.Default()`). |

- Data flow: **CLI / TUI / sub server → `core.Service` → SQLite** (source of truth) **→ `NodeConnector` (gRPC) → Xray nodes**.
- `core.Service` is built once in `cli/root.go` with a `node.Factory`. A nil factory means the real gRPC connector (`node.DefaultFactory`); `NEXON_NODE_MODE=stub` swaps in `node.StubFactory` for local work without a live node.
- The store is opened once per invocation; migrations and `SeedDefaults`/`SeedTemplates` run automatically on startup.

## Commands

```bash
go build -o nexon ./cmd/nexon   # dev build
go test ./...                   # all tests
go vet ./...                    # static checks
gofmt -l internal cmd           # list files needing formatting (should be empty)

nix build .#nexon               # reproducible build → ./result/bin/nexon
nix run .#nexon -- --help       # run without installing
nix develop                     # dev shell: go, gopls, sqlite
```

Run locally against a logging-only stub node (no VPS, no live Xray):

```bash
NEXON_DATA_DIR=./nexon-data NEXON_NODE_MODE=stub ./nexon user add alice --data-limit 100G
NEXON_DATA_DIR=./nexon-data NEXON_NODE_MODE=stub ./nexon serve   # sub server :8080 + poller
```

Before every commit these must pass: `gofmt -l` (empty), `go vet ./...`, `go test ./...`.

## Code Guidelines

### Comments
- NO file-header banner comments and NO "heading"/divider comments like `// --- helpers ---` or `// ==== VLESS ====`. Group code with functions, not comment art.
- Avoid inline `//` comments. Add one only when the code is genuinely non-obvious (a real footgun) — e.g. a wire-format quirk, a subtle SQL/migration ordering constraint, a device-limit edge case. Then keep it to a line or two.
- Doc comments on exported identifiers are expected, but keep them to a single line describing intent. Code should read for itself.
- Don't narrate the obvious (`// loop over nodes`). If a comment restates the next line, delete it.
- Keep every comment as short as possible — the fewest words that convey the non-obvious bit. Prefer one line; never write a paragraph where a clause will do.

### Style
- `gofmt` is the source of truth — never hand-format against it. Run `gofmt -l` before committing.
- Match the surrounding code: follow the existing package's naming, error-wrapping (`fmt.Errorf("...: %w", err)`), and receiver idioms in the file you're editing.
- Business logic belongs in `core` — CLI/TUI/sub-server call the `Service`, they don't reach into the store or reimplement rules. Read/format helpers may stay in the presentation layer.
- Structured logging is `log/slog` (`slog.Info/Warn/Debug` with key/value pairs), not `fmt.Print`/`log`.

### User-facing language
- Operator-visible strings are **Russian**: `store.AddLog` event-log messages, validation errors returned from `core` to the CLI/TUI (e.g. `fmt.Errorf("название обязательно")`), and the browser dashboard (`subserver/dashboard.go`).
- Structured `slog` logs, code comments, and doc comments stay **English**.
- When you add a message, match the language of its neighbours in that layer — don't mix.

## Nexon gotchas
- **Migrations are append-only.** Files in `store/migrations/` are embedded (`//go:embed`) and applied in filename order, tracked in `schema_migrations`. Never edit a migration that may already be applied — add a new `NNNN_*.sql`. `SeedDefaults` runs *after* migrations and only seeds when a table is empty, so repair existing rows with a migration, not by editing the seed.
- **Stub vs real node.** `core` talks to nodes only through `node.Factory`/`NodeConnector`. Keep new node behaviour behind that interface so `NEXON_NODE_MODE=stub` and the tests (`node.StubFactory`) keep working without a live Xray.
- **Subscription output is driven by data, not code branches.** UA→format detection lives in the seeded `sub_rules` / `client_apps` (regex, first match wins, `base64` default); output formats live in `subgen`. When a client is misdetected, fix the rule/pattern, not a special case in the server.
- **HWID/device limit** is enforced in `core.Subscription`: a device is keyed by HWID or, absent that, User-Agent; revoked rows must be re-checked against the limit on return so the limit can't be bypassed by retrying (see the regression test in `core/sub_test.go`).
- **Nix `vendorHash`.** `flake.nix` pins a `vendorHash` of the Go module set. Changing `go.mod`/`go.sum` invalidates it — rebuild and update the hash (the nix build error prints the expected value). Source-only changes don't affect it. A git-based flake only sees *tracked* files, so `git add` new files before `nix build`.

## Bash Guidelines
- Prefer tool-native flags over piping to truncate (`git log -n 10`, `go test ./internal/core/ -run TestX`, `go build ./...`). Read the full output rather than blindly `| head`.
- Don't create scratch files (scripts, notes) unless asked.
- When given failing checks, just fix them — don't argue about who introduced them.
