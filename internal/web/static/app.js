/* ── Theme Init (before any rendering) ── */
(function initTheme() {
  var saved = localStorage.getItem("teamoon-theme");
  if (saved) {
    document.documentElement.setAttribute("data-theme", saved);
  } else if (window.matchMedia("(prefers-color-scheme: light)").matches) {
    document.documentElement.setAttribute("data-theme", "light");
  }
  var meta = document.querySelector('meta[name="theme-color"]');
  if (meta) {
    var theme = document.documentElement.getAttribute("data-theme");
    meta.content = theme === "light" ? "#f5f5f5" : "#050505";
  }
})();

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
  if(meta.num_turns>0) parts.push(t("common.turns",{count:meta.num_turns,plural:meta.num_turns!==1?"s":""}));
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
var logFilterDateFrom = "";
var logFilterDateTo = "";
var logFilterDatePreset = "";
var prevView = "";
var isDataUpdate = false;
var taskLogAutoScroll = true;
var templatesCache = null;
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
var chatSystemMode = false;
var chatCounter = 0;
var chatCreatedTasks = [];
var chatToolCalls = [];
var chatTurnStartMs = 0;
var chatPendingAttachments = [];
var chatRecording = false;
var chatMediaRecorder = null;
var chatAudioChunks = [];
var taskModalAttachments = [];
var canvasFilterAssignee = "";
var canvasFilterProject = "";
var canvasDragTaskId = 0;
var canvasDragFromCol = "";
var loadingActions = {}; // track in-progress actions: key -> true
var mcpData = null; // cached MCP list response
var mcpCatalogResults = null;
var mcpCatalogSearch = "";
var mcpCatalogLoaded = false;
var mcpCatalogTab = "all"; // "trending" | "all" | "installed"
var mcpCatalogCursor = "";
var mcpCatalogHasMore = false;
var mcpCatalogVersion = 0;
var marketplaceSubTab = "mcp";
var skillsData = null;
var skillsCatalogResults = null;
var skillsCatalogSearch = "";
var skillsCatalogLoaded = false;
var skillsCatalogTab = "all"; // "trending" | "all" | "installed"
var skillsCatalogVersion = 0;
var skillsShowCount = 100;
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
var cameFromProjects = false;
var selectedMktItem = null;
var selectedMktType = "";

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
  var toastEl = document.createElement("div");
  toastEl.className = "toast toast-" + type;
  var icon = document.createElement("span");
  icon.className = "toast-icon";
  if(type === "success") icon.textContent = "\u2713";
  else if(type === "error") icon.textContent = "\u2717";
  else icon.textContent = "\u2139";
  toastEl.appendChild(icon);
  var msg = document.createElement("span");
  msg.className = "toast-msg";
  msg.textContent = message;
  toastEl.appendChild(msg);
  container.appendChild(toastEl);
  setTimeout(function(){
    toastEl.classList.add("removing");
    setTimeout(function(){ if(toastEl.parentNode) toastEl.parentNode.removeChild(toastEl); }, 300);
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
  var tmpl = document.createElement("template");
  tmpl.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>';
  ICON_SVGS.pencil = tmpl.content.firstChild;
  tmpl = document.createElement("template");
  tmpl.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>';
  ICON_SVGS.trash = tmpl.content.firstChild;
  tmpl = document.createElement("template");
  tmpl.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>';
  ICON_SVGS.plus = tmpl.content.firstChild;
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
var currentSSE = null;
function connectSSE(){
  if(currentSSE){ try{ currentSSE.close(); }catch(e){} }
  var es = new EventSource("/api/sse");
  currentSSE = es;
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
    // Check if 401 (auth required) before reconnecting
    fetch("/api/data").then(function(r){
      if(r.status === 401){
        location.hash = "#login";
        D = null;
        render();
      } else {
        setTimeout(connectSSE, 3000);
      }
    }).catch(function(){ setTimeout(connectSSE, 3000); });
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
    .then(function(r){
      if(r.status === 401 && path !== "/api/auth/login"){
        location.hash = "#login";
        render();
        return null;
      }
      return r.json().then(function(d){ return {ok: r.ok, data: d}; });
    })
    .then(function(res){
      if(!res) return;
      if(cb) cb(res.data, res.ok);
    })
    .catch(function(e){
      console.error(e);
      if(cb) cb({error: e.message}, false);
    });
}

/* ── MCP Catalog helpers ── */
function loadMCPCatalog(query, cursor){
  if(!cursor && !mcpCatalogLoaded){ mcpCatalogResults = "loading"; render(); }
  var url = "/api/mcp/catalog?limit=100";
  if(query) url += "&search=" + encodeURIComponent(query);
  if(cursor) url += "&cursor=" + encodeURIComponent(cursor);
  api("GET", url, null, function(d){
    var items = d.servers || [];
    if(cursor && mcpCatalogResults && mcpCatalogResults !== "loading"){
      mcpCatalogResults = mcpCatalogResults.concat(items);
    } else {
      mcpCatalogResults = items;
    }
    mcpCatalogCursor = d.next_cursor || "";
    mcpCatalogHasMore = !!d.next_cursor;
    mcpCatalogLoaded = true;
    mcpCatalogVersion++;
    render();
  });
}

function loadSkillsCatalog(query){
  if(!skillsCatalogLoaded){ skillsCatalogResults = "loading"; render(); }
  skillsShowCount = 100;
  var url = "/api/skills/catalog?limit=500";
  if(query) url += "&search=" + encodeURIComponent(query);
  api("GET", url, null, function(d){
    skillsCatalogResults = d.skills || [];
    skillsCatalogLoaded = true;
    skillsCatalogVersion++;
    render();
  });
}

var mcpSearchTimer = null;
var skillsSearchTimer = null;

function doInstall(name, pkg, regType, envVars, btn){
  var restore = btnLoading(btn, t("mkt.catalog.installing"));
  api("POST", "/api/mcp/install", {name: name, package: pkg, registry_type: regType || "npm", env_vars: envVars}, function(d){
    if(restore) restore();
    if(d.ok){
      mcpData = null;
      mcpCatalogLoaded = false;
      mcpCatalogResults = null;
      mcpCatalogSearch = "";
      toast(t("mkt.catalog.installed_toast",{name:name}), "success");
      render();
    } else {
      toast(t("common.error_unknown",{error:d.error||"unknown"}), "error");
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
  var confirmBtn = el("button","btn btn-success btn-sm",[t("common.install")]);
  confirmBtn.onclick = function(){
    var envVars = {};
    var missing = [];
    srv.env_vars.forEach(function(v){
      var val = inputs[v.name].value.trim();
      if(val) envVars[v.name] = val;
      else if(v.is_required) missing.push(v.name);
    });
    if(missing.length > 0){
      toast(t("mkt.catalog.missing_required",{fields:missing.join(", ")}), "error");
      return;
    }
    form.remove();
    doInstall(srv.name, srv.package, srv.registry_type, envVars, installBtn);
  };
  actRow.appendChild(confirmBtn);
  var cancelEnvBtn = el("button","btn btn-sm",[t("common.cancel")]);
  cancelEnvBtn.onclick = function(){ form.remove(); };
  actRow.appendChild(cancelEnvBtn);
  form.appendChild(actRow);
  parent.appendChild(form);
}

/* ── Router ── */
function getView(){
  var h = location.hash.replace("#","") || "dashboard";
  if(["dashboard","queue","canvas","projects","logs","chat","jobs","config","setup","login"].indexOf(h) < 0) h = "dashboard";
  return h;
}

/* ── Safe DOM helpers ── */
function txt(s){ return document.createTextNode(s || ""); }
function escHtml(s){
  var d = document.createElement("div");
  d.appendChild(document.createTextNode(s));
  return d.innerHTML;
}

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
  var lp = (typeof currentLocale === "function" ? currentLocale() : "en") + ":";
  if(!D && v !== "setup" && v !== "login") return lp;
  var tasks = D ? (D.tasks || []) : [];
  var logs = D ? (D.log_entries || []) : [];
  var projs = D ? (D.projects || []) : [];
  switch(v){
    case "dashboard":
      var tk = tasks.length + ":";
      for(var i=0;i<tasks.length;i++) tk += tasks[i].effective_state + ",";
      return lp + "d:" + JSON.stringify(D.today||{}) + JSON.stringify(D.cost||{}) +
        (D.session ? D.session.context_percent : 0) + ":" + tk + ":" + logs.length;
    case "queue":
      var tk = "";
      for(var i=0;i<tasks.length;i++){
        var tki = tasks[i];
        tk += tki.id + "," + tki.effective_state + "," + tki.is_running + "," + tki.has_plan + ";";
      }
      var selLogs = 0;
      for(var i=0;i<logs.length;i++){
        if(logs[i].task_id === selectedTaskID) selLogs++;
      }
      return lp + "q:" + queueFilterState + ":" + queueFilterProject + ":" + selectedTaskID + ":" + tk + ":" + selLogs;
    case "projects":
      var pk = "";
      for(var i=0;i<projs.length;i++){
        var p = projs[i];
        pk += p.name + "," + p.status_icon + "," + (p.modified||0) + "," + (p.branch||"") + ";";
      }
      return lp + "p:" + selectedProjectName + ":" + pk;
    case "logs":
      return lp + "l:" + logFilterLevel + ":" + logFilterTask + ":" + logFilterProject + ":" + logFilterDateFrom + ":" + logFilterDateTo + ":" + logs.length;
    case "chat":
      return lp + "chat:" + chatCounter;
    case "canvas":
      var ck = canvasFilterAssignee + ":" + canvasFilterProject + ":";
      for(var i=0;i<tasks.length;i++) ck += tasks[i].id + tasks[i].effective_state + (tasks[i].assignee||"") + ",";
      return lp + "cv:" + ck;
    case "jobs":
      var jbs = D ? (D.jobs || []) : [];
      var jk = "";
      for(var i=0;i<jbs.length;i++) jk += jbs[i].id + "," + jbs[i].status + "," + jbs[i].enabled + ";";
      return lp + "j:" + jk;
    case "config":
      return lp + "cfg:" + configLoaded + ":" + configTab + ":" + configSubTab + ":" + configSetupSubTab + ":" + (configEditing || "") + ":" + (templatesCache ? templatesCache.length : 0) + ":" + (cfgEditingTemplate ? cfgEditingTemplate.id : "") + ":" + (mcpData ? "1" : "0") + ":" + mcpCatalogLoaded + ":" + mcpCatalogTab + ":" + mcpCatalogVersion + ":" + marketplaceSubTab + ":" + (skillsData ? skillsData.length : "n") + ":" + skillsCatalogLoaded + ":" + skillsCatalogTab + ":" + skillsCatalogVersion + ":" + skillsShowCount + ":" + updateRunning + ":" + (selectedMktItem ? selectedMktItem.name : "");
    case "login":
      return lp + "login:1";
    case "setup":
      return lp + "s:" + setupStep + ":" + JSON.stringify(setupStepDone) + ":" + JSON.stringify(setupStepError);
    default:
      return "";
  }
}

function render(){
  var v = getView();
  if(!D && v !== "setup" && v !== "login") return;
  // Hide chrome on login
  var topbar = document.querySelector(".topbar");
  var dock = document.getElementById("dock");
  var logoutBtn = document.getElementById("topbar-logout");
  if(v === "login"){
    if(topbar) topbar.style.display = "none";
    if(dock) dock.style.display = "none";
  } else {
    if(topbar) topbar.style.display = "";
    if(dock) dock.style.display = "";
    if(logoutBtn) logoutBtn.style.display = (D && D.auth_enabled) ? "" : "none";
  }
  if(D) updateTopbar();
  var nav = document.querySelectorAll("#dock a");
  for(var i=0;i<nav.length;i++){
    nav[i].classList.toggle("active", nav[i].getAttribute("data-view") === v);
  }

  var content = document.getElementById("content");
  var main = document.querySelector(".main");
  var viewChanged = (v !== prevView);
  prevView = v;
  if(viewChanged && v !== "queue") cameFromProjects = false;

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
      if(logFilterDateFrom){
        var ld = new Date(l.time); ld.setHours(0,0,0,0);
        if(ld < new Date(logFilterDateFrom + "T00:00:00")) return false;
      }
      if(logFilterDateTo){
        var ld2 = new Date(l.time); ld2.setHours(0,0,0,0);
        if(ld2 > new Date(logFilterDateTo + "T00:00:00")) return false;
      }
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

  // Save marketplace search focus state
  var mktSearchFocused = false;
  var mktSearchCursor = 0;
  var existingMktSearch = content.querySelector(".mkt-search");
  if(existingMktSearch && document.activeElement === existingMktSearch){
    mktSearchFocused = true;
    mktSearchCursor = existingMktSearch.selectionStart || 0;
  }

  // Save task terminal scroll position
  var taskTermScroll = 0;
  var existingTerm = content.querySelector(".task-terminal");
  if(existingTerm){
    taskTermScroll = existingTerm.scrollTop;
    var atBottom = (existingTerm.scrollHeight - existingTerm.scrollTop - existingTerm.clientHeight) < 8;
    if(!atBottom) taskLogAutoScroll = false;
  }

  // Save plan content scroll position
  var planContentScroll = 0;
  var existingPlanContent = content.querySelector(".plan-content");
  if(existingPlanContent) planContentScroll = existingPlanContent.scrollTop;

  // Build new content into temp container
  var tmp = document.createElement("div");
  switch(v){
    case "dashboard": renderDashboard(tmp); break;
    case "queue": renderQueue(tmp); break;
    case "canvas": renderCanvas(tmp); break;
    case "projects": renderProjects(tmp); break;
    case "logs": renderLogs(tmp); break;
    case "chat": renderChat(tmp); break;
    case "jobs": renderJobs(tmp); break;
    case "config": renderConfig(tmp); break;
    case "login": renderLogin(tmp); break;
    case "setup": renderSetup(tmp); break;
  }

  // Atomic swap
  var wasLogin = prevView === "login" && v !== "login";
  content.textContent = "";
  while(tmp.firstChild) content.appendChild(tmp.firstChild);
  if(wasLogin){
    content.classList.add("view-enter");
    setTimeout(function(){ content.classList.remove("view-enter"); }, 500);
  }

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

  // Restore scroll positions (skip when scrolling to selected task)
  var willScrollToTask = (v === "queue" && selectedTaskID && viewChanged);
  if(!willScrollToTask) content.scrollTop = mainScroll;
  // Timeline scroll preserved via mainScroll

  if(v === "logs"){
    var lc = document.getElementById("log-container");
    if(lc){
      if(logAutoScroll) lc.scrollTop = lc.scrollHeight;
      else lc.scrollTop = logContainerScroll;
    }
  }

  // Restore marketplace search focus
  if(mktSearchFocused){
    var newMktSearch = content.querySelector(".mkt-search");
    if(newMktSearch){
      newMktSearch.focus();
      newMktSearch.setSelectionRange(mktSearchCursor, mktSearchCursor);
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

  // Restore plan content scroll position
  var newPlanContent = content.querySelector(".plan-content");
  if(newPlanContent) newPlanContent.scrollTop = planContentScroll;

  // Scroll to selected task in queue after navigation from project detail
  if(willScrollToTask){
    var selNode = content.querySelector(".tl-node.selected");
    if(selNode) selNode.scrollIntoView({ block: "center" });
  }
}

window.addEventListener("hashchange", render);

document.addEventListener("keydown", function(e){
  if(e.key !== "Escape") return;
  if(e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA" || e.target.tagName === "SELECT") return;
  var v = getView();
  if(v === "config" && selectedMktItem){
    selectedMktItem = null;
    selectedMktType = "";
    render();
  } else if(v === "queue" && selectedTaskID){
    selectTask(0);
  } else if(v === "queue" && cameFromProjects){
    cameFromProjects = false;
    window.location.hash = "#projects";
  } else if(v === "projects" && selectedProjectName){
    selectedProjectName = "";
    render();
  }
});

/* ── Theme Toggle ── */
document.getElementById("theme-toggle").addEventListener("click", function() {
  var current = document.documentElement.getAttribute("data-theme");
  var next = current === "light" ? "dark" : "light";
  if (next === "dark") {
    document.documentElement.removeAttribute("data-theme");
  } else {
    document.documentElement.setAttribute("data-theme", next);
  }
  localStorage.setItem("teamoon-theme", next);
  var meta = document.querySelector('meta[name="theme-color"]');
  if (meta) meta.content = next === "light" ? "#f5f5f5" : "#050505";
});

// Logout button in topbar
(function(){
  var btn = document.getElementById("topbar-logout");
  if(!btn) return;
  btn.addEventListener("click", function(){
    fetch("/api/auth/logout", {method:"POST"}).then(function(){
      location.hash = "#login";
      D = null;
      render();
    });
  });
})();

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

  var upEl = document.getElementById("sb-uptime");
  if(upEl && D.uptime_sec > 0){
    var upText = fmtUptime(D.uptime_sec);
    if(upEl.textContent !== upText) upEl.textContent = upText;
  }

  var ctx = D.session || {};
  var ctxPct = ctx.context_percent || 0;
  var ctxEl = document.getElementById("topbar-ctx");
  if(ctxEl){
    var ctxText = t("topbar.ctx",{pct:Math.round(ctxPct)});
    if(ctxEl.textContent !== ctxText) ctxEl.textContent = ctxText;
    ctxEl.style.color = ctxPct >= 80 ? "var(--danger)" : ctxPct >= 60 ? "var(--warning)" : "var(--accent)";
    ctxEl.style.background = ctxPct >= 80 ? "var(--danger-soft)" : ctxPct >= 60 ? "var(--warning-soft)" : "var(--accent-soft)";
  }
}

/* ── Dashboard View ── */
function renderDashboard(root){
  var td = D.today || {}, w = D.week || {}, m = D.month || {};
  var c = D.cost || {};
  var ctx = D.session || {};
  var tasks = D.tasks || [];
  var projs = D.projects || [];
  var logs = D.log_entries || [];

  var totalToday = (td.input||0)+(td.output||0)+(td.cache_read||0)+(td.cache_create||0);
  var totalWeek = (w.input||0)+(w.output||0)+(w.cache_read||0)+(w.cache_create||0);
  var totalMonth = (m.input||0)+(m.output||0)+(m.cache_read||0)+(m.cache_create||0);
  var tIn = td.input||0, tOut = td.output||0, tCr = td.cache_read||0, tCw = td.cache_create||0;

  // ── Hero Card ──
  var hero = div("hero-card");
  hero.appendChild(div("hero-label", [t("dashboard.tokens_label")]));
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
  hero.appendChild(div("hero-sub", [t("dashboard.tokens_cached",{in:fmtNum(tIn),out:fmtNum(tOut),cached:fmtNum(tCr)})]));
  var dashRoot = div("dashboard-root");
  dashRoot.appendChild(hero);

  // ── Bento Grid ──
  var bento = div("bento");

  // Sessions card
  var sessCard = div("card bento-area-sessions");
  var sessLabel = div("card-label");
  var sessDot = span("label-dot"); sessDot.style.background = "var(--success)";
  sessLabel.appendChild(sessDot);
  sessLabel.appendChild(txt(t("dashboard.sessions_label")));
  sessCard.appendChild(sessLabel);
  sessCard.appendChild(mkValue(String(c.sessions_week||0)));
  sessCard.appendChild(mkSub(t("dashboard.sessions_sub",{today:c.sessions_today||0})));
  bento.appendChild(sessCard);

  // Cost card
  var costCard = div("card bento-area-cost");
  var costLabel = div("card-label");
  var costDot = span("label-dot"); costDot.style.background = "var(--warning)";
  costLabel.appendChild(costDot);
  costLabel.appendChild(txt(t("dashboard.cost_label")));
  costCard.appendChild(costLabel);
  var monthCost = c.cost_month || 0;
  var todayCost = c.cost_today || 0;
  costCard.appendChild(mkValue(monthCost > 0 ? "$" + fmtCost(monthCost) : "$0.00"));
  costCard.appendChild(mkSub(t("dashboard.cost_month",{today:fmtCost(todayCost)})));
  var weeklyUse = (D.usage && D.usage.week_all) ? D.usage.week_all.utilization : 0;
  if (weeklyUse > 0) {
    var wColor = weeklyUse >= 90 ? "red" : weeklyUse >= 60 ? "yellow" : "green";
    var wBar = div("progress");
    var wFill = div("progress-fill " + wColor);
    wFill.style.width = Math.min(weeklyUse, 100).toFixed(1) + "%";
    wBar.appendChild(wFill);
    costCard.appendChild(wBar);
    var wLabel = div("usage-bar-label");
    wLabel.textContent = t("dashboard.weekly_usage",{pct:Math.round(weeklyUse)});
    costCard.appendChild(wLabel);
    // Session usage bar
    var sessUse = (D.usage && D.usage.session) ? D.usage.session.utilization : 0;
    var sColor = sessUse >= 90 ? "red" : sessUse >= 60 ? "yellow" : "green";
    var sBar = div("progress");
    var sFill = div("progress-fill " + sColor);
    sFill.style.width = Math.min(sessUse, 100).toFixed(1) + "%";
    sBar.appendChild(sFill);
    costCard.appendChild(sBar);
    var sLabel = div("usage-bar-label");
    sLabel.textContent = t("dashboard.session_usage",{pct:Math.round(sessUse)});
    costCard.appendChild(sLabel);
  }
  // Daily usage bar (100/7 ≈ 14.29% daily cap) — only when fetcher has data
  if (weeklyUse > 0 && c.cost_week > 0) {
    var dailyCap = 100 / 7;
    var todayUtil = (todayCost / c.cost_week) * weeklyUse;
    var dailyFill = Math.min(100, (todayUtil / dailyCap) * 100);
    var dColor = dailyFill >= 90 ? "red" : dailyFill >= 60 ? "yellow" : "green";
    var dBar = div("progress");
    var dFill = div("progress-fill " + dColor);
    dFill.style.width = dailyFill.toFixed(1) + "%";
    dBar.appendChild(dFill);
    costCard.appendChild(dBar);
    var dLabel = div("usage-bar-label");
    dLabel.textContent = t("dashboard.daily_usage",{pct:Math.round(dailyFill)});
    costCard.appendChild(dLabel);
  }
  bento.appendChild(costCard);

  // Context card (tall)
  var ctxPct = ctx.context_percent || 0;
  var ctxCard = div("card bento-area-context");
  var ctxLabel = div("card-label");
  var ctxDot = span("label-dot");
  ctxDot.style.background = ctxPct >= 80 ? "var(--danger)" : ctxPct >= 60 ? "var(--warning)" : "var(--success)";
  ctxLabel.appendChild(ctxDot);
  ctxLabel.appendChild(txt(t("dashboard.context_label")));
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
  ctxCard.appendChild(mkMetricRow([t("dashboard.tokens_week",{tokens:fmtNum(totalWeek),cost:fmtCost(c.cost_week||0)})]));
  ctxCard.appendChild(mkMetricRow([t("dashboard.tokens_month",{tokens:fmtNum(totalMonth),cost:fmtCost(c.cost_month||0)})]));
  bento.appendChild(ctxCard);

  // Queue summary card (wide) — only active tasks (not done/archived)
  var running=0,pendingC=0,planned=0,doneC=0;
  for(var i=0;i<tasks.length;i++){
    var s=tasks[i].effective_state;
    if(s==="running")running++;
    else if(s==="pending"||s==="generating")pendingC++;
    else if(s==="planned")planned++;
    else if(s==="done")doneC++;
  }
  var activeCount = running + pendingC + planned;
  var qCard = div("card bento-area-queue card-clickable");
  qCard.onclick = function(){ location.hash = "queue"; };
  var qLabel = div("card-label");
  var qDot = span("label-dot"); qDot.style.background = "var(--accent)";
  qLabel.appendChild(qDot);
  qLabel.appendChild(txt(t("dashboard.queue_label")));
  qLabel.appendChild(span("view-count", String(activeCount)));
  qCard.appendChild(qLabel);

  var mkQStat = function(dotColor, label, val){
    var row = div("queue-stat");
    var dot = span("queue-stat-dot"); dot.style.background = dotColor;
    row.appendChild(dot);
    row.appendChild(span("queue-stat-label", label));
    row.appendChild(span("queue-stat-val", String(val)));
    return row;
  };
  if(running) qCard.appendChild(mkQStat("var(--success)", t("dashboard.queue.running"), running));
  if(planned) qCard.appendChild(mkQStat("var(--info)", t("dashboard.queue.planned"), planned));
  if(pendingC) qCard.appendChild(mkQStat("var(--text-muted)", t("dashboard.queue.pending"), pendingC));
  if(doneC) qCard.appendChild(mkQStat("var(--success)", t("dashboard.queue.done"), doneC));
  if(!activeCount && !doneC){
    qCard.appendChild(div("empty", [t("dashboard.no_tasks")]));
  }
  bento.appendChild(qCard);

  // Activity card (full-width bottom)
  var actCard = div("card bento-area-activity");
  var actLabel = div("card-label");
  actLabel.appendChild(txt(t("dashboard.activity_title")));
  actCard.appendChild(actLabel);
  var recentLogs = logs.slice(-10).reverse();
  if(recentLogs.length === 0){
    actCard.appendChild(div("empty", [t("dashboard.no_activity")]));
  } else {
    var feed = div("feed");
    for(var i=0;i<recentLogs.length;i++){
      var le = recentLogs[i];
      var item = div("feed-item");
      item.appendChild(span("feed-time", fmtTime(le.time)));
      var lvlSpan = span("feed-level");
      lvlSpan.appendChild(levelIconEl(le.level));
      item.appendChild(lvlSpan);
      if(le.agent && agentMap[le.agent]){
        item.appendChild(agentBadge(le.agent));
      }
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

  dashRoot.appendChild(bento);
  root.appendChild(dashRoot);
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
  titleWrap.appendChild(span("view-title", t("queue.title")));
  if(tasks.length > 0) titleWrap.appendChild(span("view-count", String(tasks.length)));
  header.appendChild(titleWrap);
  var addBtn = el("button", "btn btn-primary btn-sm", [t("queue.add_task")]);
  addBtn.onclick = function(){ openAddTask(""); };
  header.appendChild(addBtn);
  root.appendChild(header);

  // Filters
  var toolbar = div("queue-toolbar");

  var stateSelect = el("select", "filter-select");
  stateSelect.appendChild(mkOption("", t("queue.all_states")));
  var states = Object.keys(stateSet).sort();
  for(var i=0;i<states.length;i++){
    stateSelect.appendChild(mkOption(states[i], ucfirst(states[i]), queueFilterState===states[i]));
  }
  stateSelect.onchange = function(){ queueFilterState = this.value; render(); };
  toolbar.appendChild(stateSelect);

  var projSelect = el("select", "filter-select");
  projSelect.appendChild(mkOption("", t("queue.all_projects")));
  var projNames = Object.keys(projSet).sort();
  for(var i=0;i<projNames.length;i++){
    projSelect.appendChild(mkOption(projNames[i], projNames[i], queueFilterProject===projNames[i]));
  }
  projSelect.onchange = function(){ queueFilterProject = this.value; render(); };
  toolbar.appendChild(projSelect);
  root.appendChild(toolbar);

  var filtered = tasks.filter(function(tsk){
    if(queueFilterState && tsk.effective_state !== queueFilterState) return false;
    if(queueFilterProject && tsk.project !== queueFilterProject) return false;
    return true;
  });

  if(filtered.length === 0){
    var emptyEl = div("empty");
    emptyEl.textContent = (queueFilterState || queueFilterProject)
      ? t("queue.empty_filtered")
      : t("queue.empty_no_tasks");
    root.appendChild(emptyEl);
    return;
  }

  // Timeline
  var timeline = div("timeline");

  for(var i=0;i<filtered.length;i++){
    (function(tsk){
      var cls = "tl-node state-" + tsk.effective_state;
      if(tsk.id === selectedTaskID) cls += " selected";
      if(tsk.is_running) cls += " has-running";
      if(tsk.effective_state === "generating") cls += " has-generating";
      var prev = prevTaskStates[tsk.id];
      if(prev && prev !== "done" && tsk.effective_state === "done") cls += " task-just-done";
      prevTaskStates[tsk.id] = tsk.effective_state;

      var node = div(cls);
      node.appendChild(div("tl-dot"));

      // Header row
      var hdr = div("tl-header");
      hdr.onclick = function(){ selectTask(tsk.id === selectedTaskID ? 0 : tsk.id); };

      var info = div("tl-info");
      info.appendChild(div("tl-desc", [tsk.description]));
      var meta = tsk.project || "\u2014";
      if(tsk.created_at) meta += " \u00b7 " + fmtRelDate(tsk.created_at);
      info.appendChild(div("tl-meta", [meta]));
      hdr.appendChild(info);

      var badges = div("tl-badges");
      badges.appendChild(span("task-state " + tsk.effective_state, stateLabel(tsk.effective_state)));
      badges.appendChild(span("task-pri " + tsk.priority, (tsk.priority||"").toUpperCase()));
      if(tsk.is_running) badges.appendChild(div("running-dot"));
      hdr.appendChild(badges);
      node.appendChild(hdr);

      // Expandable detail
      var expand = div("tl-expand");
      if(tsk.id === selectedTaskID){
        renderTaskDetail(expand, tsk);
      }
      node.appendChild(expand);

      timeline.appendChild(node);
    })(filtered[i]);
  }
  root.appendChild(timeline);
}

function renderTaskDetail(parent, tsk){
  // ── Header Card ──
  var headerCard = div("detail-card detail-header");
  var headerTop = div("detail-header-top");
  headerTop.appendChild(span("detail-title-id", "#" + tsk.id));
  var editable = (tsk.effective_state === "pending" || tsk.effective_state === "planned");
  if(editable){
    var editBtn = iconBtn("pencil", t("common.edit"), function(){});
    editBtn.onclick = function(){
      descSpan.style.display = "none";
      editBtn.style.display = "none";
      var editArea = el("textarea", "edit-desc-textarea");
      editArea.value = tsk.description;
      editArea.rows = 6;
      headerCard.appendChild(editArea);
      var editActions = div("edit-desc-actions");
      var saveBtn = el("button", "btn btn-primary btn-sm", [t("task.save")]);
      var cancelBtn = el("button", "btn btn-sm", [t("task.save_cancel")]);
      cancelBtn.onclick = function(){
        editArea.remove(); editActions.remove();
        descSpan.style.display = ""; editBtn.style.display = "";
      };
      saveBtn.onclick = function(){
        var newDesc = editArea.value.trim();
        if(!newDesc) return;
        var restore = btnLoading(saveBtn, t("task.saving"));
        api("POST","/api/tasks/update",{id:tsk.id, description:newDesc}, function(d){
          if(restore) restore();
          if(d.error){ toast(t("task.error_save",{error:d.error}), "error"); return; }
          toast(t("task.description_updated"), "success");
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
  descSpan.textContent = tsk.description;
  descSpan.title = tsk.description;
  headerCard.appendChild(descSpan);

  // Properties bar
  var props = div("detail-props");
  var propProject = div("detail-prop");
  propProject.appendChild(span("detail-prop-label", t("task.project")));
  propProject.appendChild(span("detail-prop-value", tsk.project || "\u2014"));
  props.appendChild(propProject);

  var propState = div("detail-prop");
  propState.appendChild(span("detail-prop-label", t("task.state")));
  var stateEl = span("task-state " + tsk.effective_state, stateLabel(tsk.effective_state));
  propState.appendChild(stateEl);
  props.appendChild(propState);

  var propPri = div("detail-prop");
  propPri.appendChild(span("detail-prop-label", t("task.priority")));
  propPri.appendChild(span("task-pri " + tsk.priority, (tsk.priority||"").toUpperCase()));
  props.appendChild(propPri);

  var propCreated = div("detail-prop");
  propCreated.appendChild(span("detail-prop-label", t("task.created")));
  propCreated.appendChild(span("detail-prop-value", fmtDate(tsk.created_at)));
  props.appendChild(propCreated);

  if(tsk.is_running){
    var propEngine = div("detail-prop");
    propEngine.appendChild(span("detail-prop-label", t("task.engine")));
    var engineVal = div("detail-engine-running");
    engineVal.appendChild(txt(t("task.engine_running")));
    engineVal.appendChild(div("running-dot"));
    propEngine.appendChild(engineVal);
    props.appendChild(propEngine);
  }
  headerCard.appendChild(props);
  parent.appendChild(headerCard);

  // ── Generating state ──
  if(tsk.effective_state === "generating"){
    var genSec = div("detail-card detail-generating");
    var genRow = div("detail-generating-inner");
    genRow.appendChild(div("spinner"));
    genRow.appendChild(span("detail-generating-label", t("task.generating")));
    genSec.appendChild(genRow);
    parent.appendChild(genSec);
  }

  // ── Actions (all buttons always visible, disabled when not applicable) ──
  var actions = div("detail-actions");
  var s = tsk.effective_state;
  var apKey = "autopilot:" + tsk.id;

  // PLAN — enabled for pending only
  var planLoading = (s === "generating") || (s === "pending" && loadingActions[apKey]);
  var planEnabled = (s === "pending") && !loadingActions[apKey];
  var planBtn = el("button", "btn" + (planEnabled ? " btn-primary" : ""));
  if(planLoading){
    planBtn.disabled = true;
    var psp = document.createElement("span"); psp.className = "btn-spinner"; planBtn.appendChild(psp);
    planBtn.appendChild(txt(t("task.plan_planning")));
  } else {
    planBtn.textContent = t("task.plan_btn");
    planBtn.disabled = !planEnabled;
    if(planEnabled) planBtn.onclick = function(){ taskPlanOnly(tsk.id, this); };
  }
  actions.appendChild(planBtn);

  // Divider: PLAN | execution group
  actions.appendChild(div("detail-actions-divider"));

  // RUN — enabled for planned
  var runEnabled = (s === "planned") && !loadingActions[apKey];
  var runBtn = el("button", "btn" + (runEnabled ? " btn-success" : ""));
  if(loadingActions[apKey] && s === "planned"){
    runBtn.disabled = true;
    var rsp = document.createElement("span"); rsp.className = "btn-spinner"; runBtn.appendChild(rsp);
  } else {
    runBtn.textContent = t("task.run");
    runBtn.disabled = !runEnabled;
    if(runEnabled) runBtn.onclick = function(){ taskAutopilot(tsk.id, this); };
  }
  actions.appendChild(runBtn);

  // STOP — enabled for running only
  var stopEnabled = (s === "running") && !loadingActions[apKey];
  var stopBtn = el("button", "btn" + (stopEnabled ? " btn-danger" : ""));
  if(loadingActions[apKey] && s === "running"){
    stopBtn.disabled = true;
    var ssp = document.createElement("span"); ssp.className = "btn-spinner"; stopBtn.appendChild(ssp);
  } else {
    stopBtn.textContent = t("task.stop");
    stopBtn.disabled = !stopEnabled;
    if(stopEnabled) stopBtn.onclick = function(){ taskStop(tsk.id); };
  }
  actions.appendChild(stopBtn);

  // REPLAN — enabled when has_plan and not running/generating
  var rpKey = "replan:" + tsk.id;
  var replanEnabled = tsk.has_plan && s !== "running" && s !== "generating" && !loadingActions[rpKey];
  var replanBtn = el("button", "btn");
  if(loadingActions[rpKey]){
    replanBtn.disabled = true;
    var rpsp = document.createElement("span"); rpsp.className = "btn-spinner"; replanBtn.appendChild(rpsp);
  } else {
    replanBtn.textContent = t("task.replan");
    replanBtn.disabled = !replanEnabled;
    if(replanEnabled) replanBtn.onclick = function(){ taskReplan(tsk.id, this); };
  }
  actions.appendChild(replanBtn);

  // Divider before destructive action
  actions.appendChild(div("detail-actions-divider"));

  // ARCHIVE — always enabled
  var archKey = "archive:" + tsk.id;
  if(loadingActions[archKey]){
    var archBtn = el("button", "btn btn-danger");
    archBtn.disabled = true;
    var asp = document.createElement("span"); asp.className = "btn-spinner"; archBtn.appendChild(asp);
  } else {
    var archBtn = el("button", "btn btn-danger", [t("task.archive")]);
    archBtn.onclick = function(){ if(!confirm(t("task.archive_confirm",{id:tsk.id}))) return; taskArchive(tsk.id, this); };
  }
  actions.appendChild(archBtn);

  parent.appendChild(actions);

  // ── Attachments section ──
  if(tsk.attachments && tsk.attachments.length > 0){
    var attSec = div("detail-card detail-section");
    var attTitle = div("detail-section-title");
    attTitle.appendChild(txt(t("task.attachments")));
    attSec.appendChild(attTitle);
    var attRow = div("task-detail-attachments");
    attRow.id = "task-att-" + tsk.id;
    // Fetch attachment metadata
    api("GET","/api/tasks/detail?id="+tsk.id,null,function(d){
      var area = document.getElementById("task-att-" + tsk.id);
      if(!area || !d.attachments) return;
      d.attachments.forEach(function(att){
        if(att.mime_type && att.mime_type.indexOf("image/") === 0){
          var img = document.createElement("img");
          img.className = "chat-att-image";
          img.src = "/api/uploads/" + att.id;
          img.alt = att.orig_name;
          img.onclick = function(){ window.open(img.src,"_blank"); };
          area.appendChild(img);
        } else if(att.mime_type && att.mime_type.indexOf("audio/") === 0){
          var audio = document.createElement("audio");
          audio.className = "chat-att-audio";
          audio.controls = true;
          audio.src = "/api/uploads/" + att.id;
          area.appendChild(audio);
        } else {
          var a = document.createElement("a");
          a.className = "chat-att-link";
          a.href = "/api/uploads/" + att.id;
          a.target = "_blank";
          a.textContent = att.orig_name || att.id;
          area.appendChild(a);
        }
      });
    });
    attSec.appendChild(attRow);
    // Attach more button
    var moreBtn = el("button","btn btn-sm",[t("common.add_file")]);
    moreBtn.style.marginTop = "8px";
    moreBtn.onclick = function(){ attachToExistingTask(tsk.id); };
    attSec.appendChild(moreBtn);
    parent.appendChild(attSec);
  }

  // ── Plan section (collapsible) ──
  if(tsk.has_plan){
    var planSec = div("detail-card detail-section");
    var planTitleRow = div("detail-section-title detail-section-toggle");
    planTitleRow.appendChild(txt(t("task.plan_label")));
    var chevron = span("plan-chevron", planCollapsed ? "\u25B6" : "\u25BC");
    planTitleRow.appendChild(chevron);
    planTitleRow.onclick = function(){
      planCollapsed = !planCollapsed;
      var planBody = document.getElementById("plan-body-" + tsk.id);
      if(planBody){
        planBody.style.display = planCollapsed ? "none" : "";
        chevron.textContent = planCollapsed ? "\u25B6" : "\u25BC";
      }
    };
    planSec.appendChild(planTitleRow);
    var planBody = div("plan-body");
    planBody.id = "plan-body-" + tsk.id;
    if(planCollapsed) planBody.style.display = "none";
    var planEl = div("plan-content");
    planEl.id = "plan-content-" + tsk.id;
    if(planCache[tsk.id]){
      planEl.className = "plan-content plan-md";
      planEl.innerHTML = mdToHtml(planCache[tsk.id]);
    } else {
      planEl.textContent = t("task.plan_loading");
      planEl.className = "plan-content loading-text";
    }
    planBody.appendChild(planEl);
    planSec.appendChild(planBody);
    parent.appendChild(planSec);
    if(!planCache[tsk.id]) loadPlan(tsk.id);
  }

  // ── Task Logs Terminal (SSE-driven) ──
  var logSec = div("detail-section detail-log-section");
  var logTitleRow = div("detail-section-title detail-log-title-row");
  logTitleRow.appendChild(txt(t("task.logs_title")));
  if(tsk.is_running){
    logTitleRow.appendChild(span("live-badge", t("task.live_badge")));
  }
  var scrollLabel = el("label", "log-autoscroll-label");
  var scrollCb = el("input");
  scrollCb.type = "checkbox";
  scrollCb.checked = taskLogAutoScroll;
  scrollCb.onchange = function(){
    taskLogAutoScroll = this.checked;
    if(taskLogAutoScroll){
      var term = document.getElementById("task-terminal-" + tsk.id);
      if(term) term.scrollTop = term.scrollHeight;
    }
  };
  scrollLabel.appendChild(scrollCb);
  scrollLabel.appendChild(txt(t("task.tail")));
  logTitleRow.appendChild(scrollLabel);
  logSec.appendChild(logTitleRow);

  var termWrap = div("task-terminal-wrap");
  var terminal = div("task-terminal");
  terminal.id = "task-terminal-" + tsk.id;

  // Hybrid: SSE entries first, HTTP fallback for historical
  var allLogs = (D && D.log_entries) ? D.log_entries : [];
  var sseLogs = [];
  for(var i = 0; i < allLogs.length; i++){
    if(allLogs[i].task_id === tsk.id) sseLogs.push(allLogs[i]);
  }

  // Use SSE logs if available (live), otherwise cached HTTP logs
  var rawTaskLogs = sseLogs.length > 0 ? sseLogs : (taskLogsCache[tsk.id] || []);
  // Deduplicate consecutive entries with identical messages
  var taskLogs = [];
  for(var di = 0; di < rawTaskLogs.length; di++){
    if(di > 0 && rawTaskLogs[di].message === rawTaskLogs[di-1].message) continue;
    taskLogs.push(rawTaskLogs[di]);
  }

  if(taskLogs.length === 0 && sseLogs.length === 0){
    // No SSE entries and no cache — try HTTP fetch for historical logs
    var emptyMsg = div("task-terminal-empty");
    if(tsk.effective_state === "pending"){
      emptyMsg.textContent = t("task.log_empty_pending");
    } else if(tsk.effective_state === "generating"){
      emptyMsg.textContent = t("task.log_empty_generating");
    } else {
      emptyMsg.textContent = t("task.log_empty_loading");
    }
    terminal.appendChild(emptyMsg);
    // Fetch historical logs from file
    if(tsk.effective_state !== "pending"){
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
              em.textContent = t("task.log_empty_no_entries");
              term.appendChild(em);
            }
          }
        });
      })(tsk.id);
    }
  } else {
    for(var i = 0; i < taskLogs.length; i++){
      terminal.appendChild(mkTerminalLine(taskLogs[i]));
    }
  }
  termWrap.appendChild(terminal);

  var jumpBtn = el("button", "jump-btn hidden", [t("task.jump_bottom")]);
  jumpBtn.id = "jump-btn-" + tsk.id;
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
  header.appendChild(span("view-title", t("projects.title")));
  header.appendChild(span("proj-count", t("projects.count",{count:projs.length})));
  var initBtn = el("button","btn btn-sm btn-primary");
  initBtn.textContent = t("projects.init_project");
  initBtn.onclick = function(){ openModal("modal-init"); };
  header.appendChild(initBtn);
  root.appendChild(header);

  var list = div("proj-list");

  var thead = div("proj-row proj-row-header");
  thead.appendChild(span("proj-dot-h",""));
  thead.appendChild(span("proj-row-name",t("projects.col.name")));
  thead.appendChild(span("proj-row-branch",t("projects.col.branch")));
  thead.appendChild(span("proj-row-mod",t("projects.col.tasks")));
  thead.appendChild(span("proj-status",t("projects.col.status")));
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
        var autoBadge = span("autopilot-badge",t("projects.auto"));
        nameCell.appendChild(autoBadge);
      }
      row.appendChild(nameCell);
      row.appendChild(span("proj-row-branch", p.branch || "\u2014"));
      var tasksCell = span("proj-row-mod","");
      if(p.task_total > 0){
        tasksCell.textContent = t("projects.detail.tasks_count",{done:p.task_done,total:p.task_total});
        if(p.task_running > 0) tasksCell.textContent += t("projects.detail.tasks_running",{running:p.task_running});
      }
      row.appendChild(tasksCell);
      row.appendChild(span("proj-status " + p.status_icon, statusLabel(p.status_icon)));

      var acts = div("proj-row-actions");
      // Project autopilot button
      if(p.autopilot_running){
        var stopAutoBtn = el("button","btn btn-sm btn-danger",[t("projects.stop")]);
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
        var autoBtn = el("button","btn btn-sm btn-auto-off",[t("projects.auto")]);
        autoBtn.title = t("projects.auto");
        autoBtn.onclick = function(e){
          e.stopPropagation();
          var restore = btnLoading(autoBtn, "...");
          api("POST","/api/projects/autopilot/start",{project:p.name},function(resp){
            if(restore) restore();
            if(resp.error) toast(resp.error, "error");
            else toast(t("projects.autopilot_started",{name:p.name}), "success");
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
  backBtn.textContent = t("queue.back");
  backBtn.onclick = function(){ selectedProjectName = ""; render(); };
  header.appendChild(backBtn);
  header.appendChild(span("view-title", p.name));
  // Action buttons
  var acts = div("pd-actions");
  if(p.autopilot_running){
    var stopBtn = el("button","btn btn-sm btn-danger",[t("projects.stop_auto")]);
    stopBtn.onclick = function(){
      var restore = btnLoading(stopBtn, t("projects.stopping"));
      api("POST","/api/projects/autopilot/stop",{project:p.name},function(){ if(restore) restore(); scheduleActivePoll(); });
    };
    acts.appendChild(stopBtn);
  } else {
    var autoBtn = el("button","btn btn-sm btn-success",[t("projects.auto")]);
    autoBtn.onclick = function(){
      var restore = btnLoading(autoBtn, t("projects.starting"));
      api("POST","/api/projects/autopilot/start",{project:p.name},function(resp){
        if(restore) restore();
        if(resp.error) toast(resp.error,"error");
        else toast(t("projects.autopilot_started_simple"),"success");
        scheduleActivePoll();
      });
    };
    acts.appendChild(autoBtn);
  }
  if(p.has_git){
    var pullBtn = el("button","btn btn-sm",[t("projects.pull")]);
    pullBtn.onclick = function(){ gitPull(p.path, this); };
    acts.appendChild(pullBtn);
  }
  if(p.github_repo){
    var prBtn = el("button","btn btn-sm",[t("projects.prs")]);
    prBtn.onclick = function(){ showPRs(p.github_repo); };
    acts.appendChild(prBtn);
  }
  acts.appendChild(iconBtn("plus",t("projects.add_task"),function(){ addTaskForProject(p.name); }));
  header.appendChild(acts);
  root.appendChild(header);

  // Status line
  var statusLine = div("pd-status-line");
  var statusDot = span("proj-dot pd-dot status-" + (p.status_icon||"inactive"),"");
  statusLine.appendChild(statusDot);
  statusLine.appendChild(span("pd-branch",t("projects.branch_label",{branch:p.branch||"\u2014"})));
  statusLine.appendChild(span("proj-status " + p.status_icon, statusLabel(p.status_icon)));
  if(p.autopilot_running) statusLine.appendChild(span("autopilot-badge",t("projects.auto")));
  root.appendChild(statusLine);

  // Git section
  var gitSec = div("pd-section");
  gitSec.appendChild(span("pd-section-title",t("projects.detail.git_title")));
  var gitGrid = div("pd-grid");
  gitGrid.appendChild(mkPdRow(t("projects.detail.last_commit"), p.last_commit || "\u2014"));
  gitGrid.appendChild(mkPdRow(t("projects.detail.modified_files"), p.modified > 0 ? p.modified+"" : "0"));
  if(p.github_repo) gitGrid.appendChild(mkPdRow(t("projects.detail.github"), p.github_repo));
  gitGrid.appendChild(mkPdRow(t("projects.detail.path"), p.path));
  gitSec.appendChild(gitGrid);
  root.appendChild(gitSec);

  // Tasks section
  var taskSec = div("pd-section");
  taskSec.appendChild(span("pd-section-title",t("projects.detail.tasks_title")));
  var taskSummary = div("pd-task-summary");
  taskSummary.appendChild(span("pd-task-count", p.task_total + " " + t("projects.detail.task_total")));
  if(p.task_pending > 0) taskSummary.appendChild(span("pd-task-count pending", p.task_pending + " " + t("projects.detail.task_pending")));
  if(p.task_running > 0) taskSummary.appendChild(span("pd-task-count running", p.task_running + " " + t("projects.detail.task_running")));
  if(p.task_done > 0) taskSummary.appendChild(span("pd-task-count done", p.task_done + " " + t("projects.detail.task_done")));
  taskSec.appendChild(taskSummary);

  // Progress bar
  if(p.task_total > 0){
    var pct = Math.round((p.task_done / p.task_total) * 100);
    var pbar = div("pd-progress-bar");
    var pfill = div("pd-progress-fill");
    pfill.style.width = pct + "%";
    pbar.appendChild(pfill);
    taskSec.appendChild(pbar);
    taskSec.appendChild(span("pd-progress-label", t("projects.detail.pct_complete",{pct:pct})));
  }

  // Task groups
  var allTasks = (D.tasks || []).filter(function(tsk){ return tsk.project === p.name; });
  var groups = [
    {label:"Running", state:"running", open:true},
    {label:"Generating", state:"generating", open:true},
    {label:"Pending", state:"pending", open:allTasks.length < 30},
    {label:"Planned", state:"planned", open:true},
    {label:"Done", state:"done", open:false},
  ];
  for(var g=0;g<groups.length;g++){
    var grp = groups[g];
    var grpTasks = allTasks.filter(function(tsk){ return tsk.effective_state === grp.state; });
    if(grpTasks.length === 0) continue;
    var details = document.createElement("details");
    details.className = "pd-task-group";
    if(grp.open) details.open = true;
    var summary = document.createElement("summary");
    summary.className = "pd-task-group-summary";
    summary.textContent = grp.label + " (" + grpTasks.length + ")";
    details.appendChild(summary);
    for(var ti=0;ti<grpTasks.length;ti++){
      var gt = grpTasks[ti];
      var trow = div("pd-task-row");
      trow.appendChild(span("pd-task-id","#" + gt.id));
      trow.appendChild(span("pd-task-desc", gt.description.length > 80 ? gt.description.substring(0,80)+"\u2026" : gt.description));
      trow.appendChild(span("task-state " + gt.effective_state, stateLabel(gt.effective_state)));
      trow.appendChild(span("task-pri " + gt.priority, (gt.priority||"").toUpperCase()));
      trow.onclick = (function(tid){ return function(){ selectedTaskID = tid; cameFromProjects = true; window.location.hash = "#queue"; render(); }; })(gt.id);
      trow.style.cursor = "pointer";
      details.appendChild(trow);
    }
    taskSec.appendChild(details);
  }
  root.appendChild(taskSec);

  // Config section
  var cfgSec = div("pd-section");
  cfgSec.appendChild(span("pd-section-title",t("projects.config_title")));
  var cfgGrid = div("pd-grid");
  var sk = (D.config && D.config.project_skeletons && D.config.project_skeletons[p.name]) || (D.config && D.config.skeleton) || {};
  var skEntries = ["web_search","context7_lookup","build_verify","test","pre_commit","commit","push"];
  var skLine = "";
  for(var si=0;si<skEntries.length;si++){
    var key = skEntries[si];
    var val = sk[key] !== undefined ? sk[key] : false;
    skLine += (val ? "\u2713 " : "\u2717 ") + key.replace(/_/g," ") + "   ";
  }
  cfgGrid.appendChild(mkPdRow(t("projects.detail.skeleton"), skLine.trim()));
  if(D.config && D.config.spawn){
    cfgGrid.appendChild(mkPdRow(t("projects.detail.model"), D.config.spawn.model || D.exec_model || "default"));
    cfgGrid.appendChild(mkPdRow(t("projects.detail.max_turns"), (D.config.spawn.max_turns || 15) + ""));
  }
  cfgSec.appendChild(cfgGrid);
  root.appendChild(cfgSec);

  // Recent logs
  var logSec = div("pd-section");
  logSec.appendChild(span("pd-section-title",t("projects.detail.recent_activity")));
  var logs = (D.log_entries || []).filter(function(l){ return l.project === p.name; });
  var recentLogs = logs.slice(-10).reverse();
  if(recentLogs.length === 0){
    logSec.appendChild(span("pd-empty",t("projects.detail.no_activity")));
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

  // Header
  var header = div("view-header");
  header.appendChild(span("view-title", t("logs.title")));
  var autoLabel = el("label");
  autoLabel.style.cssText = "font-size:11px;color:var(--text-muted);display:flex;align-items:center;gap:4px;font-family:var(--font)";
  var cb = el("input");
  cb.type = "checkbox";
  cb.checked = logAutoScroll;
  cb.onchange = function(){ logAutoScroll = this.checked; };
  autoLabel.appendChild(cb);
  autoLabel.appendChild(txt(t("logs.auto_scroll")));
  header.appendChild(autoLabel);
  root.appendChild(header);

  // Stats bar — clickable level pills
  var statsBar = div("log-stats");
  var levelCounts = {info:0, success:0, warn:0, error:0};
  for(var i=0;i<logs.length;i++) if(levelCounts[logs[i].level] !== undefined) levelCounts[logs[i].level]++;
  var levelMeta = [
    {key:"info", label:t("logs.level.info"), color:"var(--accent)"},
    {key:"success", label:t("logs.level.success"), color:"var(--success)"},
    {key:"warn", label:t("logs.level.warn"), color:"var(--warning)"},
    {key:"error", label:t("logs.level.error"), color:"var(--danger)"}
  ];
  for(var i=0;i<levelMeta.length;i++){
    var lm = levelMeta[i];
    var pill = el("button", "log-stat-pill" + (logFilterLevel===lm.key ? " active" : ""));
    pill.style.cssText = "--pill-color:"+lm.color;
    pill.setAttribute("aria-label", t("logs.filter_level_prefix",{level:lm.label}));
    pill.textContent = lm.label + " " + levelCounts[lm.key];
    pill.onclick = (function(k){ return function(){
      logFilterLevel = logFilterLevel===k ? "" : k;
      render();
    };})(lm.key);
    statsBar.appendChild(pill);
  }
  root.appendChild(statsBar);

  // Filter toolbar — structured with separators
  var toolbar = div("log-toolbar");
  var toolbarRow = div("log-toolbar-row");

  // Group 1: Entity filters
  var taskSet = {};
  var projSet = {};
  for(var i=0;i<logs.length;i++){
    if(logs[i].task_id) taskSet[logs[i].task_id] = true;
    if(logs[i].project) projSet[logs[i].project] = true;
  }
  var taskSel = el("select", "filter-select");
  taskSel.setAttribute("aria-label", t("logs.filter_task"));
  taskSel.appendChild(mkOption("", t("logs.all_tasks")));
  var tids = Object.keys(taskSet).sort(function(a,b){return a-b;});
  for(var i=0;i<tids.length;i++){
    taskSel.appendChild(mkOption(tids[i], "#"+tids[i], logFilterTask===tids[i]));
  }
  taskSel.onchange = function(){ logFilterTask = this.value; render(); };
  toolbarRow.appendChild(taskSel);

  var projSel = el("select", "filter-select");
  projSel.setAttribute("aria-label", t("logs.filter_project"));
  projSel.appendChild(mkOption("", t("logs.all_projects")));
  var pnames = Object.keys(projSet).sort();
  for(var i=0;i<pnames.length;i++){
    projSel.appendChild(mkOption(pnames[i], pnames[i], logFilterProject===pnames[i]));
  }
  projSel.onchange = function(){ logFilterProject = this.value; render(); };
  toolbarRow.appendChild(projSel);

  var agentSel = el("select", "filter-select");
  agentSel.setAttribute("aria-label", t("logs.filter_agent"));
  agentSel.appendChild(mkOption("", t("logs.all_agents")));
  var agentIds = Object.keys(agentMap).sort();
  for(var ai=0;ai<agentIds.length;ai++){
    var aKey = agentIds[ai];
    agentSel.appendChild(mkOption(aKey, agentMap[aKey].icon + " " + agentMap[aKey].name, logFilterAgent===aKey));
  }
  agentSel.onchange = function(){ logFilterAgent = this.value; render(); };
  toolbarRow.appendChild(agentSel);

  // Separator
  toolbarRow.appendChild(div("log-toolbar-sep"));

  // Group 2: Date filters
  var dateSel = el("select", "filter-select");
  dateSel.setAttribute("aria-label", t("logs.filter_date"));
  dateSel.appendChild(mkOption("", t("logs.all_time")));
  dateSel.appendChild(mkOption("today", t("logs.today"), logFilterDatePreset==="today"));
  dateSel.appendChild(mkOption("yesterday", t("logs.yesterday"), logFilterDatePreset==="yesterday"));
  dateSel.appendChild(mkOption("7d", t("logs.last_7d"), logFilterDatePreset==="7d"));
  dateSel.appendChild(mkOption("30d", t("logs.last_30d"), logFilterDatePreset==="30d"));
  dateSel.appendChild(mkOption("custom", t("logs.custom"), logFilterDatePreset==="custom"));
  dateSel.onchange = function(){ applyDatePreset(this.value); render(); };
  toolbarRow.appendChild(dateSel);

  var dateFrom = el("input", "filter-input");
  dateFrom.type = "date";
  dateFrom.value = logFilterDateFrom;
  dateFrom.title = t("logs.filter_from");
  dateFrom.setAttribute("aria-label", t("logs.filter_from"));
  dateFrom.onchange = function(){ logFilterDateFrom = this.value; logFilterDatePreset = "custom"; render(); };
  toolbarRow.appendChild(dateFrom);

  var dateTo = el("input", "filter-input");
  dateTo.type = "date";
  dateTo.value = logFilterDateTo;
  dateTo.title = t("logs.filter_to");
  dateTo.setAttribute("aria-label", t("logs.filter_to"));
  dateTo.onchange = function(){ logFilterDateTo = this.value; logFilterDatePreset = "custom"; render(); };
  toolbarRow.appendChild(dateTo);

  // Separator
  toolbarRow.appendChild(div("log-toolbar-sep"));

  // Group 3: Clear + Count
  var hasFilters = logFilterLevel || logFilterTask || logFilterProject || logFilterAgent || logFilterDateFrom || logFilterDateTo;
  if(hasFilters){
    var clearBtn = el("button", "btn btn-sm log-clear-btn");
    clearBtn.textContent = t("common.clear");
    clearBtn.setAttribute("aria-label", t("logs.clear_filters"));
    clearBtn.onclick = function(){
      logFilterLevel = ""; logFilterTask = ""; logFilterProject = "";
      logFilterAgent = ""; logFilterDateFrom = ""; logFilterDateTo = "";
      logFilterDatePreset = "";
      render();
    };
    toolbarRow.appendChild(clearBtn);
  }

  toolbar.appendChild(toolbarRow);
  root.appendChild(toolbar);

  // Filter entries
  var filtered = logs.filter(function(l){
    if(logFilterLevel && l.level !== logFilterLevel) return false;
    if(logFilterTask && String(l.task_id) !== logFilterTask) return false;
    if(logFilterProject && l.project !== logFilterProject) return false;
    if(logFilterAgent && l.agent !== logFilterAgent) return false;
    if(logFilterDateFrom){
      var ld = new Date(l.time); ld.setHours(0,0,0,0);
      if(ld < new Date(logFilterDateFrom + "T00:00:00")) return false;
    }
    if(logFilterDateTo){
      var ld2 = new Date(l.time); ld2.setHours(0,0,0,0);
      if(ld2 > new Date(logFilterDateTo + "T00:00:00")) return false;
    }
    return true;
  });

  // Count badge — after filtering
  var countBadge = span("log-count");
  countBadge.textContent = hasFilters ? t("logs.count_filtered", {filtered: filtered.length, total: logs.length}) : t("logs.count_total", {total: logs.length});
  toolbarRow.appendChild(countBadge);

  // Smart time: show date when range > 1 day
  var showFullDate = !logFilterDateFrom || !logFilterDateTo || logFilterDateFrom !== logFilterDateTo;

  var container = div("log-container");
  container.id = "log-container";
  if(filtered.length === 0){
    container.appendChild(div("empty", [t("logs.empty")]));
  } else {
    for(var i=0;i<filtered.length;i++){
      var l = filtered[i];
      var entry = div("log-entry " + l.level);
      entry.appendChild(span("log-time" + (showFullDate ? " log-time-full" : ""), showFullDate ? fmtShortDate(l.time) : fmtTime(l.time)));
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
  var restore = btnLoading(btn);
  api("POST","/api/tasks/autopilot",{id:id, run:false}, function(d, ok){
    delete loadingActions[key];
    if(restore) restore();
    if(!ok){ toast(t("task.plan_failed", {error: d.error || "unknown error"}), "error"); return; }
    scheduleActivePoll();
  });
}
function taskAutopilot(id, btn){
  var key = "autopilot:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  var restore = btnLoading(btn);
  api("POST","/api/tasks/autopilot",{id:id}, function(d, ok){
    delete loadingActions[key];
    if(restore) restore();
    if(!ok){ toast(t("task.retry_failed", {error: d.error || "unknown error"}), "error"); return; }
    scheduleActivePoll();
  });
}
function taskDone(id, btn){
  var key = "done:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  var restore = btnLoading(btn);
  api("POST","/api/tasks/done",{id:id}, function(d, ok){
    delete loadingActions[key];
    if(restore) restore();
    if(!ok){ toast(t("task.mark_done_failed", {error: d.error || "unknown error"}), "error"); return; }
    selectedTaskID = 0;
  });
}
function taskArchive(id, btn){
  var key = "archive:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  var restore = btnLoading(btn);
  api("POST","/api/tasks/archive",{id:id}, function(d, ok){
    delete loadingActions[key];
    if(restore) restore();
    if(!ok){ toast(t("task.archive_failed", {error: d.error || "unknown error"}), "error"); return; }
    selectedTaskID = 0;
  });
}
function taskReplan(id, btn){
  var key = "replan:" + id;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  var restore = btnLoading(btn);
  delete planCache[id];
  api("POST","/api/tasks/replan",{id:id}, function(d, ok){
    delete loadingActions[key];
    if(restore) restore();
    if(!ok){ toast(t("task.replan_failed", {error: d.error || "unknown error"}), "error"); return; }
    scheduleActivePoll();
  });
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
    var content = d.content || d.error || t("task.plan_no_content");
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
  btnLoading(btn, t("projects.pulling"));
  api("POST","/api/projects/pull",{path:path}, function(d){
    delete loadingActions[key];
    if(d.error){ toast(t("projects.pull_failed", {error: d.error}), "error"); }
    else { toast(t("projects.pull_complete"), "success"); }
    render();
  });
}

function gitInitProject(path, name, btn){
  var key = "init:" + path;
  if(loadingActions[key]) return;
  loadingActions[key] = true;
  btnLoading(btn, t("projects.git_init_loading"));
  api("POST","/api/projects/git-init",{path:path, name:name}, function(d){
    delete loadingActions[key];
    if(d.error){ toast(t("projects.git_init_failed", {error: d.error}), "error"); }
    else { toast(t("projects.git_initialized", {name: name}), "success"); }
    render();
  });
}


function showPRs(repo){
  currentPRsRepo = repo;
  document.getElementById("prs-title").textContent = t("modal.prs.title_repo", {repo: repo});
  var content = document.getElementById("prs-content");
  content.textContent = t("common.loading");
  content.className = "loading-text";
  document.getElementById("btn-merge-dep").style.display = "none";
  openModal("modal-prs");
  api("GET","/api/projects/prs?repo="+encodeURIComponent(repo), null, function(d){
    content.className = "";
    content.textContent = "";
    if(d.error){
      content.textContent = t("common.error", {error: d.error});
      return;
    }
    var prs = d.prs || [];
    var dep = d.dependabot || [];
    if(prs.length === 0){
      content.appendChild(div("empty", [t("modal.prs.empty")]));
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
        item.style.cursor = "pointer";
        item.onclick = (function(num){ return function(){ showPRDetail(repo, num); }; })(p.number);
        content.appendChild(item);
      }
    }
    if(dep.length > 0) document.getElementById("btn-merge-dep").style.display = "";
  });
}

function showPRDetail(repo, number){
  var ct = document.getElementById("pr-detail-content");
  ct.textContent = t("common.loading");
  ct.className = "loading-text";
  document.getElementById("pr-detail-title").textContent = t("modal.pr_detail.title_num", {number: number});
  document.getElementById("pr-detail-link").href = "#";
  openModal("modal-pr-detail");
  api("GET","/api/projects/pr-detail?repo="+encodeURIComponent(repo)+"&number="+number, null, function(d){
    ct.className = "";
    ct.textContent = "";
    if(d.error){ ct.textContent = t("common.error", {error: d.error}); return; }
    document.getElementById("pr-detail-title").textContent = "#" + d.number + " " + d.title;
    if(d.url) document.getElementById("pr-detail-link").href = d.url;

    // State + draft badge row
    var meta = div("pr-detail-meta");
    var stCls = "pr-badge pr-state-" + (d.state || "open").toLowerCase();
    if(d.isDraft) stCls += " pr-draft";
    var stLabel = d.isDraft ? t("modal.pr_detail.draft") : (d.state || "OPEN");
    meta.appendChild(span(stCls, stLabel));
    meta.appendChild(span("pr-detail-branch", (d.headRefName || "?") + " \u2192 " + (d.baseRefName || "?")));
    meta.appendChild(span("pr-detail-author", (d.author && d.author.login) || ""));
    ct.appendChild(meta);

    // Stats
    var stats = div("pr-detail-stats");
    stats.appendChild(span("pr-stat-add", "+" + (d.additions || 0)));
    stats.appendChild(span("pr-stat-del", "\u2212" + (d.deletions || 0)));
    stats.appendChild(span("pr-stat-files", (d.changedFiles || 0) + " files"));
    ct.appendChild(stats);

    // Labels
    if(d.labels && d.labels.length > 0){
      var lbRow = div("pr-detail-labels");
      for(var i=0;i<d.labels.length;i++){
        lbRow.appendChild(span("pr-label", d.labels[i].name));
      }
      ct.appendChild(lbRow);
    }

    // Review decision
    if(d.reviewDecision){
      var rv = div("pr-detail-review");
      var rvCls = "pr-badge pr-review-" + d.reviewDecision.toLowerCase().replace(/_/g,"-");
      var rvLabel = d.reviewDecision.replace(/_/g," ");
      rv.appendChild(span(rvCls, rvLabel));
      ct.appendChild(rv);
    }

    // Body
    if(d.body){
      var body = div("pr-detail-body");
      body.textContent = d.body;
      ct.appendChild(body);
    }

    // Dates
    var dates = div("pr-detail-dates");
    if(d.createdAt) dates.appendChild(span("pr-date", t("modal.pr_detail.created", {date: new Date(d.createdAt).toLocaleString()})));
    if(d.updatedAt) dates.appendChild(span("pr-date", t("modal.pr_detail.updated", {date: new Date(d.updatedAt).toLocaleString()})));
    ct.appendChild(dates);
  });
}

function mergeDependabot(){
  var mergeBtn = document.getElementById("btn-merge-dep");
  var closeBtn = document.querySelector("#modal-prs .modal-actions .btn:first-child");
  var restore = btnLoading(mergeBtn, t("modal.prs.merging"));
  if(closeBtn) closeBtn.disabled = true;
  api("POST","/api/projects/merge-dependabot",{repo:currentPRsRepo}, function(d){
    if(restore) restore();
    if(closeBtn) closeBtn.disabled = false;
    if(d.error){ toast(t("modal.prs.merge_error", {error: d.error}), "error"); }
    else { toast(t("modal.prs.merged", {merged: d.merged, failed: d.failed||0}), d.failed > 0 ? "error" : "success"); }
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
  sel.appendChild(mkOption("", t("modal.add_task.template_select_placeholder")));
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
    btn.textContent = t("modal.add_task.update_snippet");
    btn.onclick = function(){ updateTemplate(); };
  } else {
    btn.textContent = t("modal.add_task.save_snippet");
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
    if(d.error){ toast(t("common.error", {error: d.error}), "error"); return; }
    editingTemplateId = 0;
    nameEl.value = "";
    updateTemplateSaveBtn();
    loadTemplates();
    toast(t("template.template_updated"), "success");
  });
}

function openAddTask(proj){
  var sel = document.getElementById("add-project");
  sel.textContent = "";
  sel.appendChild(mkOption("", t("modal.add_task.project_none")));
  var projs = (D && D.projects) || [];
  for(var i=0;i<projs.length;i++){
    sel.appendChild(mkOption(projs[i].name, projs[i].name, projs[i].name===proj));
  }
  document.getElementById("add-desc").value = "";
  document.getElementById("add-priority").value = "med";
  document.getElementById("tmpl-name").value = "";
  editingTemplateId = 0;
  taskModalAttachments = [];
  renderTaskAttachPreview();
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
  var assignee = document.getElementById("add-assignee").value;
  if(!desc){ document.getElementById("add-desc").focus(); return; }
  var suffix = taskSuffix(proj);
  if(suffix && desc.indexOf("party-mode") === -1){
    desc = desc + ". " + suffix;
  }
  var addBtn = document.querySelector("#modal-add .btn-primary");
  var restore = btnLoading(addBtn, t("modal.add_task.adding"));
  var attIds = taskModalAttachments.map(function(a){ return a.id; });
  api("POST","/api/tasks/add",{project:proj, description:desc, priority:pri, assignee:assignee, attachments:attIds.length?attIds:undefined}, function(d){
    taskModalAttachments = [];
    if(restore) restore();
    closeModal("modal-add");
    if(d.id){ selectedTaskID = d.id; toast(t("modal.add_task.created", {id: d.id}), "success"); }
  });
}

function taskAttachFile(){
  var inp = document.createElement("input");
  inp.type = "file"; inp.multiple = true;
  inp.onchange = function(){
    for(var i=0;i<this.files.length;i++) taskUploadAttachment(this.files[i]);
  };
  inp.click();
}

function taskUploadAttachment(file){
  var fd = new FormData(); fd.append("file", file);
  fetch("/api/upload",{method:"POST",body:fd})
  .then(function(r){ return r.json(); })
  .then(function(att){
    taskModalAttachments.push(att);
    renderTaskAttachPreview();
    toast(t("common.attached", {name: att.orig_name}), "success");
  }).catch(function(){ toast(t("common.upload_failed"), "error"); });
}

function renderTaskAttachPreview(){
  var area = document.getElementById("task-attach-preview");
  if(!area) return;
  area.innerHTML = "";
  taskModalAttachments.forEach(function(att, idx){
    var chip = document.createElement("div");
    chip.className = "task-att-chip";
    chip.innerHTML = '<span>' + escHtml(att.orig_name) + '</span><button class="remove-att" onclick="taskModalAttachments.splice('+idx+',1);renderTaskAttachPreview()">\u00d7</button>';
    area.appendChild(chip);
  });
}

function attachToExistingTask(taskId){
  var inp = document.createElement("input");
  inp.type = "file"; inp.multiple = true;
  inp.onchange = function(){
    for(var i=0;i<this.files.length;i++){
      (function(f){
        var fd = new FormData(); fd.append("file", f);
        fetch("/api/upload",{method:"POST",body:fd})
        .then(function(r){ return r.json(); })
        .then(function(att){
          return fetch("/api/tasks/attach",{
            method:"POST",
            headers:{"Content-Type":"application/json"},
            body:JSON.stringify({id:taskId, upload_id:att.id})
          });
        }).then(function(){ toast(t("common.attached", {name: "#"+taskId}), "success"); })
        .catch(function(){ toast(t("common.attach_failed"), "error"); });
      })(this.files[i]);
    }
  };
  inp.click();
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

function fmtUptime(sec){
  if(!sec || sec < 0) return "0s";
  var d = Math.floor(sec / 86400);
  var h = Math.floor((sec % 86400) / 3600);
  var m = Math.floor((sec % 3600) / 60);
  if(d > 0) return d + "d " + h + "h " + m + "m";
  if(h > 0) return h + "h " + m + "m";
  return m + "m " + (sec % 60) + "s";
}

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

function dateStr(d){ return d.getFullYear()+"-"+String(d.getMonth()+1).padStart(2,"0")+"-"+String(d.getDate()).padStart(2,"0"); }
function fmtShortDate(ts){
  if(!ts) return "";
  var d = new Date(ts);
  var months = [t("common.month.jan"),t("common.month.feb"),t("common.month.mar"),t("common.month.apr"),t("common.month.may"),t("common.month.jun"),t("common.month.jul"),t("common.month.aug"),t("common.month.sep"),t("common.month.oct"),t("common.month.nov"),t("common.month.dec")];
  return months[d.getMonth()]+" "+d.getDate()+" "+String(d.getHours()).padStart(2,"0")+":"+String(d.getMinutes()).padStart(2,"0");
}
function applyDatePreset(preset){
  logFilterDatePreset = preset;
  var today = new Date(); today.setHours(0,0,0,0);
  switch(preset){
    case "today":
      logFilterDateFrom = dateStr(today);
      logFilterDateTo = dateStr(today);
      break;
    case "yesterday":
      var y = new Date(today); y.setDate(y.getDate()-1);
      logFilterDateFrom = dateStr(y);
      logFilterDateTo = dateStr(y);
      break;
    case "7d":
      var w = new Date(today); w.setDate(w.getDate()-6);
      logFilterDateFrom = dateStr(w);
      logFilterDateTo = dateStr(today);
      break;
    case "30d":
      var m = new Date(today); m.setDate(m.getDate()-29);
      logFilterDateFrom = dateStr(m);
      logFilterDateTo = dateStr(today);
      break;
    default:
      logFilterDateFrom = "";
      logFilterDateTo = "";
      break;
  }
}

function fmtRelDate(ts){
  if(!ts) return "";
  var d = new Date(ts);
  var diffMs = Date.now() - d.getTime();
  var diffH = Math.floor(diffMs / 3600000);
  if(diffH < 1) return t("common.rel_date.just_now");
  if(diffH < 24) return t("common.rel_date.hours_ago", {n: diffH});
  var diffD = Math.floor(diffH / 24);
  if(diffD < 7) return t("common.rel_date.days_ago", {n: diffD});
  if(diffD < 30) return t("common.rel_date.weeks_ago", {n: Math.floor(diffD / 7)});
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
    case "generating": return t("task.state.generating");
    case "pending": return t("task.state.pending");
    case "planned": return t("task.state.planned");
    case "running": return t("task.state.running");
    case "done": return t("task.state.done");
    default: return s ? s.toUpperCase().substring(0,4) : "\u2014";
  }
}
function statusLabel(s){
  switch(s){
    case "active": return t("projects.status.active");
    case "stale": return t("projects.status.stale");
    case "no_git": return t("projects.status.no_git");
    default: return t("projects.status.inactive");
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
  optAll.value = ""; optAll.textContent = t("chat.all_projects");
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
  projSel.onchange = function(){ chatProject = this.value; chatMessages = []; chatCounter++; render(); };
  topBar.appendChild(projSel);

  var clearBtn = el("button","btn btn-sm btn-danger");
  clearBtn.textContent = t("chat.clear");
  clearBtn.onclick = function(){
    api("POST","/api/chat/clear",{project:chatProject},function(){
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
    emptyText.textContent = t("chat.empty_text");
    emptyState.appendChild(emptyText);
    var chips = el("div","chat-suggestions");
    var suggestions = [
      t("chat.suggestion.new_project"),
      t("chat.suggestion.breakdown"),
      t("chat.suggestion.research")
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
      if(m.project !== undefined && m.project !== chatProject) continue;
      if(m.role === "user"){
        var userBubble = el("div","chat-bubble user" + (m.isSystem ? " chat-bubble-system" : ""));
        if(m.isSystem){
          var sysBadge = el("span","chat-system-badge",[t("chat.system_badge")]);
          userBubble.appendChild(sysBadge);
          userBubble.appendChild(document.createTextNode(" "));
        }
        userBubble.appendChild(document.createTextNode(m.isSystem ? m.content.replace(/^\/system /,"") : m.content));
        renderAttachmentPreviews(userBubble, m.attachment_meta);
        msgArea.appendChild(userBubble);
      } else {
        var bwrap = el("div","chat-bubble-wrap");
        bwrap.style.alignSelf = "flex-start";
        bwrap.style.maxWidth = "95%";
        var aBubble = el("div","chat-bubble assistant" + (m.isSystem ? " chat-bubble-system" : ""));
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
          procText.textContent = t("chat.processing");
          procBar.appendChild(procText);
          aBubble.appendChild(procBar);
        }
        bwrap.appendChild(aBubble);

        // Copy button (only for non-empty, non-loading)
        if(m.content && !(chatLoading && i === chatMessages.length - 1)){
          var actions = el("div","chat-bubble-actions");
          var copyBtn = el("button","chat-copy-btn");
          copyBtn.textContent = t("chat.copy");
          copyBtn.onclick = (function(content){
            return function(){
              navigator.clipboard.writeText(content).then(function(){
                toast(t("chat.copied"),"success");
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

  // System mode toggle
  var sysToggle = el("button","chat-system-toggle" + (chatSystemMode ? " active" : ""));
  sysToggle.title = t("chat.system_mode_title");
  var sysIcon = document.createElementNS("http://www.w3.org/2000/svg","svg");
  sysIcon.setAttribute("width","16");
  sysIcon.setAttribute("height","16");
  sysIcon.setAttribute("viewBox","0 0 24 24");
  sysIcon.setAttribute("fill","none");
  sysIcon.setAttribute("stroke","currentColor");
  sysIcon.setAttribute("stroke-width","2");
  var r1 = document.createElementNS("http://www.w3.org/2000/svg","rect");
  r1.setAttribute("x","2"); r1.setAttribute("y","3"); r1.setAttribute("width","20"); r1.setAttribute("height","14"); r1.setAttribute("rx","2");
  var l1 = document.createElementNS("http://www.w3.org/2000/svg","line");
  l1.setAttribute("x1","8"); l1.setAttribute("y1","21"); l1.setAttribute("x2","16"); l1.setAttribute("y2","21");
  var l2 = document.createElementNS("http://www.w3.org/2000/svg","line");
  l2.setAttribute("x1","12"); l2.setAttribute("y1","17"); l2.setAttribute("x2","12"); l2.setAttribute("y2","21");
  sysIcon.appendChild(r1); sysIcon.appendChild(l1); sysIcon.appendChild(l2);
  sysToggle.appendChild(sysIcon);
  sysToggle.onclick = function(){
    chatSystemMode = !chatSystemMode;
    this.classList.toggle("active", chatSystemMode);
    var ta = document.getElementById("chat-textarea");
    if(ta) ta.placeholder = chatSystemMode ? t("chat.placeholder_system") : t("chat.placeholder");
  };
  inputBar.appendChild(sysToggle);

  // Attach file button
  var attachBtn = el("button","chat-attach-btn");
  attachBtn.title = t("modal.add_task.add_file");
  attachBtn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48"/></svg>';
  attachBtn.onclick = function(){
    var inp = document.createElement("input");
    inp.type = "file"; inp.multiple = true;
    inp.onchange = function(){ for(var i=0;i<this.files.length;i++) chatUploadFile(this.files[i]); };
    inp.click();
  };
  inputBar.appendChild(attachBtn);

  // Voice record button
  var voiceBtn = el("button","chat-voice-btn" + (chatRecording ? " recording" : ""));
  voiceBtn.title = chatRecording ? t("chat.voice_stop") : t("chat.voice_record");
  voiceBtn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 1a3 3 0 00-3 3v8a3 3 0 006 0V4a3 3 0 00-3-3z"/><path d="M19 10v2a7 7 0 01-14 0v-2"/><line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/></svg>';
  voiceBtn.onclick = chatToggleRecording;
  inputBar.appendChild(voiceBtn);

  // Attachment preview strip
  if(chatPendingAttachments.length > 0){
    var strip = el("div","chat-attach-strip");
    chatPendingAttachments.forEach(function(att,idx){
      var chip = el("div","chat-attach-chip");
      chip.innerHTML = '<span>' + escHtml(att.orig_name) + '</span><button class="remove-att" data-idx="'+idx+'">\u00d7</button>';
      chip.querySelector(".remove-att").onclick = function(){ chatPendingAttachments.splice(idx,1); chatCounter++; render(); };
      strip.appendChild(chip);
    });
    inputBar.appendChild(strip);
  }

  var textarea = el("textarea","chat-input");
  textarea.id = "chat-textarea";
  textarea.rows = 2;
  textarea.placeholder = chatSystemMode ? t("chat.placeholder_system") : t("chat.placeholder");
  textarea.onkeydown = function(e){
    if(e.key === "Enter" && !e.shiftKey){
      e.preventDefault();
      sendChatMessage();
    }
  };
  inputBar.appendChild(textarea);

  var sendBtn = el("button","btn btn-primary chat-send-btn");
  sendBtn.id = "chat-send-btn";
  sendBtn.textContent = t("chat.send");
  sendBtn.disabled = chatLoading;
  sendBtn.onclick = sendChatMessage;
  inputBar.appendChild(sendBtn);
  wrap.appendChild(inputBar);

  container.appendChild(wrap);

  // Auto-scroll to bottom after render
  setTimeout(function(){ msgArea.scrollTop = msgArea.scrollHeight; }, 0);

  // Load history on first render
  if(chatMessages.length === 0 && !chatLoading){
    api("GET","/api/chat/history?project="+encodeURIComponent(chatProject),null,function(d){
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

  // Auto-prepend /system if system mode is active and user didn't type it
  if(chatSystemMode && msg.indexOf("/system ") !== 0){
    msg = "/system " + msg;
  }
  var isSystem = msg.indexOf("/system ") === 0;
  chatLoading = true;
  chatToolCalls = [];
  chatTurnStartMs = Date.now();
  var sendAtts = chatPendingAttachments.map(function(a){ return a.id; });
  var sendAttMeta = chatPendingAttachments.slice();
  chatPendingAttachments = [];
  chatMessages.push({role:"user", content:msg, project:chatProject, isSystem:isSystem, attachment_meta:sendAttMeta});
  chatMessages.push({role:"assistant", content:"", project:chatProject, isSystem:isSystem});
  chatCounter++;
  render();
  ta.value = "";

  fetch("/api/chat/send",{
    method:"POST",
    headers:{"Content-Type":"application/json"},
    body:JSON.stringify({message:msg, project:chatProject, attachments:sendAtts.length?sendAtts:undefined})
  }).then(function(res){
    if(!res.ok && !res.headers.get("content-type")?.startsWith("text/event-stream")){
      chatMessages[chatMessages.length-1].content = t("chat.error_prefix", {status: res.status});
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
            chatMessages[chatMessages.length-1].content = chatMessages[chatMessages.length-1].content || t("chat.error_no_response");
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
                  toast(t("chat.project_initialized", {name: evt.project_init}), "success");
                  fetchData();
                } else {
                  toast(t("chat.project_init_failed", {error: evt.error || "unknown"}), "error");
                }
              }
              if(evt.tasks_created){
                chatCreatedTasks = evt.tasks_created;
              }
              if(evt.error && !evt.project_init){
                toast(evt.error, "error");
                if(!chatMessages[chatMessages.length-1].content){
                  chatMessages[chatMessages.length-1].content = t("common.error", {error: evt.error});
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
                        assSpan.className = "chat-task-card-assignee" + (ct.assignee === "agent" ? " assignee-agent" : ct.assignee === "system" ? " assignee-system" : "");
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
    chatMessages[chatMessages.length-1].content = t("chat.error_connection");
    chatCounter++;
    render();
  });
}

function chatUploadFile(file){
  var fd = new FormData();
  fd.append("file", file);
  toast(t("chat.uploading", {name: file.name}), "info");
  fetch("/api/upload",{method:"POST",body:fd})
  .then(function(r){ return r.json(); })
  .then(function(att){
    chatPendingAttachments.push(att);
    chatCounter++; render();
    toast(t("common.attached", {name: att.orig_name}), "success");
  }).catch(function(){ toast(t("chat.upload_failed", {name: file.name}), "error"); });
}

function chatToggleRecording(){
  if(!chatRecording){
    if(!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia){
      toast(t("chat.microphone_unavailable"),"error"); return;
    }
    navigator.mediaDevices.getUserMedia({audio:true}).then(function(stream){
      chatMediaRecorder = new MediaRecorder(stream);
      chatAudioChunks = [];
      chatMediaRecorder.ondataavailable = function(e){ chatAudioChunks.push(e.data); };
      chatMediaRecorder.onstop = function(){
        var blob = new Blob(chatAudioChunks, {type:"audio/webm"});
        var file = new File([blob], "voice-note.webm", {type:"audio/webm"});
        stream.getTracks().forEach(function(tr){ tr.stop(); });
        chatUploadFile(file);
      };
      chatMediaRecorder.start();
      chatRecording = true;
      chatCounter++; render();
    }).catch(function(){ toast(t("chat.microphone_unavailable"),"error"); });
  } else {
    if(chatMediaRecorder) chatMediaRecorder.stop();
    chatRecording = false;
    chatCounter++; render();
  }
}

function renderAttachmentPreviews(parent, metas){
  if(!metas || !metas.length) return;
  var row = el("div","chat-message-attachments");
  metas.forEach(function(att){
    if(att.mime_type && att.mime_type.indexOf("image/") === 0){
      var img = document.createElement("img");
      img.className = "chat-att-image";
      img.src = "/api/uploads/" + att.id;
      img.alt = att.orig_name;
      img.onclick = function(){ window.open(img.src, "_blank"); };
      row.appendChild(img);
    } else if(att.mime_type && att.mime_type.indexOf("audio/") === 0){
      var audio = document.createElement("audio");
      audio.className = "chat-att-audio";
      audio.controls = true;
      audio.src = "/api/uploads/" + att.id;
      row.appendChild(audio);
    } else {
      var a = document.createElement("a");
      a.className = "chat-att-link";
      a.href = "/api/uploads/" + att.id;
      a.target = "_blank";
      a.textContent = att.orig_name || att.id;
      row.appendChild(a);
    }
  });
  parent.appendChild(row);
}

/* ── Canvas View ── */
function renderCanvas(container){
  var tasks = D ? D.tasks : [];

  // Header (consistent view-header pattern)
  var header = div("view-header");
  header.appendChild(span("view-title",t("canvas.title")));

  var controls = div("queue-toolbar");
  controls.style.display = "flex";
  controls.style.gap = "8px";
  controls.style.alignItems = "center";

  // Assignee filter
  var assigneeSel = el("select","filter-select");
  var assigneeOpts = [["",t("canvas.all_assignees")],["agent",t("canvas.assignee.agent")],["human",t("canvas.assignee.human")],["review",t("canvas.assignee.review")],["system",t("canvas.assignee.system")]];
  for(var i=0;i<assigneeOpts.length;i++){
    assigneeSel.appendChild(mkOption(assigneeOpts[i][0],assigneeOpts[i][1],canvasFilterAssignee===assigneeOpts[i][0]));
  }
  assigneeSel.onchange = function(){ canvasFilterAssignee = this.value; render(); };
  controls.appendChild(assigneeSel);

  // Project filter
  var projSet = {};
  for(var i=0;i<tasks.length;i++) if(tasks[i].project) projSet[tasks[i].project] = true;
  var projSel = el("select","filter-select");
  projSel.appendChild(mkOption("",t("canvas.all_projects")));
  var pnames = Object.keys(projSet).sort();
  for(var i=0;i<pnames.length;i++){
    projSel.appendChild(mkOption(pnames[i],pnames[i],canvasFilterProject===pnames[i]));
  }
  projSel.onchange = function(){ canvasFilterProject = this.value; render(); };
  controls.appendChild(projSel);

  // + Add Task button
  var addBtn = el("button","btn btn-primary btn-sm");
  addBtn.textContent = t("canvas.add_task");
  addBtn.onclick = function(){ openAddTask(""); };
  controls.appendChild(addBtn);

  header.appendChild(controls);
  container.appendChild(header);

  // Filter tasks
  var filtered = [];
  for(var i=0;i<tasks.length;i++){
    var ct = tasks[i];
    if(canvasFilterAssignee && (ct.assignee||"") !== canvasFilterAssignee) continue;
    if(canvasFilterProject && ct.project !== canvasFilterProject) continue;
    filtered.push(ct);
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
  board.appendChild(makeCanvasCol(t("canvas.col.backlog"),"backlog",backlog));
  board.appendChild(makeCanvasCol(t("canvas.col.ready"),"ready",ready));
  board.appendChild(makeCanvasCol(t("canvas.col.inprogress"),"inprogress",inprogress));
  board.appendChild(makeCanvasCol(t("canvas.col.done"),"done",done));
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
    empty.textContent = t("canvas.empty");
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

function makeCanvasCard(tsk, colId){
  var priClass = "pri-" + (tsk.priority || "med");
  var card = el("div","canvas-card " + priClass);

  // Draggable
  card.setAttribute("draggable","true");
  card.addEventListener("dragstart",function(e){
    canvasDragTaskId = tsk.id;
    canvasDragFromCol = colId;
    e.dataTransfer.setData("text/plain",String(tsk.id));
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
  if(tsk.project) labels.appendChild(span("canvas-label canvas-label-proj",tsk.project));
  if(tsk.assignee){
    var asnText = {agent:"Agent",human:"Human",review:"Review",system:"System"}[tsk.assignee] || tsk.assignee;
    labels.appendChild(span("canvas-label canvas-label-"+tsk.assignee, asnText));
  }
  if(tsk.auto_pilot) labels.appendChild(span("canvas-label canvas-label-ap","AP"));
  if(labels.children.length > 0) inner.appendChild(labels);

  // Title + description
  var text = tsk.description || "";
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
  footer.appendChild(span("canvas-card-id","#"+tsk.id));
  footer.appendChild(span("canvas-card-date",fmtRelDate(tsk.created_at)));
  if(tsk.has_plan){
    var planIcon = span("canvas-card-plan-icon","\u2713");
    planIcon.title = t("task.has_plan");
    footer.appendChild(planIcon);
  }
  if(tsk.effective_state === "running"){
    var lastAg = getLastAgentForTask(tsk.id);
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
    selectedTaskID = tsk.id;
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
  submitBtn.appendChild(document.createTextNode(t("modal.project_init.creating")));
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
          submitBtn.textContent = t("modal.project_init.create");
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
                  doneRow.appendChild(el("span","init-step-name",[t("modal.project_init.success")]));
                  prog.appendChild(doneRow);
                  toast(t("modal.project_init.created", {name: name}), "success");
                } else {
                  toast(t("modal.project_init.failed"), "error");
                }
                submitBtn.disabled = false;
                submitBtn.textContent = t("modal.project_init.create");
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
    submitBtn.textContent = t("modal.project_init.create");
    if(cancelBtn) cancelBtn.disabled = false;
    toast(t("modal.project_init.connection_error"), "error");
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
  name.textContent = t("modal.project_init.step", {step: evt.step, name: evt.name || ""});
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
document.addEventListener("visibilitychange", function(){
  if(!document.hidden){
    prevDataStr = "";
    prevContentKey = "";
    if(currentSSE){ try{ currentSSE.close(); }catch(e){} currentSSE = null; }
    connectSSE();
    api("GET","/api/data",null,function(d){
      D = d;
      isDataUpdate = true;
      render();
    });
  }
});

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

function renderLogin(root){
  var wrap = div("login-wrap");

  // Ambient glow behind card
  wrap.appendChild(div("login-glow"));

  var card = div("login-card");

  // Moon icon
  var iconWrap = div("login-icon");
  var svg = document.createElementNS("http://www.w3.org/2000/svg","svg");
  svg.setAttribute("viewBox","0 0 24 24");
  svg.setAttribute("fill","none");
  svg.setAttribute("stroke","currentColor");
  svg.setAttribute("stroke-width","1.5");
  svg.setAttribute("stroke-linecap","round");
  svg.setAttribute("stroke-linejoin","round");
  svg.setAttribute("width","36");
  svg.setAttribute("height","36");
  var path = document.createElementNS("http://www.w3.org/2000/svg","path");
  path.setAttribute("d","M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z");
  svg.appendChild(path);
  iconWrap.appendChild(svg);
  card.appendChild(iconWrap);

  card.appendChild(el("h2", "login-title", [t("login.title")]));
  card.appendChild(el("p", "login-subtitle", [t("login.subtitle")]));

  var form = document.createElement("form");
  form.className = "login-form";
  form.addEventListener("submit", function(e){
    e.preventDefault();
    var pwInput = form.querySelector(".login-input");
    var statusEl = form.querySelector(".login-status");
    var btn = form.querySelector(".login-btn");
    if(!pwInput.value.trim()){ statusEl.textContent = t("login.password_required"); return; }
    btn.disabled = true;
    btn.classList.add("login-btn-loading");
    statusEl.textContent = "";
    fetch("/api/auth/login", {
      method: "POST",
      headers: {"Content-Type":"application/json"},
      body: JSON.stringify({password: pwInput.value})
    }).then(function(r){ return r.json().then(function(d){ return {ok:r.ok, data:d}; }); })
    .then(function(res){
      if(res.ok && res.data.ok){
        // Animate out then switch
        var loginWrap = document.querySelector(".login-wrap");
        if(loginWrap) loginWrap.classList.add("login-exit");
        setTimeout(function(){
          location.hash = "#dashboard";
          D = null;
          connectSSE();
        }, 400);
        return;
      } else {
        btn.disabled = false;
        btn.classList.remove("login-btn-loading");
        statusEl.textContent = res.data.error || t("login.error_failed");
        pwInput.classList.add("login-input-error");
        setTimeout(function(){ pwInput.classList.remove("login-input-error"); }, 600);
      }
    }).catch(function(){
      btn.disabled = false;
      btn.classList.remove("login-btn-loading");
      statusEl.textContent = t("login.error_network");
    });
  });

  // Password field with lock icon
  var fieldWrap = div("login-field");
  var lockSvg = document.createElementNS("http://www.w3.org/2000/svg","svg");
  lockSvg.setAttribute("class","login-lock");
  lockSvg.setAttribute("viewBox","0 0 24 24");
  lockSvg.setAttribute("fill","none");
  lockSvg.setAttribute("stroke","currentColor");
  lockSvg.setAttribute("stroke-width","2");
  lockSvg.setAttribute("width","16");
  lockSvg.setAttribute("height","16");
  var rect = document.createElementNS("http://www.w3.org/2000/svg","rect");
  rect.setAttribute("x","3"); rect.setAttribute("y","11"); rect.setAttribute("width","18"); rect.setAttribute("height","11"); rect.setAttribute("rx","2"); rect.setAttribute("ry","2");
  var lockPath = document.createElementNS("http://www.w3.org/2000/svg","path");
  lockPath.setAttribute("d","M7 11V7a5 5 0 0 1 10 0v4");
  lockSvg.appendChild(rect);
  lockSvg.appendChild(lockPath);
  fieldWrap.appendChild(lockSvg);

  var pwInput = document.createElement("input");
  pwInput.type = "password";
  pwInput.className = "login-input";
  pwInput.placeholder = t("login.password_placeholder");
  pwInput.autocomplete = "current-password";
  fieldWrap.appendChild(pwInput);
  form.appendChild(fieldWrap);

  var btn = document.createElement("button");
  btn.type = "submit";
  btn.className = "login-btn";
  var btnText = document.createElement("span");
  btnText.className = "login-btn-text";
  btnText.textContent = t("login.sign_in");
  btn.appendChild(btnText);
  form.appendChild(btn);

  var status = el("div", "login-status");
  form.appendChild(status);

  card.appendChild(form);

  var footer = el("p", "login-footer", [t("login.footer")]);
  card.appendChild(footer);

  wrap.appendChild(card);
  root.appendChild(wrap);

  // Auto-focus password field
  setTimeout(function(){ pwInput.focus(); }, 50);
}

function renderSetup(root){
  loadSetupStatus();
  var container = div("setup-container");

  // Sidebar
  var sidebar = div("setup-sidebar");
  sidebar.appendChild(el("div","setup-sidebar-title",[t("setup.sidebar_title")]));

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
  var skipBtn = el("button","btn btn-sm",[t("setup.go_dashboard")]);
  skipBtn.onclick = function(){ location.hash = "dashboard"; };
  skipWrap.appendChild(skipBtn);
  sidebar.appendChild(skipWrap);
  container.appendChild(sidebar);

  // Main area
  var main = div("setup-main");

  if(allSetupDone()){
    main.appendChild(el("div","setup-title",[t("setup.about.setup_complete")]));
    main.appendChild(el("div","setup-desc",[t("setup.about.all_done")]));
    var content = div("setup-content");
    var doneWrap = div("setup-complete");
    doneWrap.appendChild(el("div","setup-complete-icon",["\u2705"]));
    doneWrap.appendChild(el("div","setup-complete-msg",[t("setup.about.ready")]));
    var goBtn = el("button","btn btn-primary",[t("setup.about.go_dashboard")]);
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

  var btn = el("button","btn btn-primary",[setupStepDone[1] ? t("setup.prereqs.re_check") : t("setup.prereqs.check")]);
  btn.onclick = function(){
    prog.textContent = "";
    actions.textContent = "";
    prereqsMissing = [];
    var restore = btnLoading(btn, t("setup.prereqs.checking"));
    streamStep("/api/onboarding/prereqs", null, prog, function(p, evt){
      if(evt.type !== "tool") return;
      var ok = evt.found;
      var cls = "setup-progress-item " + (ok ? "ok" : (evt.required ? "error" : "skip"));
      var item = div(cls);
      item.id = "setup-prereq-" + evt.id;
      item.appendChild(span("icon", ok ? "\u2713" : (evt.required ? "\u2717" : "~")));
      item.appendChild(span("label", evt.name));
      var verText = evt.version || (ok ? t("setup.prereqs.ok") : (evt.required ? t("setup.prereqs.not_found") : t("setup.prereqs.optional")));
      item.appendChild(span("version", verText));
      if(!ok && evt.installable) item.appendChild(span("tag-installable", t("setup.prereqs.tag_installable")));
      p.appendChild(item);
      if(!ok) prereqsMissing.push(evt);
    }, function(status, msg){
      if(restore) restore();
      if(status === "success"){
        setupStepDone[1] = true;
        delete setupStepError[1];
        prog.appendChild(div("setup-status ok",[t("setup.prereqs.all_found")]));
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
      } else {
        setupStepError[1] = true;
        var installable = prereqsMissing.filter(function(m){ return m.installable; });
        if(installable.length > 0){
          var installBtn = el("button","btn btn-success",[t("setup.install_missing", {count: installable.length})]);
          installBtn.onclick = function(){ runPrereqsInstall(prog, actions); };
          actions.appendChild(installBtn);
        }
        prog.appendChild(div("setup-status err",[msg || t("setup.prereqs.missing")]));
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
  installProg.appendChild(el("div","setup-progress-header",[t("setup.prereqs.installing")]));
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
        if(cVer) cVer.textContent = t("mkt.catalog.installed");
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
      installProg.appendChild(div("setup-status ok",[t("setup.prereqs.all_installed")]));
      var ns = nextIncompleteStep();
      if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
    } else {
      installProg.appendChild(div("setup-status err",[msg || t("setup.prereqs.some_failed")]));
    }
    render();
  });
}

function renderSetupConfig(content){
  var form = div("setup-form");
  var fields = [
    {id:"setup-projects-dir", label:t("setup.config.projects_dir"), val:"~/Projects", type:"text"},
    {id:"setup-web-port", label:t("setup.config.port"), val:"7777", type:"number"},
    {id:"setup-web-host", label:t("setup.config.bind_address"), val:"localhost", type:"select", options:["localhost","0.0.0.0"]},
    {id:"setup-web-password", label:t("setup.config.password"), val:"", type:"password"},
    {id:"setup-max-concurrent", label:t("setup.config.max_concurrent"), val:"3", type:"number"}
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
  var btn = el("button","btn btn-primary",[setupStepDone[2] ? t("setup.config.re_configure") : t("setup.config.save")]);
  btn.onclick = function(){
    var restore = btnLoading(btn, t("common.saving"));
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
        toast(t("setup.config.saved"),"success");
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 600);
      } else {
        setupStepError[2] = true;
        toast(t("setup.config.save_failed"),"error");
      }
      render();
    });
  };
  content.appendChild(form);
  content.appendChild(btn);
}

function renderSetupSkills(content){
  var prog = div("setup-progress");
  var btn = el("button","btn btn-primary",[setupStepDone[3] ? t("setup.skills.re_install") : t("setup.skills.install")]);
  btn.onclick = function(){
    prog.textContent = "";
    var restore = btnLoading(btn, t("setup.installing"));
    streamStep("/api/onboarding/skills", null, prog, function(p, evt){
      if(evt.type === "symlink"){
        var item = div("setup-progress-item ok");
        item.appendChild(span("icon","\u2713"));
        item.appendChild(span("label",t("setup.skills.symlink")));
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
        prog.appendChild(div("setup-status ok",[t("setup.skills.all_installed")]));
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
      } else {
        setupStepError[3] = true;
        prog.appendChild(div("setup-status err",[msg || t("setup.skills.failed")]));
      }
      render();
    });
  };
  content.appendChild(btn);
  content.appendChild(prog);
}

function renderSetupBMAD(content){
  var prog = div("setup-progress");
  var btn = el("button","btn btn-primary",[setupStepDone[4] ? t("setup.skills.re_install") : t("setup.bmad.install")]);
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

    var restore = btnLoading(btn, t("setup.installing"));
    streamStep("/api/onboarding/bmad", null, prog, function(p, evt){
      if(evt.type === "progress"){
        var fill = document.getElementById("setup-bmad-bar");
        var ctr = document.getElementById("setup-bmad-counter");
        if(fill) fill.style.width = Math.round((evt.count/evt.total)*100)+"%";
        if(ctr) ctr.textContent = evt.count+" / "+evt.total+" files";
      } else if(evt.type === "symlink"){
        var item = div("setup-progress-item ok");
        item.appendChild(span("icon","\u2713"));
        item.appendChild(span("label",t("setup.bmad.symlink")));
        p.appendChild(item);
      }
    }, function(status, msg){
      if(restore) restore();
      if(status === "success"){
        setupStepDone[4] = true;
        delete setupStepError[4];
        prog.appendChild(div("setup-status ok",[t("setup.bmad.commands_installed")]));
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
      } else {
        setupStepError[4] = true;
        prog.appendChild(div("setup-status err",[msg || t("setup.bmad.failed")]));
      }
      render();
    });
  };
  content.appendChild(btn);
  content.appendChild(prog);
}

function renderSetupHooks(content){
  var prog = div("setup-progress");
  var btn = el("button","btn btn-primary",[setupStepDone[5] ? t("setup.skills.re_install") : t("setup.hooks.install")]);
  btn.onclick = function(){
    prog.textContent = "";
    var restore = btnLoading(btn, t("setup.installing"));
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
        prog.appendChild(div("setup-status ok",[t("setup.hooks.installed")]));
        var ns = nextIncompleteStep();
        if(ns) setTimeout(function(){ setupStep = ns; render(); }, 800);
      } else {
        setupStepError[5] = true;
        prog.appendChild(div("setup-status err",[msg || t("setup.hooks.failed")]));
      }
      render();
    });
  };
  content.appendChild(btn);
  content.appendChild(prog);
}

function renderSetupMCP(content){
  var prog = div("setup-progress");
  var btn = el("button","btn btn-primary",[setupStepDone[6] ? t("setup.skills.re_install") : t("setup.mcp.install")]);
  btn.onclick = function(){
    prog.textContent = "";
    var restore = btnLoading(btn, t("setup.installing"));
    streamStep("/api/onboarding/mcp", null, prog, function(p, evt){
      var cls = evt.status === "done" ? "ok" : (evt.status === "skipped" ? "skip" : "running");
      var iconText = evt.status === "done" ? "\u2713" : (evt.status === "skipped" ? "~" : "\u2022");
      var item = div("setup-progress-item "+cls);
      item.appendChild(span("icon", iconText));
      item.appendChild(span("label", evt.name));
      if(evt.status === "skipped") item.appendChild(span("version",t("setup.mcp.already_configured")));
      p.appendChild(item);
    }, function(status, msg){
      if(restore) restore();
      if(status === "success"){
        setupStepDone[6] = true;
        delete setupStepError[6];
        prog.appendChild(div("setup-status ok",[t("setup.mcp.installed")]));
        if(allSetupDone()) setTimeout(function(){ render(); }, 800);
      } else {
        setupStepError[6] = true;
        prog.appendChild(div("setup-status err",[msg || t("setup.mcp.failed")]));
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

/* ── Jobs View ── */

function renderJobs(root){
  var jobs = D.jobs || [];

  var header = div("view-header");
  header.appendChild(span("view-title", t("jobs.title")));
  header.appendChild(span("proj-count", t("jobs.count", {count: jobs.length})));
  var addBtn = el("button","btn btn-sm btn-primary");
  addBtn.textContent = t("jobs.create");
  addBtn.onclick = function(){ openJobModal(null); };
  header.appendChild(addBtn);
  root.appendChild(header);

  if(jobs.length === 0){
    var empty = div("jobs-empty");
    empty.textContent = t("jobs.empty");
    root.appendChild(empty);
    return;
  }

  var list = div("jobs-list");

  var thead = div("job-row job-row-header");
  thead.appendChild(span("job-col-name",t("jobs.col.name")));
  thead.appendChild(span("job-col-schedule",t("jobs.col.schedule")));
  thead.appendChild(span("job-col-project",t("jobs.col.project")));
  thead.appendChild(span("job-col-instruction",t("jobs.col.instruction")));
  thead.appendChild(span("job-col-status",t("jobs.col.status")));
  thead.appendChild(span("job-col-last",t("jobs.col.last_run")));
  thead.appendChild(span("job-col-actions",""));
  list.appendChild(thead);

  for(var i=0;i<jobs.length;i++){
    (function(j){
      var row = div("job-row");

      row.appendChild(span("job-col-name", j.name));

      var schedEl = span("job-col-schedule","");
      schedEl.title = j.schedule;
      schedEl.textContent = j.schedule_human || j.schedule;
      row.appendChild(schedEl);

      row.appendChild(span("job-col-project", j.project === "_system" ? t("jobs.system_project") : j.project));

      var instrEl = span("job-col-instruction","");
      instrEl.textContent = (j.instruction||"").length > 60 ? j.instruction.substring(0,60) + "..." : (j.instruction||"");
      instrEl.title = j.instruction || "";
      row.appendChild(instrEl);

      var statusBadge = span("job-status job-status-" + (j.status||"idle"), (j.status||"idle").toUpperCase());
      row.appendChild(span("job-col-status","", [statusBadge]));

      var lastEl = span("job-col-last","");
      if(j.last_run_at && j.last_run_at !== "0001-01-01T00:00:00Z"){
        var d = new Date(j.last_run_at);
        lastEl.textContent = d.toLocaleDateString() + " " + d.toLocaleTimeString([], {hour:"2-digit",minute:"2-digit"});
        if(j.last_result) lastEl.title = j.last_result;
      } else {
        lastEl.textContent = "\u2014";
      }
      row.appendChild(lastEl);

      var acts = div("job-col-actions");

      var toggleBtn = el("button","btn btn-sm " + (j.enabled ? "btn-success" : ""));
      toggleBtn.textContent = j.enabled ? "ON" : "OFF";
      toggleBtn.onclick = function(e){
        e.stopPropagation();
        api("POST","/api/jobs/update",{id:j.id, name:j.name, schedule:j.schedule, project:j.project, instruction:j.instruction, enabled:!j.enabled}, function(){ toast(j.enabled ? t("jobs.disabled") : t("jobs.enabled"),"info"); });
      };
      acts.appendChild(toggleBtn);

      var runBtn = el("button","btn btn-sm btn-primary");
      runBtn.textContent = t("task.run");
      runBtn.onclick = function(e){
        e.stopPropagation();
        var restore = btnLoading(runBtn, "...");
        api("POST","/api/jobs/run",{id:j.id}, function(){ if(restore) restore(); toast(t("jobs.triggered"),"success"); });
      };
      acts.appendChild(runBtn);

      var editBtn = el("button","btn btn-sm");
      editBtn.textContent = t("common.edit");
      editBtn.onclick = function(e){
        e.stopPropagation();
        openJobModal(j);
      };
      acts.appendChild(editBtn);

      var delBtn = el("button","btn btn-sm btn-danger");
      delBtn.textContent = t("template.delete_btn");
      delBtn.onclick = function(e){
        e.stopPropagation();
        if(!confirm(t("jobs.confirm_delete", {name: j.name}))) return;
        api("POST","/api/jobs/delete",{id:j.id}, function(){ toast(t("jobs.deleted"),"success"); });
      };
      acts.appendChild(delBtn);

      row.appendChild(acts);
      list.appendChild(row);
    })(jobs[i]);
  }
  root.appendChild(list);
}

function openJobModal(job){
  var projs = D.projects || [];
  var sel = document.getElementById("job-project");
  while(sel.firstChild) sel.removeChild(sel.firstChild);
  var sysOpt = document.createElement("option");
  sysOpt.value = "_system";
  sysOpt.textContent = t("jobs.system_project");
  sel.appendChild(sysOpt);
  for(var i=0;i<projs.length;i++){
    var opt = document.createElement("option");
    opt.value = projs[i].name;
    opt.textContent = projs[i].name;
    sel.appendChild(opt);
  }

  if(job){
    document.getElementById("job-modal-title").textContent = t("modal.job.edit_title");
    document.getElementById("job-edit-id").value = job.id;
    document.getElementById("job-name").value = job.name;
    document.getElementById("job-schedule").value = job.schedule;
    document.getElementById("job-project").value = job.project;
    document.getElementById("job-instruction").value = job.instruction;
    document.getElementById("job-enabled").value = job.enabled ? "true" : "false";
    document.getElementById("job-submit").textContent = t("modal.job.save");
    document.getElementById("job-submit").onclick = function(){ submitJobUpdate(); };
  } else {
    document.getElementById("job-modal-title").textContent = t("modal.job.new_title");
    document.getElementById("job-edit-id").value = "";
    document.getElementById("job-name").value = "";
    document.getElementById("job-schedule").value = "";
    document.getElementById("job-project").value = "_system";
    document.getElementById("job-instruction").value = "";
    document.getElementById("job-enabled").value = "true";
    document.getElementById("job-submit").textContent = t("modal.job.create");
    document.getElementById("job-submit").onclick = function(){ submitJobAdd(); };
  }
  openModal("modal-job");
}

function submitJobAdd(){
  var name = document.getElementById("job-name").value.trim();
  var schedule = document.getElementById("job-schedule").value.trim();
  var project = document.getElementById("job-project").value;
  var instruction = document.getElementById("job-instruction").value.trim();
  var enabled = document.getElementById("job-enabled").value === "true";
  if(!name || !schedule || !instruction){ toast(t("jobs.fill_all_fields"),"error"); return; }
  var btn = document.getElementById("job-submit");
  var restore = btnLoading(btn, t("modal.job.creating"));
  api("POST","/api/jobs/add",{name:name,schedule:schedule,project:project,instruction:instruction,enabled:enabled}, function(){
    if(restore) restore();
    closeModal("modal-job");
    toast(t("jobs.created"),"success");
  });
}

function submitJobUpdate(){
  var id = parseInt(document.getElementById("job-edit-id").value);
  var name = document.getElementById("job-name").value.trim();
  var schedule = document.getElementById("job-schedule").value.trim();
  var project = document.getElementById("job-project").value;
  var instruction = document.getElementById("job-instruction").value.trim();
  var enabled = document.getElementById("job-enabled").value === "true";
  if(!name || !schedule || !instruction){ toast(t("jobs.fill_all_fields"),"error"); return; }
  var btn = document.getElementById("job-submit");
  var restore = btnLoading(btn, t("modal.job.saving"));
  api("POST","/api/jobs/update",{id:id,name:name,schedule:schedule,project:project,instruction:instruction,enabled:enabled}, function(){
    if(restore) restore();
    closeModal("modal-job");
    toast(t("jobs.updated"),"success");
  });
}

function submitJob(){
  var editId = document.getElementById("job-edit-id").value;
  if(editId) submitJobUpdate();
  else submitJobAdd();
}

/* ── Config View ── */

function renderConfig(root){
  var header = div("view-header");
  header.appendChild(span("view-title", t("config.title")));
  root.appendChild(header);

  // Top tab bar: About | General | Templates
  var tabBar = div("config-tabs");
  [[t("config.tab.general"),"general"],[t("config.tab.marketplace"),"marketplace"],[t("config.tab.templates"),"templates"],[t("config.tab.setup"),"setup"],[t("config.tab.about"),"about"]].forEach(function(item){
    var label = item[0]; var key = item[1];
    var tb = el("button", "config-tab-btn" + (configTab === key ? " active" : ""), [label]);
    tb.onclick = function(){ configTab = key; configEditing = null; render(); };
    tabBar.appendChild(tb);
  });
  root.appendChild(tabBar);

  if(configTab === "about"){
    if(!configData){
      loadConfig(function(){ render(); });
      var ld = div("empty"); ld.textContent = t("config.loading"); root.appendChild(ld);
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
    loading.textContent = t("config.loading");
    root.appendChild(loading);
    return;
  }

  // Sub-tab bar: Paths | Server | Budget | Autopilot
  var subTabs = div("config-subtabs");
  [[t("config.subtab.paths"),"paths"],[t("config.subtab.server"),"server"],[t("config.subtab.limits"),"limits"],[t("config.subtab.autopilot"),"autopilot"]].forEach(function(pair){
    var tb = el("button", "config-subtab-btn" + (configSubTab === pair[1] ? " active" : ""), [pair[0]]);
    tb.onclick = function(){ configSubTab = pair[1]; configEditing = null; render(); };
    subTabs.appendChild(tb);
  });
  root.appendChild(subTabs);

  switch(configSubTab){
    case "paths":  renderConfigPaths(root); break;
    case "server": renderConfigServer(root); break;
    case "limits": renderConfigLimits(root); break;
    case "autopilot": renderConfigAutopilot(root); break;
    case "security": renderConfigLimits(root); break;
  }
}

function renderConfigAutopilot(root){
  var c = configData;
  var editing = configEditing === "autopilot";

  // ── Spawn Settings Section ──
  var sec = div("config-section");
  var hdr = div("section-header");
  hdr.appendChild(el("h3","config-section-title",[t("config.autopilot.spawn_settings")]));
  if(!editing){
    hdr.appendChild(iconBtn("pencil","Edit",function(){ configEditing = "autopilot"; render(); }));
  }
  sec.appendChild(hdr);
  var grid = div("config-grid");
  if(editing){
    // Model select
    var modelRow = div("config-field");
    var modelLbl = el("label","config-label",[t("config.autopilot.model")]);
    modelLbl.setAttribute("for","cfg-spawn_model");
    modelRow.appendChild(modelLbl);
    var modelSel = el("select","config-input");
    modelSel.id = "cfg-spawn_model";
    [[t("config.autopilot.model_opusplan"),"opusplan"],[t("config.autopilot.model_sonnet"),"sonnet"],[t("config.autopilot.model_opus"),"opus"]].forEach(function(opt){
      var o = el("option","",[ opt[0] ]);
      o.value = opt[1];
      if((c.spawn_model||"") === opt[1]) o.selected = true;
      modelSel.appendChild(o);
    });
    modelRow.appendChild(modelSel);
    grid.appendChild(modelRow);

    // Effort select
    var effortRow = div("config-field");
    var effortLbl = el("label","config-label",[t("config.autopilot.effort")]);
    effortLbl.setAttribute("for","cfg-spawn_effort");
    effortRow.appendChild(effortLbl);
    var effortSel = el("select","config-input");
    effortSel.id = "cfg-spawn_effort";
    [[t("config.autopilot.effort_inherit"),""],[ t("config.autopilot.effort_high"),"high"],[t("config.autopilot.effort_medium"),"medium"],[t("config.autopilot.effort_low"),"low"]].forEach(function(opt){
      var o = el("option","",[ opt[0] ]);
      o.value = opt[1];
      if((c.spawn_effort||"") === opt[1]) o.selected = true;
      effortSel.appendChild(o);
    });
    effortRow.appendChild(effortSel);
    grid.appendChild(effortRow);

    grid.appendChild(configInput("spawn_max_turns",t("config.autopilot.max_turns"), String(c.spawn_max_turns || 15)));
    grid.appendChild(configInput("spawn_step_timeout_min",t("config.autopilot.step_timeout"), String(c.spawn_step_timeout_min != null ? c.spawn_step_timeout_min : 4)));
    grid.appendChild(configInput("max_concurrent",t("config.autopilot.max_concurrent"), String(c.max_concurrent || 3)));

    // Autostart toggle
    var asRow = div("config-field");
    var asLbl = el("label","config-label",[t("config.autopilot.autopilot_autostart")]);
    asLbl.setAttribute("for","cfg-autopilot_autostart");
    asRow.appendChild(asLbl);
    var asCb = el("input","config-checkbox");
    asCb.type = "checkbox";
    asCb.id = "cfg-autopilot_autostart";
    asCb.checked = !!c.autopilot_autostart;
    asRow.appendChild(asCb);
    grid.appendChild(asRow);
  } else {
    var modelLabels = {"opusplan":t("config.autopilot.model_opusplan"),"sonnet":t("config.autopilot.model_sonnet"),"opus":t("config.autopilot.model_opus")};
    grid.appendChild(configReadRow(t("config.autopilot.model"), modelLabels[c.spawn_model] || c.spawn_model || t("config.autopilot.model_inherit")));
    grid.appendChild(configReadRow(t("config.autopilot.effort"), c.spawn_effort || t("config.autopilot.effort_inherit")));
    grid.appendChild(configReadRow(t("config.autopilot.max_turns"), String(c.spawn_max_turns || 15)));
    grid.appendChild(configReadRow(t("config.autopilot.step_timeout"), (c.spawn_step_timeout_min != null ? c.spawn_step_timeout_min : 4) + " min"));
    grid.appendChild(configReadRow(t("config.autopilot.max_concurrent"), String(c.max_concurrent || 3)));
    grid.appendChild(configReadRow(t("config.autopilot.autopilot_autostart"), c.autopilot_autostart ? t("common.yes") : t("common.no")));
  }
  sec.appendChild(grid);
  if(editing){
    var acts = div("config-actions");
    var cancelBtn = el("button","btn",[t("common.cancel")]);
    cancelBtn.onclick = function(){ configEditing = null; render(); };
    acts.appendChild(cancelBtn);
    var saveBtn = el("button","btn btn-primary",[t("common.save")]);
    saveBtn.onclick = function(){ saveAutopilotConfig(); };
    acts.appendChild(saveBtn);
    sec.appendChild(acts);
  }
  root.appendChild(sec);

  // ── Skeleton Steps Section ──
  var skSec = div("config-section");
  skSec.appendChild(el("h3","config-section-title",[t("config.skeleton.title")]));
  var skDesc = el("p","config-empty");
  skDesc.textContent = t("config.skeleton.title_desc");
  skSec.appendChild(skDesc);

  var sk = (c.skeleton || {});
  var skToggles = [
    {key:"investigate", label:t("config.skeleton.investigate"), desc:t("config.skeleton.investigate_desc"), tag:t("config.skeleton.investigate_tag"), locked:true},
    {key:"web_search", label:t("config.skeleton.web_search"), desc:t("config.skeleton.web_search_desc"), tag:t("config.skeleton.web_search_tag")},
    {key:"build_verify", label:t("config.skeleton.build_verify"), desc:t("config.skeleton.build_verify_desc"), tag:t("config.skeleton.build_verify_tag")},
    {key:"test", label:t("config.skeleton.test"), desc:t("config.skeleton.test_desc"), tag:t("config.skeleton.test_tag")},
    {key:"pre_commit", label:t("config.skeleton.pre_commit"), desc:t("config.skeleton.pre_commit_desc"), tag:t("config.skeleton.pre_commit_tag")},
    {key:"commit", label:t("config.skeleton.commit"), desc:t("config.skeleton.commit_desc"), tag:t("config.skeleton.commit_tag")},
    {key:"push", label:t("config.skeleton.push"), desc:t("config.skeleton.push_desc"), tag:t("config.skeleton.push_tag")}
  ];

  // Collect dynamic MCP skeleton steps
  var mcpSteps = [];
  var mcpServers = c.mcp_servers || {};
  for(var mcpName in mcpServers){
    var mcp = mcpServers[mcpName];
    if(mcp.skeleton_step && mcp.enabled){
      mcpSteps.push({mcpName: mcpName, step: mcp.skeleton_step, enabled: mcp.enabled});
    }
  }

  var skList = div("config-grid");

  // Render fixed skeleton toggles
  function renderSkToggle(sk_item){
    var row = div("mcp-toggle");
    var cb = el("input","");
    cb.type = "checkbox";
    cb.setAttribute("aria-label", sk_item.label);
    if(sk_item.locked){
      cb.checked = true;
      cb.disabled = true;
      cb.title = t("config.autopilot.always_enabled");
    } else {
      cb.checked = sk[sk_item.key] !== false;
      cb.onchange = function(){
        var full = {
          web_search: sk.web_search !== false,
          build_verify: sk.build_verify !== false,
          test: sk.test !== false,
          pre_commit: sk.pre_commit !== false,
          commit: sk.commit !== false,
          push: !!sk.push
        };
        full[sk_item.key] = cb.checked;
        api("POST","/api/config/save",{skeleton:full},function(d){
          if(d.ok){
            toast(t("config.skeleton.saved"),"success");
            configData = null;
            loadConfig(function(){ render(); });
          } else {
            toast(t("common.error", {error: d.error||"unknown"}),"error");
            cb.checked = !cb.checked;
          }
        });
      };
    }
    row.appendChild(cb);
    row.appendChild(span("mcp-server-name", sk_item.label));
    row.appendChild(span("mcp-server-cmd", sk_item.desc));
    if(sk_item.tag){
      row.appendChild(span("skeleton-tag", sk_item.tag));
    }
    skList.appendChild(row);
  }

  // First two fixed steps (Investigate + Web Search)
  renderSkToggle(skToggles[0]);
  renderSkToggle(skToggles[1]);

  // Dynamic MCP skeleton steps (after investigate, before build)
  mcpSteps.forEach(function(ms){
    var row = div("mcp-toggle");
    var cb = el("input","");
    cb.type = "checkbox";
    cb.checked = true;
    cb.setAttribute("aria-label", ms.step.label);
    cb.title = t("config.skeleton.mcp_tag", {name: ms.mcpName});
    cb.disabled = true; // controlled by MCP enabled/disabled
    row.appendChild(cb);
    row.appendChild(span("mcp-server-name", ms.step.label));
    row.appendChild(span("mcp-server-cmd", ms.step.description));
    row.appendChild(span("skeleton-tag mcp", t("config.skeleton.mcp_tag", {name: ms.mcpName})));
    skList.appendChild(row);
  });

  // Remaining fixed steps (Build, Test, Pre-commit, Commit, Push)
  for(var si=2; si<skToggles.length; si++){
    renderSkToggle(skToggles[si]);
  }
  skSec.appendChild(skList);
  root.appendChild(skSec);
}

function renderConfigMarketplace(root){
  if(!configData){
    loadConfig(function(){ render(); });
    var ld = div("empty"); ld.textContent = t("common.loading"); root.appendChild(ld);
    return;
  }

  var subTabs = div("config-subtabs");
  subTabs.setAttribute("role", "tablist");
  [[t("mkt.subtab.mcp"),"mcp"],[t("mkt.subtab.skills"),"skills"]].forEach(function(pair){
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

function formatInstalls(n){
  if(!n || n <= 0) return "";
  if(n >= 1000000) return (n/1000000).toFixed(1).replace(/\.0$/,"") + "M";
  if(n >= 1000) return (n/1000).toFixed(1).replace(/\.0$/,"") + "K";
  return String(n);
}

function mkLeaderboardRow(item, rank, type){
  var row = div("mkt-row");
  row.style.cursor = "pointer";
  row.onclick = (function(itm, tp){ return function(){
    selectedMktItem = itm;
    selectedMktType = tp;
    render();
  }; })(item, type);

  // Rank
  row.appendChild(span("mkt-rank", "#" + rank));

  // Icon
  var iconWrap = div("mkt-icon");
  if(type === "mcp" && item.icon){
    var img = document.createElement("img");
    img.src = item.icon;
    img.alt = item.name;
    img.onerror = function(){ iconWrap.textContent = "\u2699"; img.remove(); };
    iconWrap.appendChild(img);
  } else if(type === "mcp"){
    iconWrap.textContent = "\u2699";
  } else {
    iconWrap.textContent = "\u26A1";
  }
  row.appendChild(iconWrap);

  // Info column
  var info = div("mkt-info");
  info.appendChild(span("mkt-name", item.name));
  var source = type === "mcp" ? (item.package || "") : (item.source || "");
  if(source) info.appendChild(span("mkt-source", source));
  if(item.description) info.appendChild(span("mkt-desc", item.description));
  row.appendChild(info);

  // Meta badges
  var meta = div("mkt-meta");
  if(type === "mcp" && item.registry_type) meta.appendChild(span("mkt-version", item.registry_type));
  if(type === "mcp" && item.version) meta.appendChild(span("mkt-version", "v" + item.version));
  if(type === "skill" && item.installs > 0) meta.appendChild(span("mkt-installs", formatInstalls(item.installs)));
  if(item.env_vars && item.env_vars.length > 0){
    var envNames = item.env_vars.map(function(v){ return v.name + (v.is_required ? "*" : ""); }).join(", ");
    meta.appendChild(span("mkt-env-hint", t("mkt.catalog.env_hint", {names: envNames})));
  }
  row.appendChild(meta);

  // Action
  var action = div("mkt-action");
  if(item.installed){
    action.appendChild(span("mkt-installed-badge", t("mkt.catalog.installed")));
  } else {
    var installBtn = el("button","btn btn-success btn-sm",[t("mkt.catalog.install")]);
    installBtn.onclick = function(e){ e.stopPropagation(); };
    if(type === "mcp"){
      installBtn.onclick = (function(s, btn){
        return function(e){
          e.stopPropagation();
          if(s.env_vars && s.env_vars.length > 0 && s.env_vars.some(function(v){ return v.is_required; })){
            showEnvVarsPrompt(s, btn);
            return;
          }
          doInstall(s.name, s.package, s.registry_type, {}, btn);
        };
      })(item, installBtn);
    } else {
      installBtn.onclick = (function(skill, btn){
        return function(e){
          e.stopPropagation();
          var restore = btnLoading(btn, t("mkt.catalog.installing"));
          api("POST","/api/skills/install",{id: skill.source},function(d){
            if(restore) restore();
            if(d.ok){
              skillsData = null; skillsCatalogLoaded = false; skillsCatalogResults = null; skillsCatalogSearch = "";
              toast(t("mkt.catalog.installed_toast", {name: skill.name}), "success");
              render();
            } else {
              toast(t("common.error", {error: d.error || "unknown"}), "error");
            }
          });
        };
      })(item, installBtn);
    }
    action.appendChild(installBtn);
  }
  row.appendChild(action);

  return row;
}

function renderMktDetail(root, item, type){
  // Back button
  var backBtn = el("button","btn btn-sm",  [t("mkt.back")]);
  backBtn.onclick = function(){ selectedMktItem = null; selectedMktType = ""; render(); };
  root.appendChild(backBtn);

  // Header card
  var header = div("mkt-detail-header");
  var iconWrap = div("mkt-detail-icon");
  if(type === "mcp" && item.icon){
    var img = document.createElement("img");
    img.src = item.icon;
    img.alt = item.name;
    img.onerror = function(){ iconWrap.textContent = "\u2699"; img.remove(); };
    iconWrap.appendChild(img);
  } else if(type === "mcp"){
    iconWrap.textContent = "\u2699";
  } else {
    iconWrap.textContent = "\u26A1";
  }
  header.appendChild(iconWrap);

  var hInfo = div("mkt-detail-hinfo");
  // Title (display name) or fallback to name
  var displayName = (type === "mcp" && item.title) ? item.title : item.name;
  hInfo.appendChild(el("h2","mkt-detail-name",[displayName]));
  // Subtitle: reverse-DNS name for MCP (if title differs), source for skill
  if(type === "mcp"){
    if(item.title && item.title !== item.name) hInfo.appendChild(span("mkt-detail-subtitle", item.name));
    if(item.package) hInfo.appendChild(span("mkt-detail-source", item.package));
  } else {
    if(item.source){
      var srcLink = document.createElement("a");
      srcLink.className = "mkt-detail-source mkt-detail-link-inline";
      srcLink.href = "https://github.com/" + item.source;
      srcLink.target = "_blank";
      srcLink.rel = "noopener";
      srcLink.textContent = item.source;
      hInfo.appendChild(srcLink);
    }
  }

  // Author from repo
  if(type === "mcp" && item.repository_url){
    var authorMatch = item.repository_url.match(/github\.com\/([^\/]+)/);
    if(authorMatch) hInfo.appendChild(span("mkt-detail-author", "by " + authorMatch[1]));
  }

  var badges = div("mkt-detail-badges");
  if(type === "mcp" && item.registry_type) badges.appendChild(span("mkt-version", item.registry_type));
  if(type === "mcp" && item.version) badges.appendChild(span("mkt-version", "v" + item.version));
  if(type === "mcp" && item.runtime_hint) badges.appendChild(span("mkt-version", item.runtime_hint));
  if(type === "mcp" && item.status) badges.appendChild(span("mkt-version mkt-badge-status", item.status));
  if(type === "mcp" && item.is_latest) badges.appendChild(span("mkt-installed-badge", t("mkt.catalog.latest")));
  if(type === "skill" && item.installs > 0) badges.appendChild(span("mkt-installs", t("mkt.catalog.installs", {count: formatInstalls(item.installs)})));
  if(item.installed) badges.appendChild(span("mkt-installed-badge", t("mkt.catalog.installed")));
  hInfo.appendChild(badges);

  // Dates
  if(type === "mcp" && (item.published_at || item.updated_at)){
    var dates = div("mkt-detail-dates");
    if(item.published_at) dates.appendChild(span("mkt-detail-date", t("mkt.catalog.published", {date: fmtDate(item.published_at)})));
    if(item.updated_at) dates.appendChild(span("mkt-detail-date", t("mkt.catalog.updated", {date: fmtDate(item.updated_at)})));
    hInfo.appendChild(dates);
  }

  header.appendChild(hInfo);
  root.appendChild(header);

  // Links section (MCP only)
  if(type === "mcp" && (item.repository_url || item.website_url)){
    var linksSec = div("mkt-detail-links");
    if(item.repository_url){
      var repoLink = document.createElement("a");
      repoLink.className = "mkt-detail-link";
      repoLink.href = item.repository_url;
      repoLink.target = "_blank";
      repoLink.rel = "noopener";
      repoLink.textContent = "\uD83D\uDCE6 Repository";
      linksSec.appendChild(repoLink);
    }
    if(item.website_url){
      var webLink = document.createElement("a");
      webLink.className = "mkt-detail-link";
      webLink.href = item.website_url;
      webLink.target = "_blank";
      webLink.rel = "noopener";
      webLink.textContent = "\uD83C\uDF10 Website";
      linksSec.appendChild(webLink);
    }
    root.appendChild(linksSec);
  }

  // Description
  if(item.description){
    var descSec = div("mkt-detail-section");
    descSec.appendChild(el("h3","mkt-detail-label",[t("mkt.catalog.description")]));
    descSec.appendChild(el("p","mkt-detail-text",[item.description]));
    root.appendChild(descSec);
  }

  // Env vars (MCP only)
  if(type === "mcp" && item.env_vars && item.env_vars.length > 0){
    var envSec = div("mkt-detail-section");
    envSec.appendChild(el("h3","mkt-detail-label",[t("mkt.catalog.env_vars")]));
    var envList = div("mkt-detail-env-list");
    item.env_vars.forEach(function(v){
      var envRow = div("mkt-detail-env-row");
      envRow.appendChild(span("mkt-detail-env-name", v.name));
      var tags = [];
      if(v.is_required) tags.push(t("common.required"));
      else tags.push(t("common.optional"));
      if(v.is_secret) tags.push(t("common.secret"));
      if(v.default) tags.push("default: " + v.default);
      envRow.appendChild(span("mkt-detail-env-tags", tags.join(" \u00b7 ")));
      if(v.description) envRow.appendChild(span("mkt-detail-env-desc", v.description));
      envList.appendChild(envRow);
    });
    envSec.appendChild(envList);
    root.appendChild(envSec);
  }

  // Actions
  var actSec = div("mkt-detail-actions");
  if(item.installed){
    var unBtn = el("button","btn btn-danger",[t("common.uninstall")]);
    unBtn.onclick = (function(itm, tp, btn){
      return function(){
        if(!confirm(t("mkt.catalog.uninstall_confirm", {name: itm.name}))) return;
        var restore = btnLoading(btn, t("mkt.catalog.removing"));
        var endpoint = tp === "mcp" ? "/api/mcp/uninstall" : "/api/skills/uninstall";
        var payload = { name: itm.name };
        api("POST", endpoint, payload, function(d){
          if(restore) restore();
          if(d.ok){
            if(tp === "mcp"){
              mcpData = null; mcpCatalogLoaded = false;
            } else {
              skillsData = null; skillsCatalogLoaded = false; skillsCatalogResults = null; skillsCatalogSearch = "";
            }
            selectedMktItem = null; selectedMktType = "";
            toast(t("mkt.catalog.uninstalled", {name: itm.name}), "success");
            render();
          } else {
            toast(t("common.error", {error: d.error || "unknown"}), "error");
          }
        });
      };
    })(item, type, unBtn);
    actSec.appendChild(unBtn);
  } else {
    if(type === "mcp"){
      // Env var inputs if required
      if(item.env_vars && item.env_vars.length > 0 && item.env_vars.some(function(v){ return v.is_required; })){
        var envForm = div("mkt-detail-env-form");
        var inputs = {};
        item.env_vars.forEach(function(v){
          var fRow = div("mkt-detail-env-input-row");
          var lbl = el("label","mkt-detail-env-label",[v.name + (v.is_required ? " *" : "")]);
          fRow.appendChild(lbl);
          var inp = el("input","mkt-detail-env-input");
          inp.type = v.is_secret ? "password" : "text";
          inp.placeholder = v.name;
          fRow.appendChild(inp);
          inputs[v.name] = inp;
          envForm.appendChild(fRow);
        });
        actSec.appendChild(envForm);
        var instBtn = el("button","btn btn-success",[t("mkt.catalog.install")]);
        instBtn.onclick = (function(itm, ins, btn){
          return function(){
            var envVars = {};
            var missing = [];
            itm.env_vars.forEach(function(v){
              var val = ins[v.name].value.trim();
              if(val) envVars[v.name] = val;
              else if(v.is_required) missing.push(v.name);
            });
            if(missing.length > 0){
              toast(t("mkt.catalog.required_hint", {fields: missing.join(", ")}), "error");
              return;
            }
            doInstall(itm.name, itm.package, itm.registry_type, envVars, btn);
            selectedMktItem = null; selectedMktType = "";
          };
        })(item, inputs, instBtn);
        actSec.appendChild(instBtn);
      } else {
        var instBtn = el("button","btn btn-success",[t("mkt.catalog.install")]);
        instBtn.onclick = (function(itm, btn){
          return function(){
            doInstall(itm.name, itm.package, itm.registry_type, {}, btn);
            selectedMktItem = null; selectedMktType = "";
          };
        })(item, instBtn);
        actSec.appendChild(instBtn);
      }
    } else {
      var instBtn = el("button","btn btn-success",[t("mkt.catalog.install")]);
      instBtn.onclick = (function(skill, btn){
        return function(){
          var restore = btnLoading(btn, t("mkt.catalog.installing"));
          api("POST","/api/skills/install",{id: skill.source},function(d){
            if(restore) restore();
            if(d.ok){
              skillsData = null; skillsCatalogLoaded = false; skillsCatalogResults = null; skillsCatalogSearch = "";
              selectedMktItem = null; selectedMktType = "";
              toast(t("mkt.catalog.installed_toast", {name: skill.name}), "success");
              render();
            } else {
              toast(t("common.error", {error: d.error || "unknown"}), "error");
            }
          });
        };
      })(item, instBtn);
      actSec.appendChild(instBtn);
    }
  }
  root.appendChild(actSec);
}

function mkMarketplaceHeader(searchVal, onSearch, tabs, activeTab, onTab){
  var header = div("mkt-header");
  var searchInput = el("input","mkt-search");
  searchInput.type = "text";
  searchInput.placeholder = t("mkt.catalog.search_placeholder");
  searchInput.name = "search";
  searchInput.setAttribute("aria-label", t("mkt.catalog.search_aria"));
  searchInput.setAttribute("autocomplete", "off");
  searchInput.value = searchVal;
  searchInput.oninput = function(){ onSearch(searchInput.value); };
  header.appendChild(searchInput);

  var tabRow = div("mkt-tabs");
  tabs.forEach(function(tb){
    var btn = el("button","mkt-tab" + (tb.id === activeTab ? " active" : ""), [tb.label]);
    btn.onclick = function(){ onTab(tb.id); };
    tabRow.appendChild(btn);
  });
  header.appendChild(tabRow);
  return header;
}

function renderMarketplaceMCP(root){
  if(selectedMktItem && selectedMktType === "mcp"){
    renderMktDetail(root, selectedMktItem, "mcp");
    return;
  }
  if(!mcpData){
    api("GET","/api/mcp/list",null,function(d){ mcpData = d; render(); });
    root.appendChild(span("mkt-empty", t("mkt.catalog.loading_empty")));
    return;
  }

  // Auto-load catalog
  if(!mcpCatalogLoaded){
    loadMCPCatalog(mcpCatalogSearch, "");
    var shimmer = div("mkt-list mkt-loading");
    for(var si=0;si<5;si++){
      var ph = div("mkt-row");
      ph.textContent = "\u00A0";
      ph.style.height = "52px";
      shimmer.appendChild(ph);
    }
    root.appendChild(shimmer);
    return;
  }

  // Header: search + tabs
  var tabs = [{id:"trending",label:t("mkt.catalog.tab.trending")},{id:"all",label:t("mkt.catalog.tab.all")},{id:"installed",label:t("mkt.catalog.tab.installed")}];
  root.appendChild(mkMarketplaceHeader(
    mcpCatalogSearch,
    function(val){
      mcpCatalogSearch = val;
      clearTimeout(mcpSearchTimer);
      mcpSearchTimer = setTimeout(function(){
        mcpCatalogCursor = "";
        loadMCPCatalog(val, "");
      }, 300);
    },
    tabs, mcpCatalogTab,
    function(tab){ mcpCatalogTab = tab; render(); }
  ));

  // Filter items by tab
  var items = mcpCatalogResults || [];
  if(mcpCatalogTab === "installed"){
    items = items.filter(function(i){ return i.installed; });
  } else if(mcpCatalogTab === "trending"){
    items = items.slice().sort(function(a,b){
      if(a.updated_at && b.updated_at) return b.updated_at.localeCompare(a.updated_at);
      return 0;
    });
  }

  // Leaderboard
  if(items.length === 0){
    root.appendChild(span("mkt-empty", mcpCatalogTab === "installed" ? t("mkt.catalog.no_mcp_installed") : t("mkt.catalog.no_mcp")));
  } else {
    var list = div("mkt-list");
    items.forEach(function(item, i){ list.appendChild(mkLeaderboardRow(item, i + 1, "mcp")); });
    root.appendChild(list);
  }

  // Load More
  if(mcpCatalogHasMore && mcpCatalogTab !== "installed"){
    var moreWrap = div("mkt-load-more");
    var moreBtn = el("button","btn btn-sm",[t("mkt.catalog.load_more")]);
    moreBtn.onclick = function(){ btnLoading(moreBtn, t("mkt.catalog.loading")); loadMCPCatalog(mcpCatalogSearch, mcpCatalogCursor); };
    moreWrap.appendChild(moreBtn);
    root.appendChild(moreWrap);
  }

  // Separator + Manage section
  var sep = document.createElement("hr");
  sep.className = "mkt-separator";
  root.appendChild(sep);

  var manageSec = div("config-section");
  manageSec.appendChild(el("h3","config-section-title",[t("mkt.manage.title")]));

  if(mcpData.using_global){
    manageSec.appendChild(span("config-empty", t("mkt.manage.global_notice")));
    if(mcpData.global){
      var gList = div("config-grid");
      Object.keys(mcpData.global).forEach(function(name){
        var gs = mcpData.global[name];
        var row = div("mcp-toggle");
        row.appendChild(span("mcp-server-name", name));
        row.appendChild(span("mcp-server-cmd", gs.command + " " + (gs.args||[]).join(" ")));
        gList.appendChild(row);
      });
      manageSec.appendChild(gList);
    }
    var initBtn = el("button","btn btn-primary btn-sm",[t("mkt.manage.custom_config")]);
    initBtn.style.marginTop = "12px";
    initBtn.onclick = function(){
      var restore = btnLoading(initBtn, t("mkt.manage.initializing"));
      api("POST","/api/mcp/init",{},function(d){
        if(restore) restore();
        if(d.ok){ mcpData = null; toast(t("mkt.manage.initialized"),"success"); render(); }
        else { toast(t("common.error", {error: d.error||"unknown"}),"error"); }
      });
    };
    manageSec.appendChild(initBtn);
  } else {
    var servers = mcpData.custom || {};
    var names = Object.keys(servers);
    if(names.length === 0){
      manageSec.appendChild(span("config-empty", t("mkt.manage.no_servers")));
    } else {
      var toggleList = div("config-grid");
      names.forEach(function(name){
        var sv = servers[name];
        var row = div("mcp-toggle");
        var cb = el("input","");
        cb.type = "checkbox";
        cb.checked = sv.enabled;
        cb.setAttribute("aria-label", t("mkt.manage.toggle_aria", {name: name}));
        cb.onchange = function(){
          api("POST","/api/mcp/toggle",{name:name, enabled:cb.checked},function(d){
            if(d.ok){ mcpData = null; toast(t("mkt.manage.saved"),"success"); render(); }
            else { toast(t("common.error", {error: d.error||"unknown"}),"error"); cb.checked = !cb.checked; }
          });
        };
        row.appendChild(cb);
        row.appendChild(span("mcp-server-name", name));
        row.appendChild(span("mcp-server-cmd", sv.command + " " + (sv.args||[]).join(" ")));
        toggleList.appendChild(row);
      });
      manageSec.appendChild(toggleList);
    }
  }
  root.appendChild(manageSec);
}

function renderMarketplaceSkills(root){
  if(selectedMktItem && selectedMktType === "skill"){
    renderMktDetail(root, selectedMktItem, "skill");
    return;
  }
  if(!skillsData){
    api("GET","/api/skills/list",null,function(d){ skillsData = d.skills || []; render(); });
    root.appendChild(span("mkt-empty", t("mkt.catalog.loading_empty")));
    return;
  }

  // Auto-load catalog
  if(!skillsCatalogLoaded){
    loadSkillsCatalog(skillsCatalogSearch);
    var shimmer = div("mkt-list mkt-loading");
    for(var si=0;si<5;si++){
      var ph = div("mkt-row");
      ph.textContent = "\u00A0";
      ph.style.height = "52px";
      shimmer.appendChild(ph);
    }
    root.appendChild(shimmer);
    return;
  }

  // Header: search + tabs
  var tabs = [{id:"trending",label:t("mkt.catalog.tab.trending")},{id:"all",label:t("mkt.catalog.tab.all")},{id:"installed",label:t("mkt.catalog.tab.installed")}];
  root.appendChild(mkMarketplaceHeader(
    skillsCatalogSearch,
    function(val){
      skillsCatalogSearch = val;
      clearTimeout(skillsSearchTimer);
      skillsSearchTimer = setTimeout(function(){
        loadSkillsCatalog(val);
      }, 300);
    },
    tabs, skillsCatalogTab,
    function(tab){ skillsCatalogTab = tab; render(); }
  ));

  // Filter items by tab
  var items = skillsCatalogResults || [];
  if(skillsCatalogTab === "installed"){
    items = items.filter(function(i){ return i.installed; });
  } else if(skillsCatalogTab === "trending"){
    items = items.slice().sort(function(a,b){ return (b.installs || 0) - (a.installs || 0); });
  }

  // Leaderboard with progressive loading
  var visible = items.slice(0, skillsShowCount);
  if(visible.length === 0){
    root.appendChild(span("mkt-empty", skillsCatalogTab === "installed" ? t("mkt.catalog.no_skills_installed") : t("mkt.catalog.no_skills")));
  } else {
    var list = div("mkt-list");
    visible.forEach(function(item, i){ list.appendChild(mkLeaderboardRow(item, i + 1, "skill")); });
    root.appendChild(list);
  }

  // Load More (client-side)
  if(items.length > skillsShowCount){
    var moreWrap = div("mkt-load-more");
    var remaining = items.length - skillsShowCount;
    var moreBtn = el("button","btn btn-sm",[t("mkt.catalog.load_more_count", {count: remaining})]);
    moreBtn.onclick = function(){ skillsShowCount += 100; render(); };
    moreWrap.appendChild(moreBtn);
    root.appendChild(moreWrap);
  }
}

function buildMarketplaceLink(label, subTab){
  var btn = el("button","skill-link-btn",[label]);
  btn.setAttribute("aria-label", t("mkt.go_to", {label: label}));
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
  c.spawn_step_timeout_min = parseInt(document.getElementById("cfg-spawn_step_timeout_min").value) || 0;
  c.max_concurrent = parseInt(document.getElementById("cfg-max_concurrent").value) || 3;
  c.autopilot_autostart = document.getElementById("cfg-autopilot_autostart").checked;
  var saveBtn = document.querySelector(".config-actions .btn-primary");
  var restore = btnLoading(saveBtn, t("common.saving"));
  api("POST","/api/config/save",c,function(){
    if(restore) restore();
    configEditing = null;
    toast(t("config.autopilot.saved"),"success");
    configData = null;
    loadConfig(function(){ render(); });
  });
}

function renderConfigAbout(root){
  var REPO = "https://github.com/JuanVilla424/teamoon";

  // Card 1: About — version with inline update
  var sec = div("config-section");
  sec.appendChild(el("h3","config-section-title",[t("config.about.title")]));
  var grid = div("config-grid");

  // Version row with inline update button
  var verRow = div("config-field");
  verRow.appendChild(span("config-label",t("config.about.version")));
  var verVal = div("config-value-inline");
  verVal.appendChild(span("config-value-text","v" + (D.version||"?") + " #" + (D.build_num||"0")));
  var updBtn = el("button","btn btn-sm",[t("config.update.check")]);
  updBtn.style.marginLeft = "12px";
  verVal.appendChild(updBtn);
  verRow.appendChild(verVal);
  grid.appendChild(verRow);

  grid.appendChild(configReadRow(t("config.about.plan_model"), D.plan_model));
  grid.appendChild(configReadRow(t("config.about.exec_model"), D.exec_model));
  grid.appendChild(configReadRow(t("config.about.effort"), D.effort));
  sec.appendChild(grid);

  // Inline update area (hidden until clicked)
  var updContent = div("config-update-area");
  updContent.style.display = "none";
  updBtn.onclick = function(){
    updContent.style.display = "";
    updBtn.style.display = "none";
    renderUpdateArea(updContent);
  };
  sec.appendChild(updContent);
  root.appendChild(sec);

  // Card 2: Repository
  var repoSec = div("config-section");
  repoSec.appendChild(el("h3","config-section-title",[t("config.about.repository")]));
  var repoGrid = div("config-grid");
  repoGrid.appendChild(configLinkRow(t("config.about.source_code"), t("config.about.source_code_desc"), REPO));
  repoGrid.appendChild(configLinkRow(t("config.about.issues"), t("config.about.issues_desc"), REPO + "/issues"));
  repoGrid.appendChild(configLinkRow(t("config.about.contributing"), t("config.about.contributing_desc"), REPO + "/blob/main/CONTRIBUTING.md"));
  repoGrid.appendChild(configLinkRow(t("config.about.license_label"), t("config.about.license"), REPO + "/blob/main/LICENSE"));
  repoSec.appendChild(repoGrid);
  root.appendChild(repoSec);
}

function renderUpdateArea(container){
  var checkBtn = el("button","btn btn-primary btn-sm",[t("config.update.check")]);
  var statusEl = div("update-status");
  var channelEl = div("update-channel");
  var actionEl = div("update-action");
  var progressEl = div("update-progress");

  if(updateRunning) checkBtn.disabled = true;

  function doCheck(branch){
    statusEl.textContent = "";
    channelEl.textContent = "";
    actionEl.textContent = "";
    progressEl.textContent = "";
    var restore = btnLoading(checkBtn, t("config.update.checking"));
    var url = "/api/update/check";
    if(branch) url += "?branch=" + encodeURIComponent(branch);
    api("GET", url, null, function(data){
      if(restore) restore();
      updateCheckResult = data;
      if(data.error){
        statusEl.textContent = t("common.error", {error: data.error});
        statusEl.className = "update-status error";
        return;
      }

      // Current version info
      var curVer = "v" + (data.current_version || "?");
      if(D.build_num && D.build_num !== "0") curVer += " #" + D.build_num;
      var curInfo = div("update-current");
      curInfo.appendChild(span("update-label",t("config.update.current_label")));
      curInfo.appendChild(span("update-value", curVer));
      curInfo.appendChild(span("update-meta",t("config.update.current_meta", {branch: data.current_branch, commit: data.local_commit})));
      statusEl.appendChild(curInfo);
      statusEl.className = "update-status";

      // Branch selector
      var label = span("",t("config.update.branch"));
      var sel = document.createElement("select");
      sel.className = "update-channel-select";
      var branches = data.branches || ["main"];
      for(var i = 0; i < branches.length; i++){
        var opt = document.createElement("option");
        opt.value = branches[i];
        opt.textContent = branches[i];
        if(branches[i] === data.current_branch) opt.textContent += t("config.update.branch_current");
        sel.appendChild(opt);
      }
      sel.value = data.selected_branch || data.current_branch || "main";
      channelEl.appendChild(label);
      channelEl.appendChild(sel);

      // Remote version info
      var remoteInfo = div("update-remote");
      var remVer = data.remote_version ? "v" + data.remote_version : "unknown";
      remoteInfo.appendChild(span("update-label",t("config.update.latest_label", {branch: data.selected_branch || "main"})));
      remoteInfo.appendChild(span("update-value", remVer));
      remoteInfo.appendChild(span("update-meta",t("config.update.remote_meta", {commit: data.remote_commit})));
      channelEl.appendChild(remoteInfo);

      // Behind info
      var infoEl = div("update-info");
      var b = data.behind || 0;
      var onSameBranch = data.current_branch === (data.selected_branch || "main");
      if(onSameBranch && b === 0){
        infoEl.textContent = t("config.update.already_up_to_date");
        infoEl.className = "update-info current";
      } else if(b > 0){
        infoEl.textContent = t("config.update.behind", {count: b, plural: b > 1 ? "s" : ""});
        infoEl.className = "update-info available";
      } else {
        infoEl.textContent = t("config.update.switch_branch", {branch: data.selected_branch || "main"});
        infoEl.className = "update-info available";
      }
      channelEl.appendChild(infoEl);

      // Re-check on branch change
      sel.onchange = function(){ doCheck(sel.value); };

      // Update button
      var updateBtn = el("button","btn btn-success btn-sm",[t("config.update.to_branch", {branch: data.selected_branch || "main"})]);
      updateBtn.onclick = function(){ runUpdate(progressEl, updateBtn, checkBtn, sel.value); };
      actionEl.appendChild(updateBtn);
    });
  }

  checkBtn.onclick = function(){
    if(updateRunning) return;
    updateCheckResult = null;
    doCheck("");
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
      progressEl.appendChild(div("setup-status ok",[t("config.update.complete")]));
      showReconnecting(progressEl);
    } else {
      progressEl.appendChild(div("setup-status err",[msg || t("config.update.failed")]));
      if(checkBtn) checkBtn.disabled = false;
    }
  });
}

function showReconnecting(container){
  var msg = div("update-reconnecting");
  msg.textContent = t("config.update.reconnecting");
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
    if(!res.ok){ res.text().then(function(txt){ onDone("error", txt); }); return; }
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
  {id:"prereqs", label:t("config.setup.tab.prereqs")},
  {id:"config", label:t("config.setup.tab.config")},
  {id:"skills", label:t("config.setup.tab.skills")},
  {id:"bmad", label:t("config.setup.tab.bmad")},
  {id:"hooks", label:t("config.setup.tab.hooks")},
  {id:"mcp", label:t("config.setup.tab.mcp")}
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
  container.appendChild(el("h3","config-section-title",[t("config.setup.prereqs.title")]));
  container.appendChild(el("p","config-section-desc",[t("config.setup.prereqs.desc")]));
  var prog = div("setup-progress");
  var actions = div("setup-prereqs-actions");
  var localMissing = [];
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",[t("config.setup.prereqs.check")]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    actions.textContent = "";
    localMissing = [];
    var restore = btnLoading(btn, t("setup.prereqs.checking"));
    streamStepStandalone("/api/onboarding/prereqs", null, prog, function(p, evt){
      if(evt.type !== "tool") return;
      var ok = evt.found;
      var cls = "setup-progress-item " + (ok ? "ok" : (evt.required ? "error" : "skip"));
      var item = div(cls);
      item.appendChild(span("icon", ok ? "\u2713" : (evt.required ? "\u2717" : "~")));
      item.appendChild(span("label", evt.name));
      item.appendChild(span("version", evt.version || (ok ? t("setup.prereqs.ok") : (evt.required ? t("setup.prereqs.not_found") : t("setup.prereqs.optional")))));
      if(!ok && evt.installable) item.appendChild(span("tag-installable",t("setup.prereqs.tag_installable")));
      p.appendChild(item);
      if(!ok) localMissing.push(evt);
    }, function(status, msg){
      running = false;
      if(restore) restore();
      if(status === "success"){
        prog.appendChild(div("setup-status ok",[t("config.setup.prereqs.tools_found")]));
      } else {
        var installable = localMissing.filter(function(m){ return m.installable; });
        if(installable.length > 0){
          var installBtn = el("button","btn btn-success btn-sm",[t("config.setup.prereqs.install_missing", {count: installable.length})]);
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
              if(s2 === "success") installProg.appendChild(div("setup-status ok",[t("config.setup.prereqs.tools_installed")]));
              else installProg.appendChild(div("setup-status err",[t("config.setup.prereqs.install_failed")]));
            });
          };
          actions.appendChild(installBtn);
        }
        prog.appendChild(div("setup-status err",[msg || t("config.setup.prereqs.missing_tools")]));
      }
    });
  };
  container.appendChild(btn);
  container.appendChild(actions);
  container.appendChild(prog);
}

function renderCfgSetupConfig(container){
  container.appendChild(el("h3","config-section-title",[t("config.setup.config.title")]));
  container.appendChild(el("p","config-section-desc",[t("config.setup.config.desc")]));
  var running = false;

  var grid = div("config-grid");
  var projInput = document.createElement("input"); projInput.type = "text"; projInput.className = "config-input"; projInput.value = (configData && configData.projects_dir) || "~/Projects"; projInput.placeholder = "~/Projects";
  var portInput = document.createElement("input"); portInput.type = "number"; portInput.className = "config-input"; portInput.value = (configData && configData.web_port) || 7777;
  var hostSelect = document.createElement("select"); hostSelect.className = "config-input";
  ["localhost","0.0.0.0"].forEach(function(v){ var o = document.createElement("option"); o.value = v; o.textContent = v; if(configData && configData.web_host === v) o.selected = true; hostSelect.appendChild(o); });
  if(!configData || !configData.web_host) hostSelect.value = "localhost";
  var pwInput = document.createElement("input"); pwInput.type = "password"; pwInput.className = "config-input"; pwInput.placeholder = t("config.setup.config.password_placeholder");
  var maxInput = document.createElement("input"); maxInput.type = "number"; maxInput.className = "config-input"; maxInput.value = (configData && configData.max_concurrent) || 3;

  grid.appendChild(configFieldRow(t("config.setup.config.projects_dir"), projInput));
  grid.appendChild(configFieldRow(t("config.setup.config.port"), portInput));
  grid.appendChild(configFieldRow(t("config.setup.config.bind"), hostSelect));
  grid.appendChild(configFieldRow(t("config.setup.config.password"), pwInput));
  grid.appendChild(configFieldRow(t("config.setup.config.max_concurrent"), maxInput));
  container.appendChild(grid);

  var statusEl = div("setup-progress");
  var btn = el("button","btn btn-primary btn-sm",[t("config.setup.config.save")]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    var restore = btnLoading(btn, t("common.saving"));
    var body = {projects_dir: projInput.value, web_port: parseInt(portInput.value)||7777, web_host: hostSelect.value || "localhost", web_password: pwInput.value, max_concurrent: parseInt(maxInput.value)||3};
    api("POST", "/api/onboarding/config", body, function(data){
      running = false;
      if(restore) restore();
      if(data && data.ok){
        statusEl.textContent = "";
        statusEl.appendChild(div("setup-status ok",[t("config.setup.config.saved")]));
        toast(t("config.setup.config.saved"),"success");
      } else {
        statusEl.appendChild(div("setup-status err",[(data && data.error) || t("config.setup.config.save_failed")]));
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
  container.appendChild(el("h3","config-section-title",[t("config.setup.skills.title")]));
  container.appendChild(el("p","config-section-desc",[t("config.setup.skills.desc")]));
  var prog = div("setup-progress");
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",[t("config.setup.skills.install")]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    var restore = btnLoading(btn, t("setup.installing"));
    streamStepStandalone("/api/onboarding/skills", null, prog, function(p, evt){
      if(evt.type === "symlink"){ p.appendChild(div("setup-progress-item ok",[span("icon","\u2713"), span("label",t("setup.skills.symlink"))])); return; }
      var cls = "setup-progress-item " + (evt.status === "done" ? "ok" : evt.status === "error" ? "error" : "skip");
      var item = div(cls);
      item.appendChild(span("icon", evt.status === "done" ? "\u2713" : evt.status === "error" ? "\u2717" : "~"));
      item.appendChild(span("label", evt.name || "skill"));
      if(evt.status === "skipped") item.appendChild(span("version",t("config.setup.skills.already_installed")));
      p.appendChild(item);
    }, function(status){
      running = false;
      if(restore) restore();
      if(status === "success") prog.appendChild(div("setup-status ok",[t("config.setup.skills.installed")]));
      else prog.appendChild(div("setup-status err",[t("config.setup.skills.install_failed")]));
    });
  };
  container.appendChild(btn);
  container.appendChild(prog);
}

function renderCfgSetupBMAD(container){
  container.appendChild(el("h3","config-section-title",[t("config.setup.bmad.title")]));
  container.appendChild(el("p","config-section-desc",[t("config.setup.bmad.desc")]));
  var prog = div("setup-progress");
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",[t("config.setup.bmad.install")]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    var restore = btnLoading(btn, t("setup.installing"));
    var progressBar = null;
    streamStepStandalone("/api/onboarding/bmad", null, prog, function(p, evt){
      if(evt.type === "symlink"){ p.appendChild(div("setup-progress-item ok",[span("icon","\u2713"), span("label",t("config.setup.bmad.symlink"))])); return; }
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
      if(status === "success") prog.appendChild(div("setup-status ok",[t("config.setup.bmad.installed")]));
      else prog.appendChild(div("setup-status err",[t("config.setup.bmad.install_failed")]));
    });
  };
  container.appendChild(btn);
  container.appendChild(prog);
}

function renderCfgSetupHooks(container){
  container.appendChild(el("h3","config-section-title",[t("config.setup.hooks.title")]));
  container.appendChild(el("p","config-section-desc",[t("config.setup.hooks.desc")]));
  var prog = div("setup-progress");
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",[t("config.setup.hooks.install")]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    var restore = btnLoading(btn, t("setup.installing"));
    streamStepStandalone("/api/onboarding/hooks", null, prog, function(p, evt){
      var item = div("setup-progress-item ok");
      item.appendChild(span("icon", "\u2713"));
      item.appendChild(span("label", evt.name || evt.type || "hook"));
      p.appendChild(item);
    }, function(status){
      running = false;
      if(restore) restore();
      if(status === "success") prog.appendChild(div("setup-status ok",[t("config.setup.hooks.installed")]));
      else prog.appendChild(div("setup-status err",[t("config.setup.hooks.install_failed")]));
    });
  };
  container.appendChild(btn);
  container.appendChild(prog);
}

function renderCfgSetupMCP(container){
  container.appendChild(el("h3","config-section-title",[t("config.setup.mcp.title")]));
  container.appendChild(el("p","config-section-desc",[t("config.setup.mcp.desc")]));
  var prog = div("setup-progress");
  var running = false;

  var btn = el("button","btn btn-primary btn-sm",[t("config.setup.mcp.install")]);
  btn.onclick = function(){
    if(running) return;
    running = true;
    prog.textContent = "";
    var restore = btnLoading(btn, t("setup.installing"));
    streamStepStandalone("/api/onboarding/mcp", null, prog, function(p, evt){
      var ok = evt.status === "done";
      var skipped = evt.status === "skipped";
      var cls = "setup-progress-item " + (ok ? "ok" : skipped ? "skip" : "error");
      var item = div(cls);
      item.appendChild(span("icon", ok ? "\u2713" : skipped ? "~" : "\u2717"));
      item.appendChild(span("label", evt.name || "server"));
      if(skipped) item.appendChild(span("version",t("config.setup.mcp.already_configured")));
      p.appendChild(item);
    }, function(status){
      running = false;
      if(restore) restore();
      if(status === "success") prog.appendChild(div("setup-status ok",[t("config.setup.mcp.installed")]));
      else prog.appendChild(div("setup-status err",[t("config.setup.mcp.install_failed")]));
    });
  };
  container.appendChild(btn);
  container.appendChild(prog);
}

function renderConfigPaths(root){
  var c = configData;
  var editing = configEditing === "paths";

  // ── Language Selector ──
  var langSec = div("config-section");
  var langGrid = div("config-grid");
  var langField = div("config-field");
  langField.appendChild(el("label","config-label",[t("config.paths.language")]));
  var langSel = document.createElement("select");
  langSel.className = "config-input";
  var langs = [["en","English"],["es","Español"],["fr","Français"],["de","Deutsch"],["it","Italiano"],["pt","Português"],["ja","日本語"],["zh","中文"]];
  langs.forEach(function(l){
    var o = document.createElement("option");
    o.value = l[0]; o.textContent = l[1];
    if(currentLocale() === l[0]) o.selected = true;
    langSel.appendChild(o);
  });
  langSel.onchange = function(){ setLocale(langSel.value); };
  langField.appendChild(langSel);
  langGrid.appendChild(langField);
  langSec.appendChild(langGrid);
  root.appendChild(langSec);

  var sec = div("config-section");
  var hdr = div("section-header");
  hdr.appendChild(el("h3","config-section-title",[t("config.paths.title")]));
  if(!editing){
    hdr.appendChild(iconBtn("pencil",t("common.edit"),function(){ configEditing = "paths"; render(); }));
  }
  sec.appendChild(hdr);
  var grid = div("config-grid");
  if(editing){
    grid.appendChild(configInput("projects_dir",t("config.paths.projects_dir"), c.projects_dir || ""));
    grid.appendChild(configInput("claude_dir",t("config.paths.claude_dir"), c.claude_dir || ""));
    grid.appendChild(configInput("refresh_interval_sec",t("config.paths.refresh_interval"), String(c.refresh_interval_sec || 30)));
  } else {
    grid.appendChild(configReadRow(t("config.paths.projects_dir"), c.projects_dir));
    grid.appendChild(configReadRow(t("config.paths.claude_dir"), c.claude_dir));
    grid.appendChild(configReadRow(t("config.paths.refresh_interval"), t("config.paths.refresh_interval_display", {value: c.refresh_interval_sec || 30})));
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
  hdr.appendChild(el("h3","config-section-title",[t("config.server.title")]));
  if(!editing){
    hdr.appendChild(iconBtn("pencil",t("common.edit"),function(){ configEditing = "server"; render(); }));
  }
  sec.appendChild(hdr);
  var grid = div("config-grid");
  if(editing){
    grid.appendChild(configInput("web_port",t("config.server.port"), String(c.web_port || 7777)));
    var hostField = div("config-field");
    hostField.appendChild(el("label","config-label",[t("config.server.bind_address")]));
    var hostSel = document.createElement("select"); hostSel.className = "config-input"; hostSel.id = "cfg-web_host";
    ["localhost","0.0.0.0"].forEach(function(v){ var o = document.createElement("option"); o.value = v; o.textContent = v; if(c.web_host === v) o.selected = true; hostSel.appendChild(o); });
    hostField.appendChild(hostSel);
    grid.appendChild(hostField);
    grid.appendChild(configInput("web_password",t("config.server.password"), c.web_password || "", "password"));
  } else {
    grid.appendChild(configReadRow(t("config.server.port"), String(c.web_port || 7777)));
    grid.appendChild(configReadRow(t("config.server.bind_address"), c.web_host || "localhost"));
    grid.appendChild(configReadRow(t("config.server.password"), c.web_password, true));
  }
  sec.appendChild(grid);
  if(editing) sec.appendChild(configEditActions("server"));
  root.appendChild(sec);

}

function renderConfigLimits(root){
  var c = configData;
  var editing = configEditing === "limits";
  var sec = div("config-section");
  var hdr = div("section-header");
  hdr.appendChild(el("h3","config-section-title",[t("config.limits.title")]));
  if(!editing){
    hdr.appendChild(iconBtn("pencil",t("common.edit"),function(){ configEditing = "limits"; render(); }));
  }
  sec.appendChild(hdr);
  var grid = div("config-grid");
  if(editing){
    grid.appendChild(configInput("context_limit",t("config.limits.context_limit"), String(c.context_limit || 0)));
    grid.appendChild(configInput("log_retention_days",t("config.limits.log_retention"), String(c.log_retention_days || 20)));
  } else {
    grid.appendChild(configReadRow(t("config.limits.context_limit"), t("config.limits.context_limit_display", {value: c.context_limit || 0})));
    grid.appendChild(configReadRow(t("config.limits.log_retention"), t("config.limits.log_retention_display", {value: c.log_retention_days || 20})));
  }
  sec.appendChild(grid);
  if(editing) sec.appendChild(configEditActions("limits"));
  root.appendChild(sec);

  // ── System Executor Section ──
  var seSec = div("config-section");
  var seHdr = div("section-header");
  seHdr.appendChild(el("h3","config-section-title",[t("config.system_executor")]));
  seSec.appendChild(seHdr);
  var seGrid = div("config-grid");
  var sudoRow = div("config-field");
  sudoRow.appendChild(el("label","config-label",[t("config.sudo.label")]));
  var sudoToggle = el("label","toggle-switch");
  var sudoCb = document.createElement("input");
  sudoCb.type = "checkbox";
  sudoCb.checked = !!c.sudo_enabled;
  sudoCb.onchange = function(){
    api("POST","/api/config/save",{sudo_enabled:sudoCb.checked},function(){
      toast(t("config.sudo.toast", {state: sudoCb.checked ? t("config.sudo.enabled") : t("config.sudo.disabled")}), "success");
      fetchConfig();
    });
  };
  sudoToggle.appendChild(sudoCb);
  sudoToggle.appendChild(el("span","toggle-slider"));
  sudoRow.appendChild(sudoToggle);
  sudoRow.appendChild(el("div","config-field-desc",[t("config.sudo.description")]));
  seGrid.appendChild(sudoRow);
  seSec.appendChild(seGrid);
  root.appendChild(seSec);

  // ── Active Guardrails Section ──
  var grSec = div("config-section");
  var grHdr = div("section-header");
  grHdr.appendChild(el("h3","config-section-title",[t("config.guardrails.title")]));
  grSec.appendChild(grHdr);
  var guardrails = [
    [t("config.guardrails.git_safety"), t("config.guardrails.git_safety_desc")],
    [t("config.guardrails.filesystem"), t("config.guardrails.filesystem_desc")],
    [t("config.guardrails.process_control"), t("config.guardrails.process_control_desc")],
    [t("config.guardrails.remote_exec"), t("config.guardrails.remote_exec_desc")],
    [t("config.guardrails.cloud_sql"), t("config.guardrails.cloud_sql_desc")]
  ];
  var grList = el("div","config-guardrails-list");
  guardrails.forEach(function(g){
    var item = div("config-guardrail-item");
    item.appendChild(el("span","guardrail-name",[g[0]]));
    item.appendChild(el("span","guardrail-desc",[g[1]]));
    grList.appendChild(item);
  });
  grSec.appendChild(grList);
  root.appendChild(grSec);
}

function configEditActions(section){
  var acts = div("config-actions");
  var cancelBtn = el("button","btn",[t("common.cancel")]);
  cancelBtn.onclick = function(){ configEditing = null; render(); };
  acts.appendChild(cancelBtn);
  var saveBtn = el("button","btn btn-primary",[t("common.save")]);
  saveBtn.onclick = function(){ saveConfigSection(section); };
  acts.appendChild(saveBtn);
  return acts;
}

function renderConfigTemplates(root){
  if(templatesCache === null && !templatesLoading){
    templatesLoading = true;
    api("GET","/api/templates/list",null,function(d){
      templatesLoading = false;
      templatesCache = d.templates || [];
      render();
    });
    var loadingEl = div("config-section");
    loadingEl.textContent = t("config.templates.loading");
    root.appendChild(loadingEl);
    return;
  }

  // Section header with add button
  var listSec = div("config-section");
  var header = div("section-header");
  header.appendChild(el("h3","config-section-title",[t("config.templates.title", {count: templatesCache.length})]));
  var addBtn = el("button", "btn btn-primary btn-sm", [t("config.templates.add")]);
  addBtn.onclick = function(){ openTemplateModal(null); };
  header.appendChild(addBtn);
  listSec.appendChild(header);

  if(templatesCache.length === 0){
    var empty = div("config-empty");
    empty.textContent = t("config.templates.empty");
    listSec.appendChild(empty);
  } else {
    templatesCache.forEach(function(tmpl){
      var row = div("tmpl-row");
      var info = div("tmpl-row-info");
      info.appendChild(span("tmpl-row-name", tmpl.name));
      info.appendChild(span("tmpl-row-preview", tmpl.content.length > 80 ? tmpl.content.substring(0,80) + "..." : tmpl.content));
      row.appendChild(info);

      var acts = div("tmpl-row-actions");
      acts.appendChild(iconBtn("pencil", t("template.edit_title"), function(){ openTemplateModal(tmpl); }));
      acts.appendChild(iconBtn("trash", t("template.delete_title"), function(){
        api("POST","/api/templates/delete",{id:tmpl.id},function(d){
          if(d.ok){
            templatesCache = templatesCache.filter(function(tmplItem){ return tmplItem.id !== tmpl.id; });
            toast(t("template.deleted"),"success");
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
  document.getElementById("tmpl-modal-title").textContent = tmpl ? t("modal.template.edit_title") : t("modal.template.new_title");
  document.getElementById("cfg-tmpl-name").value = tmpl ? tmpl.name : "";
  document.getElementById("cfg-tmpl-content").value = tmpl ? tmpl.content : "";
  document.getElementById("cfg-tmpl-submit").textContent = tmpl ? t("modal.template.update") : t("modal.template.add");
  openModal("modal-template");
  setTimeout(function(){ document.getElementById("cfg-tmpl-name").focus(); }, 100);
}
window.openTemplateModal = openTemplateModal;

function submitConfigTemplate(){
  var name = document.getElementById("cfg-tmpl-name").value.trim();
  var content = document.getElementById("cfg-tmpl-content").value.trim();
  if(!name || !content){ toast(t("modal.template.name_content_required"),"error"); return; }
  var btn = document.getElementById("cfg-tmpl-submit");
  var restore = btnLoading(btn, cfgEditingTemplate ? t("modal.template.saving") : t("modal.template.adding"));
  if(cfgEditingTemplate){
    api("POST","/api/templates/update",{id:cfgEditingTemplate.id, name:name, content:content},function(d){
      if(restore) restore();
      if(d.error){ toast(t("common.error", {error: d.error}),"error"); return; }
      cfgEditingTemplate = null;
      loadTemplates();
      closeModal("modal-template");
      toast(t("modal.template.updated"),"success");
    });
  } else {
    api("POST","/api/templates/add",{name:name, content:content},function(d){
      if(restore) restore();
      if(d.error){ toast(t("common.error", {error: d.error}),"error"); return; }
      loadTemplates();
      closeModal("modal-template");
      toast(t("modal.template.added"),"success");
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

function configLinkRow(label, text, href){
  var row = div("config-field");
  row.appendChild(span("config-label", label));
  var a = document.createElement("a");
  a.className = "config-value-link";
  a.href = href;
  a.target = "_blank";
  a.rel = "noopener";
  a.textContent = text;
  row.appendChild(a);
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
  } else if(section === "limits"){
    c.context_limit = parseInt(document.getElementById("cfg-context_limit").value) || 0;
    c.log_retention_days = parseInt(document.getElementById("cfg-log_retention_days").value) || 20;
  }
  var saveBtn = document.querySelector(".config-actions .btn-primary");
  var restore = btnLoading(saveBtn, t("common.saving"));
  api("POST","/api/config/save",c,function(){
    if(restore) restore();
    configEditing = null;
    toast(t("config.saved_toast"),"success");
    configData = null;
    loadConfig(function(){ render(); });
  });
}

/* ── Deferred setup init (after SETUP_STEPS defined) ── */
if(getView() === "setup") render();

/* ── Expose render for i18n locale switching ── */
window.render = render;

})();
