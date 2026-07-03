-- Repair the sing-box User-Agent pattern seeded by 0001 / seed.go. The old
-- pattern `[Ss]ing[\-b]?ox` never matched the canonical UA "sing-box" (it only
-- matched "singbox"), so sing-box clients fell through to the base64 format and
-- were not recognised as real devices. REPLACE is a no-op when the substring is
-- absent, so this is safe on DBs seeded with the corrected pattern.
UPDATE client_apps SET ua_pattern = REPLACE(ua_pattern, '[Ss]ing[\-b]?ox', '[Ss]ing[-_]?box');
UPDATE sub_rules  SET regex       = REPLACE(regex,      '[Ss]ing[\-b]?ox', '[Ss]ing[-_]?box');
