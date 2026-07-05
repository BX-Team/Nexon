package subserver

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/BX-Team/Nexon/internal/core"
)

// dashLink is one connectable endpoint rendered as a copyable row.
type dashLink struct {
	Name string
	URI  template.URL
}

var dashboardTmpl = template.Must(template.New("dash").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width,initial-scale=1" />
<meta name="robots" content="noindex,nofollow" />
<meta name="theme-color" content="#4E54C8" />
<title>Nexon — {{.Username}}</title>
<link href="https://fonts.googleapis.com/css2?family=Unbounded:wght@400;600&display=swap" rel="stylesheet" />
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet" />
<style>
:root{
--bg:#0a0a0a;--bg-elev:#121212;--bg-elev-2:#171717;--border:#262626;--border-soft:#1c1c1c;
--text:#dcdcdc;--text-dim:#a3a3a3;--text-mute:#525252;--text-faint:#404040;
--accent-from:#8f94fb;--accent-to:#4e54c8;--up:#34d399;--down:#f87171;--warn:#fbbf24;
}
*{margin:0;padding:0;box-sizing:border-box}
html,body{overscroll-behavior:none}
body{font-family:"JetBrains Mono",ui-monospace,SFMono-Regular,Menlo,monospace;background:var(--bg);color:var(--text);font-size:14px;line-height:1.55;min-height:100vh;position:relative;overflow-x:hidden;-webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility}
::selection{background:rgb(143 148 251 / 0.25);color:#fff}
.bg-grid{position:fixed;inset:0;background:linear-gradient(to right,rgba(128,128,128,.063) 1px,transparent 1px),linear-gradient(to bottom,rgba(128,128,128,.063) 1px,transparent 1px);background-size:24px 24px;-webkit-mask-image:linear-gradient(to bottom,transparent,#000 12%,#000 88%,transparent);mask-image:linear-gradient(to bottom,transparent,#000 12%,#000 88%,transparent);pointer-events:none;z-index:0}
main{position:relative;z-index:1;display:flex;flex-direction:column;align-items:center;padding:48px 16px 16px}
h1{font-family:"Unbounded",ui-sans-serif,system-ui,sans-serif;font-size:22px;font-weight:600;color:#fff;display:flex;align-items:center;gap:10px}
.brand{--bg-size:400%;background-image:linear-gradient(90deg,var(--accent-to),var(--accent-from),var(--accent-to));background-size:var(--bg-size) 100%;-webkit-background-clip:text;background-clip:text;color:transparent}
@media (prefers-reduced-motion:no-preference){.brand{animation:brand-shift 6s linear infinite}}
@keyframes brand-shift{to{background-position:var(--bg-size) 0}}
.sub{font-size:12px;color:var(--text-mute);margin-top:4px}
.lang{display:flex;gap:4px;margin-top:14px;background:var(--bg-elev);border:1px solid var(--border-soft);border-radius:8px;padding:3px}
.lang-btn{font-family:inherit;font-size:11px;color:var(--text-mute);background:transparent;border:0;border-radius:6px;padding:4px 10px;cursor:pointer;transition:color .15s,background .15s}
.lang-btn:hover{color:var(--text-dim)}
.lang-btn.active{color:#fff;background:var(--bg-elev-2)}
.stack{margin-top:24px;width:100%;max-width:480px;display:flex;flex-direction:column;gap:12px}
.card{background:var(--bg-elev);border:1px solid var(--border-soft);border-radius:12px;padding:14px 18px}
.card.warn{background:color-mix(in oklab,var(--warn) 5%,transparent);border:1.5px dotted color-mix(in oklab,var(--warn) 25%,transparent);color:var(--warn);font-size:12px;text-align:center}
.card-title{font-size:10.5px;font-weight:500;color:var(--text-mute);text-transform:uppercase;letter-spacing:.14em;margin-bottom:12px}
.qr-wrap{display:flex;align-items:center;justify-content:center;background:var(--bg-elev-2);border:1px solid var(--border-soft);border-radius:10px;padding:24px}
.qr-wrap img,.qr-wrap canvas{display:block;border-radius:4px}
.qr-box{width:150px;height:150px;display:flex;align-items:center;justify-content:center}
.qr-box.loading{border-radius:8px;background:linear-gradient(90deg,#161616 25%,#1f1f1f 37%,#161616 63%);background-size:400% 100%;animation:shimmer 1.4s ease infinite}
@keyframes shimmer{0%{background-position:100% 0}100%{background-position:-100% 0}}
.qr-wrap+.rows{margin-top:12px}
.rows{display:flex;flex-direction:column;gap:6px}
.row{background:var(--bg-elev);border:1px solid var(--border-soft);border-radius:10px;padding:12px 14px;display:flex;align-items:center;justify-content:space-between;gap:10px;cursor:pointer;transition:border-color .15s,background .15s,transform .05s}
.row:hover{border-color:#333;background:var(--bg-elev-2)}
.row:active{transform:translateY(1px)}
.row-left{display:flex;align-items:center;gap:10px;min-width:0}
.icon-box{display:grid;place-items:center;width:24px;height:24px;background:var(--bg-elev-2);border:1px solid var(--border-soft);border-radius:6px;flex-shrink:0;font-size:13px;line-height:1}
.name{font-size:13px;color:var(--text);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.hint{font-size:11px;color:var(--text-mute)}
.stats{display:flex;flex-direction:column;gap:6px}
.stat-row{display:flex;justify-content:space-between;font-size:13px}
.stat-row span:first-child{color:var(--text-mute)}
.footer{margin-top:18px;font-size:11px;color:var(--text-faint);text-align:center;line-height:1.6}
.toast{position:fixed;bottom:24px;left:50%;transform:translateX(-50%) translateY(20px);background:var(--bg-elev);border:1px solid var(--border);color:var(--text-dim);padding:10px 22px;border-radius:12px;font-size:12px;opacity:0;transition:opacity .2s,transform .2s;pointer-events:none;z-index:99}
.toast.show{opacity:1;transform:translateX(-50%) translateY(0)}
</style>
</head>
<body>
<div class="bg-grid"></div>
<main>
<h1><span class="brand">Nexon</span></h1>
<div class="sub"><span data-i18n="for">for</span> {{.Username}}</div>
<div class="lang">
<button class="lang-btn" data-lang="en">EN</button>
<button class="lang-btn" data-lang="ru">RU</button>
</div>
<div class="stack">
{{if .Blocked}}
<section class="card warn" data-i18n="blocked">⚠️ subscription is blocked — contact support below</section>
{{end}}
<section class="card">
<div class="card-title" data-i18n="subscription">subscription</div>
<div class="qr-wrap"><div id="qr" class="qr-box loading"></div></div>
<div class="rows">
<div class="row" data-uri="{{.SubURL}}">
<div class="row-left"><span class="icon-box">🔗</span><span class="name" data-i18n="copySub">copy subscription</span></div>
<span class="hint" data-i18n="url">URL</span>
</div>
</div>
</section>
{{if .Links}}
<section class="card">
<div class="card-title" data-i18n="links">links</div>
<div class="rows">
{{range .Links}}
<div class="row" data-uri="{{.URI}}">
<div class="row-left"><span class="icon-box">🌐</span><span class="name">{{.Name}}</span></div>
<span class="hint" data-i18n="copy">copy</span>
</div>
{{end}}
</div>
</section>
{{end}}
<section class="card">
<div class="card-title" data-i18n="info">info</div>
<div class="stats">
<div class="stat-row"><span data-i18n="status">status</span><span id="status-val" data-status="{{.Status}}">{{.Status}}</span></div>
<div class="stat-row"><span data-i18n="traffic">traffic</span><span>{{.Used}} / {{.Limit}}</span></div>
<div class="stat-row"><span data-i18n="expires">expires</span><span>{{if .ExpireNever}}<span data-i18n="never">never</span>{{else}}{{.ExpireDate}}{{if ge .ExpireDays 0}} (<span data-days="{{.ExpireDays}}">{{.ExpireDays}}d</span>){{end}}{{end}}</span></div>
</div>
</section>
</div>
<div class="footer" data-i18n="footer">Nexon subscription</div>
</main>
<div class="toast" id="toast" data-i18n="copied">copied</div>
<script>
"use strict";
var SUB_URL = {{.SubURL}};
var I18N = {
en: {for:"for",subscription:"subscription",links:"links",info:"info",copySub:"copy subscription",url:"URL",copy:"copy",status:"status",traffic:"traffic",expires:"expires",copied:"copied",never:"never",footer:"Nexon subscription",days:"d",blocked:"⚠️ subscription is blocked — contact support below",st_active:"active",st_disabled:"disabled",st_limited:"limited",st_expired:"expired"},
ru: {for:"для",subscription:"подписка",links:"ссылки",info:"информация",copySub:"скопировать подписку",url:"URL",copy:"копировать",status:"статус",traffic:"трафик",expires:"истекает",copied:"скопировано",never:"никогда",footer:"Подписка Nexon",days:"дн",blocked:"⚠️ подписка заблокирована — напишите в поддержку ниже",st_active:"активна",st_disabled:"отключена",st_limited:"лимит",st_expired:"истекла"}
};
function detectLang(){
  try { var saved = localStorage.getItem("nexon_lang"); if (saved === "ru" || saved === "en") return saved; } catch (e) {}
  return (navigator.language || "en").toLowerCase().indexOf("ru") === 0 ? "ru" : "en";
}
var lang = detectLang();
function t(k){ var d = I18N[lang] || I18N.en; return (k in d) ? d[k] : (I18N.en[k] || k); }
function applyLang(){
  document.documentElement.lang = lang;
  document.querySelectorAll("[data-i18n]").forEach(function (el) { el.textContent = t(el.dataset.i18n); });
  var st = document.getElementById("status-val");
  if (st) st.textContent = t("st_" + st.dataset.status);
  document.querySelectorAll("[data-days]").forEach(function (el) { el.textContent = el.dataset.days + t("days"); });
  document.querySelectorAll(".lang-btn").forEach(function (b) { b.classList.toggle("active", b.dataset.lang === lang); });
}
function setLang(l){ lang = l; try { localStorage.setItem("nexon_lang", l); } catch (e) {} applyLang(); }
document.querySelectorAll(".lang-btn").forEach(function (b) {
  b.addEventListener("click", function () { setLang(b.dataset.lang); });
});
applyLang();

function copy(text){
  navigator.clipboard.writeText(text).then(function () {
    var e = document.getElementById("toast");
    e.classList.add("show");
    setTimeout(function () { e.classList.remove("show"); }, 1500);
  });
}
document.querySelectorAll(".row[data-uri]").forEach(function (row) {
  row.addEventListener("click", function () { copy(row.dataset.uri); });
});

(function () {
  var qrEl = document.getElementById("qr");
  var s = document.createElement("script");
  s.src = "https://cdn.jsdelivr.net/npm/qrcodejs@1.0.0/qrcode.min.js";
  s.onload = function () {
    new QRCode(qrEl, {
      text: SUB_URL,
      width: 150,
      height: 150,
      colorDark: "#737373",
      colorLight: "#121212",
      correctLevel: QRCode.CorrectLevel.M,
    });
    qrEl.classList.remove("loading");
  };
  s.onerror = function () { qrEl.classList.remove("loading"); };
  document.head.appendChild(s);
})();
</script>
</body>
</html>`))

func (s *Server) renderDashboard(w http.ResponseWriter, res *core.SubResult, token string) {
	subURL := s.baseURL + "/sub/" + token

	expireNever := res.User.ExpireAt == nil
	expireDate := ""
	expireDays := -1
	if !expireNever {
		expireDate = res.User.ExpireAt.Format("2006-01-02")
		if res.User.ExpireAt.After(time.Now()) {
			expireDays = int(time.Until(*res.User.ExpireAt).Hours() / 24)
		}
	}

	links := make([]dashLink, 0, len(res.Endpoints))
	for _, ep := range res.Endpoints {
		if uri := ep.URI(); uri != "" {
			links = append(links, dashLink{Name: ep.Name, URI: template.URL(uri)})
		}
	}

	data := map[string]any{
		"Username":    res.User.Username,
		"Status":      string(res.User.Status),
		"Blocked":     res.User.Status != "active",
		"Used":        humanBytes(res.User.UsedTraffic),
		"Limit":       limitStr(res.User.DataLimit),
		"ExpireNever": expireNever,
		"ExpireDate":  expireDate,
		"ExpireDays":  expireDays,
		"Links":       links,
		"SubURL":      subURL,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = dashboardTmpl.Execute(w, data)
}

func limitStr(b int64) string {
	if b == 0 {
		return "∞"
	}
	return humanBytes(b)
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
