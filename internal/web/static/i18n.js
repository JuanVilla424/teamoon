/* ── i18n Engine ── */
(function() {
"use strict";

var _locale = localStorage.getItem("teamoon-lang") || "en";
var _translations = {};
var _fallback = {};

function loadSync(url) {
  var xhr = new XMLHttpRequest();
  xhr.open("GET", url, false);
  xhr.send(null);
  if (xhr.status === 200) {
    try { return JSON.parse(xhr.responseText); } catch(e) { return {}; }
  }
  return {};
}

_fallback = loadSync("/static/locales/en.json");
if (_locale !== "en") {
  _translations = loadSync("/static/locales/" + _locale + ".json");
} else {
  _translations = _fallback;
}

function pluralSuffix(locale, count) {
  if (locale === "ja" || locale === "zh") return "";
  return count !== 1 ? "s" : "";
}

window.t = function(key, vars) {
  var str = _translations[key] || _fallback[key] || key;
  if (!vars) return str;

  if (str.indexOf("{plural}") !== -1) {
    var c = (vars.count !== undefined) ? vars.count : 1;
    str = str.replace(/\{plural\}/g, pluralSuffix(_locale, c));
  }
  for (var k in vars) {
    if (vars.hasOwnProperty(k)) {
      str = str.replace(new RegExp("\\{" + k + "\\}", "g"), vars[k]);
    }
  }
  return str;
};

window.setLocale = function(lang) {
  if (lang === _locale) return;
  localStorage.setItem("teamoon-lang", lang);
  _locale = lang;
  if (lang === "en") {
    _translations = _fallback;
  } else {
    _translations = loadSync("/static/locales/" + lang + ".json");
  }
  if (typeof window.render === "function") window.render();
  applyStaticI18n();
};

window.currentLocale = function() { return _locale; };

function applyStaticI18n() {
  var els = document.querySelectorAll("[data-i18n]");
  for (var i = 0; i < els.length; i++) {
    var el = els[i];
    var key = el.getAttribute("data-i18n");
    if (el.tagName === "OPTION") {
      el.textContent = t(key);
    } else {
      el.textContent = t(key);
    }
  }
  var titles = document.querySelectorAll("[data-i18n-title]");
  for (var j = 0; j < titles.length; j++) {
    var k = titles[j].getAttribute("data-i18n-title");
    titles[j].title = t(k);
    titles[j].setAttribute("aria-label", t(k));
  }
  var phs = document.querySelectorAll("[data-i18n-placeholder]");
  for (var p = 0; p < phs.length; p++) {
    phs[p].placeholder = t(phs[p].getAttribute("data-i18n-placeholder"));
  }
}
window.applyStaticI18n = applyStaticI18n;

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", applyStaticI18n);
} else {
  applyStaticI18n();
}

})();
