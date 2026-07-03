<div align="center">

# Nexon

Multi-protocol VPN control-plane for Xray nodes — CLI + TUI, no web panel

[![Chat on Discord](https://cdn.jsdelivr.net/npm/@intergrav/devins-badges@3/assets/cozy/social/discord-plural_vector.svg)](https://discord.gg/qNyybSSPm5)
[![github](https://cdn.jsdelivr.net/npm/@intergrav/devins-badges@3/assets/cozy/available/github_vector.svg)](https://github.com/BX-Team/Nexon)

</div>

Nexon manages Xray nodes and issues subscriptions (base64 / plain links / Xray /
Clash / Clash.Meta / sing-box). Spiritual successor to Hystron, modelled after
PasarGuard / Remnawave / Marzban — driven by a **CLI** and an interactive **TUI**,
with a **subscription server** for clients. SQLite (WAL) is the single source of
truth; users are generated with a full proxy bundle and selectively pushed to
nodes over the Xray gRPC API.

## Features

- Users with traffic limits, expiry dates and per-device (HWID) caps; auto-kick on limit
- **Node groups** — route a subset of users to a subset of nodes (e.g. your users vs a friend's)
- Subscriptions auto-detect the client by User-Agent and return the right format; a browser gets an HTML dashboard with a QR code
- **Per-client output format** — pin a UA to a format (mihomo → Clash, Happ → links/base64, …)
- **Custom templates** per format (Clash/Clash.Meta/sing-box/Xray): you own dns/rules/proxy-groups, proxies are injected automatically; edited in `$EDITOR`, validated and previewable
- Real Xray gRPC connector: add/remove users via `AlterInbound`, traffic polling via `StatsService`
- **TUI** (Bubble Tea) for interactive management, **CLI** for scripting
- Embedded SQLite migrations run automatically on startup

## Install

### Ubuntu / Debian (and other systemd Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/BX-Team/Nexon/main/scripts/install.sh -o /tmp/nexon.sh \
  && sudo bash /tmp/nexon.sh install
```

This downloads the latest static binary to `/usr/local/bin/nexon`, installs a
`nexon.service` systemd unit, and creates `/etc/nexon/nexon.env`. Then:

```bash
sudo nano /etc/nexon/nexon.env       # set NEXON_SUB_BASE_URL to your public URL
sudo systemctl restart nexon
nexon --help
```

Update or remove:

```bash
sudo nexon update                    # built-in alias: re-runs the installer's update path
sudo bash /tmp/nexon.sh uninstall    # keeps data in /var/lib/nexon
```

### Build from source

Requires Go 1.26+.

```bash
git clone https://github.com/BX-Team/Nexon.git
cd Nexon
go build -o nexon ./cmd/nexon
./nexon --help
```

### Nix / NixOS

Run directly from the flake, or build a local binary:

```bash
nix run github:BX-Team/Nexon -- --help      # run without installing
nix build github:BX-Team/Nexon              # ./result/bin/nexon
nix develop                                 # dev shell with go, gopls, sqlite
```

The repo ships a `flake.lock`, so every build is pinned and reproducible.

#### NixOS deployment

Add the flake as an input and import the module. It runs `nexon serve` as a
hardened systemd unit (`DynamicUser`, `StateDirectory=nexon`):

```nix
# flake.nix
{
  inputs.nexon.url = "github:BX-Team/Nexon";

  outputs = { self, nixpkgs, nexon, ... }: {
    nixosConfigurations.myhost = nixpkgs.lib.nixosSystem {
      # ...
      modules = [
        nexon.nixosModules.nexon
        ({ ... }: {
          services.nexon = {
            enable = true;
            subBaseURL = "https://vpn.example.com";  # required: public URL for sub links
            openFirewall = true;                     # open the subscription port
          };
        })
      ];
    };
  };
}
```

All module options (`services.nexon.*`):

| Option | Default | Meaning |
|---|---|---|
| `enable` | `false` | Enable the Nexon service. |
| `subBaseURL` | — (required) | Public base URL used to build subscription links. |
| `subListen` | `:8080` | Subscription server listen address. |
| `dataDir` | `/var/lib/nexon` | State directory (holds the SQLite database). |
| `envFile` | `null` | File with extra `NEXON_*` vars, loaded via `EnvironmentFile`. |
| `openFirewall` | `false` | Open the subscription port in the firewall. |
| `package` | flake default | The `nexon` package to run. |

Manage users/nodes on the host with the same binary, e.g.
`sudo -u nexon nexon user add alice --data-limit 100G` (point `NEXON_DATA_DIR`
at `dataDir` if you run it as another user).

## Setting up a node

A node is a VPS running `xray-core` with its gRPC API enabled. The helper script
installs Xray and writes an API inbound:

```bash
curl -fsSL https://raw.githubusercontent.com/BX-Team/Nexon/main/scripts/node-setup.sh | sudo API_PORT=8443 bash
```

Then add your real proxy inbounds (vless/trojan/hysteria2…) to
`/usr/local/etc/xray/config.json` next to the `api` inbound and register the node:

```bash
nexon node add tokyo --address 1.2.3.4 --api-port 8443
nexon node inbound add tokyo --tag vless-reality --protocol vless --port 443 \
  --tls reality --settings '{"sni":"example.com","pbk":"<public-key>","sid":"<short-id>"}'
nexon node sync tokyo
```

> **Security:** the Xray gRPC API has no authentication. Allow the API port only
> from the Nexon host (`ufw allow from <NEXON_IP> to any port 8443`) or keep it on
> a private network / tunnel (WireGuard / SSH). Nexon dials the API in plaintext.

## Quick start

```bash
nexon user add alice --data-limit 100G --expire 30d --hwid-limit 3
nexon user sub alice                     # subscription link + QR in the terminal
nexon serve                              # subscription server :8080 + traffic poller
nexon tui                                # interactive cockpit
```

Clients fetch their config from the link printed by `nexon user sub`; opening it
in a browser shows an HTML dashboard with a QR code.

## Usage

### CLI

| Command | Purpose |
|---|---|
| `user` | create/list/edit users, show sub link + QR, devices |
| `node` | register nodes, manage inbounds, sync, show status (`list`/`show`/`inbounds`) |
| `group` | node groups; assign users/nodes to groups |
| `clients` | managed client apps: UA → custom headers + pinned output format |
| `template` | custom per-format subscription templates (`list`/`show`/`edit`/`preview`/`rm`) |
| `settings` / `rule` | runtime settings and UA→format detection rules |
| `serve` | run the subscription server + traffic poller |
| `tui` | launch the interactive TUI |
| `update` | update to the latest release (re-runs the installer) |

### TUI

`nexon tui` opens an interactive cockpit with tabs: **Dashboard** (live stats),
**Users** (create/edit/delete, per-user detail with QR + devices, group cycle),
**Nodes**, **Groups**, **Clients** (UA headers + format), **Templates** (edit in
`$EDITOR`, preview) and **Settings**. Switch tabs with `tab`/`←→`, refresh with
`r`, quit with `q`.

### Custom templates

Each format (clash / clash-meta / singbox / xray) can use a custom Go
`text/template` stored in the DB. The generated proxies are injected via
`{{ .Proxies }}` (format-native entries) and `{{ .Names }}` (their names) — you
own everything else (dns, rules, proxy-groups, tun…). Rich defaults are seeded on
first run.

```bash
nexon template edit clash       # opens $EDITOR with the current/starter template
nexon template preview clash    # render against a sample subscription
nexon template rm clash         # revert to the built-in generator
```

Output is validated (YAML/JSON) on save and at render time; on any error Nexon
falls back to the built-in generator so subscriptions never break.

## Configuration

All via `NEXON_*` environment variables (see `internal/config/config.go`):

| Var | Default | Meaning |
|---|---|---|
| `NEXON_DATA_DIR` | `/var/lib/nexon` | state directory |
| `NEXON_DB` | `<data>/nexon.db` | SQLite path |
| `NEXON_SUB_LISTEN` | `:8080` | subscription server address |
| `NEXON_SUB_BASE_URL` | `http://localhost:8080` | public base for building sub links |
| `NEXON_POLL_INTERVAL` | `30` | traffic poll interval (seconds) |
| `NEXON_NODE_MODE` | — | `stub` = logging connector (dev without a real node) |

## Architecture

```
nexon CLI ──┐
nexon TUI ──┼─► core.Service ─► SQLite (source of truth)
sub server ─┘        │
                     └─► NodeConnector (gRPC) ─► Xray nodes (Handler/Stats)
```

```
cmd/nexon/            entrypoint
internal/cli/         cobra commands
internal/tui/         Bubble Tea terminal cockpit
internal/core/        Services (business logic)
internal/store/       SQLite + migrations + queries
internal/node/        NodeConnector (Xray gRPC) + stub
internal/secrets/     per-user proxy generation
internal/subgen/      format generators + templates
internal/subserver/   sub HTTP server + UA detection + HWID
nix/                  NixOS module
scripts/              install.sh, node-setup.sh
```

## License ![Static Badge](https://img.shields.io/badge/license-MIT-lightgreen)

Licensed under the [MIT License](LICENSE).
