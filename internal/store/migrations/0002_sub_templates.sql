-- Custom subscription templates (Go text/template), one per format. When a row
-- exists for the detected format, the sub server renders through it instead of
-- the built-in generator; the generated proxy entries are injected via
-- {{ .Proxies }} so the operator controls dns/rules/proxy-groups.
CREATE TABLE IF NOT EXISTS sub_templates (
    format     TEXT PRIMARY KEY,  -- clash | clash-meta | singbox | xray
    body       TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
