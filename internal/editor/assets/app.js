// RetroTech episode editor — front-end. Talks to the Go sidecar over its local
// HTTP API (no Electron IPC); the same page works in a plain browser during
// development. The author edits structured fields and never sees markdown.
//
// "New episode" creates a *draft* (content/drafts, auto-saved, invisible to the
// site) rather than a published episode. Drafts are listed in their own sidebar
// section above the published episodes; publishing a draft writes
// content/episodes/<id>.md and removes the draft.

const API = "/_write/api";

const $ = (id) => document.getElementById(id);
const els = {
  list: $("episode-list"),
  draftList: $("draft-list"),
  draftsSection: $("drafts-section"),
  filter: $("filter"),
  empty: $("empty-state"),
  form: $("form"),
  formTitle: $("form-title"),
  status: $("status"),
  autosave: $("autosave"),
  btnSave: $("btn-save"),
  btnPublish: $("btn-publish"),
  id: $("f-id"),
  audio: $("f-audio"),
  audioHelp: $("audio-help"),
  refs: $("refs"),
  refTemplate: $("ref-template"),
  preview: $("preview-panel"),
  frame: $("preview-frame"),
};

// mode: 'episode' (editing a published episode) | 'draft' (editing a draft) |
// null (nothing open). current is the episode id or the draft slug.
const state = { mode: null, current: null, episodes: [], drafts: [] };

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

// ---------- Lists ----------

async function loadAll() {
  await Promise.all([loadDrafts(), loadEpisodes()]);
}

async function loadEpisodes() {
  state.episodes = (await apiJSON("GET", "/episodes")) || [];
  renderEpisodes();
}

async function loadDrafts() {
  state.drafts = (await apiJSON("GET", "/drafts")) || [];
  renderDrafts();
}

function listItem(title, meta) {
  const li = document.createElement("li");
  const t = document.createElement("span");
  t.className = "ep-title";
  t.textContent = title;
  const m = document.createElement("span");
  m.className = "ep-meta";
  m.textContent = meta;
  li.append(t, m);
  return li;
}

function renderEpisodes() {
  const q = els.filter.value.trim().toLowerCase();
  els.list.replaceChildren();
  for (const ep of state.episodes) {
    if (q && !`${ep.id} ${ep.title}`.toLowerCase().includes(q)) continue;
    const li = listItem(ep.title || ep.id, `${ep.id} · ${ep.date}${ep.duration ? " · " + ep.duration : ""}`);
    li.dataset.id = ep.id;
    if (state.mode === "episode" && ep.id === state.current) li.classList.add("active");
    li.addEventListener("click", () => selectEpisode(ep.id));
    els.list.appendChild(li);
  }
}

function renderDrafts() {
  els.draftList.replaceChildren();
  els.draftsSection.hidden = state.drafts.length === 0;
  for (const d of state.drafts) {
    const li = listItem(d.title || "(제목 없음)", "초안 · " + (d.id ? d.id : "ID 미정"));
    li.dataset.slug = d.slug;
    if (state.mode === "draft" && d.slug === state.current) li.classList.add("active");
    li.addEventListener("click", () => selectDraft(d.slug));
    els.draftList.appendChild(li);
  }
}

function setActive() {
  for (const li of els.list.children) {
    li.classList.toggle("active", state.mode === "episode" && li.dataset.id === state.current);
  }
  for (const li of els.draftList.children) {
    li.classList.toggle("active", state.mode === "draft" && li.dataset.slug === state.current);
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
  state.mode = null;
  state.current = null;
  setActive();
}

function setStatus(message, kind) {
  els.status.textContent = message || "";
  els.status.className = "status" + (kind ? " " + kind : "");
}

function setAutosave(message, kind) {
  els.autosave.textContent = message || "";
  els.autosave.className = "autosave" + (kind ? " " + kind : "");
}

function toggleBody(structured) {
  $("structured-body").hidden = !structured;
  $("raw-body").hidden = structured;
}

// applyMode toggles the bits of the form that differ between a draft and a
// published episode: a draft's id is editable and it publishes/auto-saves,
// while a published episode's id is locked (it's the RSS guid) and it saves.
function applyMode() {
  const draft = state.mode === "draft";
  els.id.readOnly = !draft;
  $("id-help").hidden = false;
  $("id-help").textContent = draft
    ? "발행하면 이 ID 로 에피소드(content/episodes/<ID>.md)가 만들어집니다."
    : "저장 후에는 변경할 수 없습니다 (RSS guid 보호).";
  els.btnSave.hidden = draft;
  els.btnPublish.hidden = !draft;
  els.autosave.hidden = !draft;
  if (!draft) setAutosave("");
}

function fillForm(f) {
  set("f-id", f.id);
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

  const structured = f.structured !== false;
  toggleBody(structured);
  state.structured = structured;
  if (structured) {
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
  return {
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
}

async function selectEpisode(id) {
  try {
    const f = await apiJSON("GET", `/episodes/${encodeURIComponent(id)}`);
    state.mode = "episode";
    state.current = id;
    fillForm(f);
    applyMode();
    els.formTitle.textContent = (f.title || "").trim() || id;
    showForm();
    setActive();
    setStatus("");
  } catch (err) {
    setStatus(err.message, "err");
  }
}

async function selectDraft(slug, prefetched) {
  try {
    const f = prefetched || (await apiJSON("GET", `/drafts/${encodeURIComponent(slug)}`));
    state.mode = "draft";
    state.current = slug;
    fillForm(f);
    applyMode();
    els.formTitle.textContent = (f.title || "").trim() || "새 초안";
    showForm();
    setActive();
    setStatus("");
    setAutosave("저장됨", "ok");
  } catch (err) {
    setStatus(err.message, "err");
  }
}

// newDraft creates a draft and opens it. "New episode" no longer writes a
// published episode directly — everything starts as a draft.
async function newDraft() {
  try {
    const res = await apiJSON("POST", "/drafts");
    await loadDrafts();
    await selectDraft(res.slug, res.form);
    els.id.focus();
  } catch (err) {
    setStatus(err.message, "err");
  }
}

// ---------- Auto-save (drafts only) ----------

let saveTimer = null;

function markDirty() {
  if (state.mode !== "draft") return;
  setAutosave("편집 중…");
  clearTimeout(saveTimer);
  saveTimer = setTimeout(flushDraftSave, 700);
}

async function flushDraftSave() {
  clearTimeout(saveTimer);
  if (state.mode !== "draft" || state.current == null) return;
  setAutosave("저장 중…");
  try {
    await apiJSON("PUT", `/drafts/${encodeURIComponent(state.current)}`, readForm());
    setAutosave("저장됨", "ok");
    // Update the draft's sidebar entry in place (title/id may have changed)
    // without re-sorting the list, so the cursor doesn't jump while typing.
    const li = [...els.draftList.children].find((x) => x.dataset.slug === state.current);
    if (li) {
      li.querySelector(".ep-title").textContent = get("f-title").trim() || "(제목 없음)";
      li.querySelector(".ep-meta").textContent = "초안 · " + (get("f-id").trim() || "ID 미정");
    }
  } catch (err) {
    setAutosave("저장 실패", "err");
    setStatus(err.message, "err");
  }
}

// ---------- Save / publish / delete ----------

async function save(event) {
  event.preventDefault();
  if (state.mode === "draft") {
    flushDraftSave();
    return;
  }
  if (state.mode !== "episode" || state.current == null) return;
  const f = readForm();
  try {
    await apiJSON("PUT", `/episodes/${encodeURIComponent(state.current)}`, f);
    els.formTitle.textContent = f.title.trim() || f.id;
    setStatus("저장했습니다.", "ok");
    await loadEpisodes();
    setActive();
  } catch (err) {
    setStatus(err.message, "err");
  }
}

async function publishDraft() {
  if (state.mode !== "draft" || state.current == null) return;
  if (!get("f-id").trim()) {
    setStatus("발행하려면 ID 를 입력하세요.", "err");
    els.id.focus();
    return;
  }
  try {
    await flushDraftSave(); // persist the latest form (incl. id) before publishing
    const res = await apiJSON("POST", `/drafts/${encodeURIComponent(state.current)}/publish`);
    await loadAll();
    await selectEpisode(res.id);
    setStatus("발행되었습니다.", "ok");
  } catch (err) {
    setStatus(err.message, "err");
  }
}

async function remove() {
  if (state.current == null) {
    showEmpty();
    return;
  }
  const isDraft = state.mode === "draft";
  const prompt = isDraft ? "이 초안을 삭제할까요?" : `'${state.current}' 에피소드를 삭제할까요?`;
  if (!confirm(prompt)) return;
  const path = isDraft
    ? `/drafts/${encodeURIComponent(state.current)}`
    : `/episodes/${encodeURIComponent(state.current)}`;
  try {
    await request("DELETE", path);
    if (isDraft) await loadDrafts();
    else await loadEpisodes();
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
  row.querySelector(".ref-remove").addEventListener("click", () => {
    row.remove();
    markDirty();
  });
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
    markDirty();
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
// decoded metadata, formatted MM:SS). The file itself is never uploaded.
els.audio.addEventListener("change", () => {
  const file = els.audio.files[0];
  if (!file) return;
  set("f-enclosure-size", file.size);
  els.audioHelp.textContent = `${file.name} · ${file.size.toLocaleString()} bytes`;
  markDirty();

  const url = URL.createObjectURL(file);
  const audio = new Audio();
  audio.preload = "metadata";
  audio.addEventListener("loadedmetadata", () => {
    URL.revokeObjectURL(url);
    if (Number.isFinite(audio.duration)) {
      set("f-duration", formatDuration(audio.duration));
      markDirty();
    }
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

// While composing a draft, derive the enclosure URL from the id (the mp3 is
// hosted at <id>.mp3) so the author doesn't retype it.
els.id.addEventListener("input", () => {
  if (state.mode !== "draft") return;
  const tmpl = (id) => `https://retrotech-episodes.outsider.dev/${id}.mp3`;
  const urlEl = $("f-enclosure-url");
  const isTemplate = /^https:\/\/retrotech-episodes\.outsider\.dev\/.*\.mp3$/.test(urlEl.value);
  if (urlEl.value === "" || isTemplate) {
    urlEl.value = els.id.value ? tmpl(els.id.value.trim()) : "";
  }
});

// ---------- Wire up ----------

$("btn-new").addEventListener("click", newDraft);
$("btn-add-ref").addEventListener("click", () => {
  addRefRow();
  markDirty();
});
$("btn-delete").addEventListener("click", remove);
$("btn-preview").addEventListener("click", preview);
$("btn-publish").addEventListener("click", publishDraft);
$("btn-close-preview").addEventListener("click", () => (els.preview.hidden = true));
els.form.addEventListener("submit", save);
els.form.addEventListener("input", markDirty);
els.form.addEventListener("change", markDirty);
els.filter.addEventListener("input", renderEpisodes);

loadAll().catch((err) => setStatus(err.message, "err"));
