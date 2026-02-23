(function(){
"use strict";

function mdToHtml(md){
  var esc = md.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;");
  var lines = esc.split("\n");
  var html = [];
  var inCode = false, inList = false;
  for(var i = 0; i < lines.length; i++){
    var l = lines[i];
    if(l.match(/^```/)){
      if(inCode){ html.push("</code></pre>"); inCode = false; }
      else { html.push("<pre><code>"); inCode = true; }
      continue;
    }
    if(inCode){ html.push(l); continue; }
    if(l.match(/^### /)) { if(inList){html.push("</ul>");inList=false;} html.push("<h4>"+l.slice(4)+"</h4>"); continue; }
    if(l.match(/^## /)) { if(inList){html.push("</ul>");inList=false;} html.push("<h3>"+l.slice(3)+"</h3>"); continue; }
    if(l.match(/^# /)) { if(inList){html.push("</ul>");inList=false;} html.push("<h2>"+l.slice(2)+"</h2>"); continue; }
    if(l.match(/^[-*] /)){
      if(!inList){html.push("<ul>");inList=true;}
      html.push("<li>"+inlineMd(l.slice(2))+"</li>");
      continue;
    }
    if(l.match(/^\d+\. /)){
      if(!inList){html.push("<ol>");inList=true;}
      html.push("<li>"+inlineMd(l.replace(/^\d+\.\s*/,""))+"</li>");
      continue;
    }
    if(inList){html.push(inList?"</ul>":"</ol>");inList=false;}
    if(l.trim()===""){html.push("<br>");continue;}
    html.push("<p>"+inlineMd(l)+"</p>");
  }
  if(inList) html.push("</ul>");
  if(inCode) html.push("</code></pre>");
  return html.join("\n");
}
function inlineMd(s){
  return s
    .replace(/`([^`]+)`/g,"<code>$1</code>")
    .replace(/\*\*([^*]+)\*\*/g,"<strong>$1</strong>")
    .replace(/\*([^*]+)\*/g,"<em>$1</em>");
}

function renderMarkdown(text) {
  if (typeof marked !== 'undefined' && typeof DOMPurify !== 'undefined') {
    // Pre-process: extract <details> blocks, parse their inner markdown separately,
    // then re-insert as HTML (marked.js skips markdown inside HTML blocks)
    var processed = text.replace(/<details([^>]*)>\s*<summary>([\s\S]*?)<\/summary>([\s\S]*?)<\/details>/gi, function(m, attrs, summary, body) {
      var innerHtml = marked.parse(body.trim(), { breaks: true, gfm: true });
      return '<details' + attrs + '><summary>' + summary + '</summary>\n' + innerHtml + '\n</details>';
    });
    var raw = marked.parse(processed, { breaks: true, gfm: true });
    return DOMPurify.sanitize(raw, { ADD_TAGS: ['details','summary'], ADD_ATTR: ['open'] });
  }
  return mdToHtml(text);
}

var TOOL_ICONS = {
  "WebSearch":"🔍","Read":"📄","Write":"✏️","Edit":"✏️","Bash":"⚡",
  "Glob":"🗂️","Grep":"🔎","WebFetch":"🌐","Task":"🤖"
};
function toolIcon(name){
  if(TOOL_ICONS[name]) return TOOL_ICONS[name];
  if(name.indexOf("mcp__")===0) return "🔧";
  return "⚙️";
}
function toolLabel(name){
  if(name.indexOf("mcp__")===0){
    var parts=name.split("__");
    if(parts.length>=3) return parts[1]+": "+parts[2].replace(/-/g," ");
  }
  return name.replace(/([A-Z])/g," $1").trim();
}
function chatUpdateToolActivity(bubbleEl){
  var container=bubbleEl.querySelector(".chat-tool-activity");
  if(!container){
    container=document.createElement("div");
    container.className="chat-tool-activity";
    bubbleEl.insertBefore(container,bubbleEl.firstChild);
  }
  // Remove old processing indicator
  var oldProc=container.querySelector(".tool-processing-row");
  if(oldProc) oldProc.remove();
  var rows=container.querySelectorAll(".tool-activity-row:not(.tool-processing-row)");
  for(var i=0;i<chatToolCalls.length;i++){
    var call=chatToolCalls[i];
    var row=rows[i];
    if(!row){
      row=document.createElement("div");
      row.className="tool-activity-row";
      container.appendChild(row);
    }
    if(call.done){
      row.className="tool-activity-row tool-done";
      row.innerHTML='<span class="tool-act-icon">\u2713</span><span class="tool-act-label">'+toolIcon(call.name)+" "+toolLabel(call.name)+'</span>';
    } else {
      row.className="tool-activity-row tool-running";
      row.innerHTML='<span class="tool-act-spinner"><span class="spinner-sm"></span></span><span class="tool-act-label">'+toolIcon(call.name)+" "+toolLabel(call.name)+'\u2026</span>';
    }
  }
}
function chatAppendMetaFooter(bubbleEl,meta){
  if(bubbleEl.querySelector(".chat-meta-footer")) return;
  var footer=document.createElement("div");
  footer.className="chat-meta-footer";
  var parts=[];
  if(meta.num_turns>0) parts.push(meta.num_turns+" turn"+(meta.num_turns!==1?"s":""));
  if(meta.cost_usd>0) parts.push("$"+meta.cost_usd.toFixed(4));
  var secs=(meta.duration_ms/1000).toFixed(1);
  parts.push(secs+"s");
  footer.textContent=parts.join(" \u00b7 ");
  bubbleEl.appendChild(footer);
}

var D = null;
var prevDataStr = "";
var selectedTaskID = 0;
var selectedProjectName = "";
var currentPRsRepo = "";
var logAutoScroll = true;
var queueFilterState = "";
var queueFilterProject = "";
var logFilterLevel = "";
var logFilterTask = "";
var logFilterProject = "";
var logFilterAgent = "";
var prevView = "";
var isDataUpdate = false;
var taskLogAutoScroll = true;
var templatesCache = [];
var editingTemplateId = 0;
var planCollapsed = true;
var taskLogsCache = {};
var planCache = {};
var prevContentKey = "";
var prevLogCount = 0;
var prevTaskLogCounts = {};
var prevTaskStates = {};
var chatMessages = [];
var chatLoading = false;
var chatInitSteps = [];
var configLoaded = 0;
var configData = null;
var configTab = "general";
var configSubTab = "paths";
var configEditing = null;
var cfgEditingTemplate = null;
var templatesLoading = false;
var chatProject = "";
var chatCounter = 0;
var chatCreatedTasks = [];
var chatToolCalls = [];
var chatTurnStartMs = 0;
var canvasFilterAssignee = "";
var canvasFilterProject = "";
var canvasDragTaskId = 0;
var canvasDragFromCol = "";
var loadingActions = {}; // track in-progress actions: key -> true
var mcpData = null; // cached MCP list response
var mcpCatalogOpen = false;
var mcpCatalogResults = null;
var mcpCatalogSearch = "";
var marketplaceSubTab = "mcp";
var skillsData = null;
var skillsCatalogOpen = false;
var skillsCatalogResults = null;
var skillsCatalogSearch = "";
var projectAutopilots = [];
var updateCheckResult = null;
var updateRunning = false;
var configSetupSubTab = "prereqs";
var setupStep = 1;
var setupStepDone = {};
var setupStepError = {};
var setupRunning = false;
var setupStatusFetching = false;
var setupStatusLoaded = false;

/* ── BMAD Agent Map ── */
var agentMap = {
  "analyst":                {name:"Mary",     icon:"\uD83D\uDCCA", color:"#bc8cff"},
  "pm":                     {name:"John",     icon:"\uD83D\uDCCB", color:"#79c0ff"},
  "architect":              {name:"Winston",  icon:"\uD83C\uDFD7\uFE0F", color:"#d29922"},
  "ux-designer":            {name:"Sally",    icon:"\uD83C\uDFA8", color:"#f778ba"},
  "dev":                    {name:"Amelia",   icon:"\uD83D\uDCBB", color:"#58a6ff"},
  "tea":                    {name:"Murat",    icon:"\uD83E\uDDEA", color:"#3fb950"},
  "sm":                     {name:"Bob",      icon:"\uD83C\uDFC3", color:"#e3b341"},
  "tech-writer":            {name:"Paige",    icon:"\uD83D\uDCDA", color:"#39d353"},
  "cloud":                  {name:"Warren",   icon:"\u2601\uFE0F", color:"#79c0ff"},
  "quick-flow-solo-dev":    {name:"Barry",    icon:"\uD83D\uDE80", color:"#f85149"},
  "brainstorming-coach":    {name:"Carson",   icon:"\uD83E\uDDE0", color:"#d29922"},
  "creative-problem-solver":{name:"Dr. Quinn",icon:"\uD83D\uDD2C", color:"#bc8cff"},
  "design-thinking-coach":  {name:"Maya",     icon:"\uD83C\uDFA8", color:"#f778ba"},
  "innovation-strategist":  {name:"Victor",   icon:"\u26A1",       color:"#e3b341"}
};

function agentBadge(agentId){
  var a = agentMap[agentId];
  if(!a) return null;
  var badge = span("agent-badge agent-badge-" + agentId, a.icon + " " + a.name);
  badge.style.color = a.color;
  return badge;
}

function getLastAgentForTask(taskId){
  if(!D || !D.log_entries) return "";
  var entries = D.log_entries;
  for(var i=entries.length-1;i>=0;i--){
    if(entries[i].task_id === taskId && entries[i].agent) return entries[i].agent;
  }
  return "";
}

/* ── Toast Notification System ── */
(function initToastContainer(){
  var c = document.createElement("div");
  c.className = "toast-container";
  c.id = "toast-container";
  document.body.appendChild(c);
})();

function toast(message, type){
  type = type || "info";
  var container = document.getElementById("toast-container");
  if(!container) return;
  var t = document.createElement("div");
  t.className = "toast toast-" + type;
  var icon = document.createElement("span");
  icon.className = "toast-icon";
  if(type === "success") icon.textContent = "\u2713";
  else if(type === "error") icon.textContent = "\u2717";
  else icon.textContent = "\u2139";
  t.appendChild(icon);
  var msg = document.createElement("span");
  msg.className = "toast-msg";
  msg.textContent = message;
  t.appendChild(msg);
  container.appendChild(t);
  setTimeout(function(){
    t.classList.add("removing");
    setTimeout(function(){ if(t.parentNode) t.parentNode.removeChild(t); }, 300);
  }, 4000);
}

/* ── Button Loading Helper ── */
function btnLoading(btn, loadingText){
  if(!btn || btn.disabled) return null;
  btn.disabled = true;
  var origText = btn.textContent;
  btn.textContent = "";
  var spinner = document.createElement("span");
  spinner.className = "btn-spinner";
  btn.appendChild(spinner);
  if(loadingText){
    btn.appendChild(document.createTextNode(" " + loadingText));
  }
  return function(){
    btn.disabled = false;
    btn.textContent = origText;
  };
}

/* ── Icon Button Helper ── */
var ICON_SVGS = {};
(function(){
  var t = document.createElement("template");
  t.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>';
  ICON_SVGS.pencil = t.content.firstChild;
  t = document.createElement("template");
  t.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>';
  ICON_SVGS.trash = t.content.firstChild;
  t = document.createElement("template");
  t.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>';
  ICON_SVGS.plus = t.content.firstChild;
})();
function iconBtn(type, title, onclick){
  var b = document.createElement("button");
  b.className = "icon-btn" + (type === "trash" ? " icon-btn-danger" : type === "plus" ? " icon-btn-primary" : "");
  b.title = title;
  b.setAttribute("aria-label", title);
  var src = ICON_SVGS[type] || ICON_SVGS.pencil;
  b.appendChild(src.cloneNode(true));
  b.onclick = onclick;
  return b;
}

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
var pollTimer = null;
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
      scheduleActivePoll();
    }catch(e){}
  };
  es.onerror = function(){
    es.close();
    setTimeout(connectSSE, 3000);
  };
}

/* ── Active poll: 2s polling when tasks are generating/running ── */
function hasActiveTask(){
  if(!D) return false;
  if(D.project_autopilots && D.project_autopilots.length > 0) return true;
  if(!D.tasks) return false;
  for(var i=0;i<D.tasks.length;i++){
    var s = D.tasks[i].effective_state;
    if(s === "generating" || s === "running") return true;
  }
  return false;
}
function scheduleActivePoll(){
  if(pollTimer){ clearTimeout(pollTimer); pollTimer = null; }
  if(!hasActiveTask()) return;
  pollTimer = setTimeout(function(){
    pollTimer = null;
    api("GET","/api/data",null,function(data){
      var cmp = JSON.stringify((function(o){
        var c = {}; for(var k in o) if(k !== "timestamp") c[k] = o[k]; return c;
      })(data));
      if(cmp === prevDataStr) { scheduleActivePoll(); return; }
      prevDataStr = cmp;
      D = data;
      isDataUpdate = true;
      render();
      scheduleActivePoll();
    });
  }, 2000);
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

/* ── MCP Catalog helpers ── */
function searchCatalog(query){
  mcpCatalogResults = "loading";
  render();
  var url = "/api/mcp/catalog?limit=20";
  if(query) url += "&search=" + encodeURIComponent(query);
  api("GET", url, null, function(d){
    mcpCatalogResults = d.servers || [];
    render();
  });
}

function doInstall(name, pkg, envVars, btn){
  var restore = btnLoading(btn, "INSTALLING\u2026");
  api("POST", "/api/mcp/install", {name: name, package: pkg, env_vars: envVars}, function(d){
    if(restore) restore();
    if(d.ok){
      mcpData = null;
      mcpCatalogResults = null;
      mcpCatalogOpen = false;
      mcpCatalogSearch = "";
      toast("Installed " + name, "success");
      render();
    } else {
      toast("Error: " + (d.error || "unknown"), "error");
    }
  });
}

function showEnvVarsPrompt(srv, installBtn){
  var parent = installBtn.parentElement.parentElement;
  var existing = parent.querySelector(".mcp-env-form");
  if(existing){ existing.remove(); return; }

  var form = div("mcp-env-form");
  form.className = "mcp-env-form";
  form.style.width = "100%";
  form.style.marginTop = "8px";
  form.style.display = "flex";
  form.style.flexDirection = "column";
  form.style.gap = "6px";

  var inputs = {};
  srv.env_vars.forEach(function(v){
    var row = div("");
    row.style.display = "flex";
    row.style.gap = "6px";
    row.style.alignItems = "center";
    var label = span("", v.name + (v.is_required ? " *" : ""));
    label.style.fontSize = "11px";
    label.style.minWidth = "120px";
    label.style.fontFamily = "monospace";
    row.appendChild(label);
    var input = el("input","");
    input.type = v.is_secret ? "password" : "text";
    input.placeholder = v.name;
    input.style.flex = "1";
    input.style.padding = "4px 8px";
    input.style.background = "rgba(0,0,0,.4)";
    input.style.border = "1px solid var(--glass)";
    input.style.borderRadius = "4px";
    input.style.color = "var(--text)";
    input.style.fontSize = "12px";
    row.appendChild(input);
    inputs[v.name] = input;
    form.appendChild(row);
  });

  var actRow = div("");
  actRow.style.display = "flex";
  actRow.style.gap = "6px";
  actRow.style.marginTop = "4px";
  var confirmBtn = el("button","btn btn-success btn-sm",["Confirm Install"]);
  confirmBtn.onclick = function(){
    var envVars = {};
    var missing = [];
    srv.env_vars.forEach(function(v){
      var val = inputs[v.name].value.trim();
      if(val) envVars[v.name] = val;
      else if(v.is_required) missing.push(v.name);
    });
    if(missing.length > 0){
      toast("Missing required: " + missing.join(", "), "error");
      return;
    }
    form.remove();
    doInstall(srv.name, srv.package, envVars, installBtn);
  };
  actRow.appendChild(confirmBtn);
  var cancelEnvBtn = el("button","btn btn-sm",["Cancel"]);
  cancelEnvBtn.onclick = function(){ form.remove(); };
  actRow.appendChild(cancelEnvBtn);
  form.appendChild(actRow);
  parent.appendChild(form);
}

/* ── Router ── */
function getView(){
  var h = location.hash.replace("#","") || "dashboard";
  if(["dashboard","queue","canvas","projects","logs","chat","config","setup"].indexOf(h) < 0) h = "dashboard";
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
  if(!D && v !== "setup") return "";
  var tasks = D ? (D.tasks || []) : [];
  var logs = D ? (D.log_entries || []) : [];
  var projs = D ? (D.projects || []) : [];
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
      return "p:" + selectedProjectName + ":" + pk;
    case "logs":
      return "l:" + logFilterLevel + ":" + logFilterTask + ":" + logFilterProject + ":" + logs.length;
    case "chat":
      return "chat:" + chatCounter;
    case "canvas":
      var ck = canvasFilterAssignee + ":" + canvasFilterProject + ":";
      for(var i=0;i<tasks.length;i++) ck += tasks[i].id + tasks[i].effective_state + (tasks[i].assignee||"") + ",";
      return "cv:" + ck;
    case "config":
      return "cfg:" + configLoaded + ":" + configTab + ":" + configSubTab + ":" + configSetupSubTab + ":" + (configEditing || "") + ":" + templatesCache.length + ":" + (cfgEditingTemplate ? cfgEditingTemplate.id : "") + ":" + (mcpData ? "1" : "0") + ":" + mcpCatalogOpen + ":" + (mcpCatalogResults === "loading" ? "ld" : mcpCatalogResults ? mcpCatalogResults.length : "n") + ":" + marketplaceSubTab + ":" + (skillsData ? skillsData.length : "n") + ":" + skillsCatalogOpen + ":" + (skillsCatalogResults === "loading" ? "ld" : skillsCatalogResults ? skillsCatalogResults.length : "n") + ":" + updateRunning;
    case "setup":
      return "s:" + setupStep + ":" + JSON.stringify(setupStepDone) + ":" + JSON.stringify(setupStepError);
    default:
      return "";
  }
}

function render(){
  var v = getView();
  if(!D && v !== "setup") return;
  if(D) updateTopbar();
  var nav = document.querySelectorAll("#dock a");
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
  // ONLY use fast-path if the selected task's state has NOT changed
  if(!viewChanged && v === "queue" && isDataUpdate && selectedTaskID){
    var selTask = null;
    var tasks = (D && D.tasks) ? D.tasks : [];
    for(var i = 0; i < tasks.length; i++){
      if(tasks[i].id === selectedTaskID){ selTask = tasks[i]; break; }
    }
    var curState = selTask ? selTask.effective_state : "";
    var stateChanged = (prevTaskStates[selectedTaskID] || "") !== curState;
    if(!stateChanged){
      var allLogs = (D && D.log_entries) ? D.log_entries : [];
      var taskLogs = [];
      for(var i = 0; i < allLogs.length; i++){
        if(allLogs[i].task_id === selectedTaskID) taskLogs.push(allLogs[i]);
      }
      var prevCount = prevTaskLogCounts[selectedTaskID] || 0;
      if(taskLogs.length > prevCount){
        var term = document.querySelector(".task-terminal");
        if(term){
          var emptyEl = term.querySelector(".task-terminal-empty");
          if(emptyEl) emptyEl.remove();
          for(var i = prevCount; i < taskLogs.length; i++){
            term.appendChild(mkTerminalLine(taskLogs[i]));
          }
          prevTaskLogCounts[selectedTaskID] = taskLogs.length;
          if(taskLogAutoScroll) term.scrollTop = term.scrollHeight;
          updateTopbar();
          isDataUpdate = false;
          return;
        }
      }
    }
    // State changed — fall through to full re-render
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
        updateTopbar();
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
  var logContainerScroll = 0;
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
    case "config": renderConfig(tmp); break;
    case "setup": renderSetup(tmp); break;
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
  // Timeline scroll preserved via mainScroll

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

/* ── Topbar ── */
function updateTopbar(){
  var ver = document.getElementById("sb-version");
  var newVer = "v" + (D.version || "?");
  if(D.build_num && D.build_num !== "0") newVer += " #" + D.build_num;
  if(ver.textContent !== newVer) ver.textContent = newVer;

  var mdl = document.getElementById("sb-model");
  var parts = [];
  if(D.plan_model) parts.push(D.plan_model);
  if(D.exec_model && D.exec_model !== D.plan_model) parts.push(D.exec_model);
  if(D.effort) parts.push(D.effort);
  var newMdl = parts.join(" \u00b7 ");
  if(mdl.textContent !== newMdl) mdl.textContent = newMdl;

  var ctx = D.session || {};
  var ctxPct = ctx.context_percent || 0;
  var ctxEl = document.getElementById("topbar-ctx");
  if(ctxEl){
    var ctxText = Math.round(ctxPct) + "% ctx";
    if(ctxEl.textContent !== ctxText) ctxEl.textContent = ctxText;
    ctxEl.style.color = ctxPct >= 80 ? "var(--danger)" : ctxPct >= 60 ? "var(--warning)" : "var(--accent)";
    ctxEl.style.background = ctxPct >= 80 ? "var(--danger-soft)" : ctxPct >= 60 ? "var(--warning-soft)" : "var(--accent-soft)";
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
  var totalWeek = (w.input||0)+(w.output||0)+(w.cache_read||0)+(w.cache_create||0);
  var totalMonth = (m.input||0)+(m.output||0)+(m.cache_read||0)+(m.cache_create||0);
  var tIn = t.input||0, tOut = t.output||0, tCr = t.cache_read||0, tCw = t.cache_create||0;

  // ── Hero Card ──
  var hero = div("hero-card");
  hero.appendChild(div("hero-label", ["Tokens today"]));
  hero.appendChild(div("hero-value", [fmtNum(totalToday)]));
  var tTotal = tIn + tOut + tCr + tCw || 1;
  var flowBar = div("flow-bar");
  var segs = [
    {pct: tIn/tTotal*100, cls: "flow-in"},
    {pct: tOut/tTotal*100, cls: "flow-out"},
    {pct: tCr/tTotal*100, cls: "flow-cache-r"},
    {pct: tCw/tTotal*100, cls: "flow-cache-w"}
  ];
  for(var si=0;si<segs.length;si++){
    if(segs[si].pct > 0.5){
      var seg = div("flow-seg " + segs[si].cls);
      seg.style.width = segs[si].pct.toFixed(1) + "%";
      flowBar.appendChild(seg);
    }
  }
  hero.appendChild(flowBar);
  hero.appendChild(div("hero-sub", [fmtNum(tIn) + " in \u00b7 " + fmtNum(tOut) + " out \u00b7 " + fmtNum(tCr) + " cached"]));
  root.appendChild(hero);

  // ── Bento Grid ──
  var bento = div("bento");

  // Sessions card
  var sessCard = div("card");
  var sessLabel = div("card-label");
  var sessDot = span("label-dot"); sessDot.style.background = "var(--success)";
  sessLabel.appendChild(sessDot);
  sessLabel.appendChild(txt("Sessions"));
  sessCard.appendChild(sessLabel);
  sessCard.appendChild(mkValue(String(c.sessions_week||0)));
  sessCard.appendChild(mkSub("this week \u00b7 " + (c.sessions_today||0) + " today"));
  bento.appendChild(sessCard);

  // Cost card
  var costCard = div("card");
  var costLabel = div("card-label");
  var costDot = span("label-dot"); costDot.style.background = "var(--warning)";
  costLabel.appendChild(costDot);
  costLabel.appendChild(txt("Cost"));
  costCard.appendChild(costLabel);
  var monthCost = c.cost_month || 0;
  var todayCost = c.cost_today || 0;
  costCard.appendChild(mkValue(monthCost > 0 ? "$" + fmtCost(monthCost) : "$0.00"));
  costCard.appendChild(mkSub("this month \u00b7 $" + fmtCost(todayCost) + " today"));
  bento.appendChild(costCard);

  // Context card (tall)
  var ctxPct = ctx.context_percent || 0;
  var ctxCard = div("card bento-tall");
  var ctxLabel = div("card-label");
  var ctxDot = span("label-dot");
  ctxDot.style.background = ctxPct >= 80 ? "var(--danger)" : ctxPct >= 60 ? "var(--warning)" : "var(--success)";
  ctxLabel.appendChild(ctxDot);
  ctxLabel.appendChild(txt("Context"));
  ctxCard.appendChild(ctxLabel);
  var ctxRingWrap = div("ctx-ring-wrap");
  ctxRingWrap.appendChild(buildPhaseRing(ctxPct, 48));
  var ctxPctEl = span("ctx-ring-pct", Math.round(ctxPct) + "%");
  ctxRingWrap.appendChild(ctxPctEl);
  ctxCard.appendChild(ctxRingWrap);
  var ctxTokens = ctx.context_tokens || 0;
  var ctxLimit = ctx.context_limit || 0;
  ctxCard.appendChild(mkSub(fmtNum(ctxTokens) + " / " + fmtNum(ctxLimit) + " tokens"));
  // Week + Month tokens + cost inline
  ctxCard.appendChild(div("mt-16"));
  ctxCard.appendChild(mkMetricRow(["Week: "+fmtNum(totalWeek) + " \u00b7 $" + fmtCost(c.cost_week||0)]));
  ctxCard.appendChild(mkMetricRow(["Month: "+fmtNum(totalMonth) + " \u00b7 $" + fmtCost(c.cost_month||0)]));
  bento.appendChild(ctxCard);

  // Queue summary card (wide)
  var running=0,pendingC=0,blockedC=0,planned=0;
  for(var i=0;i<tasks.length;i++){
    var s=tasks[i].effective_state;
    if(s==="running")running++;else if(s==="pending"||s==="generating")pendingC++;
    else if(s==="blocked")blockedC++;else if(s==="planned")planned++;
  }
  var qCard = div("card bento-wide card-clickable");
  qCard.onclick = function(){ location.hash = "queue"; };
  var qLabel = div("card-label");
  var qDot = span("label-dot"); qDot.style.background = "var(--accent)";
  qLabel.appendChild(qDot);
  qLabel.appendChild(txt("Queue"));
  qLabel.appendChild(span("view-count", String(tasks.length)));
  qCard.appendChild(qLabel);

  var mkQStat = function(dotColor, label, val){
    var row = div("queue-stat");
    var dot = span("queue-stat-dot"); dot.style.background = dotColor;
    row.appendChild(dot);
    row.appendChild(span("queue-stat-label", label));
    row.appendChild(span("queue-stat-val", String(val)));
    return row;
  };
  if(running) qCard.appendChild(mkQStat("var(--success)", "Running", running));
  if(planned) qCard.appendChild(mkQStat("var(--info)", "Planned", planned));
  if(pendingC) qCard.appendChild(mkQStat("var(--text-muted)", "Pending", pendingC));
  if(blockedC) qCard.appendChild(mkQStat("var(--danger)", "Blocked", blockedC));
  if(!running && !planned && !pendingC && !blockedC){
    qCard.appendChild(div("empty", ["No active tasks"]));
  }
  bento.appendChild(qCard);

  // Activity card (tall, right side)
  var actCard = div("card");
  var actLabel = div("card-label");
  actLabel.appendChild(txt("Recent Activity"));
  actCard.appendChild(actLabel);
  var recentLogs = logs.slice(-10).reverse();
  if(recentLogs.length === 0){
    actCard.appendChild(div("empty", ["No activity yet"]));
  } else {
    var feed = div("feed");
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
        ps.style.color = "var(--text-secondary)";
        msg.appendChild(ps);
        msg.appendChild(txt(" "));
      }
      if(le.task_id) msg.appendChild(txt("#" + le.task_id + " "));
      msg.appendChild(txt(le.message));
      item.appendChild(msg);
      feed.appendChild(item);
    }
    actCard.appendChild(feed);
  }
  bento.appendChild(actCard);

  root.appendChild(bento);
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
  var titleWrap = div("view-title-wrap");
  titleWrap.appendChild(span("view-title", "Queue"));
  if(tasks.length > 0) titleWrap.appendChild(span("view-count", String(tasks.length)));
  header.appendChild(titleWrap);
  var addBtn = el("button", "btn btn-primary btn-sm", ["+ Add Task"]);
  addBtn.onclick = function(){ openAddTask(""); };
  header.appendChild(addBtn);
  root.appendChild(header);

  // Filters
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
  root.appendChild(toolbar);

  var filtered = tasks.filter(function(t){
    if(queueFilterState && t.effective_state !== queueFilterState) return false;
    if(queueFilterProject && t.project !== queueFilterProject) return false;
    return true;
  });

  if(filtered.length === 0){
    var emptyEl = div("empty");
    emptyEl.textContent = (queueFilterState || queueFilterProject)
      ? "No tasks match the current filters."
      : "No active tasks. Add one with + Add Task.";
    root.appendChild(emptyEl);
    return;
  }

  // Timeline
  var timeline = div("timeline");

  for(var i=0;i<filtered.length;i++){
    (function(t){
      var cls = "tl-node";
      if(t.id === selectedTaskID) cls += " selected";
      if(t.is_running) cls += " has-running";
      if(t.effective_state === "generating") cls += " has-generating";
      var prev = prevTaskStates[t.id];
      if(prev && prev !== "done" && t.effective_state === "done") cls += " task-just-done";
      prevTaskStates[t.id] = t.effective_state;

      var node = div(cls);
      node.appendChild(div("tl-dot"));

      // Header row
      var hdr = div("tl-header");
      hdr.onclick = function(){ selectTask(t.id === selectedTaskID ? 0 : t.id); };

      var info = div("tl-info");
      info.appendChild(div("tl-desc", [t.description]));
      var meta = t.project || "\u2014";
      if(t.created_at) meta += " \u00b7 " + fmtRelDate(t.created_at);
      info.appendChild(div("tl-meta", [meta]));
      hdr.appendChild(info);

      var badges = div("tl-badges");
      badges.appendChild(span("task-state " + t.effective_state, stateLabel(t.effective_state)));
      badges.appendChild(span("task-pri " + t.priority, (t.priority||"").toUpperCase()));
      if(t.is_running) badges.appendChild(div("running-dot"));
      hdr.appendChild(badges);
      node.appendChild(hdr);

      // Expandable detail
      var expand = div("tl-expand");
      if(t.id === selectedTaskID){
        renderTaskDetail(expand, t);
      }
      node.appendChild(expand);

      timeline.appendChild(node);
    })(filtered[i]);
  }
  root.appendChild(timeline);
}

function renderTaskDetail(parent, t){
  // ── Header Card ──
  var headerCard = div("detail-card detail-header");
  var headerTop = div("detail-header-top");
  headerTop.appendChild(span("detail-title-id", "#" + t.id));
  var editable = (t.effective_state === "pending" || t.effective_state === "planned" || t.effective_state === "blocked");
  if(editable){
    var editBtn = iconBtn("pencil", "Edit description", function(){});
    editBtn.onclick = function(){
      descSpan.style.display = "none";
      editBtn.style.display = "none";
      var editArea = el("textarea", "edit-desc-textarea");
      editArea.value = t.description;
      editArea.rows = 6;
      headerCard.appendChild(editArea);
      var editActions = div("edit-desc-actions");
      var saveBtn = el("button", "btn btn-primary btn-sm", ["Save"]);
      var cancelBtn = el("button", "btn btn-sm", ["Cancel"]);
      cancelBtn.onclick = function(){
        editArea.remove(); editActions.remove();
        descSpan.style.display = ""; editBtn.style.display = "";
      };
      saveBtn.onclick = function(){
        var newDesc = editArea.value.trim();
        if(!newDesc) return;
        var restore = btnLoading(saveBtn, "Saving\u2026");
        api("POST","/api/tasks/update",{id:t.id, description:newDesc}, function(d){
          if(restore) restore();
          if(d.error){ toast("Error: "+d.error, "error"); return; }
          toast("Description updated", "success");
        });
      };
      editActions.appendChild(cancelBtn);
      editActions.appendChild(saveBtn);
      headerCard.appendChild(editActions);
      editArea.focus();
    };
    headerTop.appendChild(editBtn);
  }
  headerCard.appendChild(headerTop);
  var descSpan = div("detail-title-desc");
  descSpan.textContent = t.description;
  descSpan.title = t.description;
  headerCard.appendChild(descSpan);

  // Properties bar
  var props = div("detail-props");
  var propProject = div("detail-prop");
  propProject.appendChild(span("detail-prop-label", "Project"));
  propProject.appendChild(span("detail-prop-value", t.project || "\u2014"));
  props.appendChild(propProject);

  var propState = div("detail-prop");
  propState.appendChild(span("detail-prop-label", "State"));
  var stateEl = span("task-state " + t.effective_state, stateLabel(t.effective_state));
  propState.appendChild(stateEl);
  props.appendChild(propState);

  var propPri = div("detail-prop");
  propPri.appendChild(span("detail-prop-label", "Priority"));
  propPri.appendChild(span("task-pri " + t.priority, (t.priority||"").toUpperCase()));
  props.appendChild(propPri);

  var propCreated = div("detail-prop");
  propCreated.appendChild(span("detail-prop-label", "Created"));
  propCreated.appendChild(span("detail-prop-value", fmtDate(t.created_at)));
  props.appendChild(propCreated);

  if(t.is_running){
    var propEngine = div("detail-prop");
    propEngine.appendChild(span("detail-prop-label", "Engine"));
    var engineVal = div("detail-engine-running");
    engineVal.appendChild(txt("Running "));
    engineVal.appendChild(div("running-dot"));
    propEngine.appendChild(engineVal);
    props.appendChild(propEngine);
  }
  headerCard.appendChild(props);
  parent.appendChild(headerCard);

  // ── Generating state ──
  if(t.effective_state === "generating"){
    var genSec = div("detail-card detail-generating");
    var genRow = div("detail-generating-inner");
    genRow.appendChild(div("spinner"));
    genRow.appendChild(span("detail-generating-label", "Generating plan\u2026"));
    genSec.appendChild(genRow);
    parent.appendChild(genSec);
  }

  // ── Actions (all buttons always visible, disabled when not applicable) ──
  var actions = div("detail-actions");
  var s = t.effective_state;
  var apKey = "autopilot:" + t.id;

  // PLAN — enabled for pending only
  var planLoading = (s === "generating") || (s === "pending" && loadingActions[apKey]);
  var planEnabled = (s === "pending") && !loadingActions[apKey];
  var planBtn = el("button", "btn" + (planEnabled ? " btn-primary" : ""));
  if(planLoading){
    planBtn.disabled = true;
    var psp = document.createElement("span"); psp.className = "btn-spinner"; planBtn.appendChild(psp);
    planBtn.appendChild(txt(" Planning\u2026"));
  } else {
    planBtn.textContent = "Plan";
    planBtn.disabled = !planEnabled;
    if(planEnabled) planBtn.onclick = function(){ taskPlanOnly(t.id, this); };
  }
  actions.appendChild(planBtn);

  // Divider: PLAN | execution group
  actions.appendChild(div("detail-actions-divider"));

  // RUN — enabled for planned, blocked
  var runEnabled = (s === "planned" || s === "blocked") && !loadingActions[apKey];
  var runBtn = el("button", "btn" + (runEnabled ? " btn-success" : ""));
  if(loadingActions[apKey] && (s === "planned" || s === "blocked")){
    runBtn.disabled = true;
    var rsp = document.createElement("span"); rsp.className = "btn-spinner"; runBtn.appendChild(rsp);
  } else {
    runBtn.textContent = s === "blocked" ? "Resume" : "Run";
    runBtn.disabled = !runEnabled;
    if(runEnabled) runBtn.onclick = function(){ taskAutopilot(t.id, this); };
  }
  actions.appendChild(runBtn);

  // STOP — enabled for running only
  var stopEnabled = (s === "running") && !loadingActions[apKey];
  var stopBtn = el("button", "btn" + (stopEnabled ? " btn-danger" : ""));
  if(loadingActions[apKey] && s === "running"){
    stopBtn.disabled = true;
    var ssp = document.createElement("span"); ssp.className = "btn-spinner"; stopBtn.appendChild(ssp);
  } else {
    stopBtn.textContent = "Stop";
    stopBtn.disabled = !stopEnabled;
    if(stopEnabled) stopBtn.onclick = function(){ taskStop(t.id); };
  }
  actions.appendChild(stopBtn);

  // REPLAN — enabled when has_plan and not running/generating
  var rpKey = "replan:" + t.id;
  var replanEnabled = t.has_plan && s !== "running" && s !== "generating" && !loadingActions[rpKey];
  var replanBtn = el("button", "btn");
  if(loadingActions[rpKey]){
    replanBtn.disabled = true;
    var rpsp = document.createElement("span"); rpsp.className = "btn-spinner"; replanBtn.appendChild(rpsp);
  } else {
    replanBtn.textContent = "Replan";
    replanBtn.disabled = !replanEnabled;
    if(replanEnabled) replanBtn.onclick = function(){ taskReplan(t.id, this); };
  }
  actions.appendChild(replanBtn);

  // Divider before destructive action
  actions.appendChild(div("detail-actions-divider"));

  // ARCHIVE — always enabled
  var archKey = "archive:" + t.id;
  if(loadingActions[archKey]){
    var archBtn = el("button", "btn btn-danger");
    archBtn.disabled = true;
    var asp = document.createElement("span"); asp.className = "btn-spinner"; archBtn.appendChild(asp);
  } else {
    var archBtn = el("button", "btn btn-danger", ["Archive"]);
    archBtn.onclick = function(){ if(!confirm("Archive task #" + t.id + "? This cannot be undone.")) return; taskArchive(t.id, this); };
  }
  actions.appendChild(archBtn);

  parent.appendChild(actions);

  // ── Plan section (collapsible) ──
  if(t.has_plan){
    var planSec = div("detail-card detail-section");
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
      planEl.className = "plan-content plan-md";
      planEl.innerHTML = mdToHtml(planCache[t.id]);
    } else {
      planEl.textContent = "Loading\u2026";
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
  var rawTaskLogs = sseLogs.length > 0 ? sseLogs : (taskLogsCache[t.id] || []);
  // Deduplicate consecutive entries with identical messages
  var taskLogs = [];
  for(var di = 0; di < rawTaskLogs.length; di++){
    if(di > 0 && rawTaskLogs[di].message === rawTaskLogs[di-1].message) continue;
    taskLogs.push(rawTaskLogs[di]);
  }

  if(taskLogs.length === 0 && sseLogs.length === 0){
    // No SSE entries and no cache — try HTTP fetch for historical logs
    var emptyMsg = div("task-terminal-empty");
    if(t.effective_state === "pending"){
      emptyMsg.textContent = "No logs yet. Generate a plan to get started.";
    } else if(t.effective_state === "generating"){
      emptyMsg.textContent = "Plan generation in progress\u2026";
    } else {
      emptyMsg.textContent = "Loading logs\u2026";
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
  if(selectedProjectName) return renderProjectDetail(root);
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
  thead.appendChild(span("proj-row-mod","TASKS"));
  thead.appendChild(span("proj-status","STATUS"));
  thead.appendChild(span("proj-row-actions",""));
  list.appendChild(thead);

  for(var i=0;i<projs.length;i++){
    (function(p, idx){
      var rowCls = "proj-row status-" + (p.status_icon || "inactive");
      if(p.autopilot_running) rowCls += " autopilot-active";
      var row = div(rowCls);
      row.style.animationDelay = (idx * 0.02) + "s";

      row.appendChild(span("proj-dot",""));
      var nameCell = span("proj-row-name proj-name-link", p.name);
      if(p.autopilot_running){
        var autoBadge = span("autopilot-badge","AUTO");
        nameCell.appendChild(autoBadge);
      }
      row.appendChild(nameCell);
      row.appendChild(span("proj-row-branch", p.branch || "\u2014"));
      var tasksCell = span("proj-row-mod","");
      if(p.task_total > 0){
        tasksCell.textContent = p.task_done + "/" + p.task_total;
        if(p.task_running > 0) tasksCell.textContent += " (" + p.task_running + " run)";
      }
      row.appendChild(tasksCell);
      row.appendChild(span("proj-status " + p.status_icon, statusLabel(p.status_icon)));

      var acts = div("proj-row-actions");
      // Project autopilot button
      if(p.autopilot_running){
        var stopAutoBtn = el("button","btn btn-sm btn-danger",["STOP"]);
        stopAutoBtn.onclick = function(e){
          e.stopPropagation();
          var restore = btnLoading(stopAutoBtn, "...");
          api("POST","/api/projects/autopilot/stop",{project:p.name},function(){
            if(restore) restore();
            scheduleActivePoll();
          });
        };
        acts.appendChild(stopAutoBtn);
      } else {
        var autoBtn = el("button","btn btn-sm btn-auto-off",["AUTO"]);
        autoBtn.title = "Start project autopilot";
        autoBtn.onclick = function(e){
          e.stopPropagation();
          var restore = btnLoading(autoBtn, "...");
          api("POST","/api/projects/autopilot/start",{project:p.name},function(resp){
            if(restore) restore();
            if(resp.error) toast(resp.error, "error");
            else toast("Autopilot started for " + p.name, "success");
            scheduleActivePoll();
          });
        };
        acts.appendChild(autoBtn);
      }
      row.appendChild(acts);
      // Entire row is clickable for detail view
      row.style.cursor = "pointer";
      row.onclick = function(){ selectedProjectName = p.name; render(); };

      list.appendChild(row);
    })(projs[i], i);
  }
  root.appendChild(list);
}

/* ── Project Detail View ── */
function renderProjectDetail(root){
  var projs = D.projects || [];
  var p = null;
  for(var i=0;i<projs.length;i++){
    if(projs[i].name === selectedProjectName){ p = projs[i]; break; }
  }
  if(!p){ selectedProjectName = ""; renderProjects(root); return; }

  // Header
  var header = div("view-header");
  var backBtn = el("button","btn btn-sm");
  backBtn.textContent = "\u2190 Back";
  backBtn.onclick = function(){ selectedProjectName = ""; render(); };
  header.appendChild(backBtn);
  header.appendChild(span("view-title", p.name));
  // Action buttons
  var acts = div("pd-actions");
  if(p.autopilot_running){
    var stopBtn = el("button","btn btn-sm btn-danger",["STOP AUTO"]);
    stopBtn.onclick = function(){
      var restore = btnLoading(stopBtn, "STOPPING\u2026");
      api("POST","/api/projects/autopilot/stop",{project:p.name},function(){ if(restore) restore(); scheduleActivePoll(); });
    };
    acts.appendChild(stopBtn);
  } else {
    var autoBtn = el("button","btn btn-sm btn-success",["AUTO"]);
    autoBtn.onclick = function(){
      var restore = btnLoading(autoBtn, "STARTING\u2026");
      api("POST","/api/projects/autopilot/start",{project:p.name},function(resp){
        if(restore) restore();
        if(resp.error) toast(resp.error,"error");
        else toast("Autopilot started","success");
        scheduleActivePoll();
      });
    };
    acts.appendChild(autoBtn);
  }
  if(p.has_git){
    var pullBtn = el("button","btn btn-sm",["PULL"]);
    pullBtn.onclick = function(){ gitPull(p.path, this); };
    acts.appendChild(pullBtn);
  }
  if(p.github_repo){
    var prBtn = el("button","btn btn-sm",["PRS"]);
    prBtn.onclick = function(){ showPRs(p.github_repo); };
    acts.appendChild(prBtn);
  }
  acts.appendChild(iconBtn("plus","Add task",function(){ addTaskForProject(p.name); }));
  header.appendChild(acts);
  root.appendChild(header);

  // Status line
  var statusLine = div("pd-status-line");
  var statusDot = span("proj-dot pd-dot status-" + (p.status_icon||"inactive"),"");
  statusLine.appendChild(statusDot);
  statusLine.appendChild(span("pd-branch","branch: " + (p.branch || "\u2014")));
  statusLine.appendChild(span("proj-status " + p.status_icon, statusLabel(p.status_icon)));
  if(p.autopilot_running) statusLine.appendChild(span("autopilot-badge","AUTO"));
  root.appendChild(statusLine);

  // Git section
  var gitSec = div("pd-section");
  gitSec.appendChild(span("pd-section-title","Git"));
  var gitGrid = div("pd-grid");
  gitGrid.appendChild(mkPdRow("Last commit", p.last_commit || "\u2014"));
  gitGrid.appendChild(mkPdRow("Modified files", p.modified > 0 ? p.modified+"" : "0"));
  if(p.github_repo) gitGrid.appendChild(mkPdRow("GitHub", p.github_repo));
  gitGrid.appendChild(mkPdRow("Path", p.path));
  gitSec.appendChild(gitGrid);
  root.appendChild(gitSec);

  // Tasks section
  var taskSec = div("pd-section");
  taskSec.appendChild(span("pd-section-title","Tasks"));
  var taskSummary = div("pd-task-summary");
  taskSummary.appendChild(span("pd-task-count", p.task_total + " total"));
  if(p.task_pending > 0) taskSummary.appendChild(span("pd-task-count pending", p.task_pending + " pending"));
  if(p.task_running > 0) taskSummary.appendChild(span("pd-task-count running", p.task_running + " running"));
  if(p.task_done > 0) taskSummary.appendChild(span("pd-task-count done", p.task_done + " done"));
  if(p.task_blocked > 0) taskSummary.appendChild(span("pd-task-count blocked", p.task_blocked + " blocked"));
  taskSec.appendChild(taskSummary);

  // Progress bar
  if(p.task_total > 0){
    var pct = Math.round((p.task_done / p.task_total) * 100);
    var pbar = div("pd-progress-bar");
    var pfill = div("pd-progress-fill");
    pfill.style.width = pct + "%";
    pbar.appendChild(pfill);
    taskSec.appendChild(pbar);
    taskSec.appendChild(span("pd-progress-label", pct + "% complete"));
  }

  // Task groups
  var allTasks = (D.tasks || []).filter(function(t){ return t.project === p.name; });
  var groups = [
    {label:"Running", state:"running", open:true},
    {label:"Generating", state:"generating", open:true},
    {label:"Pending", state:"pending", open:allTasks.length < 30},
    {label:"Planned", state:"planned", open:true},
    {label:"Done", state:"done", open:false},
    {label:"Blocked", state:"blocked", open:true}
  ];
  for(var g=0;g<groups.length;g++){
    var grp = groups[g];
    var grpTasks = allTasks.filter(function(t){ return t.effective_state === grp.state; });
    if(grpTasks.length === 0) continue;
    var details = document.createElement("details");
    details.className = "pd-task-group";
    if(grp.open) details.open = true;
    var summary = document.createElement("summary");
    summary.className = "pd-task-group-summary";
    summary.textContent = grp.label + " (" + grpTasks.length + ")";
    details.appendChild(summary);
    for(var ti=0;ti<grpTasks.length;ti++){
      var t = grpTasks[ti];
      var trow = div("pd-task-row");
      trow.appendChild(span("pd-task-id","#" + t.id));
      trow.appendChild(span("pd-task-desc", t.description.length > 80 ? t.description.substring(0,80)+"\u2026" : t.description));
      trow.appendChild(span("task-state " + t.effective_state, stateLabel(t.effective_state)));
      trow.appendChild(span("task-pri " + t.priority, (t.priority||"").toUpperCase()));
      trow.onclick = (function(tid){ return function(){ selectedTaskID = tid; window.location.hash = "#queue"; render(); }; })(t.id);
      trow.style.cursor = "pointer";
      details.appendChild(trow);
    }
    taskSec.appendChild(details);
  }
  root.appendChild(taskSec);

  // Config section
  var cfgSec = div("pd-section");
  cfgSec.appendChild(span("pd-section-title","Config"));
  var cfgGrid = div("pd-grid");
  var sk = (D.config && D.config.project_skeletons && D.config.project_skeletons[p.name]) || (D.config && D.config.skeleton) || {};
  var skEntries = ["web_search","context7_lookup","build_verify","test","pre_commit","commit","push"];
  var skLine = "";
  for(var si=0;si<skEntries.length;si++){
    var key = skEntries[si];
    var val = sk[key] !== undefined ? sk[key] : false;
    skLine += (val ? "\u2713 " : "\u2717 ") + key.replace(/_/g," ") + "   ";
  }
  cfgGrid.appendChild(mkPdRow("Skeleton", skLine.trim()));
  if(D.config && D.config.spawn){
    cfgGrid.appendChild(mkPdRow("Model", D.config.spawn.model || D.exec_model || "default"));
    cfgGrid.appendChild(mkPdRow("Max turns", (D.config.spawn.max_turns || 15) + ""));
  }
  cfgSec.appendChild(cfgGrid);
  root.appendChild(cfgSec);

  // Recent logs
  var logSec = div("pd-section");
  logSec.appendChild(span("pd-section-title","Recent Activity"));
  var logs = (D.log_entries || []).filter(function(l){ return l.project === p.name; });
  var recentLogs = logs.slice(-10).reverse();
  if(recentLogs.length === 0){
    logSec.appendChild(span("pd-empty","No recent activity"));
  } else {
    for(var li=0;li<recentLogs.length;li++){
      var le = recentLogs[li];
      var logRow = div("pd-log-row");
      var ts = le.time ? new Date(le.time).toLocaleTimeString() : "";
      logRow.appendChild(span("pd-log-time", ts));
      logRow.appendChild(span("pd-log-level log-"+le.level, le.level));
      logRow.appendChild(span("pd-log-msg", le.message.length > 100 ? le.message.substring(0,100)+"\u2026" : le.message));
      logSec.appendChild(logRow);
    }
  }
  root.appendChild(logSec);
}

function mkPdRow(label, value){
  var row = div("pd-row");
  row.appendChild(span("pd-row-label", label));
  row.appendChild(span("pd-row-value", value));
  return row;
}

/* ── Logs View ── */
function renderLogs(root){
  var logs = D.log_entries || [];

  var header = div("view-header");
  header.appendChild(span("view-title", "Autopilot Logs"));
  var autoLabel = el("label");
  autoLabel.style.cssText = "font-size:11px;color:var(--text-muted);display:flex;align-items:center;gap:4px;font-family:var(--font)";
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

  var agentSel = el("select", "filter-select");
  agentSel.appendChild(mkOption("", "All Agents"));
  var agentIds = Object.keys(agentMap).sort();
  for(var ai=0;ai<agentIds.length;ai++){
    var aKey = agentIds[ai];
    agentSel.appendChild(mkOption(aKey, agentMap[aKey].icon + " " + agentMap[aKey].name, logFilterAgent===aKey));
  }
  agentSel.onchange = function(){ logFilterAgent = this.value; render(); };
  toolbar.appendChild(agentSel);

  root.appendChild(toolbar);

  var filtered = logs.filter(function(l){
    if(logFilterLevel && l.level !== logFilterLevel) return false;
    if(logFilterTask && String(l.task_id) !== logFilterTask) return false;
    if(logFilterProject && l.project !== logFilterProject) return false;
    if(logFilterAgent && l.agent !== logFilterAgent) return false;
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
      if(l.agent && agentMap[l.agent]){
        entry.appendChild(agentBadge(l.agent));
      }
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

function taskPlanOnly(id, btn){
  var key = "autopilot:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  btnLoading(btn);
  api("POST","/api/tasks/autopilot",{id:id, run:false}, function(){ delete loadingActions[key]; scheduleActivePoll(); });
}
function taskAutopilot(id, btn){
  var key = "autopilot:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  btnLoading(btn);
  api("POST","/api/tasks/autopilot",{id:id}, function(){ delete loadingActions[key]; scheduleActivePoll(); });
}
function taskDone(id, btn){
  var key = "done:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  btnLoading(btn);
  api("POST","/api/tasks/done",{id:id}, function(){ delete loadingActions[key]; selectedTaskID=0; });
}
function taskArchive(id, btn){
  var key = "archive:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  btnLoading(btn);
  api("POST","/api/tasks/archive",{id:id}, function(){ delete loadingActions[key]; selectedTaskID=0; });
}
function taskReplan(id, btn){
  var key = "replan:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  btnLoading(btn);
  delete planCache[id];
  api("POST","/api/tasks/replan",{id:id}, function(){ delete loadingActions[key]; });
}
function taskStop(id){
  api("POST","/api/tasks/stop",{id:id}, function(){});
}

function loadPlan(id){
  if(planCache[id]){
    var el = document.getElementById("plan-content-"+id);
    if(el){
      el.className = "plan-content plan-md";
      el.innerHTML = mdToHtml(planCache[id]);
    }
    return;
  }
  api("GET","/api/tasks/plan?id="+id, null, function(d){
    var content = d.content || d.error || "No plan content";
    planCache[id] = content;
    var el = document.getElementById("plan-content-"+id);
    if(el){
      el.className = "plan-content plan-md";
      el.innerHTML = mdToHtml(content);
    }
  });
}

function gitPull(path, btn){
  var key = "pull:" + path;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  btnLoading(btn, "PULLING\u2026");
  api("POST","/api/projects/pull",{path:path}, function(d){
    delete loadingActions[key];
    if(d.error){ toast("Pull failed: "+d.error, "error"); }
    else { toast("Pull complete", "success"); }
    render();
  });
}

function gitInitProject(path, name, btn){
  var key = "init:" + path;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  btnLoading(btn, "INIT\u2026");
  api("POST","/api/projects/git-init",{path:path, name:name}, function(d){
    delete loadingActions[key];
    if(d.error){ toast("Git init failed: "+d.error, "error"); }
    else { toast("Git initialized: "+name, "success"); }
    render();
  });
}


function showPRs(repo){
  currentPRsRepo = repo;
  document.getElementById("prs-title").textContent = "PRs \u2014 " + repo;
  var content = document.getElementById("prs-content");
  content.textContent = "Loading\u2026";
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
  var mergeBtn = document.getElementById("btn-merge-dep");
  var closeBtn = document.querySelector("#modal-prs .modal-actions .btn:first-child");
  var restore = btnLoading(mergeBtn, "MERGING\u2026");
  if(closeBtn) closeBtn.disabled = true;
  api("POST","/api/projects/merge-dependabot",{repo:currentPRsRepo}, function(d){
    if(restore) restore();
    if(closeBtn) closeBtn.disabled = false;
    if(d.error){ toast("Merge error: "+d.error, "error"); }
    else { toast("Merged: "+d.merged+", Failed: "+(d.failed||0), d.failed > 0 ? "error" : "success"); }
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
    var eb = document.getElementById("tmpl-edit-btn");
    if(eb) eb.style.display = "none";
    return;
  }
  for(var i=0;i<templatesCache.length;i++){
    if(templatesCache[i].id === id){
      insertTemplate(templatesCache[i].content);
      break;
    }
  }
  document.getElementById("tmpl-del-btn").style.display = "";
  var eb = document.getElementById("tmpl-edit-btn");
  if(eb) eb.style.display = "";
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
    editingTemplateId = 0;
    updateTemplateSaveBtn();
    loadTemplates();
  });
}

function editSelectedTemplate(){
  var sel = document.getElementById("tmpl-select");
  var id = parseInt(sel.value,10);
  if(!id) return;
  for(var i=0;i<templatesCache.length;i++){
    if(templatesCache[i].id === id){
      document.getElementById("tmpl-name").value = templatesCache[i].name;
      document.getElementById("add-desc").value = templatesCache[i].content;
      editingTemplateId = id;
      updateTemplateSaveBtn();
      document.getElementById("tmpl-name").focus();
      break;
    }
  }
}

function updateTemplateSaveBtn(){
  var btn = document.querySelector(".template-save-row .btn");
  if(!btn) return;
  if(editingTemplateId){
    btn.textContent = "Update snippet";
    btn.onclick = function(){ updateTemplate(); };
  } else {
    btn.textContent = "Save snippet";
    btn.onclick = function(){ saveTemplate(); };
  }
}

function updateTemplate(){
  var nameEl = document.getElementById("tmpl-name");
  var name = nameEl.value.trim();
  var content = document.getElementById("add-desc").value.trim();
  if(!name){ nameEl.focus(); return; }
  if(!content){ document.getElementById("add-desc").focus(); return; }
  api("POST","/api/templates/update",{id:editingTemplateId,name:name,content:content},function(d){
    if(d.error){ toast("Error: "+d.error, "error"); return; }
    editingTemplateId = 0;
    nameEl.value = "";
    updateTemplateSaveBtn();
    loadTemplates();
    toast("Template updated", "success");
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
  editingTemplateId = 0;
  document.getElementById("tmpl-select").onchange = onTemplateSelect;
  loadTemplates();
  updateTemplateSaveBtn();
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
  var addBtn = document.querySelector("#modal-add .btn-primary");
  var restore = btnLoading(addBtn, "ADDING\u2026");
  api("POST","/api/tasks/add",{project:proj, description:desc, priority:pri}, function(d){
    if(restore) restore();
    closeModal("modal-add");
    if(d.id){ selectedTaskID = d.id; toast("Task #"+d.id+" created", "success"); }
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
  if(l.agent && agentMap[l.agent]){
    var ab = span("term-agent", agentMap[l.agent].icon + " " + agentMap[l.agent].name);
    ab.style.color = agentMap[l.agent].color;
    line.appendChild(ab);
  }
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
    case "success": s.style.color = "var(--success)"; s.textContent = "\u2713"; break;
    case "warn": s.style.color = "var(--warning)"; s.textContent = "\u26A0"; break;
    case "error": s.style.color = "var(--danger)"; s.textContent = "\u2717"; break;
    default: s.style.color = "var(--text-muted)"; s.textContent = "\u2139"; break;
  }
  return s;
}
function buildPhaseRing(pct, size){
  // Phase ring: arc gauge encoding percentage as sweep
  size = size || 28;
  var strokeW = 2.5;
  var r = (size / 2) - strokeW;
  var cx = size / 2, cy = size / 2;
  var ns = "http://www.w3.org/2000/svg";
  var svg = document.createElementNS(ns, "svg");
  svg.setAttribute("width", size);
  svg.setAttribute("height", size);
  svg.setAttribute("viewBox", "0 0 " + size + " " + size);
  svg.style.display = "block";

  // Track (background ring)
  var track = document.createElementNS(ns, "circle");
  track.setAttribute("cx", cx);
  track.setAttribute("cy", cy);
  track.setAttribute("r", r);
  track.setAttribute("fill", "none");
  track.setAttribute("stroke", "var(--glass)");
  track.setAttribute("stroke-width", strokeW);
  svg.appendChild(track);

  // Arc (filled portion)
  if(pct > 0.5){
    var color = pct >= 80 ? "var(--danger)" : pct >= 60 ? "var(--warning)" : "var(--accent)";
    var circ = 2 * Math.PI * r;
    var dash = (pct / 100) * circ;
    var arc = document.createElementNS(ns, "circle");
    arc.setAttribute("cx", cx);
    arc.setAttribute("cy", cy);
    arc.setAttribute("r", r);
    arc.setAttribute("fill", "none");
    arc.setAttribute("stroke", color);
    arc.setAttribute("stroke-width", strokeW);
    arc.setAttribute("stroke-dasharray", dash.toFixed(1) + " " + circ.toFixed(1));
    arc.setAttribute("stroke-linecap", "round");
    arc.setAttribute("transform", "rotate(-90 " + cx + " " + cy + ")");
    svg.appendChild(arc);
  }

  return svg;
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
function chatSetSanitizedHtml(elem, md) {
  // Uses marked.js + DOMPurify for safe HTML rendering
  elem.innerHTML = renderMarkdown(md); // eslint-disable-line -- sanitized by DOMPurify
  // Post-process: style collapsible details/summary
  var details = elem.querySelectorAll("details");
  for(var i=0;i<details.length;i++){
    details[i].classList.add("chat-phase-details");
    var sum = details[i].querySelector("summary");
    if(sum) sum.classList.add("chat-phase-summary");
  }
}

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
      chatInitSteps = [];
      chatToolCalls = [];
      chatCounter++;
      render();
    });
  };
  topBar.appendChild(clearBtn);
  wrap.appendChild(topBar);

  // Messages wrapper (relative for scroll-to-bottom btn)
  var msgsWrap = el("div","chat-messages-wrap");

  var msgArea = el("div","chat-messages");
  msgArea.id = "chat-messages";

  if(chatMessages.length === 0 && !chatLoading){
    // Empty state with suggestion chips
    var emptyState = el("div","chat-empty-state");
    var emptyIcon = document.createElementNS("http://www.w3.org/2000/svg","svg");
    emptyIcon.setAttribute("viewBox","0 0 24 24");
    emptyIcon.setAttribute("width","48");
    emptyIcon.setAttribute("height","48");
    emptyIcon.setAttribute("fill","none");
    emptyIcon.setAttribute("stroke","currentColor");
    emptyIcon.setAttribute("stroke-width","1.5");
    emptyIcon.setAttribute("class","chat-empty-icon");
    var iconPath = document.createElementNS("http://www.w3.org/2000/svg","path");
    iconPath.setAttribute("d","M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z");
    emptyIcon.appendChild(iconPath);
    emptyState.appendChild(emptyIcon);
    var emptyText = el("div","chat-empty-text");
    emptyText.textContent = "Start a conversation with the AI assistant";
    emptyState.appendChild(emptyText);
    var chips = el("div","chat-suggestions");
    var suggestions = [
      "Create a new project",
      "Break down a task into subtasks",
      "Research a technology stack"
    ];
    for(var si=0;si<suggestions.length;si++){
      var chip = el("div","chat-chip");
      chip.textContent = suggestions[si];
      chip.onclick = (function(text){
        return function(){
          var ta = document.getElementById("chat-textarea");
          if(ta){ ta.value = text; ta.focus(); }
        };
      })(suggestions[si]);
      chips.appendChild(chip);
    }
    emptyState.appendChild(chips);
    msgArea.appendChild(emptyState);
  } else {
    for(var i=0;i<chatMessages.length;i++){
      var m = chatMessages[i];
      if(m.role === "user"){
        var userBubble = el("div","chat-bubble user");
        userBubble.textContent = m.content;
        msgArea.appendChild(userBubble);
      } else {
        var bwrap = el("div","chat-bubble-wrap");
        bwrap.style.alignSelf = "flex-start";
        bwrap.style.maxWidth = "95%";
        var aBubble = el("div","chat-bubble assistant");
        aBubble.style.maxWidth = "100%";

        // Typing indicator for empty loading bubble
        if(chatLoading && m.content === "" && i === chatMessages.length - 1){
          var typing = el("div","chat-typing");
          for(var td=0;td<3;td++){
            typing.appendChild(el("div","chat-typing-dot"));
          }
          aBubble.appendChild(typing);
        } else {
          var textDiv = el("div","chat-text-content");
          chatSetSanitizedHtml(textDiv, m.content);
          aBubble.appendChild(textDiv);
        }
        // Persistent processing bar — visible until done event
        if(chatLoading && i === chatMessages.length - 1){
          var procBar = el("div","chat-processing-bar");
          var procSpinner = document.createElement("span");
          procSpinner.className = "spinner-sm";
          procBar.appendChild(procSpinner);
          var procText = document.createElement("span");
          procText.textContent = " Processing\u2026";
          procBar.appendChild(procText);
          aBubble.appendChild(procBar);
        }
        bwrap.appendChild(aBubble);

        // Copy button (only for non-empty, non-loading)
        if(m.content && !(chatLoading && i === chatMessages.length - 1)){
          var actions = el("div","chat-bubble-actions");
          var copyBtn = el("button","chat-copy-btn");
          copyBtn.textContent = "Copy";
          copyBtn.onclick = (function(content){
            return function(){
              navigator.clipboard.writeText(content).then(function(){
                toast("Copied to clipboard","success");
              });
            };
          })(m.content);
          actions.appendChild(copyBtn);
          bwrap.appendChild(actions);
        }
        msgArea.appendChild(bwrap);
      }
    }
  }
  msgsWrap.appendChild(msgArea);

  // Scroll-to-bottom button
  var scrollBtn = el("div","chat-scroll-btn");
  scrollBtn.textContent = "\u2193";
  scrollBtn.onclick = function(){ msgArea.scrollTo({ top: msgArea.scrollHeight, behavior: "smooth" }); };
  msgsWrap.appendChild(scrollBtn);

  msgArea.onscroll = function(){
    var atBottom = msgArea.scrollHeight - msgArea.scrollTop - msgArea.clientHeight < 100;
    if(atBottom) scrollBtn.classList.remove("visible");
    else scrollBtn.classList.add("visible");
  };

  wrap.appendChild(msgsWrap);

  // Input bar
  var inputBar = el("div","chat-input-bar");
  var textarea = el("textarea","chat-input");
  textarea.id = "chat-textarea";
  textarea.rows = 2;
  textarea.placeholder = "Type a message\u2026";
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

  // Auto-scroll to bottom after render
  setTimeout(function(){ msgArea.scrollTop = msgArea.scrollHeight; }, 0);

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
  chatToolCalls = [];
  chatTurnStartMs = Date.now();
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
    if(!res.ok && !res.headers.get("content-type")?.startsWith("text/event-stream")){
      chatMessages[chatMessages.length-1].content = "Error: server returned " + res.status;
      chatLoading = false; chatCounter++; render();
      return;
    }
    var reader = res.body.getReader();
    var decoder = new TextDecoder();
    var buffer = "";
    var gotDone = false;
    function pump(){
      reader.read().then(function(result){
        if(result.done){
          if(!gotDone){
            chatMessages[chatMessages.length-1].content = chatMessages[chatMessages.length-1].content || "No response from Claude. Check that claude is installed and authenticated.";
          }
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
              if(evt.tool_use){
                // Mark previous tool as done if pending
                if(chatToolCalls.length>0 && !chatToolCalls[chatToolCalls.length-1].done){
                  chatToolCalls[chatToolCalls.length-1].done=true;
                }
                chatToolCalls.push({name:evt.tool_use,done:false});
                var msgDivT=document.getElementById("chat-messages");
                if(msgDivT){
                  var bubblesT=msgDivT.querySelectorAll(".chat-bubble.assistant");
                  if(bubblesT.length>0){
                    var lastBT=bubblesT[bubblesT.length-1];
                    var typingT=lastBT.querySelector(".chat-typing");
                    if(typingT) lastBT.removeChild(typingT);
                    chatUpdateToolActivity(lastBT);
                    msgDivT.scrollTop=msgDivT.scrollHeight;
                  }
                }
              }
              if(evt.tool_done){
                if(chatToolCalls.length>0 && !chatToolCalls[chatToolCalls.length-1].done){
                  chatToolCalls[chatToolCalls.length-1].done=true;
                }
                var msgDivTD=document.getElementById("chat-messages");
                if(msgDivTD){
                  var bubblesTD=msgDivTD.querySelectorAll(".chat-bubble.assistant");
                  if(bubblesTD.length>0) chatUpdateToolActivity(bubblesTD[bubblesTD.length-1]);
                }
              }
              if(evt.token){
                chatMessages[chatMessages.length-1].content += evt.token;
                var msgDiv = document.getElementById("chat-messages");
                if(msgDiv){
                  var bubbles = msgDiv.querySelectorAll(".chat-bubble.assistant");
                  if(bubbles.length > 0){
                    var lastB = bubbles[bubbles.length-1];
                    // Remove typing indicator on first token
                    var typingEl = lastB.querySelector(".chat-typing");
                    if(typingEl) lastB.removeChild(typingEl);
                    // Render into text-content div, not the whole bubble
                    var textDiv = lastB.querySelector(".chat-text-content");
                    if(!textDiv){
                      textDiv = document.createElement("div");
                      textDiv.className = "chat-text-content";
                      lastB.appendChild(textDiv);
                    }
                    chatSetSanitizedHtml(textDiv, chatMessages[chatMessages.length-1].content);
                    msgDiv.scrollTop = msgDiv.scrollHeight;
                  }
                }
              }
              if(evt.init_step){
                var step = evt.init_step;
                chatInitSteps.push(step);
                var msgDiv2 = document.getElementById("chat-messages");
                if(msgDiv2){
                  var bubbles2 = msgDiv2.querySelectorAll(".chat-bubble.assistant");
                  if(bubbles2.length > 0){
                    var lastB2 = bubbles2[bubbles2.length-1];
                    // Remove typing indicator if still showing
                    var typingEl2 = lastB2.querySelector(".chat-typing");
                    if(typingEl2) lastB2.removeChild(typingEl2);
                    // Find or create init steps container
                    var stepsDiv = lastB2.querySelector(".chat-init-steps");
                    if(!stepsDiv){
                      stepsDiv = document.createElement("div");
                      stepsDiv.className = "chat-init-steps";
                      lastB2.appendChild(stepsDiv);
                    }
                    var stepEl = document.createElement("div");
                    var stepOk = (step.status === "done" || step.status === "ok");
                    stepEl.className = "chat-init-step " + (stepOk ? "ok" : (step.status === "running" ? "running" : "error"));
                    var iconSpan = document.createElement("span");
                    iconSpan.className = "step-icon";
                    iconSpan.textContent = stepOk ? "\u2713" : (step.status === "running" ? "\u22ef" : "\u2717");
                    stepEl.appendChild(iconSpan);
                    var nameSpan = document.createElement("span");
                    nameSpan.className = "step-name";
                    nameSpan.textContent = step.name + (step.message && !stepOk ? ": " + step.message : "");
                    stepEl.appendChild(nameSpan);
                    stepsDiv.appendChild(stepEl);
                    msgDiv2.scrollTop = msgDiv2.scrollHeight;
                  }
                }
              }
              if(evt.project_init){
                if(evt.status === "success"){
                  chatProject = evt.project_init;
                  toast("Project " + evt.project_init + " initialized!", "success");
                  fetchData();
                } else {
                  toast("Project init failed: " + (evt.error || "unknown"), "error");
                }
              }
              if(evt.tasks_created){
                chatCreatedTasks = evt.tasks_created;
              }
              if(evt.error && !evt.project_init){
                toast(evt.error, "error");
                if(!chatMessages[chatMessages.length-1].content){
                  chatMessages[chatMessages.length-1].content = "Error: " + evt.error;
                }
              }
              if(evt.done){
                gotDone = true;
                if(evt.result) chatMessages[chatMessages.length-1].content = evt.result;
                // Strip directives from stored/displayed content
                var cleanContent = chatMessages[chatMessages.length-1].content
                  .replace(/\[TASK_CREATE\][\s\S]*?\[\/TASK_CREATE\]/g, "")
                  .replace(/\[PROJECT_INIT\][\s\S]*?\[\/PROJECT_INIT\]/g, "").trim();
                chatMessages[chatMessages.length-1].content = cleanContent;
                // Mark all pending tools as done
                for(var tc=0;tc<chatToolCalls.length;tc++) chatToolCalls[tc].done=true;
                var turnMeta = {
                  num_turns: evt.num_turns||0,
                  cost_usd: evt.cost_usd||0,
                  duration_ms: Date.now()-chatTurnStartMs
                };
                chatInitSteps = [];
                chatLoading = false;
                chatCounter++;
                render();
                // Append meta footer to last bubble
                var mdMeta = document.getElementById("chat-messages");
                if(mdMeta){
                  var bbsMeta = mdMeta.querySelectorAll(".chat-bubble.assistant");
                  if(bbsMeta.length>0) chatAppendMetaFooter(bbsMeta[bbsMeta.length-1], turnMeta);
                }
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
                            selectedTaskID = taskId;
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
  if(t.effective_state === "running"){
    var lastAg = getLastAgentForTask(t.id);
    if(lastAg && agentMap[lastAg]){
      var ab = span("canvas-agent-badge", agentMap[lastAg].icon + " " + agentMap[lastAg].name);
      ab.style.color = agentMap[lastAg].color;
      footer.appendChild(ab);
    }
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
  var cancelBtn = document.querySelector("#modal-init .modal-actions .btn:first-child");
  submitBtn.disabled = true;
  submitBtn.textContent = "";
  var sp = document.createElement("span"); sp.className = "btn-spinner"; submitBtn.appendChild(sp);
  submitBtn.appendChild(document.createTextNode(" Creating\u2026"));
  if(cancelBtn) cancelBtn.disabled = true;

  var prog = document.getElementById("init-progress");
  prog.style.display = "block";
  prog.textContent = "";

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
          if(cancelBtn) cancelBtn.disabled = false;
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
                  var doneIcon = el("span","init-step-icon");
                  doneIcon.style.color = "var(--success)";
                  doneIcon.textContent = "\u2713";
                  doneRow.appendChild(doneIcon);
                  doneRow.appendChild(el("span","init-step-name",["Project created successfully!"]));
                  prog.appendChild(doneRow);
                  toast("Project " + name + " created!", "success");
                } else {
                  toast("Project creation failed", "error");
                }
                submitBtn.disabled = false;
                submitBtn.textContent = "Create Project";
                if(cancelBtn) cancelBtn.disabled = false;
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
    if(cancelBtn) cancelBtn.disabled = false;
    toast("Connection error during project creation", "error");
  });
}

function initStepIcon(status){
  var s = document.createElement("span");
  if(status === "done"){ s.style.color = "var(--success)"; s.textContent = "\u2713"; }
  else if(status === "error"){ s.style.color = "var(--danger)"; s.textContent = "\u2717"; }
  else if(status === "running"){ s.style.color = "var(--warning)"; s.textContent = "\u2022"; }
  else { s.style.color = "var(--danger)"; s.textContent = "\u2717"; }
  return s;
}

function renderInitStep(prog, evt){
  var existing = document.getElementById("init-step-" + evt.step);
  if(existing){
    var icon = existing.querySelector(".init-step-icon");
    if(evt.status === "done"){
      icon.textContent = "";
      icon.appendChild(initStepIcon("done"));
    } else if(evt.status === "error"){
      icon.textContent = "";
      icon.appendChild(initStepIcon("error"));
      var msg = el("span","init-step-error");
      msg.textContent = evt.message || "";
      existing.appendChild(msg);
    }
    return;
  }
  var row = el("div","init-progress-step");
  row.id = "init-step-" + evt.step;
  var icon = el("span","init-step-icon");
  icon.appendChild(initStepIcon(evt.status));
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
  checkOnboarding();
});
connectSSE();

/* Export for inline event handlers in index.html */
window.closeModal = closeModal;
window.submitAddTask = submitAddTask;
window.saveTemplate = saveTemplate;
window.deleteSelectedTemplate = deleteSelectedTemplate;
window.editSelectedTemplate = editSelectedTemplate;
window.mergeDependabot = mergeDependabot;
window.submitProjectInit = submitProjectInit;

/* ── Setup View (Ubuntu Installer) ── */

var SETUP_STEPS = [
  {id:1, name:"Prerequisites", desc:"Check and install required development tools (system packages, Go, Node.js, Python, Rust, GitHub CLI, Claude Code)."},
  {id:2, name:"Configuration", desc:"Set up projects directory, web port, and authentication."},
  {id:3, name:"Skills", desc:"Install Claude Code skills for enhanced capabilities (21 skills)."},
  {id:4, name:"BMAD", desc:"Install BMAD commands for project management workflows."},
  {id:5, name:"Hooks", desc:"Install security hooks to prevent destructive operations."},
  {id:6, name:"MCP Servers", desc:"Install MCP servers for documentation, memory, and reasoning."}
];

function checkOnboarding(){
  fetch("/api/onboarding/status").then(function(r){ return r.json(); }).then(function(s){
    if(s && s.needed && location.hash !== "#setup"){
      location.hash = "setup";
    }
  }).catch(function(){});
}

function nextIncompleteStep(){
  for(var i=0;i<SETUP_STEPS.length;i++){
    if(!setupStepDone[SETUP_STEPS[i].id]) return SETUP_STEPS[i].id;
  }
  return 0;
}

function allSetupDone(){
  for(var i=0;i<SETUP_STEPS.length;i++){
    if(!setupStepDone[SETUP_STEPS[i].id]) return false;
  }
  return true;
}

function loadSetupStatus(){
  if(setupStatusFetching) return;
  setupStatusFetching = true;
  fetch("/api/onboarding/status").then(function(r){ return r.json(); }).then(function(s){
    setupStatusFetching = false;
    if(!s || !s.steps) return;
    if(s.steps.config) setupStepDone[2] = true;
    if(s.steps.skills) setupStepDone[3] = true;
    if(s.steps.bmad) setupStepDone[4] = true;
    if(s.steps.hooks) setupStepDone[5] = true;
    if(s.steps.mcp) setupStepDone[6] = true;
    setupStatusLoaded = true;
    if(!setupRunning) render();
  }).catch(function(){ setupStatusFetching = false; });
}

function renderSetup(root){
  loadSetupStatus();
  var container = div("setup-container");

  // Sidebar
  var sidebar = div("setup-sidebar");
  sidebar.appendChild(el("div","setup-sidebar-title",["teamoon setup"]));

  for(var i=0;i<SETUP_STEPS.length;i++){
    var st = SETUP_STEPS[i];
    var cls = "setup-step";
    if(st.id === setupStep) cls += " active";
    if(setupStepDone[st.id]) cls += " done";
    if(setupStepError[st.id]) cls += " error";

    var item = div(cls);
    item.setAttribute("data-step", st.id);

    var icon = div("setup-step-icon");
    if(setupStepDone[st.id]) icon.textContent = "\u2713";
    else if(setupStepError[st.id]) icon.textContent = "\u2717";
    else if(st.id === setupStep) icon.textContent = "\u25CF";
    else icon.textContent = "\u25CB";

    item.appendChild(icon);
    item.appendChild(div("setup-step-label",[st.name]));

    (function(sid){
      item.onclick = function(){ if(!setupRunning){ setupStep = sid; render(); } };
    })(st.id);

    sidebar.appendChild(item);
  }

  var skipWrap = div("setup-sidebar-actions");
  var skipBtn = el("button","btn btn-sm",["Go to Dashboard"]);
  skipBtn.onclick = function(){ location.hash = "dashboard"; };
  skipWrap.appendChild(skipBtn);
  sidebar.appendChild(skipWrap);
  container.appendChild(sidebar);

  // Main area
  var main = div("setup-main");

  if(allSetupDone()){
    main.appendChild(el("div","setup-title",["Setup Complete"]));
    main.appendChild(el("div","setup-desc",["All steps have been completed. Your environment is ready."]));
    var content = div("setup-content");
    var doneWrap = div("setup-complete");
    doneWrap.appendChild(el("div","setup-complete-icon",["\u2705"]));
    doneWrap.appendChild(el("div","setup-complete-msg",["teamoon is fully configured and ready to use."]));
    var goBtn = el("button","btn btn-primary",["Go to Dashboard"]);
    goBtn.onclick = function(){ location.hash = "dashboard"; };
    doneWrap.appendChild(goBtn);
    content.appendChild(doneWrap);
    main.appendChild(content);
  } else {
    var step = SETUP_STEPS[setupStep - 1];
    main.appendChild(el("div","setup-title",[step.name]));
    main.appendChild(el("div","setup-desc",[step.desc]));

    var content = div("setup-content");
    switch(setupStep){
      case 1: renderSetupPrereqs(content); break;
      case 2: renderSetupConfig(content); break;
      case 3: renderSetupSkills(content); break;
      case 4: renderSetupBMAD(content); break;
      case 5: renderSetupHooks(content); break;
      case 6: renderSetupMCP(content); break;
    }
    main.appendChild(content);
  }
  container.appendChild(main);
  root.appendChild(container);
}

function streamStep(url, body, progressEl, renderEvent, onDone){
  setupRunning = true;
  var opts = {method:"POST"};
  if(body){ opts.headers = {"Content-Type":"application/json"}; opts.body = JSON.stringify(body); }
  fetch(url, opts).then(function(res){
    var reader = res.body.getReader();
    var decoder = new TextDecoder();
    var buffer = "";
    function pump(){
      reader.read().then(function(result){
        if(result.done){ setupRunning = false; return; }
        buffer += decoder.decode(result.value, {stream:true});
        var lines = buffer.split("\n");
        buffer = lines.pop();
        for(var i=0;i<lines.length;i++){
          var line = lines[i].trim();
          if(line.indexOf("data: ")===0){
            try{
              var evt = JSON.parse(line.substring(6));
              if(evt.done){
                setupRunning = false;
                onDone(evt.status, evt.message);
                return;
              }
              renderEvent(progressEl, evt);
            }catch(e){}
          }
        }
        pump();
      });
    }
    pump();
  }).catch(function(e){
    setupRunning = false;
    onDone("error", e.message || "Connection error");
  });
}

var prereqsMissing = [];

function renderSetupPrereqs(content){
  var prog = div("setup-progress");
  var actions = div("setup-prereqs-actions");

  var btn = el("button","btn btn-primary",[setupStepDone[1] ? "Re-check" : "Check Prerequisites"]);
  btn.onclick = function(){
    prog.textContent = "";
    actions.textContent = "";
    prereqsMissing = [];
    var restore = btnLoading(btn, "Checking\u2026");
    streamStep("/api/onboarding/prereqs", null, prog, function(p, evt){
      if(evt.type !== "tool") return;
      var ok = evt.found;
      var cls = "setup-progress-item " + (ok ? "ok" : (evt.required ? "error" : "skip"));
      var item = div(cls);
      item.id = "setup-prereq-" + evt.id;
      item.appendChild(span("icon", ok ? "\u2713" : (evt.required ? "\u2717" : "~")));
      item.appendChild(span("label", evt.name));
      var verText = evt.version || (ok ? "OK" : (evt.required ? "not found" : "optional, not found"));
      item.appendChild(span("version", verText));
      if(!ok && evt.installable) item.appendChild(span("tag-installable", "installable"));
      p.appendChild(item);
      if(!ok) prereqsMissing.push(evt);
    }, function(status, msg){
      if(restore) restore();
      if(status === "success"){
        setupStepDone[1] = true;
        delete setupStepError[1];
        prog.appendChild(div("setup-status ok",["All required tools found"]));
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
      } else {
        setupStepError[1] = true;
        var installable = prereqsMissing.filter(function(m){ return m.installable; });
        if(installable.length > 0){
          var installBtn = el("button","btn btn-success",["Install Missing (" + installable.length + ")"]);
          installBtn.onclick = function(){ runPrereqsInstall(prog, actions); };
          actions.appendChild(installBtn);
        }
        prog.appendChild(div("setup-status err",[msg || "Missing required tools"]));
      }
      render();
    });
  };
  content.appendChild(btn);
  content.appendChild(actions);
  content.appendChild(prog);

}

function runPrereqsInstall(prog, actions){
  actions.textContent = "";
  var installProg = div("setup-progress");
  installProg.style.marginTop = "16px";
  installProg.appendChild(el("div","setup-progress-header",["Installing missing tools..."]));
  prog.parentNode.appendChild(installProg);

  var installBtn = actions.querySelector && actions.querySelector("button");
  setupRunning = true;

  streamStep("/api/onboarding/prereqs/install", null, installProg, function(p, evt){
    if(evt.type === "detail"){
      var detail = div("setup-progress-item running");
      detail.appendChild(span("icon","\u2022"));
      detail.appendChild(span("label", evt.message));
      p.appendChild(detail);
      return;
    }
    if(evt.type !== "install") return;

    var existing = document.getElementById("setup-install-" + evt.id);
    if(existing){
      var ic = existing.querySelector(".icon");
      var lb = existing.querySelector(".label");
      if(evt.status === "done"){ ic.textContent = "\u2713"; existing.className = "setup-progress-item ok"; }
      else if(evt.status === "error"){ ic.textContent = "\u2717"; existing.className = "setup-progress-item error"; if(evt.message && lb) lb.textContent = evt.name + " — " + evt.message; }
      else if(evt.status === "skipped"){ ic.textContent = "~"; existing.className = "setup-progress-item skip"; }
      // Also update the check result row
      var checkRow = document.getElementById("setup-prereq-" + evt.id);
      if(checkRow && evt.status === "done"){
        checkRow.className = "setup-progress-item ok";
        var cIcon = checkRow.querySelector(".icon");
        if(cIcon) cIcon.textContent = "\u2713";
        var cVer = checkRow.querySelector(".version");
        if(cVer) cVer.textContent = "installed";
        var cTag = checkRow.querySelector(".tag-installable");
        if(cTag) cTag.remove();
      }
      return;
    }

    var cls = "setup-progress-item";
    var iconText = "\u2022";
    if(evt.status === "done"){ cls += " ok"; iconText = "\u2713"; }
    else if(evt.status === "skipped"){ cls += " skip"; iconText = "~"; }
    else if(evt.status === "error"){ cls += " error"; iconText = "\u2717"; }
    else { cls += " running"; }
    var item = div(cls);
    item.id = "setup-install-" + evt.id;
    item.appendChild(span("icon", iconText));
    item.appendChild(span("label", evt.name + (evt.message && evt.status === "error" ? " — " + evt.message : "")));
    p.appendChild(item);
  }, function(status, msg){
    setupRunning = false;
    if(status === "success"){
      setupStepDone[1] = true;
      delete setupStepError[1];
      installProg.appendChild(div("setup-status ok",["All tools installed successfully"]));
      var ns = nextIncompleteStep();
      if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
    } else {
      installProg.appendChild(div("setup-status err",[msg || "Some tools failed to install"]));
    }
    render();
  });
}

function renderSetupConfig(content){
  var form = div("setup-form");
  var fields = [
    {id:"setup-projects-dir", label:"Projects Directory", val:"~/Projects", type:"text"},
    {id:"setup-web-port", label:"Web Dashboard Port", val:"7777", type:"number"},
    {id:"setup-web-host", label:"Bind Address", val:"localhost", type:"select", options:["localhost","0.0.0.0"]},
    {id:"setup-web-password", label:"Web Password (empty = no auth)", val:"", type:"password"},
    {id:"setup-max-concurrent", label:"Max Concurrent Sessions", val:"3", type:"number"}
  ];
  for(var i=0;i<fields.length;i++){
    var f = fields[i];
    var field = div("setup-form-field");
    field.appendChild(el("label","setup-form-label",[f.label]));
    if(f.type === "select" && f.options){
      var sel = document.createElement("select"); sel.className = "setup-form-input"; sel.id = f.id;
      for(var j=0;j<f.options.length;j++){
        var opt = document.createElement("option"); opt.value = f.options[j]; opt.textContent = f.options[j];
        if(f.options[j] === f.val) opt.selected = true;
        sel.appendChild(opt);
      }
      field.appendChild(sel);
    } else {
      var inp = elAttr("input","setup-form-input",{type:f.type, id:f.id, value:f.val, placeholder:f.val});
      field.appendChild(inp);
    }
    form.appendChild(field);
  }
  var btn = el("button","btn btn-primary",[setupStepDone[2] ? "Re-configure" : "Save Configuration"]);
  btn.onclick = function(){
    var restore = btnLoading(btn, "Saving\u2026");
    var wc = {
      projects_dir: document.getElementById("setup-projects-dir").value || "~/Projects",
      web_port: parseInt(document.getElementById("setup-web-port").value) || 7777,
      web_host: document.getElementById("setup-web-host").value,
      web_password: document.getElementById("setup-web-password").value,
      max_concurrent: parseInt(document.getElementById("setup-max-concurrent").value) || 3
    };
    api("POST","/api/onboarding/config", wc, function(data){
      if(restore) restore();
      if(data && data.ok){
        setupStepDone[2] = true;
        delete setupStepError[2];
        toast("Configuration saved","success");
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 600);
      } else {
        setupStepError[2] = true;
        toast("Failed to save configuration","error");
      }
      render();
    });
  };
  content.appendChild(form);
  content.appendChild(btn);
}

function renderSetupSkills(content){
  var prog = div("setup-progress");
  var btn = el("button","btn btn-primary",[setupStepDone[3] ? "Re-install" : "Install Skills"]);
  btn.onclick = function(){
    prog.textContent = "";
    var restore = btnLoading(btn, "Installing\u2026");
    streamStep("/api/onboarding/skills", null, prog, function(p, evt){
      if(evt.type === "symlink"){
        var item = div("setup-progress-item ok");
        item.appendChild(span("icon","\u2713"));
        item.appendChild(span("label","Symlink: ~/.agents/skills"));
        p.appendChild(item);
        return;
      }
      var existing = document.getElementById("setup-skill-"+evt.name);
      if(existing){
        var ic = existing.querySelector(".icon");
        if(evt.status === "done"){ ic.textContent = "\u2713"; existing.className = "setup-progress-item ok"; }
        else if(evt.status.indexOf("error")===0){ ic.textContent = "\u2717"; existing.className = "setup-progress-item error"; }
        else if(evt.status === "skipped"){ ic.textContent = "~"; existing.className = "setup-progress-item skip"; }
        return;
      }
      var cls = "setup-progress-item";
      var iconText = "\u2022";
      if(evt.status === "done"){ cls += " ok"; iconText = "\u2713"; }
      else if(evt.status === "skipped"){ cls += " skip"; iconText = "~"; }
      else if(evt.status.indexOf("error")===0){ cls += " error"; iconText = "\u2717"; }
      else { cls += " running"; }
      var item = div(cls);
      item.id = "setup-skill-"+evt.name;
      item.appendChild(span("icon", iconText));
      item.appendChild(span("label", evt.name));
      p.appendChild(item);
    }, function(status, msg){
      if(restore) restore();
      if(status === "success"){
        setupStepDone[3] = true;
        delete setupStepError[3];
        prog.appendChild(div("setup-status ok",["All skills installed"]));
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
      } else {
        setupStepError[3] = true;
        prog.appendChild(div("setup-status err",[msg || "Some skills failed"]));
      }
      render();
    });
  };
  content.appendChild(btn);
  content.appendChild(prog);
}

function renderSetupBMAD(content){
  var prog = div("setup-progress");
  var btn = el("button","btn btn-primary",[setupStepDone[4] ? "Re-install" : "Install BMAD Commands"]);
  btn.onclick = function(){
    prog.textContent = "";
    var barWrap = div("setup-progress-bar-wrap");
    var barFill = div("setup-progress-bar-fill");
    barFill.id = "setup-bmad-bar";
    barWrap.appendChild(barFill);
    var counter = span("setup-progress-counter","0 / ?");
    counter.id = "setup-bmad-counter";
    prog.appendChild(barWrap);
    prog.appendChild(counter);

    var restore = btnLoading(btn, "Installing\u2026");
    streamStep("/api/onboarding/bmad", null, prog, function(p, evt){
      if(evt.type === "progress"){
        var fill = document.getElementById("setup-bmad-bar");
        var ctr = document.getElementById("setup-bmad-counter");
        if(fill) fill.style.width = Math.round((evt.count/evt.total)*100)+"%";
        if(ctr) ctr.textContent = evt.count+" / "+evt.total+" files";
      } else if(evt.type === "symlink"){
        var item = div("setup-progress-item ok");
        item.appendChild(span("icon","\u2713"));
        item.appendChild(span("label","Symlink: ~/.claude/commands/bmad"));
        p.appendChild(item);
      }
    }, function(status, msg){
      if(restore) restore();
      if(status === "success"){
        setupStepDone[4] = true;
        delete setupStepError[4];
        prog.appendChild(div("setup-status ok",["BMAD commands installed"]));
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
      } else {
        setupStepError[4] = true;
        prog.appendChild(div("setup-status err",[msg || "BMAD installation failed"]));
      }
      render();
    });
  };
  content.appendChild(btn);
  content.appendChild(prog);
}

function renderSetupHooks(content){
  var prog = div("setup-progress");
  var btn = el("button","btn btn-primary",[setupStepDone[5] ? "Re-install" : "Install Security Hooks"]);
  btn.onclick = function(){
    prog.textContent = "";
    var restore = btnLoading(btn, "Installing\u2026");
    streamStep("/api/onboarding/hooks", null, prog, function(p, evt){
      var item = div("setup-progress-item ok");
      item.appendChild(span("icon","\u2713"));
      item.appendChild(span("label", evt.name || evt.type));
      p.appendChild(item);
    }, function(status, msg){
      if(restore) restore();
      if(status === "success"){
        setupStepDone[5] = true;
        delete setupStepError[5];
        prog.appendChild(div("setup-status ok",["Security hooks installed"]));
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
      } else {
        setupStepError[5] = true;
        prog.appendChild(div("setup-status err",[msg || "Hooks installation failed"]));
      }
      render();
    });
  };
  content.appendChild(btn);
  content.appendChild(prog);
}

function renderSetupMCP(content){
  var prog = div("setup-progress");
  var btn = el("button","btn btn-primary",[setupStepDone[6] ? "Re-install" : "Install MCP Servers"]);
  btn.onclick = function(){
    prog.textContent = "";
    var restore = btnLoading(btn, "Installing\u2026");
    streamStep("/api/onboarding/mcp", null, prog, function(p, evt){
      var cls = evt.status === "done" ? "ok" : (evt.status === "skipped" ? "skip" : "running");
      var iconText = evt.status === "done" ? "\u2713" : (evt.status === "skipped" ? "~" : "\u2022");
      var item = div("setup-progress-item "+cls);
      item.appendChild(span("icon", iconText));
      item.appendChild(span("label", evt.name));
      if(evt.status === "skipped") item.appendChild(span("version","already configured"));
      p.appendChild(item);
    }, function(status, msg){
      if(restore) restore();
      if(status === "success"){
        setupStepDone[6] = true;
        delete setupStepError[6];
        prog.appendChild(div("setup-status ok",["MCP servers installed"]));
        if(allSetupDone()) setTimeout(function(){ render(); }, 800);
      } else {
        setupStepError[6] = true;
        prog.appendChild(div("setup-status err",[msg || "MCP installation failed"]));
      }
      render();
    });
  };
  content.appendChild(btn);
  content.appendChild(prog);
}

/* ── Configuration View ── */
function loadConfig(cb){
  api("GET","/api/config",null,function(data){
    configData = data;
    configLoaded++;
    if(cb) cb();
  });
}

function renderConfig(root){
  var header = div("view-header");
  header.appendChild(span("view-title", "Configuration"));
  root.appendChild(header);

  // Top tab bar: About | General | Templates
  var tabBar = div("config-tabs");
  ["General","Marketplace","Templates","Setup","About"].forEach(function(label){
    var key = label.toLowerCase();
    var tb = el("button", "config-tab-btn" + (configTab === key ? " active" : ""), [label]);
    tb.onclick = function(){ configTab = key; configEditing = null; render(); };
    tabBar.appendChild(tb);
  });
  root.appendChild(tabBar);

  if(configTab === "about"){
    if(!configData){
      loadConfig(function(){ render(); });
      var ld = div("empty"); ld.textContent = "Loading\u2026"; root.appendChild(ld);
      return;
    }
    renderConfigAbout(root);
    return;
  }

  if(configTab === "setup"){
    renderConfigSetup(root);
    return;
  }

  if(configTab === "marketplace"){
    renderConfigMarketplace(root);
    return;
  }

  if(configTab === "templates"){
    renderConfigTemplates(root);
    return;
  }

  // ── General tab ──
  if(!configData){
    loadConfig(function(){ render(); });
    var loading = div("empty");
    loading.textContent = "Loading configuration\u2026";
    root.appendChild(loading);
    return;
  }

  // Sub-tab bar: Paths | Server | Budget | Autopilot
  var subTabs = div("config-subtabs");
  [["Paths","paths"],["Server","server"],["Budget","budget"],["Autopilot","autopilot"]].forEach(function(pair){
    var tb = el("button", "config-subtab-btn" + (configSubTab === pair[1] ? " active" : ""), [pair[0]]);
    tb.onclick = function(){ configSubTab = pair[1]; configEditing = null; render(); };
    subTabs.appendChild(tb);
  });
  root.appendChild(subTabs);

  switch(configSubTab){
    case "paths":  renderConfigPaths(root); break;
    case "server": renderConfigServer(root); break;
    case "budget": renderConfigBudget(root); break;
    case "autopilot": renderConfigAutopilot(root); break;
  }
}

function renderConfigAutopilot(root){
  var c = configData;
  var editing = configEditing === "autopilot";

  // ── Spawn Settings Section ──
  var sec = div("config-section");
  var hdr = div("section-header");
  hdr.appendChild(el("h3","config-section-title",["Spawn Settings"]));
  if(!editing){
    hdr.appendChild(iconBtn("pencil","Edit",function(){ configEditing = "autopilot"; render(); }));
  }
  sec.appendChild(hdr);
  var grid = div("config-grid");
  if(editing){
    // Model select
    var modelRow = div("config-field");
    var modelLbl = el("label","config-label",["Model"]);
    modelLbl.setAttribute("for","cfg-spawn_model");
    modelRow.appendChild(modelLbl);
    var modelSel = el("select","config-input");
    modelSel.id = "cfg-spawn_model";
    [["(inherit)",""],["sonnet","sonnet"],["opus","opus"],["haiku","haiku"]].forEach(function(opt){
      var o = el("option","",[ opt[0] ]);
      o.value = opt[1];
      if((c.spawn_model||"") === opt[1]) o.selected = true;
      modelSel.appendChild(o);
    });
    modelRow.appendChild(modelSel);
    grid.appendChild(modelRow);

    // Effort select
    var effortRow = div("config-field");
    var effortLbl = el("label","config-label",["Effort"]);
    effortLbl.setAttribute("for","cfg-spawn_effort");
    effortRow.appendChild(effortLbl);
    var effortSel = el("select","config-input");
    effortSel.id = "cfg-spawn_effort";
    [["(inherit)",""],["high","high"],["medium","medium"],["low","low"]].forEach(function(opt){
      var o = el("option","",[ opt[0] ]);
      o.value = opt[1];
      if((c.spawn_effort||"") === opt[1]) o.selected = true;
      effortSel.appendChild(o);
    });
    effortRow.appendChild(effortSel);
    grid.appendChild(effortRow);

    grid.appendChild(configInput("spawn_max_turns","Max Turns", String(c.spawn_max_turns || 15)));
    grid.appendChild(configInput("max_concurrent","Max Concurrent Autopilots", String(c.max_concurrent || 3)));
  } else {
    grid.appendChild(configReadRow("Model", c.spawn_model || "(inherit)"));
    grid.appendChild(configReadRow("Effort", c.spawn_effort || "(inherit)"));
    grid.appendChild(configReadRow("Max Turns", String(c.spawn_max_turns || 15)));
    grid.appendChild(configReadRow("Max Concurrent", String(c.max_concurrent || 3)));
  }
  sec.appendChild(grid);
  if(editing){
    var acts = div("config-actions");
    var cancelBtn = el("button","btn",["Cancel"]);
    cancelBtn.onclick = function(){ configEditing = null; render(); };
    acts.appendChild(cancelBtn);
    var saveBtn = el("button","btn btn-primary",["Save"]);
    saveBtn.onclick = function(){ saveAutopilotConfig(); };
    acts.appendChild(saveBtn);
    sec.appendChild(acts);
  }
  root.appendChild(sec);

  // ── Skeleton Steps Section ──
  var skSec = div("config-section");
  skSec.appendChild(el("h3","config-section-title",["Skeleton Steps"]));
  var skDesc = el("p","config-empty");
  skDesc.textContent = "Configure which steps are included in every autopilot task plan.";
  skSec.appendChild(skDesc);

  var sk = (c.skeleton || {});
  var skToggles = [
    {key:"investigate", label:"Investigate", desc:"Read codebase, CLAUDE.md, understand patterns", locked:true},
    {key:"web_search", label:"Web Search", desc:"Search the web during investigation"},
    {key:"context7_lookup", label:"Context7 Lookup", desc:"Look up library documentation"},
    {key:"build_verify", label:"Build & Verify", desc:"Compile/build, create build system if missing"},
    {key:"test", label:"Test", desc:"Run + create tests, set up test infra if missing"},
    {key:"pre_commit", label:"Pre-commit", desc:"Run linters/formatters, set up hooks if missing"},
    {key:"commit", label:"Commit", desc:"Git commit (type(core): desc, single commit)"},
    {key:"push", label:"Push", desc:"Git push to remote (off by default)"}
  ];

  var skList = div("config-grid");
  skToggles.forEach(function(t){
    var row = div("mcp-toggle");
    var cb = el("input","");
    cb.type = "checkbox";
    cb.setAttribute("aria-label", t.label);
    if(t.locked){
      cb.checked = true;
      cb.disabled = true;
      cb.title = "Always enabled";
    } else {
      cb.checked = sk[t.key] !== false;
      cb.onchange = function(){
        var full = {
          web_search: sk.web_search !== false,
          context7_lookup: sk.context7_lookup !== false,
          build_verify: sk.build_verify !== false,
          test: sk.test !== false,
          pre_commit: sk.pre_commit !== false,
          commit: sk.commit !== false,
          push: !!sk.push
        };
        full[t.key] = cb.checked;
        api("POST","/api/config/save",{skeleton:full},function(d){
          if(d.ok){
            toast("Skeleton config saved","success");
            configData = null;
            loadConfig(function(){ render(); });
          } else {
            toast("Error: " + (d.error||"unknown"),"error");
            cb.checked = !cb.checked;
          }
        });
      };
    }
    row.appendChild(cb);
    row.appendChild(span("mcp-server-name", t.label));
    row.appendChild(span("mcp-server-cmd", t.desc));
    if(t.key === "context7_lookup" || t.key === "web_search"){
      row.appendChild(buildMarketplaceLink("MCP", "mcp"));
    }
    skList.appendChild(row);
  });
  skSec.appendChild(skList);
  root.appendChild(skSec);
}

function renderConfigMarketplace(root){
  if(!configData){
    loadConfig(function(){ render(); });
    var ld = div("empty"); ld.textContent = "Loading\u2026"; root.appendChild(ld);
    return;
  }

  var subTabs = div("config-subtabs");
  subTabs.setAttribute("role", "tablist");
  [["MCP","mcp"],["Skills","skills"]].forEach(function(pair){
    var tb = el("button", "config-subtab-btn" + (marketplaceSubTab === pair[1] ? " active" : ""), [pair[0]]);
    tb.setAttribute("role", "tab");
    tb.setAttribute("aria-selected", marketplaceSubTab === pair[1] ? "true" : "false");
    tb.onclick = function(){ marketplaceSubTab = pair[1]; render(); };
    subTabs.appendChild(tb);
  });
  root.appendChild(subTabs);

  if(marketplaceSubTab === "skills"){
    renderMarketplaceSkills(root);
  } else {
    renderMarketplaceMCP(root);
  }
}

function renderMarketplaceMCP(root){
  var mcpSec = div("config-section");
  mcpSec.appendChild(el("h3","config-section-title",["MCP Servers"]));

  if(!mcpData){
    api("GET","/api/mcp/list",null,function(d){
      mcpData = d;
      render();
    });
    var ld = div("config-empty");
    ld.textContent = "Loading MCP servers\u2026";
    mcpSec.appendChild(ld);
    root.appendChild(mcpSec);
    return;
  }

  if(mcpData.using_global){
    var info = div("config-empty");
    info.textContent = "Using global MCP servers from ~/.claude/settings.json";
    mcpSec.appendChild(info);

    if(mcpData.global){
      var gList = div("config-grid");
      Object.keys(mcpData.global).forEach(function(name){
        var s = mcpData.global[name];
        var row = div("mcp-toggle");
        row.appendChild(span("mcp-server-name", name));
        row.appendChild(span("mcp-server-cmd", s.command + " " + (s.args||[]).join(" ")));
        gList.appendChild(row);
      });
      mcpSec.appendChild(gList);
    }

    var initBtn = el("button","btn btn-primary btn-sm",["Initialize Custom Config"]);
    initBtn.style.marginTop = "12px";
    initBtn.onclick = function(){
      var restore = btnLoading(initBtn, "INITIALIZING\u2026");
      api("POST","/api/mcp/init",{},function(d){
        if(restore) restore();
        if(d.ok){
          mcpData = null;
          toast("MCP servers initialized","success");
          render();
        } else {
          toast("Error: " + (d.error||"unknown"),"error");
        }
      });
    };
    mcpSec.appendChild(initBtn);
  } else {
    var servers = mcpData.custom || {};
    var names = Object.keys(servers);
    if(names.length === 0){
      var empty = div("config-empty");
      empty.textContent = "No MCP servers configured.";
      mcpSec.appendChild(empty);
    } else {
      var list = div("config-grid");
      names.forEach(function(name){
        var s = servers[name];
        var row = div("mcp-toggle");

        var cb = el("input","");
        cb.type = "checkbox";
        cb.checked = s.enabled;
        cb.setAttribute("aria-label", "Toggle " + name);
        cb.onchange = function(){
          api("POST","/api/mcp/toggle",{name:name, enabled:cb.checked},function(d){
            if(d.ok){
              mcpData = null;
              toast("MCP configuration saved","success");
              render();
            } else {
              toast("Error: " + (d.error||"unknown"),"error");
              cb.checked = !cb.checked;
            }
          });
        };
        row.appendChild(cb);
        row.appendChild(span("mcp-server-name", name));
        row.appendChild(span("mcp-server-cmd", s.command + " " + (s.args||[]).join(" ")));
        list.appendChild(row);
      });
      mcpSec.appendChild(list);
    }
  }

  // Add Server (Catalog)
  var addBtn = el("button","btn btn-primary btn-sm",["Add Server"]);
  addBtn.style.marginTop = "12px";
  addBtn.onclick = function(){
    mcpCatalogOpen = !mcpCatalogOpen;
    mcpCatalogResults = null;
    render();
  };
  mcpSec.appendChild(addBtn);

  if(mcpCatalogOpen){
    var catBox = div("config-section");
    catBox.style.marginTop = "12px";
    catBox.style.padding = "12px";
    catBox.style.border = "1px solid var(--glass)";
    catBox.style.borderRadius = "8px";

    var searchRow = div("");
    searchRow.style.display = "flex";
    searchRow.style.gap = "8px";
    searchRow.style.marginBottom = "12px";

    var searchInput = el("input","");
    searchInput.type = "text";
    searchInput.placeholder = "Search MCP servers\u2026";
    searchInput.name = "search";
    searchInput.setAttribute("aria-label", "Search MCP servers");
    searchInput.setAttribute("autocomplete", "off");
    searchInput.value = mcpCatalogSearch;
    searchInput.style.flex = "1";
    searchInput.style.padding = "6px 10px";
    searchInput.style.background = "rgba(0,0,0,.4)";
    searchInput.style.border = "1px solid var(--glass)";
    searchInput.style.borderRadius = "6px";
    searchInput.style.color = "var(--text)";
    searchInput.style.fontSize = "13px";
    searchInput.onkeydown = function(e){
      if(e.key === "Enter"){
        mcpCatalogSearch = searchInput.value;
        searchCatalog(searchInput.value);
      }
    };
    searchRow.appendChild(searchInput);

    var searchBtn = el("button","btn btn-primary btn-sm",["Search"]);
    searchBtn.onclick = function(){
      mcpCatalogSearch = searchInput.value;
      searchCatalog(searchInput.value);
    };
    searchRow.appendChild(searchBtn);

    var closeBtn = el("button","btn btn-sm",["Close"]);
    closeBtn.onclick = function(){
      mcpCatalogOpen = false;
      mcpCatalogResults = null;
      mcpCatalogSearch = "";
      render();
    };
    searchRow.appendChild(closeBtn);
    catBox.appendChild(searchRow);

    if(mcpCatalogResults === null){
      var hint = div("config-empty");
      hint.textContent = "Search the official MCP registry to find and install servers.";
      catBox.appendChild(hint);
    } else if(mcpCatalogResults === "loading"){
      var ld2 = div("config-empty");
      ld2.textContent = "Searching\u2026";
      catBox.appendChild(ld2);
    } else if(mcpCatalogResults.length === 0){
      var noRes = div("config-empty");
      noRes.textContent = "No servers found.";
      catBox.appendChild(noRes);
    } else {
      var resList = div("config-grid");
      mcpCatalogResults.forEach(function(srv){
        var row = div("mcp-toggle");
        row.style.flexDirection = "column";
        row.style.alignItems = "flex-start";
        row.style.gap = "4px";
        row.style.padding = "8px 0";
        row.style.borderBottom = "1px solid var(--glass)";

        var topRow = div("");
        topRow.style.display = "flex";
        topRow.style.justifyContent = "space-between";
        topRow.style.width = "100%";
        topRow.style.alignItems = "center";

        var nameEl = span("mcp-server-name", srv.name);
        nameEl.style.fontWeight = "600";
        topRow.appendChild(nameEl);

        if(srv.installed){
          var badge = span("","Installed");
          badge.style.color = "var(--success)";
          badge.style.fontSize = "11px";
          badge.style.fontWeight = "600";
          topRow.appendChild(badge);
        } else {
          var installBtn = el("button","btn btn-success btn-sm",["Install"]);
          installBtn.dataset.pkg = srv.package;
          installBtn.dataset.name = srv.name;
          installBtn.onclick = (function(s, btn){
            return function(){
              if(s.env_vars && s.env_vars.length > 0){
                var hasRequired = s.env_vars.some(function(v){ return v.is_required; });
                if(hasRequired){
                  showEnvVarsPrompt(s, btn);
                  return;
                }
              }
              doInstall(s.name, s.package, {}, btn);
            };
          })(srv, installBtn);
          topRow.appendChild(installBtn);
        }
        row.appendChild(topRow);

        if(srv.description){
          var desc = span("mcp-server-cmd", srv.description);
          desc.style.fontSize = "12px";
          desc.style.opacity = "0.8";
          row.appendChild(desc);
        }

        var pkgEl = span("mcp-server-cmd", srv.package);
        pkgEl.style.fontSize = "11px";
        pkgEl.style.opacity = "0.6";
        row.appendChild(pkgEl);

        if(srv.env_vars && srv.env_vars.length > 0){
          var envHint = span("","Env: " + srv.env_vars.map(function(v){ return v.name + (v.is_required ? "*" : ""); }).join(", "));
          envHint.style.fontSize = "11px";
          envHint.style.opacity = "0.5";
          envHint.style.fontStyle = "italic";
          row.appendChild(envHint);
        }

        resList.appendChild(row);
      });
      catBox.appendChild(resList);
    }
    mcpSec.appendChild(catBox);
  }

  root.appendChild(mcpSec);
}

function renderMarketplaceSkills(root){
  // Installed Skills section
  var sec = div("config-section");
  sec.appendChild(el("h3","config-section-title",["Installed Skills"]));

  if(!skillsData){
    api("GET","/api/skills/list",null,function(d){
      skillsData = d.skills || [];
      render();
    });
    var ld = div("config-empty");
    ld.textContent = "Loading installed skills\u2026";
    sec.appendChild(ld);
    root.appendChild(sec);
    return;
  }

  if(skillsData.length === 0){
    var empty = div("config-empty");
    empty.textContent = "No skills installed yet.";
    sec.appendChild(empty);
  } else {
    var list = div("config-grid");
    skillsData.forEach(function(sk){
      var row = div("mcp-toggle");
      row.appendChild(span("mcp-server-name", sk.name));
      row.appendChild(span("mcp-server-cmd", sk.description || sk.path));
      list.appendChild(row);
    });
    sec.appendChild(list);
  }

  // Add Skill (Catalog)
  var addBtn = el("button","btn btn-primary btn-sm",["Add Skill"]);
  addBtn.style.marginTop = "12px";
  addBtn.onclick = function(){
    skillsCatalogOpen = !skillsCatalogOpen;
    skillsCatalogResults = null;
    render();
  };
  sec.appendChild(addBtn);

  if(skillsCatalogOpen){
    var catBox = div("config-section");
    catBox.style.marginTop = "12px";
    catBox.style.padding = "12px";
    catBox.style.border = "1px solid var(--glass)";
    catBox.style.borderRadius = "8px";

    var searchRow = div("");
    searchRow.style.display = "flex";
    searchRow.style.gap = "8px";
    searchRow.style.marginBottom = "12px";

    var searchInput = el("input","");
    searchInput.type = "text";
    searchInput.placeholder = "Search skills\u2026";
    searchInput.name = "search";
    searchInput.setAttribute("aria-label", "Search skills");
    searchInput.setAttribute("autocomplete", "off");
    searchInput.value = skillsCatalogSearch;
    searchInput.style.flex = "1";
    searchInput.style.padding = "6px 10px";
    searchInput.style.background = "rgba(0,0,0,.4)";
    searchInput.style.border = "1px solid var(--glass)";
    searchInput.style.borderRadius = "6px";
    searchInput.style.color = "var(--text)";
    searchInput.style.fontSize = "13px";
    searchInput.onkeydown = function(e){
      if(e.key === "Enter"){
        skillsCatalogSearch = searchInput.value;
        searchSkillsCatalog(searchInput.value);
      }
    };
    searchRow.appendChild(searchInput);

    var searchBtn = el("button","btn btn-primary btn-sm",["Search"]);
    searchBtn.onclick = function(){
      skillsCatalogSearch = searchInput.value;
      searchSkillsCatalog(searchInput.value);
    };
    searchRow.appendChild(searchBtn);

    var closeBtn = el("button","btn btn-sm",["Close"]);
    closeBtn.onclick = function(){
      skillsCatalogOpen = false;
      skillsCatalogResults = null;
      skillsCatalogSearch = "";
      render();
    };
    searchRow.appendChild(closeBtn);
    catBox.appendChild(searchRow);

    if(skillsCatalogResults === null){
      var hint = div("config-empty");
      hint.textContent = "Search skills.sh to find and install Claude Code skills.";
      catBox.appendChild(hint);
    } else if(skillsCatalogResults === "loading"){
      var ld2 = div("config-empty");
      ld2.textContent = "Searching\u2026";
      catBox.appendChild(ld2);
    } else if(skillsCatalogResults.length === 0){
      var noRes = div("config-empty");
      noRes.textContent = "No skills found.";
      catBox.appendChild(noRes);
    } else {
      var resList = div("config-grid");
      skillsCatalogResults.forEach(function(sk){
        var row = div("mcp-toggle");
        row.style.flexDirection = "column";
        row.style.alignItems = "flex-start";
        row.style.gap = "4px";
        row.style.padding = "8px 0";
        row.style.borderBottom = "1px solid var(--glass)";

        var topRow = div("");
        topRow.style.display = "flex";
        topRow.style.justifyContent = "space-between";
        topRow.style.width = "100%";
        topRow.style.alignItems = "center";

        var nameEl = span("mcp-server-name", sk.name);
        nameEl.style.fontWeight = "600";
        topRow.appendChild(nameEl);

        if(sk.installed){
          var badge = span("","Installed");
          badge.style.color = "var(--success)";
          badge.style.fontSize = "11px";
          badge.style.fontWeight = "600";
          topRow.appendChild(badge);
        } else {
          var installBtn = el("button","btn btn-success btn-sm",["Install"]);
          installBtn.onclick = (function(skill, btn){
            return function(){
              var restore = btnLoading(btn, "INSTALLING\u2026");
              api("POST","/api/skills/install",{id: skill.source},function(d){
                if(restore) restore();
                if(d.ok){
                  skillsData = null;
                  skillsCatalogResults = null;
                  skillsCatalogOpen = false;
                  skillsCatalogSearch = "";
                  toast("Installed " + skill.name, "success");
                  render();
                } else {
                  toast("Error: " + (d.error || "unknown"), "error");
                }
              });
            };
          })(sk, installBtn);
          topRow.appendChild(installBtn);
        }
        row.appendChild(topRow);

        var metaRow = div("");
        metaRow.style.display = "flex";
        metaRow.style.gap = "8px";
        metaRow.style.alignItems = "center";
        if(sk.source){
          metaRow.appendChild(span("skill-source-badge", sk.source));
        }
        if(sk.installs > 0){
          var instCount = span("mcp-server-cmd", new Intl.NumberFormat().format(sk.installs) + " installs");
          instCount.style.fontSize = "11px";
          instCount.style.opacity = "0.6";
          metaRow.appendChild(instCount);
        }
        if(metaRow.childNodes.length > 0) row.appendChild(metaRow);

        resList.appendChild(row);
      });
      catBox.appendChild(resList);
    }
    sec.appendChild(catBox);
  }

  root.appendChild(sec);
}

function searchSkillsCatalog(query){
  skillsCatalogResults = "loading";
  render();
  var url = "/api/skills/catalog?limit=20";
  if(query) url += "&search=" + encodeURIComponent(query);
  api("GET", url, null, function(d){
    skillsCatalogResults = d.skills || [];
    render();
  });
}

function buildMarketplaceLink(label, subTab){
  var btn = el("button","skill-link-btn",[label]);
  btn.setAttribute("aria-label", "Go to Marketplace " + label);
  btn.onclick = function(e){
    e.stopPropagation();
    configTab = "marketplace";
    marketplaceSubTab = subTab;
    location.hash = "config";
    render();
  };
  return btn;
}

function saveAutopilotConfig(){
  var c = {};
  Object.keys(configData).forEach(function(k){ c[k] = configData[k]; });
  c.web_enabled = true;
  c.spawn_model = document.getElementById("cfg-spawn_model").value;
  c.spawn_effort = document.getElementById("cfg-spawn_effort").value;
  c.spawn_max_turns = parseInt(document.getElementById("cfg-spawn_max_turns").value) || 25;
  c.max_concurrent = parseInt(document.getElementById("cfg-max_concurrent").value) || 3;
  var saveBtn = document.querySelector(".config-actions .btn-primary");
  var restore = btnLoading(saveBtn, "SAVING\u2026");
  api("POST","/api/config/save",c,function(){
    if(restore) restore();
    configEditing = null;
    toast("Autopilot configuration saved","success");
    configData = null;
    loadConfig(function(){ render(); });
  });
}

function renderConfigAbout(root){
  var sec = div("config-section");
  sec.appendChild(el("h3","config-section-title",["About"]));
  var grid = div("config-grid");
  grid.appendChild(configReadRow("Version", "v" + (D.version||"?") + " #" + (D.build_num||"0")));
  grid.appendChild(configReadRow("Plan Model", D.plan_model));
  grid.appendChild(configReadRow("Exec Model", D.exec_model));
  grid.appendChild(configReadRow("Effort", D.effort));
  sec.appendChild(grid);
  root.appendChild(sec);

  var updSec = div("config-section");
  updSec.appendChild(el("h3","config-section-title",["Updates"]));
  var updContent = div("config-update-area");
  renderUpdateArea(updContent);
  updSec.appendChild(updContent);
  root.appendChild(updSec);
}

function renderUpdateArea(container){
  var checkBtn = el("button","btn btn-primary btn-sm",["Check for Updates"]);
  var statusEl = div("update-status");
  var channelEl = div("update-channel");
  var actionEl = div("update-action");
  var progressEl = div("update-progress");

  if(updateRunning) checkBtn.disabled = true;

  checkBtn.onclick = function(){
    if(updateRunning) return;
    updateCheckResult = null;
    statusEl.textContent = "";
    channelEl.textContent = "";
    actionEl.textContent = "";
    progressEl.textContent = "";
    var restore = btnLoading(checkBtn, "Checking\u2026");
    api("GET", "/api/update/check", null, function(data){
      if(restore) restore();
      updateCheckResult = data;
      if(data.error){
        statusEl.textContent = "Error: " + data.error;
        statusEl.className = "update-status error";
        return;
      }
      // Show current version
      statusEl.textContent = "Current: " + data.current_version;
      statusEl.className = "update-status";

      // Channel selector
      var label = span("","Channel: ");
      var sel = document.createElement("select");
      sel.className = "update-channel-select";
      var optMain = document.createElement("option");
      optMain.value = "main";
      optMain.textContent = "main (latest)";
      sel.appendChild(optMain);
      var tags = data.tags || [];
      for(var i = 0; i < tags.length; i++){
        var opt = document.createElement("option");
        opt.value = tags[i];
        opt.textContent = tags[i];
        if(tags[i] === data.current_tag) opt.textContent += " (current)";
        sel.appendChild(opt);
      }
      // Pre-select: if on a tag, select that tag; else select main
      if(data.current_tag) sel.value = data.current_tag;
      else sel.value = "main";
      channelEl.appendChild(label);
      channelEl.appendChild(sel);

      // Info line based on selection
      var infoEl = div("update-info");
      function updateInfo(){
        var v = sel.value;
        if(v === "main"){
          var b = parseInt(data.behind) || 0;
          if(b > 0){
            infoEl.textContent = data.behind + " commits behind main (" + data.remote_commit + ")";
            infoEl.className = "update-info available";
          } else {
            infoEl.textContent = "Already on latest main";
            infoEl.className = "update-info current";
          }
        } else if(v === data.current_tag){
          infoEl.textContent = "Already on " + v;
          infoEl.className = "update-info current";
        } else {
          infoEl.textContent = "Switch to " + v;
          infoEl.className = "update-info available";
        }
      }
      sel.onchange = updateInfo;
      updateInfo();
      channelEl.appendChild(infoEl);

      // Update button
      var updateBtn = el("button","btn btn-success btn-sm",["Update Now"]);
      updateBtn.onclick = function(){ runUpdate(progressEl, updateBtn, checkBtn, sel.value); };
      actionEl.appendChild(updateBtn);
    });
  };

  container.appendChild(checkBtn);
  container.appendChild(statusEl);
  container.appendChild(channelEl);
  container.appendChild(actionEl);
  container.appendChild(progressEl);
}

function runUpdate(progressEl, updateBtn, checkBtn, target){
  if(updateRunning) return;
  updateRunning = true;
  progressEl.textContent = "";
  if(updateBtn) updateBtn.disabled = true;
  if(checkBtn) checkBtn.disabled = true;
  var url = "/api/update" + (target ? "?target=" + encodeURIComponent(target) : "");

  streamStepStandalone(url, null, progressEl, function(p, evt){
    if(evt.type !== "step") return;
    var cls = "setup-progress-item";
    var icon = "\u2022";
    if(evt.status === "done"){ cls += " ok"; icon = "\u2713"; }
    else if(evt.status === "error"){ cls += " error"; icon = "\u2717"; }
    else if(evt.status === "restarting"){ cls += " running"; }
    var item = div(cls);
    item.appendChild(span("icon", icon));
    item.appendChild(span("label", evt.name + ": " + evt.message));
    p.appendChild(item);
  }, function(status, msg){
    updateRunning = false;
    if(status === "success"){
      progressEl.appendChild(div("setup-status ok",["Update complete. Restarting..."]));
      showReconnecting(progressEl);
    } else {
      progressEl.appendChild(div("setup-status err",[msg || "Update failed"]));
      if(checkBtn) checkBtn.disabled = false;
    }
  });
}

function showReconnecting(container){
  var msg = div("update-reconnecting");
  msg.textContent = "Waiting for service to restart...";
  container.appendChild(msg);
  function poll(){
    fetch("/api/data").then(function(r){
      if(r.ok) window.location.reload();
      else setTimeout(poll, 2000);
    }).catch(function(){ setTimeout(poll, 2000); });
  }
  setTimeout(poll, 3000);
}

/* ── streamStepStandalone — like streamStep but no global setupRunning ── */
function streamStepStandalone(url, body, progressEl, renderEvent, onDone){
  var opts = {method:"POST"};
  if(body){ opts.headers = {"Content-Type":"application/json"}; opts.body = JSON.stringify(body); }
  fetch(url, opts).then(function(res){
    if(!res.ok){ res.text().then(function(t){ onDone("error", t); }); return; }
    var reader = res.body.getReader();
    var decoder = new TextDecoder();
    var buffer = "";
    function pump(){
      reader.read().then(function(result){
        if(result.done){ onDone("error","Stream ended unexpectedly"); return; }
        buffer += decoder.decode(result.value, {stream:true});
        var lines = buffer.split("\n");
        buffer = lines.pop();
        for(var i=0;i<lines.length;i++){
          var line = lines[i].trim();
          if(line.indexOf("data: ")===0){
            try{
              var evt = JSON.parse(line.substring(6));
              if(evt.done){ onDone(evt.status, evt.message); return; }
              renderEvent(progressEl, evt);
            }catch(e){}
          }
        }
        pump();
      });
    }
    pump();
  }).catch(function(e){ onDone("error", e.message || "Connection error"); });
}

/* ── Config > Setup Tab ── */
var CONFIG_SETUP_TABS = [
  {id:"prereqs", label:"Prerequisites"},
  {id:"config", label:"Configuration"},
  {id:"skills", label:"Skills"},
  {id:"bmad", label:"BMAD"},
  {id:"hooks", label:"Hooks"},
  {id:"mcp", label:"MCP Servers"}
];

function renderConfigSetup(root){
  var subTabs = div("config-subtabs");
  CONFIG_SETUP_TABS.forEach(function(tab){
    var tb = el("button", "config-subtab-btn" + (configSetupSubTab === tab.id ? " active" : ""), [tab.label]);
    tb.onclick = function(){ configSetupSubTab = tab.id; render(); };
    subTabs.appendChild(tb);
  });
  root.appendChild(subTabs);

  var sec = div("config-section");
  switch(configSetupSubTab){
    case "prereqs": renderCfgSetupPrereqs(sec); break;
    case "config":  renderCfgSetupConfig(sec);  break;
    case "skills":  renderCfgSetupSkills(sec);  break;
    case "bmad":    renderCfgSetupBMAD(sec);    break;
    case "hooks":   renderCfgSetupHooks(sec);   break;
    case "mcp":     renderCfgSetupMCP(sec);     break;
  }
  root.appendChild(sec);
}

function renderCfgSetupPrereqs(container){
  container.appendChild(el("h3","config-section-title",["Prerequisites"]));
  container.appendChild(el("p","config-section-desc",["Check and install required development tools."]));
  var prog = div("setup-progress");
  var actions = div("setup-prereqs-actions");
  var localMissing = [];
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",["Check Prerequisites"]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    actions.textContent = "";
    localMissing = [];
    var restore = btnLoading(btn, "Checking\u2026");
    streamStepStandalone("/api/onboarding/prereqs", null, prog, function(p, evt){
      if(evt.type !== "tool") return;
      var ok = evt.found;
      var cls = "setup-progress-item " + (ok ? "ok" : (evt.required ? "error" : "skip"));
      var item = div(cls);
      item.appendChild(span("icon", ok ? "\u2713" : (evt.required ? "\u2717" : "~")));
      item.appendChild(span("label", evt.name));
      item.appendChild(span("version", evt.version || (ok ? "OK" : (evt.required ? "not found" : "optional"))));
      if(!ok && evt.installable) item.appendChild(span("tag-installable","installable"));
      p.appendChild(item);
      if(!ok) localMissing.push(evt);
    }, function(status, msg){
      running = false;
      if(restore) restore();
      if(status === "success"){
        prog.appendChild(div("setup-status ok",["All required tools found"]));
      } else {
        var installable = localMissing.filter(function(m){ return m.installable; });
        if(installable.length > 0){
          var installBtn = el("button","btn btn-success btn-sm",["Install Missing (" + installable.length + ")"]);
          installBtn.onclick = function(){
            actions.textContent = "";
            var installProg = div("setup-progress");
            installProg.style.marginTop = "16px";
            container.appendChild(installProg);
            streamStepStandalone("/api/onboarding/prereqs/install", null, installProg, function(p2, evt2){
              if(evt2.type !== "install") return;
              var cls2 = "setup-progress-item " + (evt2.status === "done" ? "ok" : evt2.status === "error" ? "error" : "running");
              var item2 = div(cls2);
              item2.appendChild(span("icon", evt2.status === "done" ? "\u2713" : evt2.status === "error" ? "\u2717" : "\u2022"));
              item2.appendChild(span("label", evt2.name));
              p2.appendChild(item2);
            }, function(s2){
              if(s2 === "success") installProg.appendChild(div("setup-status ok",["Tools installed"]));
              else installProg.appendChild(div("setup-status err",["Install failed"]));
            });
          };
          actions.appendChild(installBtn);
        }
        prog.appendChild(div("setup-status err",[msg || "Missing required tools"]));
      }
    });
  };
  container.appendChild(btn);
  container.appendChild(actions);
  container.appendChild(prog);
}

function renderCfgSetupConfig(container){
  container.appendChild(el("h3","config-section-title",["Configuration"]));
  container.appendChild(el("p","config-section-desc",["Set up projects directory, web port, and authentication."]));
  var running = false;

  var grid = div("config-grid");
  var projInput = document.createElement("input"); projInput.type = "text"; projInput.className = "config-input"; projInput.value = (configData && configData.projects_dir) || "~/Projects"; projInput.placeholder = "~/Projects";
  var portInput = document.createElement("input"); portInput.type = "number"; portInput.className = "config-input"; portInput.value = (configData && configData.web_port) || 7777;
  var hostSelect = document.createElement("select"); hostSelect.className = "config-input";
  ["localhost","0.0.0.0"].forEach(function(v){ var o = document.createElement("option"); o.value = v; o.textContent = v; if(configData && configData.web_host === v) o.selected = true; hostSelect.appendChild(o); });
  if(!configData || !configData.web_host) hostSelect.value = "localhost";
  var pwInput = document.createElement("input"); pwInput.type = "password"; pwInput.className = "config-input"; pwInput.placeholder = "leave blank for no auth";
  var maxInput = document.createElement("input"); maxInput.type = "number"; maxInput.className = "config-input"; maxInput.value = (configData && configData.max_concurrent) || 3;

  grid.appendChild(configFieldRow("Projects Directory", projInput));
  grid.appendChild(configFieldRow("Web Port", portInput));
  grid.appendChild(configFieldRow("Bind Address", hostSelect));
  grid.appendChild(configFieldRow("Web Password", pwInput));
  grid.appendChild(configFieldRow("Max Concurrent", maxInput));
  container.appendChild(grid);

  var statusEl = div("setup-progress");
  var btn = el("button","btn btn-primary btn-sm",["Save Configuration"]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    var restore = btnLoading(btn, "Saving\u2026");
    var body = {projects_dir: projInput.value, web_port: parseInt(portInput.value)||7777, web_host: hostSelect.value || "localhost", web_password: pwInput.value, max_concurrent: parseInt(maxInput.value)||3};
    api("POST", "/api/onboarding/config", body, function(data){
      running = false;
      if(restore) restore();
      if(data && data.ok){
        statusEl.textContent = "";
        statusEl.appendChild(div("setup-status ok",["Configuration saved"]));
        toast("Configuration saved","success");
      } else {
        statusEl.appendChild(div("setup-status err",[(data && data.error) || "Save failed"]));
      }
    });
  };
  container.appendChild(btn);
  container.appendChild(statusEl);

  if(!configData) loadConfig(function(){ render(); });
}

function configFieldRow(label, input){
  var row = div("config-field");
  row.appendChild(el("label","config-label",[label]));
  row.appendChild(input);
  return row;
}

function renderCfgSetupSkills(container){
  container.appendChild(el("h3","config-section-title",["Skills"]));
  container.appendChild(el("p","config-section-desc",["Install Claude Code skills for enhanced capabilities."]));
  var prog = div("setup-progress");
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",["Install Skills"]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    var restore = btnLoading(btn, "Installing\u2026");
    streamStepStandalone("/api/onboarding/skills", null, prog, function(p, evt){
      if(evt.type === "symlink"){ p.appendChild(div("setup-progress-item ok",[span("icon","\u2713"), span("label","Symlink: ~/.agents/skills")])); return; }
      var cls = "setup-progress-item " + (evt.status === "done" ? "ok" : evt.status === "error" ? "error" : "skip");
      var item = div(cls);
      item.appendChild(span("icon", evt.status === "done" ? "\u2713" : evt.status === "error" ? "\u2717" : "~"));
      item.appendChild(span("label", evt.name || "skill"));
      if(evt.status === "skipped") item.appendChild(span("version","already installed"));
      p.appendChild(item);
    }, function(status){
      running = false;
      if(restore) restore();
      if(status === "success") prog.appendChild(div("setup-status ok",["Skills installed"]));
      else prog.appendChild(div("setup-status err",["Install failed"]));
    });
  };
  container.appendChild(btn);
  container.appendChild(prog);
}

function renderCfgSetupBMAD(container){
  container.appendChild(el("h3","config-section-title",["BMAD Commands"]));
  container.appendChild(el("p","config-section-desc",["Install BMAD commands for project management workflows."]));
  var prog = div("setup-progress");
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",["Install BMAD"]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    var restore = btnLoading(btn, "Installing\u2026");
    var progressBar = null;
    streamStepStandalone("/api/onboarding/bmad", null, prog, function(p, evt){
      if(evt.type === "symlink"){ p.appendChild(div("setup-progress-item ok",[span("icon","\u2713"), span("label","Symlink: ~/.claude/commands/bmad")])); return; }
      if(evt.type === "progress"){
        if(!progressBar){
          var wrap = div("setup-progress-bar-wrap");
          progressBar = div("setup-progress-bar-fill");
          progressBar.style.width = "0%";
          wrap.appendChild(progressBar);
          var counter = div("setup-progress-counter");
          counter.id = "bmad-counter";
          p.appendChild(wrap);
          p.appendChild(counter);
        }
        var pct = evt.total > 0 ? Math.round(evt.count/evt.total*100) : 0;
        progressBar.style.width = pct + "%";
        var c = document.getElementById("bmad-counter");
        if(c) c.textContent = evt.count + " / " + evt.total + " files";
      }
    }, function(status){
      running = false;
      if(restore) restore();
      if(status === "success") prog.appendChild(div("setup-status ok",["BMAD installed"]));
      else prog.appendChild(div("setup-status err",["Install failed"]));
    });
  };
  container.appendChild(btn);
  container.appendChild(prog);
}

function renderCfgSetupHooks(container){
  container.appendChild(el("h3","config-section-title",["Security Hooks"]));
  container.appendChild(el("p","config-section-desc",["Install security hooks to prevent destructive operations."]));
  var prog = div("setup-progress");
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",["Install Hooks"]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    var restore = btnLoading(btn, "Installing\u2026");
    streamStepStandalone("/api/onboarding/hooks", null, prog, function(p, evt){
      var item = div("setup-progress-item ok");
      item.appendChild(span("icon", "\u2713"));
      item.appendChild(span("label", evt.name || evt.type || "hook"));
      p.appendChild(item);
    }, function(status){
      running = false;
      if(restore) restore();
      if(status === "success") prog.appendChild(div("setup-status ok",["Hooks installed"]));
      else prog.appendChild(div("setup-status err",["Install failed"]));
    });
  };
  container.appendChild(btn);
  container.appendChild(prog);
}

function renderCfgSetupMCP(container){
  container.appendChild(el("h3","config-section-title",["MCP Servers"]));
  container.appendChild(el("p","config-section-desc",["Install MCP servers for documentation, memory, and reasoning."]));
  var prog = div("setup-progress");
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",["Install MCP Servers"]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    var restore = btnLoading(btn, "Installing\u2026");
    streamStepStandalone("/api/onboarding/mcp", null, prog, function(p, evt){
      var ok = evt.status === "done";
      var skipped = evt.status === "skipped";
      var cls = "setup-progress-item " + (ok ? "ok" : skipped ? "skip" : "error");
      var item = div(cls);
      item.appendChild(span("icon", ok ? "\u2713" : skipped ? "~" : "\u2717"));
      item.appendChild(span("label", evt.name || "server"));
      if(skipped) item.appendChild(span("version","already configured"));
      p.appendChild(item);
    }, function(status){
      running = false;
      if(restore) restore();
      if(status === "success") prog.appendChild(div("setup-status ok",["MCP servers installed"]));
      else prog.appendChild(div("setup-status err",["Install failed"]));
    });
  };
  container.appendChild(btn);
  container.appendChild(prog);
}

function renderConfigPaths(root){
  var c = configData;
  var editing = configEditing === "paths";
  var sec = div("config-section");
  var hdr = div("section-header");
  hdr.appendChild(el("h3","config-section-title",["Paths & Refresh"]));
  if(!editing){
    hdr.appendChild(iconBtn("pencil","Edit",function(){ configEditing = "paths"; render(); }));
  }
  sec.appendChild(hdr);
  var grid = div("config-grid");
  if(editing){
    grid.appendChild(configInput("projects_dir","Projects Directory", c.projects_dir || ""));
    grid.appendChild(configInput("claude_dir","Claude Directory", c.claude_dir || ""));
    grid.appendChild(configInput("refresh_interval_sec","Refresh Interval (sec)", String(c.refresh_interval_sec || 30)));
  } else {
    grid.appendChild(configReadRow("Projects Directory", c.projects_dir));
    grid.appendChild(configReadRow("Claude Directory", c.claude_dir));
    grid.appendChild(configReadRow("Refresh Interval", (c.refresh_interval_sec || 30) + "s"));
  }
  sec.appendChild(grid);
  if(editing) sec.appendChild(configEditActions("paths"));
  root.appendChild(sec);
}

function renderConfigServer(root){
  var c = configData;
  var editing = configEditing === "server";
  var sec = div("config-section");
  var hdr = div("section-header");
  hdr.appendChild(el("h3","config-section-title",["Web Server"]));
  if(!editing){
    hdr.appendChild(iconBtn("pencil","Edit",function(){ configEditing = "server"; render(); }));
  }
  sec.appendChild(hdr);
  var grid = div("config-grid");
  if(editing){
    grid.appendChild(configInput("web_port","Port", String(c.web_port || 7777)));
    var hostField = div("config-field");
    hostField.appendChild(el("label","config-label",["Bind Address"]));
    var hostSel = document.createElement("select"); hostSel.className = "config-input"; hostSel.id = "cfg-web_host";
    ["localhost","0.0.0.0"].forEach(function(v){ var o = document.createElement("option"); o.value = v; o.textContent = v; if(c.web_host === v) o.selected = true; hostSel.appendChild(o); });
    hostField.appendChild(hostSel);
    grid.appendChild(hostField);
    grid.appendChild(configInput("web_password","Password", c.web_password || "", "password"));
  } else {
    grid.appendChild(configReadRow("Port", String(c.web_port || 7777)));
    grid.appendChild(configReadRow("Bind Address", c.web_host || "localhost"));
    grid.appendChild(configReadRow("Password", c.web_password, true));
  }
  sec.appendChild(grid);
  if(editing) sec.appendChild(configEditActions("server"));
  root.appendChild(sec);
}

function renderConfigBudget(root){
  var c = configData;
  var editing = configEditing === "budget";
  var sec = div("config-section");
  var hdr = div("section-header");
  hdr.appendChild(el("h3","config-section-title",["Budget & Limits"]));
  if(!editing){
    hdr.appendChild(iconBtn("pencil","Edit",function(){ configEditing = "budget"; render(); }));
  }
  sec.appendChild(hdr);
  var grid = div("config-grid");
  if(editing){
    grid.appendChild(configInput("budget_monthly","Monthly Budget ($)", String(c.budget_monthly || 0)));
    grid.appendChild(configInput("context_limit","Context Limit (tokens)", String(c.context_limit || 0)));
  } else {
    grid.appendChild(configReadRow("Monthly Budget", "$" + (c.budget_monthly || 0)));
    grid.appendChild(configReadRow("Context Limit", (c.context_limit || 0) + " tokens"));
  }
  sec.appendChild(grid);
  if(editing) sec.appendChild(configEditActions("budget"));
  root.appendChild(sec);
}

function configEditActions(section){
  var acts = div("config-actions");
  var cancelBtn = el("button","btn",["Cancel"]);
  cancelBtn.onclick = function(){ configEditing = null; render(); };
  acts.appendChild(cancelBtn);
  var saveBtn = el("button","btn btn-primary",["Save"]);
  saveBtn.onclick = function(){ saveConfigSection(section); };
  acts.appendChild(saveBtn);
  return acts;
}

function renderConfigTemplates(root){
  if(!templatesCache.length && !templatesLoading){
    templatesLoading = true;
    api("GET","/api/templates/list",null,function(d){
      templatesLoading = false;
      if(d.templates) templatesCache = d.templates;
      render();
    });
    var loadingEl = div("config-section");
    loadingEl.textContent = "Loading templates\u2026";
    root.appendChild(loadingEl);
    return;
  }

  // Section header with add button
  var listSec = div("config-section");
  var header = div("section-header");
  header.appendChild(el("h3","config-section-title",["Saved Templates (" + templatesCache.length + ")"]));
  var addBtn = el("button", "btn btn-primary btn-sm", ["+ Add Template"]);
  addBtn.onclick = function(){ openTemplateModal(null); };
  header.appendChild(addBtn);
  listSec.appendChild(header);

  if(templatesCache.length === 0){
    var empty = div("config-empty");
    empty.textContent = "No templates yet. Click + Add Template to create one.";
    listSec.appendChild(empty);
  } else {
    templatesCache.forEach(function(tmpl){
      var row = div("tmpl-row");
      var info = div("tmpl-row-info");
      info.appendChild(span("tmpl-row-name", tmpl.name));
      info.appendChild(span("tmpl-row-preview", tmpl.content.length > 80 ? tmpl.content.substring(0,80) + "..." : tmpl.content));
      row.appendChild(info);

      var acts = div("tmpl-row-actions");
      acts.appendChild(iconBtn("pencil", "Edit template", function(){ openTemplateModal(tmpl); }));
      acts.appendChild(iconBtn("trash", "Delete template", function(){
        api("POST","/api/templates/delete",{id:tmpl.id},function(d){
          if(d.ok){
            templatesCache = templatesCache.filter(function(t){ return t.id !== tmpl.id; });
            toast("Template deleted","success");
            render();
          }
        });
      }));
      row.appendChild(acts);
      listSec.appendChild(row);
    });
  }
  root.appendChild(listSec);
}

function openTemplateModal(tmpl){
  cfgEditingTemplate = tmpl;
  document.getElementById("tmpl-modal-title").textContent = tmpl ? "Edit Template" : "New Template";
  document.getElementById("cfg-tmpl-name").value = tmpl ? tmpl.name : "";
  document.getElementById("cfg-tmpl-content").value = tmpl ? tmpl.content : "";
  document.getElementById("cfg-tmpl-submit").textContent = tmpl ? "Update Template" : "Add Template";
  openModal("modal-template");
  setTimeout(function(){ document.getElementById("cfg-tmpl-name").focus(); }, 100);
}
window.openTemplateModal = openTemplateModal;

function submitConfigTemplate(){
  var name = document.getElementById("cfg-tmpl-name").value.trim();
  var content = document.getElementById("cfg-tmpl-content").value.trim();
  if(!name || !content){ toast("Name and content required","error"); return; }
  var btn = document.getElementById("cfg-tmpl-submit");
  var restore = btnLoading(btn, cfgEditingTemplate ? "SAVING\u2026" : "ADDING\u2026");
  if(cfgEditingTemplate){
    api("POST","/api/templates/update",{id:cfgEditingTemplate.id, name:name, content:content},function(d){
      if(restore) restore();
      if(d.error){ toast("Error: "+d.error,"error"); return; }
      cfgEditingTemplate = null;
      loadTemplates();
      closeModal("modal-template");
      toast("Template updated","success");
    });
  } else {
    api("POST","/api/templates/add",{name:name, content:content},function(d){
      if(restore) restore();
      if(d.error){ toast("Error: "+d.error,"error"); return; }
      loadTemplates();
      closeModal("modal-template");
      toast("Template added","success");
    });
  }
}
window.submitConfigTemplate = submitConfigTemplate;

function configRow(label, value){
  var row = div("config-row");
  row.appendChild(span("config-label", label));
  row.appendChild(span("config-value", value));
  return row;
}

function configInput(key, label, value, type){
  var row = div("config-field");
  var lbl = el("label","config-label",[label]);
  lbl.setAttribute("for","cfg-"+key);
  row.appendChild(lbl);
  var inp = el("input","config-input");
  inp.type = type || "text";
  inp.id = "cfg-" + key;
  inp.value = value;
  row.appendChild(inp);
  return row;
}

function configReadRow(label, value, masked){
  var row = div("config-field");
  row.appendChild(span("config-label", label));
  var cls = "config-value-text";
  if(masked) cls += " masked";
  row.appendChild(span(cls, masked ? "\u2022\u2022\u2022\u2022\u2022\u2022" : (value || "\u2014")));
  return row;
}

function saveConfigSection(section){
  var c = {};
  Object.keys(configData).forEach(function(k){ c[k] = configData[k]; });
  c.web_enabled = true;
  if(section === "paths"){
    c.projects_dir = document.getElementById("cfg-projects_dir").value;
    c.claude_dir = document.getElementById("cfg-claude_dir").value;
    c.refresh_interval_sec = parseInt(document.getElementById("cfg-refresh_interval_sec").value) || 30;
  } else if(section === "server"){
    c.web_port = parseInt(document.getElementById("cfg-web_port").value) || 7777;
    c.web_host = document.getElementById("cfg-web_host").value || "localhost";
    c.web_password = document.getElementById("cfg-web_password").value;
  } else if(section === "budget"){
    c.budget_monthly = parseFloat(document.getElementById("cfg-budget_monthly").value) || 0;
    c.context_limit = parseInt(document.getElementById("cfg-context_limit").value) || 0;
  }
  var saveBtn = document.querySelector(".config-actions .btn-primary");
  var restore = btnLoading(saveBtn, "SAVING\u2026");
  api("POST","/api/config/save",c,function(){
    if(restore) restore();
    configEditing = null;
    toast("Configuration saved","success");
    configData = null;
    loadConfig(function(){ render(); });
  });
}

/* ── Deferred setup init (after SETUP_STEPS defined) ── */
if(getView() === "setup") render();

})();
