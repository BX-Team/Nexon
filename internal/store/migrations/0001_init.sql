-- Nexon initial schema (control-plane source of truth).
-- SQLite in WAL mode. See PLAN.md section 4 for the data model.

CREATE TABLE IF NOT EXISTS node_groups (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    is_default INTEGER NOT NULL DEFAULT 0
);

-- Seed the default group as id 1 so a NULL group_id resolves to it.
INSERT INTO node_groups (id, name, is_default)
SELECT 1, 'Default', 1 WHERE NOT EXISTS (SELECT 1 FROM node_groups);

CREATE TABLE IF NOT EXISTS users (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    username                TEXT    NOT NULL UNIQUE,
    created_at              TEXT    NOT NULL DEFAULT (datetime('now')),
    status                  TEXT    NOT NULL DEFAULT 'active', -- active|disabled|limited|expired
    data_limit              INTEGER NOT NULL DEFAULT 0,        -- bytes, 0 = unlimited
    used_traffic            INTEGER NOT NULL DEFAULT 0,
    traffic_reset_strategy  TEXT    NOT NULL DEFAULT 'no_reset', -- no_reset|day|week|month
    expire_at               TEXT,
    hwid_limit              INTEGER NOT NULL DEFAULT 0,        -- 0 = no limit
    proxies                 TEXT    NOT NULL DEFAULT '{}',     -- JSON per-user secrets
    sub_token               TEXT    NOT NULL UNIQUE,
    sub_last_user_agent     TEXT,
    sub_updated_at          TEXT,
    traffic_reset_at        TEXT,                              -- last monthly reset
    expiry_notified_for     TEXT,                              -- expire_at already warned about
    group_id                INTEGER REFERENCES node_groups(id) -- NULL = default group
);

CREATE TABLE IF NOT EXISTS nodes (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL UNIQUE,
    address       TEXT    NOT NULL,
    api_port      INTEGER NOT NULL,
    status        TEXT    NOT NULL DEFAULT 'disconnected', -- connected|disconnected|error
    xray_version  TEXT,
    last_seen     TEXT,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    group_id      INTEGER REFERENCES node_groups(id)        -- NULL = default group
);

CREATE TABLE IF NOT EXISTS inbounds (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id       INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    tag           TEXT    NOT NULL,
    protocol      TEXT    NOT NULL, -- vmess|vless|trojan|shadowsocks|hysteria
    network       TEXT,
    tls           TEXT,
    port          INTEGER,
    settings_json TEXT    NOT NULL DEFAULT '{}',
    UNIQUE (node_id, tag)
);

CREATE TABLE IF NOT EXISTS devices (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    hwid        TEXT,
    user_agent  TEXT,
    first_seen  TEXT    NOT NULL DEFAULT (datetime('now')),
    last_seen   TEXT    NOT NULL DEFAULT (datetime('now')),
    ip_last     TEXT,
    revoked     INTEGER NOT NULL DEFAULT 0,
    UNIQUE (user_id, hwid)
);

CREATE TABLE IF NOT EXISTS traffic_usage (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    period    TEXT    NOT NULL, -- hour|day|week|month|total
    bucket    TEXT    NOT NULL, -- e.g. 2026-06-16, 2026-06-16T14, 2026-W24
    bytes     INTEGER NOT NULL DEFAULT 0,
    UNIQUE (user_id, period, bucket)
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Ordered subscription-format detection rules (see PLAN.md section 6).
CREATE TABLE IF NOT EXISTS sub_rules (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    priority INTEGER NOT NULL,
    regex    TEXT    NOT NULL,
    format   TEXT    NOT NULL
);

-- Managed VPN client apps: UA pattern + per-app custom response headers
-- (PasarGuard-style). Drives device visibility and subscription headers.
CREATE TABLE IF NOT EXISTS client_apps (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    ua_pattern TEXT    NOT NULL,                      -- regex
    headers    TEXT    NOT NULL DEFAULT '{}',         -- JSON map of extra headers
    enabled    INTEGER NOT NULL DEFAULT 1,
    sort       INTEGER NOT NULL DEFAULT 100,
    format     TEXT    NOT NULL DEFAULT ''            -- pinned output format ('' = auto/UA rules)
);

-- Lightweight event log surfaced to the operator (CLI/TUI).
CREATE TABLE IF NOT EXISTS event_log (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    ts       TEXT NOT NULL DEFAULT (datetime('now')),
    level    TEXT NOT NULL DEFAULT 'info',           -- info|warn|error
    category TEXT NOT NULL DEFAULT 'system',
    message  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_inbounds_node    ON inbounds(node_id);
CREATE INDEX IF NOT EXISTS idx_devices_user     ON devices(user_id);
CREATE INDEX IF NOT EXISTS idx_traffic_user     ON traffic_usage(user_id);
CREATE INDEX IF NOT EXISTS idx_users_token      ON users(sub_token);
CREATE INDEX IF NOT EXISTS idx_event_log_ts     ON event_log(id DESC);

-- Seed the known VPN clients (used to recognise real devices vs browsers/curl).
INSERT INTO client_apps (name, ua_pattern, sort) VALUES
    ('Happ',        '^[Hh]app',                                              10),
    ('Clash Meta',  '[Cc]lash[\-\.]?[Mm]eta|[Mm]ihomo|[Ff][Ll][Cc]lash',    20),
    ('Clash',       '^([Cc]lash|[Cc]lash[\-\.]?[Vv]erge|[Ss]tash)',         30),
    ('sing-box',    '(SFA|SFI|SFM|SFT|[Kk]aring|[Hh]iddify)|[Ss]ing[-_]?box', 40),
    ('v2rayNG',     '^([Vv]2rayNG|[Vv]2rayN)',                              50),
    ('Streisand',   '^[Ss]treisand',                                        60),
    ('Xray/V2Ray',  'INCY/[\d.]+|[Xx]ray|[Vv]2[Rr]ay',                      70);

-- Global subscription header settings (profile name / support url / announce).
INSERT INTO settings (key, value) VALUES
    ('sub.profile_title', 'Nexon VPN'),
    ('sub.support_url',   ''),
    ('sub.announce',      '');
