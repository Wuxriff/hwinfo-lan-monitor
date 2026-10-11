"use strict";

const $ = id => document.getElementById(id);

const store = {
  get(k, d) {
    try { const v = localStorage.getItem("hw." + k); return v === null ? d : JSON.parse(v); } catch { return d; }
  },
  set(k, v) {
    try { localStorage.setItem("hw." + k, JSON.stringify(v)); } catch { }
  },
  del(k) {
    try { localStorage.removeItem("hw." + k); } catch { }
  },
};

const I18N = {
  en: {
    sensor: "Sensor", cur: "Current", min: "Minimum", max: "Maximum", avg: "Average",
    search: "Search", settings: "Settings", theme: "Theme", themeAuto: "Auto (system)", themeLight: "Light", themeDark: "Dark",
    refresh: "Refresh interval", asHwinfo: "As HWiNFO", language: "Language", columns: "Columns",
    expandAll: "Expand all", collapseAll: "Collapse all", resetLayout: "Reset layout", close: "Close",
    up: "Up", down: "Down", polling: "HWiNFO polling", disconnected: "Disconnected, retrying...",
    starting: "Connecting to HWiNFO...",
    unavailable: "HWiNFO shared memory not found. Start HWiNFO with \"Shared Memory Support\" enabled and keep the sensors window open.",
    inactive: "HWiNFO shared memory is inactive: HWiNFO is closing or the 12-hour limit expired.",
    error: "Error reading HWiNFO shared memory: ",
  },
  ru: {
    sensor: "Датчик", cur: "Текущее", min: "Минимум", max: "Максимум", avg: "Среднее",
    search: "Поиск", settings: "Настройки", theme: "Тема", themeAuto: "Авто (системная)", themeLight: "Светлая", themeDark: "Тёмная",
    refresh: "Частота обновления", asHwinfo: "Как в HWiNFO", language: "Язык", columns: "Столбцы",
    expandAll: "Развернуть всё", collapseAll: "Свернуть всё", resetLayout: "Сбросить вид", close: "Закрыть",
    up: "Выше", down: "Ниже", polling: "Опрос HWiNFO", disconnected: "Нет связи, переподключение...",
    starting: "Подключение к HWiNFO...",
    unavailable: "Shared memory HWiNFO не найдена. Запустите HWiNFO с включённой опцией \"Shared Memory Support\" и не закрывайте окно сенсоров.",
    inactive: "Shared memory HWiNFO неактивна: HWiNFO закрывается или истёк 12-часовой лимит.",
    error: "Ошибка чтения shared memory HWiNFO: ",
  },
};

const COLS = ["cur", "min", "max", "avg"];
const REFRESH_STEPS = [0, 500, 1000, 2000, 5000, 10000];
const DECIMALS = { "V": 3, "W": 3, "A": 3, "MHz": 1, "°C": 1, "°F": 1, "%": 1, "x": 1, "FPS": 1, "ms": 3, "MB": 0, "GB": 0, "KB": 0, "RPM": 0, "T": 0, "MB/s": 3, "KB/s": 3, "GB/s": 3, "GT/s": 1 };

const settings = {
  lang: store.get("lang", (navigator.language || "en").toLowerCase().startsWith("ru") ? "ru" : "en"),
  theme: store.get("theme", "auto"),
  refresh: store.get("refresh", 0),
  cols: store.get("cols", null),
};
let toggled = new Set(store.get("toggled", []));

const t = k => I18N[settings.lang][k] ?? I18N.en[k];

// Model built from the "structure" event.
let model = null;
let values = null;
let prevValues = null;
let periodMs = 2000;
let query = "";

function defaultCols() {
  const narrow = window.matchMedia("(max-width: 600px)").matches;
  return COLS.map(id => ({ id, on: !narrow || id === "cur" || id === "max" }));
}
const visibleCols = () => (settings.cols ?? defaultCols()).filter(c => c.on).map(c => COLS.indexOf(c.id));

// --- formatting ---------------------------------------------------------

const fmtCache = new Map();
function formatter(unit) {
  let f = fmtCache.get(unit);
  if (f) return f;
  if (unit === "Yes/No") {
    f = v => (v ? "Yes" : "No");
  } else {
    const d = DECIMALS[unit] ?? 3;
    const nf = new Intl.NumberFormat("en-US", { minimumFractionDigits: d, maximumFractionDigits: d });
    const suffix = unit ? " " + unit : "";
    f = v => nf.format(v) + suffix;
  }
  fmtCache.set(unit, f);
  return f;
}

// --- open/closed state --------------------------------------------------

const defaultOpen = node => node.kind === "sensor";
const isOpen = node => defaultOpen(node) !== toggled.has(node.key);
function setOpen(node, open) {
  if (open === defaultOpen(node)) toggled.delete(node.key); else toggled.add(node.key);
  store.set("toggled", [...toggled]);
}

// --- rendering ----------------------------------------------------------

function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function nameCell(depth, twisty, icon, text) {
  const c = el("td", "name" + (depth ? " d" + depth : ""));
  if (twisty) c.append(el("span", "tw", "›"));
  c.append(el("span", "ic " + icon), document.createTextNode(text));
  c.title = text;
  return c;
}

function valueCells(tr) {
  const cells = [null, null, null, null];
  for (const k of visibleCols()) {
    const c = el("td", "v");
    tr.append(c);
    cells[k] = c;
  }
  return cells;
}

function buildModel(s) {
  const m = { sensors: [], readings: [], groups: [] };
  for (const se of s.sensors) {
    const sensor = { kind: "sensor", key: se.key, name: se.name, items: [], readings: [] };
    const starts = new Map(se.groups.map(g => [g.start, g]));
    for (let j = 0; j < se.readings.length;) {
      const g = starts.get(j);
      if (g) {
        const first = se.readings[j];
        const group = { kind: "group", key: first.key + ".g", name: g.name, sensor, type: first.type, unit: first.unit, members: [] };
        for (let k = j; k < j + g.count; k++) group.members.push(addReading(m, sensor, se.readings[k], group));
        m.groups.push(group);
        sensor.items.push(group);
        j += g.count;
      } else {
        sensor.items.push(addReading(m, sensor, se.readings[j], null));
        j++;
      }
    }
    m.sensors.push(sensor);
  }
  return m;
}

function addReading(m, sensor, r, group) {
  const reading = { kind: "reading", key: r.key, name: r.label, type: r.type, unit: r.unit, index: m.readings.length, sensor, group };
  m.readings.push(reading);
  sensor.readings.push(reading);
  return reading;
}

function renderHead() {
  document.documentElement.style.setProperty("--cols", visibleCols().length);
  const row = $("headRow");
  row.replaceChildren(el("th", "name", t("sensor")));
  for (const k of visibleCols()) {
    const th = el("th", "", t(COLS[k]));
    th.draggable = true;
    th.dataset.col = COLS[k];
    row.append(th);
  }
}

function renderRows() {
  if (!model) return;
  const frag = document.createDocumentFragment();
  model.sensors.forEach((sensor, i) => {
    if (i > 0) {
      const gap = el("tr", "gap");
      gap.append(el("td", "name"));
      for (const _ of visibleCols()) gap.append(el("td", "v"));
      frag.append(gap);
      sensor.gap = gap;
    }
    const head = el("tr", "sensor");
    head.append(nameCell(0, true, "sensor", sensor.name));
    valueCells(head);
    head.addEventListener("click", () => toggle(sensor));
    sensor.row = head;
    frag.append(head);
    for (const item of sensor.items) {
      if (item.kind === "group") {
        const gr = el("tr", "group");
        gr.append(nameCell(1, true, item.type, item.name));
        item.cells = valueCells(gr);
        gr.firstChild.addEventListener("click", () => toggle(item));
        item.row = gr;
        frag.append(gr);
        for (const r of item.members) frag.append(readingRow(r, 2));
      } else {
        frag.append(readingRow(item, 1));
      }
    }
  });
  $("rows").replaceChildren(frag);
  applyVisibility();
}

function readingRow(r, depth) {
  const tr = el("tr");
  tr.append(nameCell(depth, false, r.type, r.name));
  r.cells = valueCells(tr);
  r.row = tr;
  return tr;
}

function toggle(node) {
  if (query) return;
  setOpen(node, !isOpen(node));
  applyVisibility();
}

function applyVisibility() {
  if (!model) return;
  const q = query.toLowerCase();
  for (const sensor of model.sensors) {
    const sensorHit = q !== "" && sensor.name.toLowerCase().includes(q);
    const sOpen = q ? true : isOpen(sensor);
    let any = false;
    for (const item of sensor.items) {
      if (item.kind === "group") {
        const groupHit = sensorHit || (q !== "" && item.name.toLowerCase().includes(q));
        const gOpen = q ? true : isOpen(item);
        let anyMember = false;
        for (const r of item.members) {
          const hit = !q || groupHit || r.name.toLowerCase().includes(q);
          anyMember ||= hit;
          show(r, sOpen && gOpen && hit);
        }
        show(item, sOpen && (!q || anyMember));
        item.row.classList.toggle("open", gOpen);
        any ||= anyMember;
      } else {
        const hit = !q || sensorHit || item.name.toLowerCase().includes(q);
        any ||= hit;
        show(item, sOpen && hit);
      }
    }
    const sensorVisible = !q || any;
    sensor.row.hidden = !sensorVisible;
    if (sensor.gap) sensor.gap.hidden = !sensorVisible;
    sensor.row.classList.toggle("open", sOpen);
  }
  if (values) renderValues(true);
}

function show(node, visible) {
  node.visible = visible;
  node.row.hidden = !visible;
}

function setCell(c, text, changed) {
  if (!c) return;
  if (c.textContent !== text) c.textContent = text;
  if (c.classList.contains("chg") !== changed) c.classList.toggle("chg", changed);
}

function renderValues(force) {
  const v = values, p = prevValues;
  for (const r of model.readings) {
    if (!r.visible) continue;
    const f = formatter(r.unit), b = r.index * 4, c = r.cells;
    const changed = !force && p !== null && p[b] !== v[b];
    for (let k = 0; k < 4; k++) {
      const x = v[b + k];
      setCell(c[k], x === null ? "" : f(x), k === 0 && (force ? c[0]?.classList.contains("chg") : changed));
    }
  }
  for (const g of model.groups) {
    if (!g.visible) continue;
    const a = aggregate(g, v), f = formatter(g.unit);
    const changed = !force && p !== null && aggregate(g, p)[0] !== a[0];
    for (let k = 0; k < 4; k++) setCell(g.cells[k], f(a[k]), k === 0 && (force ? g.cells[0]?.classList.contains("chg") : changed));
  }
}

// Group row semantics as HWiNFO documents them: mean of members' current values,
// run-time min/max over all members, average of the group's current value over time.
function aggregate(g, v) {
  let cur = 0, min = Infinity, max = -Infinity, avg = 0;
  for (const r of g.members) {
    const b = r.index * 4;
    cur += v[b]; avg += v[b + 3];
    if (v[b + 1] < min) min = v[b + 1];
    if (v[b + 2] > max) max = v[b + 2];
  }
  const n = g.members.length;
  return [cur / n, min, max, avg / n];
}

// --- status -------------------------------------------------------------

let lastStatus = { status: "starting" };
function renderStatus(st) {
  lastStatus = st;
  const banner = $("banner");
  if (st.status === "ok") {
    banner.hidden = true;
    periodMs = st.periodMs || periodMs;
    $("status").textContent = "⟳ " + (periodMs / 1000) + " s";
    $("status").title = t("polling");
    fillRefresh();
    return;
  }
  banner.hidden = false;
  banner.textContent = t(st.status) + (st.status === "error" ? st.error : "");
  $("status").textContent = "";
}

// --- stream -------------------------------------------------------------

let es = null;
function connect() {
  if (es) es.close();
  const ms = settings.refresh > periodMs ? settings.refresh : 0;
  es = new EventSource("api/stream?interval=" + ms);
  es.addEventListener("status", e => renderStatus(JSON.parse(e.data)));
  es.addEventListener("structure", e => {
    const s = JSON.parse(e.data);
    periodMs = s.periodMs || periodMs;
    model = buildModel(s);
    values = prevValues = null;
    renderRows();
  });
  es.addEventListener("names", e => {
    if (!model) return;
    JSON.parse(e.data).forEach((name, i) => {
      const s = model.sensors[i];
      if (!s || s.name === name) return;
      s.name = name;
      const cell = s.row.firstChild;
      cell.lastChild.textContent = name;
      cell.title = name;
    });
  });
  es.addEventListener("values", e => {
    prevValues = values;
    values = JSON.parse(e.data).v;
    if (model) renderValues(false);
  });
  es.onerror = () => { $("status").textContent = t("disconnected"); };
}

document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    if (es) { es.close(); es = null; }
  } else if (!es) {
    connect();
  }
});

// --- settings UI --------------------------------------------------------

function applyTheme() {
  if (settings.theme === "auto") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = settings.theme;
}

function applyLanguage() {
  document.documentElement.lang = settings.lang;
  for (const e of document.querySelectorAll("[data-i18n]")) e.textContent = t(e.dataset.i18n);
  $("search").placeholder = t("search");
  $("settingsBtn").setAttribute("aria-label", t("settings"));
  renderHead();
  renderColList();
  fillRefresh();
  renderStatus(lastStatus);
}

function fillRefresh() {
  const sel = $("refreshSel");
  sel.replaceChildren();
  for (const ms of REFRESH_STEPS) {
    if (ms !== 0 && ms < periodMs) continue;
    const o = el("option", "", ms === 0 ? `${t("asHwinfo")} (${periodMs / 1000} s)` : `${ms / 1000} s`);
    o.value = ms;
    sel.append(o);
  }
  sel.value = settings.refresh >= periodMs ? settings.refresh : 0;
}

function renderColList() {
  const cols = settings.cols ?? defaultCols();
  const list = $("colList");
  list.replaceChildren();
  cols.forEach((c, i) => {
    const li = el("li");
    const lab = el("label");
    const cb = el("input");
    cb.type = "checkbox";
    cb.checked = c.on;
    cb.addEventListener("change", () => updateCols(cs => { cs[i].on = cb.checked; }));
    lab.append(cb, document.createTextNode(t(c.id)));
    const up = el("button", "", "▲");
    up.type = "button"; up.disabled = i === 0; up.title = t("up");
    up.addEventListener("click", () => updateCols(cs => cs.splice(i - 1, 0, cs.splice(i, 1)[0])));
    const down = el("button", "", "▼");
    down.type = "button"; down.disabled = i === cols.length - 1; down.title = t("down");
    down.addEventListener("click", () => updateCols(cs => cs.splice(i + 1, 0, cs.splice(i, 1)[0])));
    li.append(lab, up, down);
    list.append(li);
  });
}

function updateCols(fn) {
  const cs = structuredClone(settings.cols ?? defaultCols());
  fn(cs);
  settings.cols = cs;
  store.set("cols", cs);
  renderHead();
  renderColList();
  renderRows();
}

function setAll(open) {
  if (!model) return;
  for (const n of [...model.sensors, ...model.groups]) {
    if (open === defaultOpen(n)) toggled.delete(n.key); else toggled.add(n.key);
  }
  store.set("toggled", [...toggled]);
  applyVisibility();
}

// iOS Safari opens a <select> picker as soon as it gets focus, so the dialog must not focus its first control.
$("settingsBtn").addEventListener("click", () => {
  $("settings").showModal();
  const active = document.activeElement;
  if (active && active.tagName === "SELECT") active.blur();
});
$("settings").addEventListener("click", e => { if (e.target === $("settings")) $("settings").close(); });
$("themeSel").addEventListener("change", e => { settings.theme = e.target.value; store.set("theme", settings.theme); applyTheme(); });
$("langSel").addEventListener("change", e => { settings.lang = e.target.value; store.set("lang", settings.lang); applyLanguage(); renderRows(); });
$("refreshSel").addEventListener("change", e => { settings.refresh = +e.target.value; store.set("refresh", settings.refresh); connect(); });
$("expandAll").addEventListener("click", () => setAll(true));
$("collapseAll").addEventListener("click", () => setAll(false));
$("resetLayout").addEventListener("click", () => {
  settings.cols = null;
  store.del("cols");
  toggled = new Set();
  store.del("toggled");
  renderHead();
  renderColList();
  renderRows();
});

let searchTimer = 0;
$("search").addEventListener("input", e => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => { query = e.target.value.trim(); applyVisibility(); }, 150);
});

// Column reordering by dragging header cells (desktop).
let dragCol = null;
$("headRow").addEventListener("dragstart", e => { dragCol = e.target.dataset?.col ?? null; });
$("headRow").addEventListener("dragover", e => {
  const th = e.target.closest("th[data-col]");
  if (!dragCol || !th) return;
  e.preventDefault();
  for (const x of $("headRow").children) x.classList.toggle("drop", x === th);
});
$("headRow").addEventListener("drop", e => {
  const th = e.target.closest("th[data-col]");
  for (const x of $("headRow").children) x.classList.remove("drop");
  if (!dragCol || !th || th.dataset.col === dragCol) return;
  e.preventDefault();
  const target = th.dataset.col, moving = dragCol;
  updateCols(cs => {
    const from = cs.findIndex(c => c.id === moving), to = cs.findIndex(c => c.id === target);
    cs.splice(to, 0, cs.splice(from, 1)[0]);
  });
});
$("headRow").addEventListener("dragend", () => { dragCol = null; });

$("themeSel").value = settings.theme;
$("langSel").value = settings.lang;
applyTheme();
applyLanguage();
connect();
