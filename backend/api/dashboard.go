package api

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strings"
	"time"
)

var dashboardTmpl = template.Must(template.New("dashboard").Funcs(dashFuncs).Parse(dashboardHTML))

var dashFuncs = template.FuncMap{
	"sgd": func(p *int64) string {
		if p == nil || *p == 0 {
			return "—"
		}
		v := *p
		if v >= 1_000_000 {
			f := float64(v) / 1_000_000
			s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", f), "0"), ".")
			return "S$" + s + "M"
		}
		return fmt.Sprintf("S$%dK", v/1000)
	},
	"area": func(f *float64) string {
		if f == nil || *f == 0 {
			return ""
		}
		return fmt.Sprintf("%.0f sqft", *f)
	},
	"psf": func(f *float64) string {
		if f == nil {
			return "—"
		}
		return "$" + commaSep(int64(math.Round(*f))) + " psf"
	},
	"pct": func(f *float64) string {
		if f == nil {
			return ""
		}
		if *f > 0 {
			return fmt.Sprintf("+%.1f%%", *f)
		}
		return fmt.Sprintf("%.1f%%", *f)
	},
	"pctClass": func(f *float64) string {
		if f == nil || *f == 0 {
			return ""
		}
		if *f > 0 {
			return "up"
		}
		return "down"
	},
	"beds": func(beds, baths *int) string {
		b := "Studio"
		if beds != nil && *beds > 0 {
			b = fmt.Sprintf("%d bd", *beds)
		}
		if baths != nil && *baths > 0 {
			return b + fmt.Sprintf(" / %d ba", *baths)
		}
		return b
	},
	"str": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"reltime": func(t *time.Time) string {
		if t == nil {
			return "—"
		}
		d := time.Since(*t)
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			return fmt.Sprintf("%dm ago", int(d.Minutes()))
		case d < 24*time.Hour:
			return fmt.Sprintf("%dh ago", int(d.Hours()))
		case d < 7*24*time.Hour:
			return fmt.Sprintf("%dd ago", int(d.Hours()/24))
		default:
			return t.Local().Format("Jan 2")
		}
	},
	"now": func() string {
		return time.Now().Local().Format("2 Jan 2006, 15:04")
	},
}

func commaSep(n int64) string {
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := s.store.GetDashboardStats(ctx)
	if err != nil {
		http.Error(w, "failed to load stats", http.StatusInternalServerError)
		return
	}

	units, err := s.store.GetDashboardUnits(ctx)
	if err != nil {
		http.Error(w, "failed to load units", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = dashboardTmpl.Execute(w, map[string]interface{}{
		"Stats": stats,
		"Units": units,
	})
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>UnitTrace</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#f0f2f5;color:#1a1a2e;min-height:100vh}

/* Nav */
nav{background:#1a1a2e;color:#fff;padding:14px 24px;display:flex;align-items:center;gap:16px}
nav .logo{font-size:17px;font-weight:700;letter-spacing:-0.3px}
nav .tagline{font-size:13px;color:#8899bb;flex:1}
nav .updated{font-size:12px;color:#556688}

/* Stats */
.stats{display:flex;gap:12px;padding:20px 24px 8px}
.stat{background:#fff;border-radius:10px;padding:16px 20px;flex:1;box-shadow:0 1px 3px rgba(0,0,0,.07)}
.stat-val{font-size:30px;font-weight:700;color:#1a1a2e;line-height:1}
.stat-lbl{font-size:12px;color:#8899aa;margin-top:5px}

/* Table */
.wrap{padding:12px 24px 32px;overflow-x:auto}
table{width:100%;border-collapse:collapse;background:#fff;border-radius:10px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.07)}
thead{background:#1a1a2e;color:#fff}
th{padding:11px 14px;text-align:left;font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.6px;white-space:nowrap}
td{padding:11px 14px;border-bottom:1px solid #f2f4f7;font-size:13px;vertical-align:top}
tr:last-child td{border-bottom:none}
tbody tr:hover td{background:#fafbff}

/* Cell types */
.unit-name{font-weight:600;color:#1a1a2e;line-height:1.3;max-width:240px}
.unit-addr{font-size:11px;color:#8899aa;margin-top:3px;max-width:240px}
.unit-agent{font-size:11px;color:#b0bbc9;margin-top:2px}
.loc-district{font-weight:500}
.loc-type{font-size:11px;color:#8899aa;margin-top:2px}
.loc-beds{font-size:11px;color:#8899aa;margin-top:2px}
.price-val{font-weight:700;font-size:14px}
.price-area{font-size:11px;color:#8899aa;margin-top:2px}
.psf-val{font-size:13px;font-weight:500;color:#334}
.up{color:#dc2626;font-weight:600}
.down{color:#16a34a;font-weight:600}
.dim{color:#b0bbc9;font-size:12px}
.visits{font-size:13px}
.relist-badge{display:inline-block;margin-top:4px;padding:2px 7px;background:#fef3c7;color:#b45309;border-radius:4px;font-size:11px;font-weight:600}
.reltime{color:#556688;font-size:12px}
.open-btn{display:inline-flex;align-items:center;justify-content:center;width:28px;height:28px;background:#e8f0fe;color:#1a73e8;border-radius:6px;text-decoration:none;font-size:14px}
.open-btn:hover{background:#d2e3fc}
.empty{text-align:center;padding:48px 24px;color:#8899aa;font-size:14px}
</style>
</head>
<body>

<nav>
  <span class="logo">UnitTrace</span>
  <span class="tagline">Charlie &amp; Ellie's property tracker</span>
  <span class="updated">{{now}}</span>
</nav>

<div class="stats">
  <div class="stat">
    <div class="stat-val">{{.Stats.TotalUnits}}</div>
    <div class="stat-lbl">Units Tracked</div>
  </div>
  <div class="stat">
    <div class="stat-val">{{.Stats.RecentUnits}}</div>
    <div class="stat-lbl">Visited (7 days)</div>
  </div>
  <div class="stat">
    <div class="stat-val">{{.Stats.TotalVisits}}</div>
    <div class="stat-lbl">Total Visits</div>
  </div>
  <div class="stat">
    <div class="stat-val">{{.Stats.PossibleRelists}}</div>
    <div class="stat-lbl">Possible Relists</div>
  </div>
</div>

<div class="wrap">
{{if not .Units}}
  <div class="empty">No units tracked yet. Install the extension and visit some PropertyGuru listings.</div>
{{else}}
<table>
  <thead>
    <tr>
      <th>Unit</th>
      <th>Location</th>
      <th>Price</th>
      <th>PSF</th>
      <th>Δ Price</th>
      <th>Visits</th>
      <th>Last Seen</th>
      <th></th>
    </tr>
  </thead>
  <tbody>
  {{range .Units}}
  <tr>
    <td>
      <div class="unit-name">{{if .Title}}{{str .Title}}{{else}}<span class="dim">—</span>{{end}}</div>
      {{if .AddressText}}<div class="unit-addr">{{str .AddressText}}</div>{{end}}
      {{if .AgentName}}<div class="unit-agent">{{str .AgentName}}</div>{{end}}
    </td>
    <td>
      <div class="loc-district">{{if .District}}{{str .District}}{{else}}<span class="dim">—</span>{{end}}</div>
      {{if .PropertyType}}<div class="loc-type">{{str .PropertyType}}</div>{{end}}
      <div class="loc-beds">{{beds .Bedrooms .Bathrooms}}</div>
    </td>
    <td>
      <div class="price-val">{{sgd .CurrentPrice}}</div>
      {{if .FloorArea}}<div class="price-area">{{area .FloorArea}}</div>{{end}}
    </td>
    <td class="psf-val">{{psf .PSF}}</td>
    <td>
      {{if .PriceChangePct}}
        <span class="{{pctClass .PriceChangePct}}">{{pct .PriceChangePct}}</span>
      {{else}}<span class="dim">—</span>{{end}}
    </td>
    <td>
      <div class="visits">{{.VisitCount}}</div>
      {{if gt .PossibleRelistCount 0}}<span class="relist-badge">{{.PossibleRelistCount}}× relist</span>{{end}}
    </td>
    <td><span class="reltime">{{reltime .LastVisitedAt}}</span></td>
    <td>
      {{if .ListingURL}}<a href="{{str .ListingURL}}" target="_blank" class="open-btn" title="Open on PropertyGuru">↗</a>{{end}}
    </td>
  </tr>
  {{end}}
  </tbody>
</table>
{{end}}
</div>

</body>
</html>`
