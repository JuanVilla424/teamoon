(function(){
"use strict";

var D = null;
var prevDataStr = "";
var selectedTaskID = 0;
var currentPRsRepo = "";
var logAutoScroll = true;
var queueFilterState = "";
var queueFilterProject = "";
var logFilterLevel = "";
var logFilterTask = "";
var logFilterProject = "";
var prevView = "";
var isDataUpdate = false;
var taskLogAutoScroll = true;
var templatesCache = [];
var planCollapsed = true;
var taskLogsCache = {};
var planCache = {};
var prevContentKey = "";
var prevLogCount = 0;
var prevTaskLogCounts = {};
var chatMessages = [];
var chatLoading = false;
var chatProject = "";
var chatCounter = 0;
var chatCreatedTasks = [];
var canvasFilterAssignee = "";
var canvasFilterProject = "";
var canvasDragTaskId = 0;
var canvasDragFromCol = "";

/* ── SVG Icons ── */
var ICONS = {
  tokens: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>',
  usage: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>',
  context: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>',
  queue: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>',
  projects: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>',
  week: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>',
  month: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>'
};

function svgIcon(str){
  var wrapper = document.createElement("span");
  wrapper.style.cssText = "display:inline-flex;align-items:center;line-height:0";
  wrapper.innerHTML = str;
  return wrapper;
}

/* ── SSE ── */
function connectSSE(){
  var es = new EventSource("/api/sse");
  es.onmessage = function(ev){
    try{
      var parsed = JSON.parse(ev.data);
      var cmp = JSON.stringify((function(o){
        var c = {}; for(var k in o) if(k !== "timestamp") c[k] = o[k]; return c;
      })(parsed));
      if(cmp === prevDataStr) return;
      prevDataStr = cmp;
      D = parsed;
      isDataUpdate = true;
      render();
    }catch(e){}
  };
  es.onerror = function(){
    es.close();
    setTimeout(connectSSE, 3000);
  };
}

/* ── Fetch helper ── */
function api(method, path, body, cb){
  var opts = {method: method, headers: {"Content-Type":"application/json"}};
  if(body) opts.body = JSON.stringify(body);
  fetch(path, opts)
    .then(function(r){ return r.json(); })
    .then(function(d){ if(cb) cb(d); })
    .catch(function(e){ console.error(e); });
}

/* ── Router ── */
function getView(){
  var h = location.hash.replace("#","") || "dashboard";
  if(["dashboard","queue","canvas","projects","logs","chat"].indexOf(h) < 0) h = "dashboard";
  return h;
}

/* ── Safe DOM helpers ── */
function txt(s){ return document.createTextNode(s || ""); }
function el(tag, cls, children){
  var e = document.createElement(tag);
  if(cls) e.className = cls;
  if(children){
    if(!Array.isArray(children)) children = [children];
    for(var i=0;i<children.length;i++){
      if(typeof children[i] === "string") e.appendChild(txt(children[i]));
      else if(children[i]) e.appendChild(children[i]);
    }
  }
  return e;
}
function elAttr(tag, cls, attrs, children){
  var e = el(tag, cls, children);
  if(attrs) for(var k in attrs) if(attrs.hasOwnProperty(k)) e.setAttribute(k, attrs[k]);
  return e;
}
function span(cls, text){
  var s = document.createElement("span");
  if(cls) s.className = cls;
  s.textContent = text || "";
  return s;
}
function div(cls, children){ return el("div", cls, children); }

function computeContentKey(v){
  if(!D) return "";
  var tasks = D.tasks || [];
  var logs = D.log_entries || [];
  var projs = D.projects || [];
  switch(v){
    case "dashboard":
      var tk = tasks.length + ":";
      for(var i=0;i<tasks.length;i++) tk += tasks[i].effective_state + ",";
      return "d:" + JSON.stringify(D.today||{}) + JSON.stringify(D.cost||{}) +
        (D.session ? D.session.context_percent : 0) + ":" + tk + ":" + logs.length;
    case "queue":
      var tk = "";
      for(var i=0;i<tasks.length;i++){
        var t = tasks[i];
        tk += t.id + "," + t.effective_state + "," + t.is_running + "," + t.has_plan + ";";
      }
      var selLogs = 0;
      for(var i=0;i<logs.length;i++){
        if(logs[i].task_id === selectedTaskID) selLogs++;
      }
      return "q:" + queueFilterState + ":" + queueFilterProject + ":" + selectedTaskID + ":" + tk + ":" + selLogs;
    case "projects":
      var pk = "";
      for(var i=0;i<projs.length;i++){
        var p = projs[i];
        pk += p.name + "," + p.status_icon + "," + (p.modified||0) + "," + (p.branch||"") + ";";
      }
      return "p:" + pk;
    case "logs":
      return "l:" + logFilterLevel + ":" + logFilterTask + ":" + logFilterProject + ":" + logs.length;
    case "chat":
      return "chat:" + chatCounter;
    case "canvas":
      var ck = canvasFilterAssignee + ":" + canvasFilterProject + ":";
      for(var i=0;i<tasks.length;i++) ck += tasks[i].id + tasks[i].effective_state + (tasks[i].assignee||"") + ",";
      return "cv:" + ck;
    default:
      return "";
  }
}

function render(){
  if(!D) return;
  updateSidebar();
  var v = getView();
  var nav = document.querySelectorAll("#sb-nav a");
  for(var i=0;i<nav.length;i++){
    nav[i].classList.toggle("active", nav[i].getAttribute("data-view") === v);
  }

  var content = document.getElementById("content");
  var main = document.querySelector(".main");
  var viewChanged = (v !== prevView);
  prevView = v;

  // Skip full content rebuild if view-specific data unchanged
  var contentKey = computeContentKey(v);
  if(!viewChanged && contentKey === prevContentKey){
    isDataUpdate = false;
    return;
  }
  prevContentKey = contentKey;

  // Incremental log append for task terminal (queue view)
  if(!viewChanged && v === "queue" && isDataUpdate && selectedTaskID){
    var allLogs = (D && D.log_entries) ? D.log_entries : [];
    var taskLogs = [];
    for(var i = 0; i < allLogs.length; i++){
      if(allLogs[i].task_id === selectedTaskID) taskLogs.push(allLogs[i]);
    }
    var prevCount = prevTaskLogCounts[selectedTaskID] || 0;
    if(taskLogs.length > prevCount){
      var term = document.querySelector(".task-terminal");
      if(term){
        // Remove empty message if present
        var emptyEl = term.querySelector(".task-terminal-empty");
        if(emptyEl) emptyEl.remove();
        for(var i = prevCount; i < taskLogs.length; i++){
          term.appendChild(mkTerminalLine(taskLogs[i]));
        }
        prevTaskLogCounts[selectedTaskID] = taskLogs.length;
        if(taskLogAutoScroll) term.scrollTop = term.scrollHeight;
        updateSidebar();
        isDataUpdate = false;
        return;
      }
    }
  }

  // Incremental log append for logs view
  if(!viewChanged && v === "logs" && isDataUpdate){
    var allLogs = (D && D.log_entries) ? D.log_entries : [];
    var filtered = allLogs.filter(function(l){
      if(logFilterLevel && l.level !== logFilterLevel) return false;
      if(logFilterTask && String(l.task_id) !== logFilterTask) return false;
      if(logFilterProject && l.project !== logFilterProject) return false;
      return true;
    });
    if(filtered.length > prevLogCount){
      var lc = document.getElementById("log-container");
      if(lc){
        var emptyEl = lc.querySelector(".empty");
        if(emptyEl) emptyEl.remove();
        for(var i = prevLogCount; i < filtered.length; i++){
          var l = filtered[i];
          var entry = div("log-entry " + l.level);
          entry.appendChild(span("log-time", fmtTime(l.time)));
          var icon = span("log-icon");
          entry.appendChild(icon);
          entry.appendChild(span("log-proj", l.project || "\u2014"));
          entry.appendChild(span("log-task", l.task_id ? "#"+l.task_id : ""));
          entry.appendChild(span("log-msg", l.message));
          lc.appendChild(entry);
        }
        prevLogCount = filtered.length;
        if(logAutoScroll) lc.scrollTop = lc.scrollHeight;
        updateSidebar();
        isDataUpdate = false;
        return;
      }
    }
  }

  if(isDataUpdate && !viewChanged){
    main.classList.add("updating");
  } else {
    main.classList.remove("updating");
  }
  isDataUpdate = false;

  // Save scroll positions before swap
  var mainScroll = content.scrollTop;
  var splitLeftScroll = 0, splitRightScroll = 0, logContainerScroll = 0;
  var existingSL = content.querySelector(".split-left");
  if(existingSL) splitLeftScroll = existingSL.scrollTop;
  var existingSR = content.querySelector(".split-right");
  if(existingSR) splitRightScroll = existingSR.scrollTop;
  var existingLC = document.getElementById("log-container");
  if(existingLC) logContainerScroll = existingLC.scrollTop;

  // Save task terminal scroll position
  var taskTermScroll = 0;
  var existingTerm = content.querySelector(".task-terminal");
  if(existingTerm){
    taskTermScroll = existingTerm.scrollTop;
    var atBottom = (existingTerm.scrollHeight - existingTerm.scrollTop - existingTerm.clientHeight) < 8;
    if(!atBottom) taskLogAutoScroll = false;
  }

  // Build new content into temp container
  var tmp = document.createElement("div");
  switch(v){
    case "dashboard": renderDashboard(tmp); break;
    case "queue": renderQueue(tmp); break;
    case "canvas": renderCanvas(tmp); break;
    case "projects": renderProjects(tmp); break;
    case "logs": renderLogs(tmp); break;
    case "chat": renderChat(tmp); break;
  }

  // Atomic swap
  content.textContent = "";
  while(tmp.firstChild) content.appendChild(tmp.firstChild);

  // Update incremental log counters after full render
  if(v === "logs"){
    var allLogs = (D && D.log_entries) ? D.log_entries : [];
    var filtered = allLogs.filter(function(l){
      if(logFilterLevel && l.level !== logFilterLevel) return false;
      if(logFilterTask && String(l.task_id) !== logFilterTask) return false;
      if(logFilterProject && l.project !== logFilterProject) return false;
      return true;
    });
    prevLogCount = filtered.length;
  }
  if(v === "queue" && selectedTaskID){
    var allLogs = (D && D.log_entries) ? D.log_entries : [];
    var count = 0;
    for(var i = 0; i < allLogs.length; i++){
      if(allLogs[i].task_id === selectedTaskID) count++;
    }
    prevTaskLogCounts[selectedTaskID] = count;
  }

  // Restore scroll positions
  content.scrollTop = mainScroll;
  var newSL = content.querySelector(".split-left");
  if(newSL) newSL.scrollTop = splitLeftScroll;
  var newSR = content.querySelector(".split-right");
  if(newSR) newSR.scrollTop = splitRightScroll;

  if(v === "logs"){
    var lc = document.getElementById("log-container");
    if(lc){
      if(logAutoScroll) lc.scrollTop = lc.scrollHeight;
      else lc.scrollTop = logContainerScroll;
    }
  }

  // Restore task terminal scroll
  var newTerm = content.querySelector(".task-terminal");
  if(newTerm){
    if(taskLogAutoScroll){
      newTerm.scrollTop = newTerm.scrollHeight;
      var jb = newTerm.parentElement ? newTerm.parentElement.querySelector(".jump-btn") : null;
      if(jb) jb.classList.add("hidden");
    } else {
      newTerm.scrollTop = taskTermScroll;
      var jb = newTerm.parentElement ? newTerm.parentElement.querySelector(".jump-btn") : null;
      if(jb) jb.classList.remove("hidden");
    }
    newTerm.onscroll = function(){
      var atBot = (newTerm.scrollHeight - newTerm.scrollTop - newTerm.clientHeight) < 8;
      taskLogAutoScroll = atBot;
      var jb = newTerm.parentElement ? newTerm.parentElement.querySelector(".jump-btn") : null;
      if(jb){
        if(atBot) jb.classList.add("hidden");
        else jb.classList.remove("hidden");
      }
      var cb = document.querySelector(".log-autoscroll-label input");
      if(cb) cb.checked = atBot;
    };
  }
}

window.addEventListener("hashchange", render);

/* ── Sidebar ── */
function updateSidebar(){
  var ver = document.getElementById("sb-version");
  var newVer = "v" + (D.version || "?");
  if(D.build_num && D.build_num !== "0") newVer += " #" + D.build_num;
  if(ver.textContent !== newVer) ver.textContent = newVer;

  var mdl = document.getElementById("sb-model");
  var parts = [];
  if(D.plan_model) parts.push(D.plan_model);
  if(D.effort) parts.push(D.effort);
  var newMdl = parts.join(" \u00b7 ");
  if(mdl.textContent !== newMdl) mdl.textContent = newMdl;

  var tasks = D.tasks || [];
  var running = 0, pending = 0, blocked = 0;
  for(var i=0;i<tasks.length;i++){
    var s = tasks[i].effective_state;
    if(s === "running") running++;
    else if(s === "pending" || s === "generating") pending++;
    else if(s === "blocked") blocked++;
  }

  var ctx = D.session || {};
  var ctxPct = ctx.context_percent || 0;
  var ctxCls = ctxPct >= 80 ? "red" : ctxPct >= 60 ? "orange" : "green";

  var container = document.getElementById("sb-stats");
  container.textContent = "";

  var row1 = div("sb-stat");
  row1.appendChild(span("", "Context"));
  row1.appendChild(span("val " + ctxCls, Math.round(ctxPct) + "%"));
  container.appendChild(row1);

  var prog = div("progress");
  var fill = div("progress-fill " + ctxCls);
  fill.style.width = ctxPct + "%";
  prog.appendChild(fill);
  container.appendChild(prog);

  var row2 = div("sb-stat mt-8");
  row2.appendChild(span("", "Tasks"));
  row2.appendChild(span("val", String(tasks.length)));
  container.appendChild(row2);

  var row3 = div("sb-stat");
  row3.appendChild(span("", "Running"));
  row3.appendChild(span("val green", String(running)));
  container.appendChild(row3);

  if(pending){
    var row4 = div("sb-stat");
    row4.appendChild(span("", "Pending"));
    row4.appendChild(span("val", String(pending)));
    container.appendChild(row4);
  }
  if(blocked){
    var row5 = div("sb-stat");
    row5.appendChild(span("", "Blocked"));
    row5.appendChild(span("val red", String(blocked)));
    container.appendChild(row5);
  }
}

/* ── Dashboard View ── */
function renderDashboard(root){
  var t = D.today || {}, w = D.week || {}, m = D.month || {};
  var c = D.cost || {};
  var ctx = D.session || {};
  var tasks = D.tasks || [];
  var projs = D.projects || [];
  var logs = D.log_entries || [];

  var totalToday = (t.input||0)+(t.output||0)+(t.cache_read||0)+(t.cache_create||0);

  var header = div("view-header");
  header.appendChild(span("view-title", "Dashboard"));
  root.appendChild(header);

  // Top 3 metric cards
  var cards = div("cards cards-3");

  // Tokens card — cyan accent
  var tc = div("card card-accent-cyan");
  tc.appendChild(mkLabel("Tokens Today", ICONS.tokens));
  var tcVal = mkValue(fmtNum(totalToday));
  tcVal.classList.add("grad-cyan");
  tc.appendChild(tcVal);
  tc.appendChild(mkMetricRow([
    "In: " + fmtNum(t.input||0),
    "Out: " + fmtNum(t.output||0)
  ]));
  tc.appendChild(mkMetricRow([
    "Cache R: " + fmtNum(t.cache_read||0),
    "Cache W: " + fmtNum(t.cache_create||0)
  ]));
  cards.appendChild(tc);

  // Usage card — yellow accent
  var cc = div("card card-accent-yellow");
  cc.appendChild(mkLabel("Usage", ICONS.usage));
  var cv = mkValue(String(c.sessions_month||0) + " sessions");
  cv.classList.add("grad-yellow");
  cc.appendChild(cv);
  cc.appendChild(mkSub("This month"));
  cc.appendChild(mkMetricRow([
    "Today: " + (c.sessions_today||0) + " sess",
    "Out: " + fmtNum(c.output_today||0)
  ]));
  cc.appendChild(mkMetricRow([
    "Week: " + (c.sessions_week||0) + " sess",
    "Out: " + fmtNum(c.output_week||0)
  ]));
  if(c.plan_cost > 0){
    cc.appendChild(mkMetricRow([
      "Budget: $" + fmtCost(c.plan_cost),
      "Month Out: " + fmtNum(c.output_month||0)
    ]));
  }
  cards.appendChild(cc);

  // Context card — dynamic accent
  var ctxPct = ctx.context_percent || 0;
  var ctxColor = ctxPct >= 80 ? "red" : ctxPct >= 60 ? "orange" : "green";
  var ctxCard = div("card card-accent-" + ctxColor);
  ctxCard.appendChild(mkLabel("Context Window", ICONS.context));
  var ctxVal = mkValue(Math.round(ctxPct) + "%");
  ctxVal.classList.add("grad-" + ctxColor);
  ctxCard.appendChild(ctxVal);
  var prog = div("progress mt-8");
  var pf = div("progress-fill " + (ctxPct >= 80 ? "red" : ctxPct >= 60 ? "yellow" : "green"));
  pf.style.width = ctxPct + "%";
  prog.appendChild(pf);
  ctxCard.appendChild(prog);
  if(ctx.session_file){
    ctxCard.appendChild(mkSub("Session: " + String(ctx.session_file).substring(0, 16) + "..."));
  }
  cards.appendChild(ctxCard);
  root.appendChild(cards);

  // Activity feed
  var recentLogs = logs.slice(-10).reverse();
  var feed = div("feed");
  feed.appendChild(mkFeedTitle("Recent Activity"));
  if(recentLogs.length === 0){
    feed.appendChild(div("empty", ["No activity yet"]));
  } else {
    for(var i=0;i<recentLogs.length;i++){
      var le = recentLogs[i];
      var item = div("feed-item");
      item.appendChild(span("feed-time", fmtTime(le.time)));
      var lvlSpan = span("feed-level");
      lvlSpan.appendChild(levelIconEl(le.level));
      item.appendChild(lvlSpan);
      var msg = span("feed-msg");
      if(le.project){
        var ps = span("", le.project);
        ps.style.color = "var(--cyan)";
        msg.appendChild(ps);
        msg.appendChild(txt(" "));
      }
      if(le.task_id) msg.appendChild(txt("#" + le.task_id + " "));
      msg.appendChild(txt(le.message));
      item.appendChild(msg);
      feed.appendChild(item);
    }
  }
  root.appendChild(feed);

  // Summary cards
  var running=0,pendingC=0,blockedC=0,planned=0;
  for(var i=0;i<tasks.length;i++){
    var s=tasks[i].effective_state;
    if(s==="running")running++;else if(s==="pending"||s==="generating")pendingC++;
    else if(s==="blocked")blockedC++;else if(s==="planned")planned++;
  }
  var activeProj=0,staleProj=0;
  for(var i=0;i<projs.length;i++){
    if(projs[i].active)activeProj++;
    if(projs[i].stale)staleProj++;
  }

  var sumCards = div("cards cards-2 mt-16");

  var qCard = div("card card-accent-purple");
  qCard.style.cursor = "pointer";
  qCard.onclick = function(){ location.hash = "queue"; };
  qCard.appendChild(mkLabel("Queue Summary", ICONS.queue));
  qCard.appendChild(mkMetricRow(["Total: " + tasks.length]));
  qCard.appendChild(mkMetricRow(["Running: " + running, "Planned: " + planned]));
  qCard.appendChild(mkMetricRow(["Pending: " + pendingC, "Blocked: " + blockedC]));
  sumCards.appendChild(qCard);

  var pCard = div("card card-accent-blue");
  pCard.style.cursor = "pointer";
  pCard.onclick = function(){ location.hash = "projects"; };
  pCard.appendChild(mkLabel("Projects", ICONS.projects));
  pCard.appendChild(mkMetricRow(["Total: " + projs.length]));
  pCard.appendChild(mkMetricRow(["Active: " + activeProj, "Stale: " + staleProj]));
  sumCards.appendChild(pCard);
  root.appendChild(sumCards);

  // Week/Month token cards
  var totalWeek = (w.input||0)+(w.output||0)+(w.cache_read||0)+(w.cache_create||0);
  var totalMonth = (m.input||0)+(m.output||0)+(m.cache_read||0)+(m.cache_create||0);

  var wmCards = div("cards cards-2 mt-16");
  var wCard = div("card card-accent-cyan");
  wCard.appendChild(mkLabel("Tokens This Week", ICONS.week));
  var wVal = mkValue(fmtNum(totalWeek));
  wVal.classList.add("grad-cyan");
  wCard.appendChild(wVal);
  wCard.appendChild(mkMetricRow(["In: "+fmtNum(w.input||0), "Out: "+fmtNum(w.output||0)]));
  wmCards.appendChild(wCard);

  var mCard = div("card card-accent-purple");
  mCard.appendChild(mkLabel("Tokens This Month", ICONS.month));
  var mVal = mkValue(fmtNum(totalMonth));
  mVal.classList.add("grad-purple");
  mCard.appendChild(mVal);
  mCard.appendChild(mkMetricRow(["In: "+fmtNum(m.input||0), "Out: "+fmtNum(m.output||0)]));
  wmCards.appendChild(mCard);
  root.appendChild(wmCards);
}

/* ── Queue View ── */
function renderQueue(root){
  var tasks = D.tasks || [];

  var stateSet = {}, projSet = {};
  for(var i=0;i<tasks.length;i++){
    stateSet[tasks[i].effective_state] = true;
    if(tasks[i].project) projSet[tasks[i].project] = true;
  }

  var header = div("view-header");
  header.appendChild(span("view-title", "Queue"));
  var addBtn = el("button", "btn btn-primary", ["+ Add Task"]);
  addBtn.onclick = function(){ openAddTask(""); };
  header.appendChild(addBtn);
  root.appendChild(header);

  var split = div("split");

  // Left panel
  var left = div("split-left");
  var toolbar = div("queue-toolbar");

  var stateSelect = el("select", "filter-select");
  stateSelect.appendChild(mkOption("", "All States"));
  var states = Object.keys(stateSet).sort();
  for(var i=0;i<states.length;i++){
    stateSelect.appendChild(mkOption(states[i], ucfirst(states[i]), queueFilterState===states[i]));
  }
  stateSelect.onchange = function(){ queueFilterState = this.value; render(); };
  toolbar.appendChild(stateSelect);

  var projSelect = el("select", "filter-select");
  projSelect.appendChild(mkOption("", "All Projects"));
  var projNames = Object.keys(projSet).sort();
  for(var i=0;i<projNames.length;i++){
    projSelect.appendChild(mkOption(projNames[i], projNames[i], queueFilterProject===projNames[i]));
  }
  projSelect.onchange = function(){ queueFilterProject = this.value; render(); };
  toolbar.appendChild(projSelect);
  left.appendChild(toolbar);

  var filtered = tasks.filter(function(t){
    if(queueFilterState && t.effective_state !== queueFilterState) return false;
    if(queueFilterProject && t.project !== queueFilterProject) return false;
    return true;
  });

  if(filtered.length === 0){
    var emptyLeft = div("empty");
    emptyLeft.textContent = (queueFilterState || queueFilterProject)
      ? "No tasks match the current filters."
      : "No active tasks. Add one with + Add Task.";
    left.appendChild(emptyLeft);
  } else {
    for(var i=0;i<filtered.length;i++){
      (function(t){
        var cls = "task-item";
        if(t.id === selectedTaskID) cls += " selected";
        if(t.is_running) cls += " has-running";
        if(t.effective_state === "generating") cls += " has-generating";
        var item = div(cls);
        item.onclick = function(){ selectTask(t.id); };
        item.appendChild(span("task-state " + t.effective_state, stateLabel(t.effective_state)));
        item.appendChild(span("task-pri " + t.priority, (t.priority||"").toUpperCase()));
        var info = div("task-info");
        info.appendChild(div("task-desc", [t.description]));
        info.appendChild(div("task-proj", [t.project || "\u2014"]));
        item.appendChild(info);
        if(t.is_running) item.appendChild(div("running-dot"));
        left.appendChild(item);
      })(filtered[i]);
    }
  }
  split.appendChild(left);

  // Right panel
  var right = div("split-right");
  var sel = findTask(selectedTaskID);
  if(!sel){
    var empty = div("empty detail-empty");
    empty.appendChild(div("empty-icon", ["\u2610"]));
    empty.appendChild(div("", ["Select a task from the list to view details"]));
    right.appendChild(empty);
  } else {
    renderTaskDetail(right, sel);
  }
  split.appendChild(right);
  root.appendChild(split);
}

function renderTaskDetail(parent, t){
  // ── Header ──
  var header = div("detail-header");
  var titleEl = div("detail-title");
  titleEl.textContent = "#" + t.id + " " + t.description;
  header.appendChild(titleEl);

  var meta = el("dl", "detail-meta");
  meta.appendChild(el("dt", "", ["Project"]));
  meta.appendChild(el("dd", "", [t.project || "\u2014"]));
  meta.appendChild(el("dt", "", ["State"]));
  var dd2 = el("dd");
  dd2.appendChild(span("task-state " + t.effective_state, stateLabel(t.effective_state)));
  meta.appendChild(dd2);
  meta.appendChild(el("dt", "", ["Priority"]));
  var dd3 = el("dd");
  dd3.appendChild(span("task-pri " + t.priority, (t.priority||"").toUpperCase()));
  meta.appendChild(dd3);
  meta.appendChild(el("dt", "", ["Created"]));
  meta.appendChild(el("dd", "", [fmtDate(t.created_at)]));
  if(t.is_running){
    meta.appendChild(el("dt", "", ["Engine"]));
    var dd5 = el("dd", "detail-engine-running");
    dd5.appendChild(txt("Running "));
    dd5.appendChild(div("running-dot"));
    meta.appendChild(dd5);
  }
  header.appendChild(meta);
  parent.appendChild(header);

  // ── Generating state ──
  if(t.effective_state === "generating"){
    var genSec = div("detail-section detail-generating");
    var genRow = div("detail-generating-inner");
    genRow.appendChild(div("spinner"));
    genRow.appendChild(span("detail-generating-label", "Generating plan..."));
    genSec.appendChild(genRow);
    parent.appendChild(genSec);
  }

  // ── Actions ──
  var actions = div("detail-actions");
  var s = t.effective_state;
  if(s === "pending" || s === "planned" || s === "blocked"){
    var label = s === "pending" ? "Generate Plan" : s === "planned" ? "Run" : "Resume";
    var btn = el("button", "btn btn-primary", [label]);
    btn.onclick = function(){ taskAutopilot(t.id); };
    actions.appendChild(btn);
  }
  if(s === "running"){
    var stopBtn = el("button", "btn btn-danger", ["Stop"]);
    stopBtn.onclick = function(){ taskAutopilot(t.id); };
    actions.appendChild(stopBtn);
  }
  if(s !== "running"){
    var doneBtn = el("button", "btn btn-success", ["Done"]);
    doneBtn.onclick = function(){ taskDone(t.id); };
    actions.appendChild(doneBtn);
  }
  if(t.has_plan){
    var replanBtn = el("button", "btn", ["Replan"]);
    replanBtn.onclick = function(){ taskReplan(t.id); };
    actions.appendChild(replanBtn);
  }
  var archBtn = el("button", "btn btn-danger btn-sm", ["Archive"]);
  archBtn.onclick = function(){ taskArchive(t.id); };
  actions.appendChild(archBtn);
  parent.appendChild(actions);

  // ── Plan section (collapsible) ──
  if(t.has_plan){
    var planSec = div("detail-section");
    var planTitleRow = div("detail-section-title detail-section-toggle");
    planTitleRow.appendChild(txt("Plan"));
    var chevron = span("plan-chevron", planCollapsed ? "\u25B6" : "\u25BC");
    planTitleRow.appendChild(chevron);
    planTitleRow.onclick = function(){
      planCollapsed = !planCollapsed;
      var planBody = document.getElementById("plan-body-" + t.id);
      if(planBody){
        planBody.style.display = planCollapsed ? "none" : "";
        chevron.textContent = planCollapsed ? "\u25B6" : "\u25BC";
      }
    };
    planSec.appendChild(planTitleRow);
    var planBody = div("plan-body");
    planBody.id = "plan-body-" + t.id;
    if(planCollapsed) planBody.style.display = "none";
    var planEl = div("plan-content");
    planEl.id = "plan-content-" + t.id;
    if(planCache[t.id]){
      planEl.className = "plan-content";
      planEl.textContent = planCache[t.id];
    } else {
      planEl.textContent = "Loading...";
      planEl.className = "plan-content loading-text";
    }
    planBody.appendChild(planEl);
    planSec.appendChild(planBody);
    parent.appendChild(planSec);
    if(!planCache[t.id]) loadPlan(t.id);
  }

  // ── Task Logs Terminal (SSE-driven) ──
  var logSec = div("detail-section detail-log-section");
  var logTitleRow = div("detail-section-title detail-log-title-row");
  logTitleRow.appendChild(txt("Task Logs"));
  if(t.is_running){
    logTitleRow.appendChild(span("live-badge", "LIVE"));
  }
  var scrollLabel = el("label", "log-autoscroll-label");
  var scrollCb = el("input");
  scrollCb.type = "checkbox";
  scrollCb.checked = taskLogAutoScroll;
  scrollCb.onchange = function(){
    taskLogAutoScroll = this.checked;
    if(taskLogAutoScroll){
      var term = document.getElementById("task-terminal-" + t.id);
      if(term) term.scrollTop = term.scrollHeight;
    }
  };
  scrollLabel.appendChild(scrollCb);
  scrollLabel.appendChild(txt(" tail"));
  logTitleRow.appendChild(scrollLabel);
  logSec.appendChild(logTitleRow);

  var termWrap = div("task-terminal-wrap");
  var terminal = div("task-terminal");
  terminal.id = "task-terminal-" + t.id;

  // Hybrid: SSE entries first, HTTP fallback for historical
  var allLogs = (D && D.log_entries) ? D.log_entries : [];
  var sseLogs = [];
  for(var i = 0; i < allLogs.length; i++){
    if(allLogs[i].task_id === t.id) sseLogs.push(allLogs[i]);
  }

  // Use SSE logs if available (live), otherwise cached HTTP logs
  var taskLogs = sseLogs.length > 0 ? sseLogs : (taskLogsCache[t.id] || []);

  if(taskLogs.length === 0 && sseLogs.length === 0){
    // No SSE entries and no cache — try HTTP fetch for historical logs
    var emptyMsg = div("task-terminal-empty");
    if(t.effective_state === "pending"){
      emptyMsg.textContent = "No logs yet. Generate a plan to get started.";
    } else if(t.effective_state === "generating"){
      emptyMsg.textContent = "Plan generation in progress...";
    } else {
      emptyMsg.textContent = "Loading logs...";
    }
    terminal.appendChild(emptyMsg);
    // Fetch historical logs from file
    if(t.effective_state !== "pending"){
      (function(taskId){
        api("GET", "/api/tasks/detail?id=" + taskId, null, function(d){
          var logs = d.logs || [];
          if(logs.length > 0){
            taskLogsCache[taskId] = logs;
            var term = document.getElementById("task-terminal-" + taskId);
            if(term){
              term.textContent = "";
              for(var i = 0; i < logs.length; i++){
                term.appendChild(mkTerminalLine(logs[i]));
              }
              if(taskLogAutoScroll) term.scrollTop = term.scrollHeight;
            }
          } else {
            var term = document.getElementById("task-terminal-" + taskId);
            if(term){
              term.textContent = "";
              var em = div("task-terminal-empty");
              em.textContent = "No log entries for this task.";
              term.appendChild(em);
            }
          }
        });
      })(t.id);
    }
  } else {
    for(var i = 0; i < taskLogs.length; i++){
      terminal.appendChild(mkTerminalLine(taskLogs[i]));
    }
  }
  termWrap.appendChild(terminal);

  var jumpBtn = el("button", "jump-btn hidden", ["\u2193 Bottom"]);
  jumpBtn.id = "jump-btn-" + t.id;
  jumpBtn.onclick = function(){
    taskLogAutoScroll = true;
    if(scrollCb) scrollCb.checked = true;
    terminal.scrollTop = terminal.scrollHeight;
    jumpBtn.classList.add("hidden");
  };
  termWrap.appendChild(jumpBtn);
  logSec.appendChild(termWrap);
  parent.appendChild(logSec);
}

/* ── Projects View ── */
function renderProjects(root){
  var projs = D.projects || [];

  var header = div("view-header");
  header.appendChild(span("view-title", "Projects"));
  header.appendChild(span("proj-count", projs.length + " projects"));
  var initBtn = el("button","btn btn-sm btn-primary");
  initBtn.textContent = "+ New Project";
  initBtn.onclick = function(){ openModal("modal-init"); };
  header.appendChild(initBtn);
  root.appendChild(header);

  var list = div("proj-list");

  var thead = div("proj-row proj-row-header");
  thead.appendChild(span("proj-dot-h",""));
  thead.appendChild(span("proj-row-name","NAME"));
  thead.appendChild(span("proj-row-branch","BRANCH"));
  thead.appendChild(span("proj-row-commit","LAST COMMIT"));
  thead.appendChild(span("proj-row-mod","MOD"));
  thead.appendChild(span("proj-status","STATUS"));
  thead.appendChild(span("proj-row-actions",""));
  list.appendChild(thead);

  for(var i=0;i<projs.length;i++){
    (function(p, idx){
      var row = div("proj-row status-" + (p.status_icon || "inactive"));
      row.style.animationDelay = (idx * 0.02) + "s";

      row.appendChild(span("proj-dot",""));
      row.appendChild(span("proj-row-name", p.name));
      row.appendChild(span("proj-row-branch", p.branch || "—"));
      row.appendChild(span("proj-row-commit", p.last_commit || "—"));
      row.appendChild(span("proj-row-mod", p.modified > 0 ? p.modified+"" : ""));
      row.appendChild(span("proj-status " + p.status_icon, statusLabel(p.status_icon)));

      var acts = div("proj-row-actions");
      if(p.has_git){
        var pullBtn = el("button","btn btn-sm",["PULL"]);
        pullBtn.onclick = function(){ gitPull(p.path); };
        acts.appendChild(pullBtn);
      } else {
        var initBtn = el("button","btn btn-sm btn-success",["INIT"]);
        initBtn.onclick = function(){ gitInitProject(p.path, p.name); };
        acts.appendChild(initBtn);
      }
      if(p.github_repo){
        var prBtn = el("button","btn btn-sm",["PRS"]);
        prBtn.onclick = function(){ showPRs(p.github_repo); };
        acts.appendChild(prBtn);
      }
      var taskBtn = el("button","btn btn-sm btn-primary",["+"]);
      taskBtn.onclick = function(){ addTaskForProject(p.name); };
      acts.appendChild(taskBtn);
      row.appendChild(acts);

      list.appendChild(row);
    })(projs[i], i);
  }
  root.appendChild(list);
}

/* ── Logs View ── */
function renderLogs(root){
  var logs = D.log_entries || [];

  var header = div("view-header");
  header.appendChild(span("view-title", "Autopilot Logs"));
  var autoLabel = el("label");
  autoLabel.style.cssText = "font-size:11px;color:var(--text-sec);display:flex;align-items:center;gap:4px;font-family:var(--font-sans)";
  var cb = el("input");
  cb.type = "checkbox";
  cb.checked = logAutoScroll;
  cb.onchange = function(){ logAutoScroll = this.checked; };
  autoLabel.appendChild(cb);
  autoLabel.appendChild(txt(" auto-scroll"));
  header.appendChild(autoLabel);
  root.appendChild(header);

  var toolbar = div("log-toolbar");

  var levelSel = el("select", "filter-select");
  levelSel.appendChild(mkOption("", "All Levels"));
  levelSel.appendChild(mkOption("info", "Info", logFilterLevel==="info"));
  levelSel.appendChild(mkOption("success", "Success", logFilterLevel==="success"));
  levelSel.appendChild(mkOption("warn", "Warning", logFilterLevel==="warn"));
  levelSel.appendChild(mkOption("error", "Error", logFilterLevel==="error"));
  levelSel.onchange = function(){ logFilterLevel = this.value; render(); };
  toolbar.appendChild(levelSel);

  var taskSet = {};
  var projSet = {};
  for(var i=0;i<logs.length;i++){
    if(logs[i].task_id) taskSet[logs[i].task_id] = true;
    if(logs[i].project) projSet[logs[i].project] = true;
  }
  var taskSel = el("select", "filter-select");
  taskSel.appendChild(mkOption("", "All Tasks"));
  var tids = Object.keys(taskSet).sort(function(a,b){return a-b;});
  for(var i=0;i<tids.length;i++){
    taskSel.appendChild(mkOption(tids[i], "#"+tids[i], logFilterTask===tids[i]));
  }
  taskSel.onchange = function(){ logFilterTask = this.value; render(); };
  toolbar.appendChild(taskSel);

  var projSel = el("select", "filter-select");
  projSel.appendChild(mkOption("", "All Projects"));
  var pnames = Object.keys(projSet).sort();
  for(var i=0;i<pnames.length;i++){
    projSel.appendChild(mkOption(pnames[i], pnames[i], logFilterProject===pnames[i]));
  }
  projSel.onchange = function(){ logFilterProject = this.value; render(); };
  toolbar.appendChild(projSel);
  root.appendChild(toolbar);

  var filtered = logs.filter(function(l){
    if(logFilterLevel && l.level !== logFilterLevel) return false;
    if(logFilterTask && String(l.task_id) !== logFilterTask) return false;
    if(logFilterProject && l.project !== logFilterProject) return false;
    return true;
  });

  var container = div("log-container");
  container.id = "log-container";
  if(filtered.length === 0){
    container.appendChild(div("empty", ["No log entries"]));
  } else {
    for(var i=0;i<filtered.length;i++){
      var l = filtered[i];
      var entry = div("log-entry " + l.level);
      entry.appendChild(span("log-time", fmtTime(l.time)));
      var icon = span("log-icon");
      icon.appendChild(levelIconEl(l.level));
      entry.appendChild(icon);
      entry.appendChild(span("log-proj", l.project || "\u2014"));
      entry.appendChild(span("log-task", l.task_id ? "#"+l.task_id : ""));
      entry.appendChild(span("log-msg", l.message));
      container.appendChild(entry);
    }
  }
  root.appendChild(container);
}

/* ── Actions ── */
function selectTask(id){
  if(id !== selectedTaskID){
    taskLogAutoScroll = true;
    // Don't clear cache — keep historical logs for previously viewed tasks
  }
  selectedTaskID = id;
  isDataUpdate = false;
  render();
}

function taskAutopilot(id){
  api("POST","/api/tasks/autopilot",{id:id}, function(){});
}
function taskDone(id){
  api("POST","/api/tasks/done",{id:id}, function(){ selectedTaskID=0; });
}
function taskArchive(id){
  api("POST","/api/tasks/archive",{id:id}, function(){ selectedTaskID=0; });
}
function taskReplan(id){
  delete planCache[id];
  api("POST","/api/tasks/replan",{id:id}, function(){});
}
function taskStop(id){
  api("POST","/api/tasks/stop",{id:id}, function(){});
}

function loadPlan(id){
  if(planCache[id]){
    var el = document.getElementById("plan-content-"+id);
    if(el){
      el.className = "plan-content";
      el.textContent = planCache[id];
    }
    return;
  }
  api("GET","/api/tasks/plan?id="+id, null, function(d){
    var content = d.content || d.error || "No plan content";
    planCache[id] = content;
    var el = document.getElementById("plan-content-"+id);
    if(el){
      el.className = "plan-content";
      el.textContent = content;
    }
  });
}

function gitPull(path){
  api("POST","/api/projects/pull",{path:path}, function(d){
    if(d.error) alert("Pull failed: "+d.error);
  });
}

function gitInitProject(path, name){
  api("POST","/api/projects/git-init",{path:path, name:name}, function(d){
    if(d.error) alert("Git init failed: "+d.error);
  });
}

function showPRs(repo){
  currentPRsRepo = repo;
  document.getElementById("prs-title").textContent = "PRs \u2014 " + repo;
  var content = document.getElementById("prs-content");
  content.textContent = "Loading...";
  content.className = "loading-text";
  document.getElementById("btn-merge-dep").style.display = "none";
  openModal("modal-prs");
  api("GET","/api/projects/prs?repo="+encodeURIComponent(repo), null, function(d){
    content.className = "";
    content.textContent = "";
    if(d.error){
      content.textContent = "Error: " + d.error;
      return;
    }
    var prs = d.prs || [];
    var dep = d.dependabot || [];
    if(prs.length === 0){
      content.appendChild(div("empty", ["No open PRs"]));
    } else {
      for(var i=0;i<prs.length;i++){
        var p = prs[i];
        var item = div("pr-item");
        item.appendChild(span("pr-num", "#"+p.number));
        item.appendChild(span("pr-title", p.title));
        item.appendChild(span("pr-author", (p.author && p.author.login) || ""));
        var isDep = false;
        for(var j=0;j<dep.length;j++) if(dep[j].number===p.number) isDep=true;
        if(isDep) item.appendChild(span("pr-dep", "bot"));
        content.appendChild(item);
      }
    }
    if(dep.length > 0) document.getElementById("btn-merge-dep").style.display = "";
  });
}

function mergeDependabot(){
  api("POST","/api/projects/merge-dependabot",{repo:currentPRsRepo}, function(d){
    if(d.error) alert("Error: "+d.error);
    else alert("Merged: "+d.merged+", Failed: "+(d.failed||0));
    closeModal("modal-prs");
  });
}

function loadTemplates(){
  api("GET","/api/templates/list",null,function(d){
    templatesCache = d.templates || [];
    renderTemplateSelect();
  });
}

function renderTemplateSelect(){
  var sel = document.getElementById("tmpl-select");
  if(!sel) return;
  sel.textContent = "";
  sel.appendChild(mkOption("","— select template —"));
  for(var i=0;i<templatesCache.length;i++){
    sel.appendChild(mkOption(String(templatesCache[i].id), templatesCache[i].name));
  }
  sel.value = "";
  var delBtn = document.getElementById("tmpl-del-btn");
  if(delBtn) delBtn.style.display = "none";
}

function onTemplateSelect(){
  var sel = document.getElementById("tmpl-select");
  var id = parseInt(sel.value,10);
  if(!id) {
    document.getElementById("tmpl-del-btn").style.display = "none";
    return;
  }
  for(var i=0;i<templatesCache.length;i++){
    if(templatesCache[i].id === id){
      insertTemplate(templatesCache[i].content);
      break;
    }
  }
  document.getElementById("tmpl-del-btn").style.display = "";
}

function insertTemplate(content){
  var ta = document.getElementById("add-desc");
  if(!ta) return;
  var cur = ta.value;
  if(cur && cur[cur.length-1] !== "\n"){
    ta.value = cur + "\n" + content;
  } else {
    ta.value = cur + content;
  }
  ta.focus();
  ta.scrollTop = ta.scrollHeight;
}

function saveTemplate(){
  var nameEl = document.getElementById("tmpl-name");
  var name = nameEl.value.trim();
  var content = document.getElementById("add-desc").value.trim();
  if(!name){ nameEl.focus(); return; }
  if(!content){ document.getElementById("add-desc").focus(); return; }
  api("POST","/api/templates/add",{name:name,content:content},function(d){
    if(d.error) return;
    nameEl.value = "";
    loadTemplates();
  });
}

function deleteSelectedTemplate(){
  var sel = document.getElementById("tmpl-select");
  var id = parseInt(sel.value,10);
  if(!id) return;
  api("POST","/api/templates/delete",{id:id},function(d){
    if(d.error) return;
    loadTemplates();
  });
}

function openAddTask(proj){
  var sel = document.getElementById("add-project");
  sel.textContent = "";
  sel.appendChild(mkOption("", "\u2014 none \u2014"));
  var projs = (D && D.projects) || [];
  for(var i=0;i<projs.length;i++){
    sel.appendChild(mkOption(projs[i].name, projs[i].name, projs[i].name===proj));
  }
  document.getElementById("add-desc").value = "";
  document.getElementById("add-priority").value = "med";
  document.getElementById("tmpl-name").value = "";
  document.getElementById("tmpl-select").onchange = onTemplateSelect;
  loadTemplates();
  openModal("modal-add");
  setTimeout(function(){ document.getElementById("add-desc").focus(); }, 100);
}
function addTaskForProject(proj){ openAddTask(proj); }

var FRONTEND_PROJECTS = ["anisakys-frontend","cloud-adm-frontend"];

function taskSuffix(proj){
  var parts = [];
  var isFront = FRONTEND_PROJECTS.indexOf(proj) !== -1;
  if(isFront){
    parts.push("al terminar: npx vite build (nginx toma dist automaticamente, NO usar npm run dev). usar chrome-devtools MCP para probar en el sitio correspondiente (no hay login, acceso libre)");
  }
  parts.push("usar /bmad:core:workflows:party-mode");
  return parts.join(". ");
}

function submitAddTask(){
  var proj = document.getElementById("add-project").value;
  var desc = document.getElementById("add-desc").value.trim();
  var pri = document.getElementById("add-priority").value;
  if(!desc){ document.getElementById("add-desc").focus(); return; }
  var suffix = taskSuffix(proj);
  if(suffix && desc.indexOf("party-mode") === -1){
    desc = desc + ". " + suffix;
  }
  api("POST","/api/tasks/add",{project:proj, description:desc, priority:pri}, function(d){
    closeModal("modal-add");
    if(d.id) selectedTaskID = d.id;
  });
}

/* ── Modals ── */
function openModal(id){ document.getElementById(id).classList.add("show"); }
function closeModal(id){ document.getElementById(id).classList.remove("show"); }

/* ── DOM helpers ── */
function mkLabel(text, iconStr){
  var d = div("card-label");
  if(iconStr){ d.appendChild(svgIcon(iconStr)); }
  d.appendChild(txt(text));
  return d;
}
function mkValue(text){
  var d = div("card-value");
  d.textContent = text;
  return d;
}
function mkSub(text){
  var d = div("card-sub");
  d.textContent = text;
  return d;
}
function mkMetricRow(items){
  var row = div("metric-row");
  for(var i=0;i<items.length;i++){
    row.appendChild(span("", items[i]));
  }
  return row;
}
function mkFeedTitle(text){
  var d = div("feed-title");
  d.textContent = text;
  return d;
}
function mkTerminalLine(l){
  var line = div("term-line term-line-" + (l.level || "info"));
  line.appendChild(span("term-ts", "[" + fmtTime(l.time) + "]"));
  line.appendChild(span("term-msg term-msg-" + (l.level || "info"), l.message));
  return line;
}
function mkProjDetail(label, value){
  var d = div("proj-detail");
  d.appendChild(txt(label));
  d.appendChild(span("", value));
  return d;
}
function mkOption(value, text, selected){
  var o = document.createElement("option");
  o.value = value;
  o.textContent = text;
  if(selected) o.selected = true;
  return o;
}

/* ── Helpers ── */
function findTask(id){
  if(!id || !D || !D.tasks) return null;
  for(var i=0;i<D.tasks.length;i++) if(D.tasks[i].id === id) return D.tasks[i];
  return null;
}

function fmtNum(n){
  if(n >= 1e9) return (n/1e9).toFixed(1)+"B";
  if(n >= 1e6) return (n/1e6).toFixed(1)+"M";
  if(n >= 1e3) return (n/1e3).toFixed(1)+"K";
  return String(n);
}
function fmtCost(n){ return n.toFixed(2); }

function fmtTime(ts){
  if(!ts) return "";
  var d = new Date(ts);
  var h = String(d.getHours()).padStart(2,"0");
  var m = String(d.getMinutes()).padStart(2,"0");
  var s = String(d.getSeconds()).padStart(2,"0");
  return h+":"+m+":"+s;
}
function fmtDate(ts){
  if(!ts) return "\u2014";
  var d = new Date(ts);
  return d.toLocaleDateString()+" "+fmtTime(ts);
}

function fmtRelDate(ts){
  if(!ts) return "";
  var d = new Date(ts);
  var diffMs = Date.now() - d.getTime();
  var diffH = Math.floor(diffMs / 3600000);
  if(diffH < 1) return "just now";
  if(diffH < 24) return diffH + "h ago";
  var diffD = Math.floor(diffH / 24);
  if(diffD < 7) return diffD + "d ago";
  if(diffD < 30) return Math.floor(diffD / 7) + "w ago";
  return d.toLocaleDateString();
}

function canvasDragAction(fromCol, toCol, taskId){
  if(fromCol === toCol) return;
  if(toCol === "done"){
    api("POST","/api/tasks/done",{id:taskId}, function(){});
  } else if(toCol === "inprogress" || toCol === "ready"){
    api("POST","/api/tasks/autopilot",{id:taskId}, function(){});
  }
}

function levelIconEl(lvl){
  var s = document.createElement("span");
  switch(lvl){
    case "success": s.style.color = "var(--green)"; s.textContent = "\u2713"; break;
    case "warn": s.style.color = "var(--orange)"; s.textContent = "\u26A0"; break;
    case "error": s.style.color = "var(--red)"; s.textContent = "\u2717"; break;
    default: s.style.color = "var(--text-sec)"; s.textContent = "\u2139"; break;
  }
  return s;
}
function stateLabel(s){
  switch(s){
    case "generating": return "GEN";
    case "pending": return "OFF";
    case "planned": return "PLN";
    case "running": return "RUN";
    case "blocked": return "BLK";
    case "done": return "DONE";
    default: return s ? s.toUpperCase().substring(0,4) : "\u2014";
  }
}
function statusLabel(s){
  switch(s){
    case "active": return "active";
    case "stale": return "stale";
    case "no_git": return "no git";
    default: return "inactive";
  }
}
function ucfirst(s){ return s.charAt(0).toUpperCase()+s.slice(1); }

/* ── Chat View ── */
function renderChat(container){
  var wrap = el("div","chat-container");

  // Top bar
  var topBar = el("div","chat-top-bar");
  var projSel = el("select","filter-select");
  var optAll = document.createElement("option");
  optAll.value = ""; optAll.textContent = "All Projects";
  projSel.appendChild(optAll);
  if(D && D.projects){
    for(var i=0;i<D.projects.length;i++){
      var o = document.createElement("option");
      o.value = D.projects[i].name;
      o.textContent = D.projects[i].name;
      if(chatProject === D.projects[i].name) o.selected = true;
      projSel.appendChild(o);
    }
  }
  projSel.onchange = function(){ chatProject = this.value; };
  topBar.appendChild(projSel);

  var clearBtn = el("button","btn btn-sm btn-danger");
  clearBtn.textContent = "Clear";
  clearBtn.onclick = function(){
    api("POST","/api/chat/clear",{},function(){
      chatMessages = [];
      chatCounter++;
      render();
    });
  };
  topBar.appendChild(clearBtn);
  wrap.appendChild(topBar);

  // Messages area
  var msgArea = el("div","chat-messages");
  msgArea.id = "chat-messages";
  if(chatMessages.length === 0){
    var hint = el("div","chat-hint");
    hint.textContent = "Start a conversation. Select a project and type a message.";
    msgArea.appendChild(hint);
  } else {
    for(var i=0;i<chatMessages.length;i++){
      var m = chatMessages[i];
      var bubble = el("div","chat-bubble " + (m.role === "user" ? "user" : "assistant"));
      bubble.textContent = m.content;
      msgArea.appendChild(bubble);
    }
  }
  wrap.appendChild(msgArea);

  // Input bar
  var inputBar = el("div","chat-input-bar");
  var textarea = el("textarea","chat-input");
  textarea.id = "chat-textarea";
  textarea.rows = 2;
  textarea.placeholder = "Type a message...";
  textarea.onkeydown = function(e){
    if(e.key === "Enter" && !e.shiftKey){
      e.preventDefault();
      sendChatMessage();
    }
  };
  inputBar.appendChild(textarea);

  var sendBtn = el("button","btn btn-primary chat-send-btn");
  sendBtn.id = "chat-send-btn";
  sendBtn.textContent = "Send";
  sendBtn.disabled = chatLoading;
  sendBtn.onclick = sendChatMessage;
  inputBar.appendChild(sendBtn);
  wrap.appendChild(inputBar);

  container.appendChild(wrap);

  // Load history on first render
  if(chatMessages.length === 0 && !chatLoading){
    api("GET","/api/chat/history",null,function(d){
      if(d && d.messages && d.messages.length > 0){
        chatMessages = d.messages;
        chatCounter++;
        render();
      }
    });
  }
}

function sendChatMessage(){
  var ta = document.getElementById("chat-textarea");
  if(!ta) return;
  var msg = ta.value.trim();
  if(!msg || chatLoading) return;

  chatLoading = true;
  chatMessages.push({role:"user", content:msg, project:chatProject});
  chatMessages.push({role:"assistant", content:"", project:chatProject});
  chatCounter++;
  render();
  ta.value = "";

  fetch("/api/chat/send",{
    method:"POST",
    headers:{"Content-Type":"application/json"},
    body:JSON.stringify({message:msg, project:chatProject})
  }).then(function(res){
    var reader = res.body.getReader();
    var decoder = new TextDecoder();
    var buffer = "";
    function pump(){
      reader.read().then(function(result){
        if(result.done){
          chatLoading = false;
          chatCounter++;
          render();
          return;
        }
        buffer += decoder.decode(result.value, {stream:true});
        var lines = buffer.split("\n");
        buffer = lines.pop();
        for(var i=0;i<lines.length;i++){
          var line = lines[i].trim();
          if(line.indexOf("data: ") === 0){
            try{
              var evt = JSON.parse(line.substring(6));
              if(evt.token){
                chatMessages[chatMessages.length-1].content += evt.token;
                var msgDiv = document.getElementById("chat-messages");
                if(msgDiv){
                  var bubbles = msgDiv.querySelectorAll(".chat-bubble.assistant");
                  if(bubbles.length > 0){
                    bubbles[bubbles.length-1].textContent = chatMessages[chatMessages.length-1].content;
                    msgDiv.scrollTop = msgDiv.scrollHeight;
                  }
                }
              }
              if(evt.tasks_created){
                chatCreatedTasks = evt.tasks_created;
              }
              if(evt.done){
                if(evt.result) chatMessages[chatMessages.length-1].content = evt.result;
                // Strip directives from stored/displayed content
                var cleanContent = chatMessages[chatMessages.length-1].content
                  .replace(/\[TASK_CREATE\].*?\[\/TASK_CREATE\]/g, "").trim();
                chatMessages[chatMessages.length-1].content = cleanContent;
                chatLoading = false;
                chatCounter++;
                render();
                // Append task cards to the last assistant bubble
                if(chatCreatedTasks.length > 0){
                  var md = document.getElementById("chat-messages");
                  if(md){
                    var bbs = md.querySelectorAll(".chat-bubble.assistant");
                    if(bbs.length > 0){
                      var lastBubble = bbs[bbs.length-1];
                      var cardsDiv = document.createElement("div");
                      cardsDiv.className = "chat-task-cards";
                      for(var j=0;j<chatCreatedTasks.length;j++){
                        var ct = chatCreatedTasks[j];
                        var card = document.createElement("div");
                        card.className = "chat-task-card pri-" + (ct.priority || "med");
                        card.setAttribute("data-task-id", ct.id);
                        var descSpan = document.createElement("span");
                        descSpan.className = "chat-task-card-desc";
                        descSpan.textContent = ct.description;
                        card.appendChild(descSpan);
                        var metaSpan = document.createElement("span");
                        metaSpan.className = "chat-task-card-meta";
                        var idSpan = document.createElement("span");
                        idSpan.textContent = "#" + ct.id;
                        metaSpan.appendChild(idSpan);
                        var assSpan = document.createElement("span");
                        assSpan.className = "chat-task-card-assignee" + (ct.assignee === "agent" ? " assignee-agent" : "");
                        assSpan.textContent = ct.assignee || "human";
                        metaSpan.appendChild(assSpan);
                        card.appendChild(metaSpan);
                        card.addEventListener("click", (function(taskId){
                          return function(){
                            window.location.hash = "#queue";
                            selectedTaskId = taskId;
                            render();
                          };
                        })(ct.id));
                        cardsDiv.appendChild(card);
                      }
                      lastBubble.appendChild(cardsDiv);
                      md.scrollTop = md.scrollHeight;
                    }
                  }
                  chatCreatedTasks = [];
                }
                return;
              }
            }catch(e){}
          }
        }
        pump();
      });
    }
    pump();
  }).catch(function(){
    chatLoading = false;
    chatMessages[chatMessages.length-1].content = "[Error: connection failed]";
    chatCounter++;
    render();
  });
}

/* ── Canvas View ── */
function renderCanvas(container){
  var tasks = D ? D.tasks : [];

  // Header (consistent view-header pattern)
  var header = div("view-header");
  header.appendChild(span("view-title","Canvas"));

  var controls = div("queue-toolbar");
  controls.style.display = "flex";
  controls.style.gap = "8px";
  controls.style.alignItems = "center";

  // Assignee filter
  var assigneeSel = el("select","filter-select");
  var assigneeOpts = [["","All Assignees"],["agent","Agent"],["human","Human"],["review","Review"]];
  for(var i=0;i<assigneeOpts.length;i++){
    assigneeSel.appendChild(mkOption(assigneeOpts[i][0],assigneeOpts[i][1],canvasFilterAssignee===assigneeOpts[i][0]));
  }
  assigneeSel.onchange = function(){ canvasFilterAssignee = this.value; render(); };
  controls.appendChild(assigneeSel);

  // Project filter
  var projSet = {};
  for(var i=0;i<tasks.length;i++) if(tasks[i].project) projSet[tasks[i].project] = true;
  var projSel = el("select","filter-select");
  projSel.appendChild(mkOption("","All Projects"));
  var pnames = Object.keys(projSet).sort();
  for(var i=0;i<pnames.length;i++){
    projSel.appendChild(mkOption(pnames[i],pnames[i],canvasFilterProject===pnames[i]));
  }
  projSel.onchange = function(){ canvasFilterProject = this.value; render(); };
  controls.appendChild(projSel);

  // + Add Task button
  var addBtn = el("button","btn btn-primary btn-sm");
  addBtn.textContent = "+ Add Task";
  addBtn.onclick = function(){ openAddTask(""); };
  controls.appendChild(addBtn);

  header.appendChild(controls);
  container.appendChild(header);

  // Filter tasks
  var filtered = [];
  for(var i=0;i<tasks.length;i++){
    var t = tasks[i];
    if(canvasFilterAssignee && (t.assignee||"") !== canvasFilterAssignee) continue;
    if(canvasFilterProject && t.project !== canvasFilterProject) continue;
    filtered.push(t);
  }

  // Bucket into 4 columns
  var backlog=[], ready=[], inprogress=[], done=[];
  for(var i=0;i<filtered.length;i++){
    var s = filtered[i].effective_state;
    if(s === "pending") backlog.push(filtered[i]);
    else if(s === "planned" || s === "generating") ready.push(filtered[i]);
    else if(s === "running") inprogress.push(filtered[i]);
    else done.push(filtered[i]);
  }

  var board = el("div","canvas-board");
  board.appendChild(makeCanvasCol("Backlog","backlog",backlog));
  board.appendChild(makeCanvasCol("Ready","ready",ready));
  board.appendChild(makeCanvasCol("In Progress","inprogress",inprogress));
  board.appendChild(makeCanvasCol("Done","done",done));
  container.appendChild(board);
}

function makeCanvasCol(title, colId, tasks){
  var col = el("div","canvas-col");
  col.setAttribute("data-col",colId);

  // Header
  var hdr = el("div","canvas-col-header");
  hdr.appendChild(span("canvas-col-title",title));
  hdr.appendChild(span("canvas-col-count",String(tasks.length)));
  col.appendChild(hdr);

  // Scrollable body
  var body = el("div","canvas-col-body");
  if(tasks.length === 0){
    var empty = el("div","canvas-empty");
    empty.textContent = "No tasks";
    body.appendChild(empty);
  } else {
    for(var i=0;i<tasks.length;i++) body.appendChild(makeCanvasCard(tasks[i],colId));
  }
  col.appendChild(body);

  // Drag-and-drop: column as drop target
  col.addEventListener("dragover",function(e){
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    col.classList.add("drag-over");
  });
  col.addEventListener("dragleave",function(e){
    if(!col.contains(e.relatedTarget)) col.classList.remove("drag-over");
  });
  col.addEventListener("drop",function(e){
    e.preventDefault();
    col.classList.remove("drag-over");
    var taskId = parseInt(e.dataTransfer.getData("text/plain"),10);
    if(!taskId) return;
    canvasDragAction(canvasDragFromCol, colId, taskId);
  });

  return col;
}

function makeCanvasCard(t, colId){
  var priClass = "pri-" + (t.priority || "med");
  var isBlocked = (t.effective_state === "blocked");
  var card = el("div","canvas-card " + priClass + (isBlocked ? " is-blocked" : ""));

  // Blocked overlay
  if(isBlocked) card.appendChild(el("div","canvas-card-blocked-overlay"));

  // Draggable
  card.setAttribute("draggable","true");
  card.addEventListener("dragstart",function(e){
    canvasDragTaskId = t.id;
    canvasDragFromCol = colId;
    e.dataTransfer.setData("text/plain",String(t.id));
    e.dataTransfer.effectAllowed = "move";
    var self = card;
    setTimeout(function(){ self.classList.add("dragging"); },0);
  });
  card.addEventListener("dragend",function(){
    card.classList.remove("dragging");
    canvasDragTaskId = 0;
    canvasDragFromCol = "";
  });

  var inner = el("div","canvas-card-inner");

  // Labels row
  var labels = el("div","canvas-card-labels");
  if(t.project) labels.appendChild(span("canvas-label canvas-label-proj",t.project));
  if(t.assignee){
    var asnText = {agent:"Agent",human:"Human",review:"Review"}[t.assignee] || t.assignee;
    labels.appendChild(span("canvas-label canvas-label-"+t.assignee, asnText));
  }
  if(t.auto_pilot) labels.appendChild(span("canvas-label canvas-label-ap","AP"));
  if(labels.children.length > 0) inner.appendChild(labels);

  // Title + description
  var text = t.description || "";
  var nl = text.indexOf("\n");
  var rawTitle = nl > -1 ? text.substring(0,nl) : text;
  var titleText = rawTitle.length > 60 ? rawTitle.substring(0,60) + "\u2026" : rawTitle;
  var descText = "";
  if(nl > -1 && text.length > nl+1) descText = text.substring(nl+1).trim();
  else if(rawTitle.length > 60) descText = rawTitle.substring(60);

  inner.appendChild(el("div","canvas-card-title",[titleText]));
  if(descText) inner.appendChild(el("div","canvas-card-desc",[descText]));

  // Footer
  var footer = el("div","canvas-card-footer");
  footer.appendChild(span("canvas-card-id","#"+t.id));
  footer.appendChild(span("canvas-card-date",fmtRelDate(t.created_at)));
  if(t.has_plan){
    var planIcon = span("canvas-card-plan-icon","\u2713");
    planIcon.title = "Has plan";
    footer.appendChild(planIcon);
  }
  if(isBlocked){
    var blkIcon = span("canvas-card-blocked-icon","\u26A0");
    blkIcon.title = t.block_reason ? "Blocked: "+t.block_reason : "Blocked";
    footer.appendChild(blkIcon);
  }
  inner.appendChild(footer);
  card.appendChild(inner);

  // Click → Queue detail
  card.onclick = function(e){
    if(card.classList.contains("dragging")) return;
    selectedTaskID = t.id;
    location.hash = "queue";
    render();
  };

  return card;
}

/* ── Project Init ── */
function submitProjectInit(){
  var name = document.getElementById("init-name").value.trim();
  if(!name) return;
  var type = document.getElementById("init-type").value;
  var version = document.getElementById("init-version").value.trim() || "1.0.0";
  var vis = document.getElementById("init-visibility").value;
  var structure = document.getElementById("init-structure").value;

  var submitBtn = document.getElementById("init-submit");
  submitBtn.disabled = true;
  submitBtn.textContent = "Creating...";

  var prog = document.getElementById("init-progress");
  prog.style.display = "block";
  prog.innerHTML = "";

  fetch("/api/projects/init",{
    method:"POST",
    headers:{"Content-Type":"application/json"},
    body:JSON.stringify({name:name, type:type, version:version, private:vis==="private", separate:structure==="separate"})
  }).then(function(res){
    var reader = res.body.getReader();
    var decoder = new TextDecoder();
    var buffer = "";
    function pump(){
      reader.read().then(function(result){
        if(result.done){
          submitBtn.disabled = false;
          submitBtn.textContent = "Create Project";
          return;
        }
        buffer += decoder.decode(result.value, {stream:true});
        var lines = buffer.split("\n");
        buffer = lines.pop();
        for(var i=0;i<lines.length;i++){
          var line = lines[i].trim();
          if(line.indexOf("data: ") === 0){
            try{
              var evt = JSON.parse(line.substring(6));
              if(evt.done){
                if(evt.status === "success"){
                  var doneRow = el("div","init-progress-step");
                  doneRow.innerHTML = '<span class="init-step-icon" style="color:var(--green)">&#10003;</span><span class="init-step-name">Project created successfully!</span>';
                  prog.appendChild(doneRow);
                }
                submitBtn.disabled = false;
                submitBtn.textContent = "Create Project";
                return;
              }
              renderInitStep(prog, evt);
            }catch(e){}
          }
        }
        pump();
      });
    }
    pump();
  }).catch(function(){
    submitBtn.disabled = false;
    submitBtn.textContent = "Create Project";
  });
}

function renderInitStep(prog, evt){
  var existing = document.getElementById("init-step-" + evt.step);
  if(existing){
    var icon = existing.querySelector(".init-step-icon");
    if(evt.status === "done"){
      icon.innerHTML = '<span style="color:var(--green)">&#10003;</span>';
    } else if(evt.status === "error"){
      icon.innerHTML = '<span style="color:var(--red)">&#10007;</span>';
      var msg = el("span","init-step-error");
      msg.textContent = evt.message || "";
      existing.appendChild(msg);
    }
    return;
  }
  var row = el("div","init-progress-step");
  row.id = "init-step-" + evt.step;
  var icon = el("span","init-step-icon");
  if(evt.status === "running") icon.innerHTML = '<span style="color:var(--yellow)">&#8226;</span>';
  else if(evt.status === "done") icon.innerHTML = '<span style="color:var(--green)">&#10003;</span>';
  else icon.innerHTML = '<span style="color:var(--red)">&#10007;</span>';
  row.appendChild(icon);
  var name = el("span","init-step-name");
  name.textContent = "Step " + evt.step + ": " + (evt.name || "");
  row.appendChild(name);
  if(evt.status === "error" && evt.message){
    var msg = el("span","init-step-error");
    msg.textContent = evt.message;
    row.appendChild(msg);
  }
  prog.appendChild(row);
}

/* ── Init ── */
api("GET","/api/data", null, function(d){
  D = d;
  render();
});
connectSSE();

/* Export for inline event handlers in index.html */
window.closeModal = closeModal;
window.submitAddTask = submitAddTask;
window.saveTemplate = saveTemplate;
window.deleteSelectedTemplate = deleteSelectedTemplate;
window.mergeDependabot = mergeDependabot;
window.submitProjectInit = submitProjectInit;

})();
