package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/2144983846/aperture/internal/provider"
	"github.com/2144983846/aperture/internal/router"
	"github.com/2144983846/aperture/internal/router/strategy"
	"github.com/2144983846/aperture/internal/store"
)

type Dashboard struct {
	store    *store.Store
	router   *router.Router
	registry *provider.Registry
	feed     *LiveFeed
}

type LiveFeed struct {
	mu       sync.RWMutex
	events   []LiveEvent
	subs     map[chan LiveEvent]struct{}
}

type LiveEvent struct {
	Timestamp   string  `json:"timestamp"`
	Model       string  `json:"model"`
	Provider    string  `json:"provider"`
	Complexity  string  `json:"complexity"`
	TokensIn    int     `json:"tokens_in"`
	TokensOut   int     `json:"tokens_out"`
	CostUSD     float64 `json:"cost_usd"`
	LatencyMs   int64   `json:"latency_ms"`
}

func New(s *store.Store, r *router.Router, reg *provider.Registry) *Dashboard {
	d := &Dashboard{
		store:    s,
		router:   r,
		registry: reg,
		feed: &LiveFeed{
			events: make([]LiveEvent, 0, 100),
			subs:   make(map[chan LiveEvent]struct{}),
		},
	}
	return d
}

func (d *Dashboard) PushEvent(e LiveEvent) {
	d.feed.mu.Lock()
	d.feed.events = append(d.feed.events, e)
	if len(d.feed.events) > 100 {
		d.feed.events = d.feed.events[len(d.feed.events)-100:]
	}
	for ch := range d.feed.subs {
		select {
		case ch <- e:
		default:
		}
	}
	d.feed.mu.Unlock()
}

func (d *Dashboard) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(dashboardHTML))
	})

	mux.HandleFunc("/dashboard/data/overview", d.handleOverviewData)
	mux.HandleFunc("/dashboard/data/routing", d.handleRoutingData)
	mux.HandleFunc("/dashboard/data/keys", d.handleKeysData)
	mux.HandleFunc("/dashboard/data/log", d.handleLogData)
	mux.HandleFunc("/dashboard/data/analytics", d.handleAnalyticsJSON)

	mux.HandleFunc("/dashboard/api/routing/test", d.handleRoutingTest)
	mux.HandleFunc("/dashboard/api/keys/create", d.handleCreateKey)
	mux.HandleFunc("/dashboard/api/keys/delete", d.handleDeleteKey)

	mux.HandleFunc("/dashboard/sse", d.handleSSE)

	return mux
}

func (d *Dashboard) handleOverviewData(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from := now.Add(-24 * time.Hour)
	summary, _ := d.store.GetAnalyticsSummary(from, now, "")

	providerStatuses := ""
	for _, p := range d.registry.List() {
		err := p.Health(r.Context())
		status := "healthy"
		if err != nil {
			status = "unhealthy"
		}
		providerStatuses += fmt.Sprintf(`<span style="margin:4px;padding:4px 10px;border-radius:12px;font-size:12px;background:rgba(52,211,153,0.1);color:#34d399">%s: %s</span>`, p.ID(), status)
	}

	totalReq := 0
	totalCost := 0.0
	totalSaving := 0.0
	savingPct := 0.0
	avgLat := 0.0
	if summary != nil {
		totalReq = summary.TotalRequests
		totalCost = summary.TotalCostUSD
		totalSaving = summary.TotalSavingUSD
		savingPct = summary.SavingPercent
		avgLat = summary.AvgLatencyMs
	}

	modelRows := ""
	if summary != nil {
		for _, mb := range summary.ByModel {
			modelRows += fmt.Sprintf(`<tr><td>%s</td><td>%d</td><td>$%.4f</td></tr>`, mb.Model, mb.Requests, mb.CostUSD)
		}
	}

	html := fmt.Sprintf(`<h2>Overview</h2>
<div class="stats">
  <div class="stat"><div class="label">Requests (24h)</div><div class="value" style="color:var(--accent2)">%d</div></div>
  <div class="stat"><div class="label">Cost Today</div><div class="value" style="color:var(--yellow)">$%.4f</div></div>
  <div class="stat"><div class="label">Saved</div><div class="value" style="color:var(--green)">$%.4f</div><div class="sub">%.1f%% savings</div></div>
  <div class="stat"><div class="label">Avg Latency</div><div class="value">%.0fms</div></div>
</div>
<div style="margin-bottom:16px">%s</div>
<div class="charts">
  <div class="chart-box"><h3>Requests by Model</h3><canvas id="modelChart"></canvas></div>
  <div class="chart-box"><h3>Cost Distribution</h3><canvas id="costChart"></canvas></div>
</div>
%s
<script>
(function(){
var mc=document.getElementById('modelChart'), cc=document.getElementById('costChart');
if(!mc||!cc)return;
var summary=%s;
var labels=[],counts=[],clabels=[],ccosts=[];
if(summary&&summary.by_model){summary.by_model.forEach(function(m){labels.push(m.model);counts.push(m.requests);clabels.push(m.model);ccosts.push(m.cost_usd);});}
if(labels.length===0){labels=['no data'];counts=[1];clabels=['no data'];ccosts=[0];}
new Chart(mc,{type:'bar',data:{labels:labels,datasets:[{label:'Requests',data:counts,backgroundColor:'#818cf8',borderRadius:4}]},options:{responsive:true,plugins:{legend:{display:false}},scales:{y:{beginAtZero:true,grid:{color:'#1e1e2a'},ticks:{color:'#8888a0'}},x:{grid:{display:false},ticks:{color:'#8888a0'}}}}});
new Chart(cc,{type:'doughnut',data:{labels:clabels,datasets:[{data:ccosts,backgroundColor:['#818cf8','#22d3ee','#34d399','#fbbf24','#f87171'],borderWidth:0}]},options:{responsive:true,plugins:{legend:{position:'bottom',labels:{color:'#8888a0',padding:12,font:{size:11}}}}}});
})();
</script>`, totalReq, totalCost, totalSaving, savingPct, avgLat, providerStatuses, buildModelTable(modelRows), summaryJSON(summary))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (d *Dashboard) handleRoutingData(w http.ResponseWriter, r *http.Request) {
	html := `<h2>Routing</h2>
<div style="display:grid;grid-template-columns:1fr 1fr;gap:20px">
<div class="chart-box">
  <h3>Test Routing</h3>
  <textarea id="routing-test-input" placeholder="Enter a message to test routing..." rows="3" style="margin-bottom:8px;width:100%%"></textarea>
  <button class="btn btn-primary btn-sm" onclick="testRoute()">Classify</button>
  <div id="routing-test-result" style="margin-top:12px;font-size:13px"></div>
</div>
<div class="chart-box">
  <h3>Routing Strategies</h3>
  <p style="font-size:13px;color:var(--muted)">Active strategies are tried in order. First strategy with confidence ≥ threshold wins.</p>
  <table style="margin-top:8px"><tr><th>Tier</th><th>Strategy</th><th>Status</th></tr>
  <tr><td>1</td><td>Rule Engine</td><td><span style="color:var(--green)">Active</span></td></tr>
  <tr><td>2</td><td>Embedding</td><td><span style="color:var(--yellow)">Available</span></td></tr>
  <tr><td>3</td><td>ML Classifier</td><td><span style="color:var(--muted)">Inactive</span></td></tr>
  </table>
</div>
</div>
<script>
function testRoute(){
  var input=document.getElementById('routing-test-input').value;
  var adminKey=localStorage.getItem('adminKey')||'';
  fetch('/dashboard/api/routing/test',{method:'POST',headers:{'Content-Type':'application/json','X-Admin-Key':adminKey},body:JSON.stringify({messages:[{role:'user',content:input}]})})
  .then(r=>r.json()).then(d=>{
    var badge='<span class="badge badge-'+d.complexity+'">'+['trivial','simple','moderate','complex','expert'][d.complexity]+'</span>';
    document.getElementById('routing-test-result').innerHTML='<p><strong>Model:</strong> '+d.provider+'/'+d.model+' '+badge+'</p><p style="color:var(--muted);font-size:12px;margin-top:4px">'+d.reason+'</p>';
  }).catch(e=>{document.getElementById('routing-test-result').innerHTML='<p style="color:var(--red)">Error: '+e.message+'</p>';});
}
</script>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (d *Dashboard) handleKeysData(w http.ResponseWriter, r *http.Request) {
	keys, _ := d.store.ListAPIKeys()

	rows := ""
	for _, k := range keys {
		lastUsed := "never"
		if k.LastUsedAt != nil {
			lastUsed = k.LastUsedAt.Format("2006-01-02 15:04")
		}
		rows += fmt.Sprintf(`<tr><td>%s</td><td>%s</td><td>%d rpm</td><td>$%.0f</td><td>%s</td><td><button class="btn btn-danger btn-sm" onclick="deleteKey('%s')">Delete</button></td></tr>`,
			k.Prefix, k.Name, k.RateLimitRPM, k.BudgetUSD, lastUsed, k.ID)
	}

	html := fmt.Sprintf(`<h2>API Keys</h2>
<div class="chart-box" style="margin-bottom:16px">
  <h3>Create New Key</h3>
  <div style="display:flex;gap:8px;align-items:end">
    <div><label style="font-size:11px;color:var(--muted)">Name</label><input id="key-name" placeholder="production"></div>
    <div><label style="font-size:11px;color:var(--muted)">Rate Limit (rpm)</label><input id="key-rate" type="number" value="100" style="width:100px"></div>
    <button class="btn btn-primary btn-sm" onclick="createKey()" style="margin-bottom:10px">Create</button>
  </div>
  <div id="key-result" style="margin-top:8px;font-size:13px"></div>
</div>
<table><tr><th>Key</th><th>Name</th><th>Rate Limit</th><th>Budget</th><th>Last Used</th><th></th></tr>%s</table>
<script>
function createKey(){
  var adminKey=localStorage.getItem('adminKey')||'';
  fetch('/dashboard/api/keys/create',{method:'POST',headers:{'Content-Type':'application/json','X-Admin-Key':adminKey},body:JSON.stringify({name:document.getElementById('key-name').value||'default',rate_limit_rpm:parseInt(document.getElementById('key-rate').value)||100})})
  .then(r=>r.json()).then(d=>{
    document.getElementById('key-result').innerHTML='<p style="color:var(--green)">Key created: <code>'+d.raw_key+'</code> (save this!)</p>';
    setTimeout(function(){htmx.ajax('GET','/dashboard/data/keys','#keys-content');},500);
  });
}
function deleteKey(id){
  if(!confirm('Delete this key?'))return;
  var adminKey=localStorage.getItem('adminKey')||'';
  fetch('/dashboard/api/keys/delete?id='+id,{method:'DELETE',headers:{'X-Admin-Key':adminKey}})
  .then(function(){htmx.ajax('GET','/dashboard/data/keys','#keys-content');});
}
</script>`, rows)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (d *Dashboard) handleLogData(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from := now.Add(-24 * time.Hour)
	decisions, total, _ := d.store.ListDecisions(from, now, "", 50, 0)

	rows := ""
	for _, dec := range decisions {
		badgeClass := "badge-moderate"
		if dec.Complexity == "trivial" || dec.Complexity == "simple" {
			badgeClass = "badge-trivial"
		} else if dec.Complexity == "complex" || dec.Complexity == "expert" {
			badgeClass = "badge-expert"
		}

		status := "200"
		if dec.Error != "" {
			status = "ERR"
		}

		rows += fmt.Sprintf(`<tr class="log-table"><td>%s</td><td>%s/%s</td><td><span class="badge %s">%s</span></td><td>%d/%d</td><td>$%.5f</td><td>%dms</td><td>%s</td></tr>`,
			dec.Timestamp.Format("15:04:05"), dec.Provider, dec.Model, badgeClass, dec.Complexity,
			dec.TokensIn, dec.TokensOut, dec.CostUSD, dec.LatencyMs, status)
	}

	html := fmt.Sprintf(`<h2>Request Log <span class="live-dot" style="margin-left:8px"></span></h2>
<div style="margin-bottom:16px;color:var(--muted);font-size:13px">Showing last 50 of %d requests (24h)</div>
<div class="chart-box">
<table><tr><th>Time</th><th>Model</th><th>Complexity</th><th>Tokens</th><th>Cost</th><th>Latency</th><th>Status</th></tr>%s</table>
</div>
<div hx-ext="sse" sse-connect="/dashboard/sse" sse-swap="new-request" hx-swap="afterbegin" hx-target="tbody"></div>`, total, rows)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (d *Dashboard) handleAnalyticsJSON(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from := now.Add(-24 * time.Hour)
	summary, err := d.store.GetAnalyticsSummary(from, now, "")
	if err != nil {
		summary = &store.AnalyticsSummary{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (d *Dashboard) handleRoutingTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages []strategy.Message `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	decision, err := d.router.Classify(r.Context(), &strategy.Request{Messages: req.Messages})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decision)
}

func (d *Dashboard) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		RateLimitRPM int    `json:"rate_limit_rpm"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "default"
	}
	if req.RateLimitRPM <= 0 {
		req.RateLimitRPM = 100
	}

	_, rawKey, err := d.store.CreateAPIKey(req.Name, "default", req.RateLimitRPM, 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"raw_key": rawKey})
}

func (d *Dashboard) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	d.store.DeleteAPIKey(id)
	w.WriteHeader(200)
}

func (d *Dashboard) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan LiveEvent, 20)
	d.feed.mu.Lock()
	d.feed.subs[ch] = struct{}{}
	d.feed.mu.Unlock()

	defer func() {
		d.feed.mu.Lock()
		delete(d.feed.subs, ch)
		d.feed.mu.Unlock()
	}()

	flusher, _ := w.(http.Flusher)
	for {
		select {
		case e := <-ch:
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

func buildModelTable(rows string) string {
	if rows == "" {
		return ""
	}
	return fmt.Sprintf(`<div class="chart-box"><h3>Model Breakdown</h3><table><tr><th>Model</th><th>Requests</th><th>Cost</th></tr>%s</table></div>`, rows)
}

func summaryJSON(s *store.AnalyticsSummary) string {
	if s == nil {
		return "{}"
	}
	data, _ := json.Marshal(s)
	return string(data)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Aperture Dashboard</title>
<script src="https://unpkg.com/htmx.org@1.9.12"></script>
<script src="https://unpkg.com/htmx-ext-sse@2.2.1/sse.js"></script>
<script src="https://unpkg.com/alpinejs@3.13.7/dist/cdn.min.js" defer></script>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.4/dist/chart.umd.min.js"></script>
<style>
:root{--bg:#0a0a0f;--surface:#131319;--border:#1e1e2a;--text:#e0e0e8;--muted:#8888a0;--accent:#818cf8;--accent2:#22d3ee;--green:#34d399;--red:#f87171;--yellow:#fbbf24}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,sans-serif;background:var(--bg);color:var(--text);display:flex;min-height:100vh}
.sidebar{width:220px;background:var(--surface);border-right:1px solid var(--border);padding:20px 0;flex-shrink:0;display:flex;flex-direction:column}
.sidebar h1{font-size:18px;padding:0 20px 20px;color:var(--accent);letter-spacing:-0.5px}
.sidebar h1 span{color:var(--accent2)}
.sidebar a{display:flex;align-items:center;gap:10px;padding:10px 20px;color:var(--muted);text-decoration:none;font-size:14px;transition:all 0.15s;border-left:2px solid transparent}
.sidebar a:hover,.sidebar a.active{color:var(--text);background:rgba(129,140,248,0.08);border-left-color:var(--accent)}
.main{flex:1;padding:24px 32px;overflow-y:auto;max-height:100vh}
.main h2{font-size:22px;margin-bottom:20px;font-weight:600}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px;margin-bottom:28px}
.stat{background:var(--surface);border:1px solid var(--border);border-radius:10px;padding:18px}
.stat .label{font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:0.5px;margin-bottom:6px}
.stat .value{font-size:28px;font-weight:700}
.stat .sub{font-size:12px;color:var(--muted);margin-top:4px}
.charts{display:grid;grid-template-columns:2fr 1fr;gap:16px;margin-bottom:28px}
.chart-box{background:var(--surface);border:1px solid var(--border);border-radius:10px;padding:18px}
.chart-box h3{font-size:14px;color:var(--muted);margin-bottom:12px;text-transform:uppercase;letter-spacing:0.5px}
canvas{width:100%!important;max-height:260px}
table{width:100%;border-collapse:collapse;font-size:13px}
th{text-align:left;padding:10px 12px;color:var(--muted);font-weight:500;border-bottom:1px solid var(--border);font-size:11px;text-transform:uppercase;letter-spacing:0.5px}
td{padding:10px 12px;border-bottom:1px solid var(--border)}
tr:hover td{background:rgba(129,140,248,0.04)}
.badge{display:inline-block;padding:2px 8px;border-radius:12px;font-size:11px;font-weight:500}
.badge-trivial{background:rgba(52,211,153,0.15);color:var(--green)}
.badge-complex{background:rgba(248,113,113,0.15);color:var(--red)}
.badge-moderate{background:rgba(251,191,36,0.15);color:var(--yellow)}
.badge-expert{background:rgba(129,140,248,0.2);color:var(--accent)}
.btn{display:inline-flex;align-items:center;gap:6px;padding:8px 16px;border-radius:8px;font-size:13px;cursor:pointer;border:none;font-weight:500;transition:all 0.15s}
.btn-primary{background:var(--accent);color:#fff}.btn-primary:hover{opacity:0.9}
.btn-danger{background:rgba(248,113,113,0.15);color:var(--red)}.btn-danger:hover{background:rgba(248,113,113,0.25)}
.btn-sm{padding:4px 10px;font-size:12px}
input,select,textarea{background:var(--bg);border:1px solid var(--border);border-radius:6px;padding:8px 12px;color:var(--text);font-size:13px;width:100%;margin-bottom:10px}
input:focus,select:focus{outline:none;border-color:var(--accent)}
.log-table{font-family:monospace;font-size:12px}.log-table td{padding:6px 10px}
.live-dot{width:8px;height:8px;background:var(--green);border-radius:50%;display:inline-block;animation:pulse 1.5s infinite}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:0.3}}
</style>
</head>
<body x-data="{page:'overview',adminKey:localStorage.getItem('adminKey')||''}">
<aside class="sidebar">
  <h1>Aperture<span>.</span></h1>
  <a href="#" :class="page==='overview'&&'active'" @click.prevent="page='overview';htmx.ajax('GET','/dashboard/data/overview','#page-content')">Overview</a>
  <a href="#" :class="page==='routing'&&'active'" @click.prevent="page='routing';htmx.ajax('GET','/dashboard/data/routing','#page-content')">Routing</a>
  <a href="#" :class="page==='keys'&&'active'" @click.prevent="page='keys';htmx.ajax('GET','/dashboard/data/keys','#page-content')">API Keys</a>
  <a href="#" :class="page==='log'&&'active'" @click.prevent="page='log';htmx.ajax('GET','/dashboard/data/log','#page-content')">Request Log</a>
  <div style="padding:20px;margin-top:auto">
    <label style="font-size:11px;color:var(--muted)">Admin Key</label>
    <input type="password" x-model="adminKey" @change="localStorage.setItem('adminKey',adminKey)" style="font-size:11px;padding:6px 10px">
  </div>
</aside>
<main class="main">
  <div id="page-content" hx-get="/dashboard/data/overview" hx-trigger="load" hx-swap="innerHTML">
    <div class="spinner" style="width:24px;height:24px;border-width:3px;margin:40px auto"></div>
  </div>
</main>
</body>
</html>`
