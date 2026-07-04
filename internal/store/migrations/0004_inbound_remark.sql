-- Optional per-inbound display label used as the subscription endpoint name
-- (the vless://…#remark fragment). Empty falls back to "<node>-<tag>".
ALTER TABLE inbounds ADD COLUMN remark TEXT NOT NULL DEFAULT '';
