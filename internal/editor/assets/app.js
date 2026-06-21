// RetroTech episode editor — front-end. Talks to the Go sidecar over its local
// HTTP API (no Electron IPC); the same page works in a plain browser during
// development. The author edits structured fields and never sees markdown; the
// server composes the .md file.

const API = "/_write/api";

const $ = (id) => document.getElementById(id);
const els = {
  list: $("episode-list"),
  filter: $("filter"),
  empty: $("empty-state"),
  form: $("form"),
  formTitle: $("form-title"),
  status: $("status"),
  id: $("f-id"),
  audio: $("f-audio"),
  audioHelp: $("audio-help"),
  refs: $("refs"),
  refTemplate: $("ref-template"),
  preview: $("preview-panel"),
  frame: $("preview-frame"),
};

// state.current is the id of the loaded episode, or null while composing a new
// one (which switches save from PUT to POST and keeps the id field editable).
const state = { current: null, structured: true, episodes: [] };

// ---------- HTTP ----------

async function request(method, path, body) {
  const res = await fetch(API + path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  if (!res.ok) {
    let message = `요청 실패 (${res.status})`;
    try {
      const parsed = JSON.parse(text);
      if (parsed && parsed.error) message = parsed.error;
    } catch {
      if (text) message = text;
    }
    throw new Error(message);
  }
  return text;
}

async function apiJSON(method, path, body) {
  const text = await request(method, path, body);
  return text ? JSON.parse(text) : null;
}

// ---------- List ----------

async function loadList() {
  state.episodes = (await apiJSON("GET", "/episodes")) || [];
  renderList();
}

function renderList() {
  const q = els.filter.value.trim().toLowerCase();
  els.list.replaceChildren();
  for (const ep of state.episodes) {
    if (q && !`${ep.id} ${ep.title}`.toLowerCase().includes(q)) continue;
    const li = document.createElement("li");
    li.dataset.id = ep.id;
    if (ep.id === state.current) li.classList.add("active");
    const title = document.createElement("span");
    title.className = "ep-title";
    title.textContent = ep.title || ep.id;
    const meta = document.createElement("span");
    meta.className = "ep-meta";
    meta.textContent = `${ep.id} · ${ep.date}${ep.duration ? " · " + ep.duration : ""}`;
    li.append(title, meta);
    li.addEventListener("click", () => selectEpisode(ep.id));
    els.list.appendChild(li);
  }
}

function setActive(id) {
  for (const li of els.list.children) {
    li.classList.toggle("active", li.dataset.id === id);
  }
}

// ---------- Form ----------

function get(id) {
  return $(id).value;
}
function set(id, v) {
  $(id).value = v == null ? "" : v;
}
function clean(s) {
  return (s || "").replace(/\r\n/g, "\n");
}

function showForm() {
  els.empty.hidden = true;
  els.form.hidden = false;
}

function showEmpty() {
  els.form.hidden = true;
  els.empty.hidden = false;
  els.preview.hidden = true;
  setActive(null);
}

function setStatus(message, kind) {
  els.status.textContent = message || "";
  els.status.className = "status" + (kind ? " " + kind : "");
}

function toggleBody(structured) {
  $("structured-body").hidden = !structured;
  $("raw-body").hidden = structured;
}

function fillForm(f, isNew) {
  set("f-id", f.id);
  els.id.readOnly = !isNew;
  $("id-help").hidden = isNew;
  set("f-title", f.title);
  set("f-date", f.date);
  set("f-author", f.author);
  set("f-description", f.description);
  set("f-description2", f.description2);
  set("f-enclosure-url", f.enclosureUrl);
  set("f-enclosure-size", f.enclosureSize || "");
  set("f-duration", f.duration);
  const b = f.badges || {};
  set("f-apple", b.Apple);
  set("f-youtube", b.YouTube);
  set("f-spotify", b.Spotify);
  set("f-google", b.Google);
  set("f-rss", b.RSS);

  state.structured = f.structured !== false;
  toggleBody(state.structured);
  if (state.structured) {
    set("f-intro", f.intro);
    set("f-extra", f.extra);
    renderRefs(f.references || []);
  } else {
    set("f-rawbody", f.rawBody);
  }
  els.audioHelp.textContent = "";
  els.audio.value = "";
}

function readForm() {
  const form = {
    id: get("f-id").trim(),
    title: clean(get("f-title")),
    date: get("f-date").trim(),
    description: clean(get("f-description")),
    description2: clean(get("f-description2")),
    author: get("f-author").trim(),
    enclosureUrl: get("f-enclosure-url").trim(),
    enclosureSize: Number(get("f-enclosure-size")) || 0,
    duration: get("f-duration").trim(),
    badges: {
      Apple: get("f-apple").trim(),
      YouTube: get("f-youtube").trim(),
      Spotify: get("f-spotify").trim(),
      Google: get("f-google").trim(),
      RSS: get("f-rss").trim(),
    },
    structured: state.structured,
    intro: clean(get("f-intro")),
    references: readRefs(),
    extra: clean(get("f-extra")),
    rawBody: clean(get("f-rawbody")),
  };
  return form;
}

async function selectEpisode(id) {
  try {
    const f = await apiJSON("GET", `/episodes/${encodeURIComponent(id)}`);
    state.current = id;
    fillForm(f, false);
    els.formTitle.textContent = (f.title || "").trim() || id;
    showForm();
    setActive(id);
    setStatus("");
  } catch (err) {
    setStatus(err.message, "err");
  }
}

function todayDate() {
  const d = new Date();
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${p(d.getMonth() + 1)}/${p(d.getDate())}`;
}

function newEpisode() {
  state.current = null;
  fillForm(
    {
      id: "",
      title: "",
      date: todayDate(),
      author: "Outsider",
      badges: {},
      structured: true,
      references: [],
    },
    true,
  );
  els.formTitle.textContent = "새 에피소드";
  showForm();
  setActive(null);
  setStatus("");
  els.id.focus();
}

async function save(event) {
  event.preventDefault();
  const f = readForm();
  if (!f.id) {
    setStatus("ID를 입력하세요.", "err");
    els.id.focus();
    return;
  }
  try {
    if (state.current === null) {
      await apiJSON("POST", "/episodes", f);
    } else {
      await apiJSON("PUT", `/episodes/${encodeURIComponent(state.current)}`, f);
    }
    state.current = f.id;
    els.id.readOnly = true;
    $("id-help").hidden = false;
    els.formTitle.textContent = f.title.trim() || f.id;
    setStatus("저장했습니다.", "ok");
    await loadList();
    setActive(f.id);
  } catch (err) {
    setStatus(err.message, "err");
  }
}

async function remove() {
  if (state.current === null) {
    showEmpty();
    return;
  }
  if (!confirm(`'${state.current}' 에피소드를 삭제할까요?`)) return;
  try {
    await request("DELETE", `/episodes/${encodeURIComponent(state.current)}`);
    state.current = null;
    await loadList();
    showEmpty();
  } catch (err) {
    setStatus(err.message, "err");
  }
}

async function preview() {
  try {
    const html = await request("POST", "/preview", readForm());
    els.frame.srcdoc = html;
    els.preview.hidden = false;
  } catch (err) {
    setStatus(err.message, "err");
  }
}

// ---------- References ----------

function renderRefs(refs) {
  els.refs.replaceChildren();
  for (const r of refs) addRefRow(r);
}

function addRefRow(r) {
  r = r || { text: "", url: "", indent: 0 };
  const row = els.refTemplate.content.firstElementChild.cloneNode(true);
  row.querySelector(".ref-text").value = r.text || "";
  row.querySelector(".ref-url").value = r.url || "";
  const indent = row.querySelector(".ref-indent");
  indent.checked = (r.indent || 0) > 0;
  row.classList.toggle("nested", indent.checked);
  indent.addEventListener("change", () => row.classList.toggle("nested", indent.checked));
  row.querySelector(".ref-remove").addEventListener("click", () => row.remove());
  enableDrag(row);
  els.refs.appendChild(row);
  return row;
}

function readRefs() {
  return [...els.refs.querySelectorAll(".ref-row")]
    .map((row) => ({
      text: clean(row.querySelector(".ref-text").value),
      url: row.querySelector(".ref-url").value.trim(),
      indent: row.querySelector(".ref-indent").checked ? 1 : 0,
    }))
    .filter((r) => r.text !== "" || r.url !== "");
}

// Drag-to-reorder using the handle. The row becomes draggable only while the
// handle is pressed, so the text inputs stay normally selectable.
function enableDrag(row) {
  const handle = row.querySelector(".ref-handle");
  handle.addEventListener("mousedown", () => (row.draggable = true));
  row.addEventListener("dragstart", (e) => {
    row.classList.add("dragging");
    e.dataTransfer.effectAllowed = "move";
  });
  row.addEventListener("dragend", () => {
    row.classList.remove("dragging");
    row.draggable = false;
  });
}

els.refs.addEventListener("dragover", (e) => {
  e.preventDefault();
  const dragging = els.refs.querySelector(".dragging");
  if (!dragging) return;
  const after = dragAfter(e.clientY);
  if (after == null) els.refs.appendChild(dragging);
  else els.refs.insertBefore(dragging, after);
});

function dragAfter(y) {
  const rows = [...els.refs.querySelectorAll(".ref-row:not(.dragging)")];
  let closest = { offset: -Infinity, element: null };
  for (const row of rows) {
    const box = row.getBoundingClientRect();
    const offset = y - box.top - box.height / 2;
    if (offset < 0 && offset > closest.offset) closest = { offset, element: row };
  }
  return closest.element;
}

// ---------- Audio ----------

// A local mp3 fills the byte size (from the File) and the duration (from the
// decoded metadata, formatted MM:SS). The file itself is never uploaded — mp3
// hosting is separate; only these two values are read.
els.audio.addEventListener("change", () => {
  const file = els.audio.files[0];
  if (!file) return;
  set("f-enclosure-size", file.size);
  els.audioHelp.textContent = `${file.name} · ${file.size.toLocaleString()} bytes`;

  const url = URL.createObjectURL(file);
  const audio = new Audio();
  audio.preload = "metadata";
  audio.addEventListener("loadedmetadata", () => {
    URL.revokeObjectURL(url);
    if (Number.isFinite(audio.duration)) set("f-duration", formatDuration(audio.duration));
  });
  audio.addEventListener("error", () => URL.revokeObjectURL(url));
  audio.src = url;
});

function formatDuration(seconds) {
  const total = Math.round(seconds);
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

// While composing a new episode, derive the enclosure URL from the id (the mp3
// is hosted at <id>.mp3) so the author doesn't retype it.
els.id.addEventListener("input", () => {
  if (state.current !== null) return;
  const tmpl = (id) => `https://retrotech-episodes.outsider.dev/${id}.mp3`;
  const urlEl = $("f-enclosure-url");
  const isTemplate = /^https:\/\/retrotech-episodes\.outsider\.dev\/.*\.mp3$/.test(urlEl.value);
  if (urlEl.value === "" || isTemplate) {
    urlEl.value = els.id.value ? tmpl(els.id.value.trim()) : "";
  }
});

// ---------- Wire up ----------

$("btn-new").addEventListener("click", newEpisode);
$("btn-add-ref").addEventListener("click", () => addRefRow());
$("btn-delete").addEventListener("click", remove);
$("btn-preview").addEventListener("click", preview);
$("btn-close-preview").addEventListener("click", () => (els.preview.hidden = true));
els.form.addEventListener("submit", save);
els.filter.addEventListener("input", renderList);

loadList().catch((err) => setStatus(err.message, "err"));
