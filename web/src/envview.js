// web/src/envview.js
/**
 * The .env editor.
 *
 * A .env file is a list of assignments, comments and blank lines. Editing it
 * as text means counting quotes and watching for a missing newline; editing
 * it as a list of keys and values does not. This view shows the file as a
 * table, and writes it back through the same format rules the server owns.
 *
 * Autosave is on by default: a change is written to disk a moment after the
 * last keystroke, so a .env file is never left behind in a half-edited state.
 */
import { $, $$, esc, S, doc_, api, apiPostJson, debounce } from './state.js';
import { vp, showToast, copyToClipboard } from './ui.js';
import { on, emit } from './bus.js';
import { syncEditUI } from './editview.js';
import { updateStatus } from './status.js';
import { drawTabs } from './tabs.js';

export const envViewEl = $('#envview');
const envRows = $('#envview-rows');
const envCountEl = $('#envview-count');
const envStatusEl = $('#envview-status');
const envFileEl = $('#envview-file');
const envFilterEl = $('#envview-filter');
const btnAdd = $('#envview-add');
const envSwitch = $('#env-switch');
const btnSave = $('#envview-save');

// Keys whose values are usually credentials. Their values are masked until
// the reader asks for them.
const SECRET_RE = /(secret|token|password|passwd|pwd|api[_-]?key|access[_-]?key|private[_-]?key|credential|auth)/i;

/** Whether changes are written to disk without asking. */
export const autosaveOn = () => {
  const v = S.settings?.['env.autosave'];
  return v !== false && v !== 'false';
};

/** How long to wait after the last keystroke before writing. */
export const autosaveDelay = () => {
  const v = parseInt(S.settings?.['env.autosaveDelay'], 10);
  return Number.isFinite(v) && v >= 0 ? v : 600;
};

// The pending write for each doc, so switching tabs does not lose a save.
const pending = new WeakMap();

/**
 * syncEnvView shows the .env editor when the active tab holds a .env file,
 * and takes it away when it does not.
 */
export function syncEnvView() {
  const d = doc_();
  const want = d && d.isEnv && !d.isEditing && !d.envRaw ? d : null;

  if (envViewEl) envViewEl.hidden = !want;
  if (want) {
    vp.hidden = true;
    if (envFileEl) envFileEl.textContent = want.path;
    loadEnvDoc(want);
    renderEnvTable(want);
  } else if (d && d.isEnv && d.envRaw) {
    // The raw view is the code viewport; make sure it is on screen.
    vp.hidden = false;
  }

  // The switch between the two views lives above the editor, so it stays
  // reachable in both.
  if (envSwitch) {
    const isEnv = !!(d && d.isEnv);
    envSwitch.hidden = !isEnv;
    for (const b of envSwitch.querySelectorAll('[data-env]')) {
      b.classList.toggle('active', (b.dataset.env === 'raw') === !!d?.envRaw);
    }
  }
}

/**
 * loadEnvDoc fetches the parsed contents of a .env file, once per tab.
 */
async function loadEnvDoc(d) {
  if (d.envLoaded) return;
  d.envLoaded = true;
  try {
    const j = await api('/api/env/get', { path: d.path });
    // The tab may have been closed or pointed elsewhere while this was in
    // flight.
    if (doc_() !== d) return;
    d.envEntries = j.entries || [];
    d.envTrailingNewline = j.trailingNewline !== false;
    d.envDirty = false;
    if (doc_() === d && !envViewEl?.hidden) renderEnvTable(d);
    updateEnvStatus(d);
  } catch (e) {
    d.envLoaded = false;
    d.envError = e.message;
    showToast('!', 'Failed to read ' + d.path + ': ' + e.message);
  }
}

/**
 * renderEnvTable draws the .env file as a table of keys and values.
 */
function renderEnvTable(d) {
  if (!envRows) return;
  envRows.replaceChildren();

  const filter = (envFilterEl?.value || '').trim().toLowerCase();
  let shown = 0;
  let pairs = 0;

  for (let i = 0; i < (d.envEntries || []).length; i++) {
    const e = d.envEntries[i];
    if (e.type === 'pair') pairs++;

    if (e.type === 'comment') {
      // A comment keeps its place in the file, so it keeps its place here.
      const row = document.createElement('div');
      row.className = 'env-row env-comment';
      row.dataset.idx = String(i);
      const c = document.createElement('span');
      c.className = 'env-comment-text';
      c.textContent = e.text || '';
      row.appendChild(c);
      envRows.appendChild(row);
      continue;
    }
    if (e.type === 'blank') {
      const row = document.createElement('div');
      row.className = 'env-row env-blank';
      row.dataset.idx = String(i);
      envRows.appendChild(row);
      continue;
    }

    // A pair.
    if (filter && !(e.key || '').toLowerCase().includes(filter) &&
        !(e.value || '').toLowerCase().includes(filter)) {
      continue;
    }
    shown++;
    envRows.appendChild(buildPairRow(d, e, i));
  }

  if (envCountEl) {
    envCountEl.textContent = pairs === 1 ? '1 variable' : pairs + ' variables';
  }
  if (!shown && filter) {
    const empty = document.createElement('div');
    empty.className = 'env-empty';
    empty.textContent = 'No variable matches "' + filter + '".';
    envRows.appendChild(empty);
  }
  updateEnvStatus(d);
}

/**
 * buildPairRow builds the row for one assignment: a key box, a value box and
 * the buttons that act on it.
 */
function buildPairRow(d, e, i) {
  const row = document.createElement('div');
  row.className = 'env-row env-pair';
  row.dataset.idx = String(i);

  const keyCell = document.createElement('div');
  keyCell.className = 'env-cell env-cell-key';
  const key = document.createElement('input');
  key.type = 'text';
  key.className = 'env-input env-key-input';
  key.value = e.key || '';
  key.spellcheck = false;
  key.autocomplete = 'off';
  key.setAttribute('aria-label', 'Variable name');
  key.addEventListener('input', () => {
    e.key = key.value;
    markDirty(d);
  });
  key.addEventListener('keydown', (ev) => {
    if (ev.key === 'Enter') {
      ev.preventDefault();
      const val = row.querySelector('.env-val-input');
      val?.focus();
      val?.select();
    }
  });
  keyCell.appendChild(key);

  const valCell = document.createElement('div');
  valCell.className = 'env-cell env-cell-val';
  const val = document.createElement('input');
  val.type = 'text';
  val.className = 'env-input env-val-input';
  val.value = e.value || '';
  val.spellcheck = false;
  val.autocomplete = 'off';
  val.setAttribute('aria-label', 'Variable value');

  // A value that looks like a credential is masked until it is asked for.
  let revealed = false;
  const isSecret = SECRET_RE.test(e.key || '');
  if (isSecret) {
    val.type = 'password';
    val.classList.add('env-secret');
  }
  val.addEventListener('input', () => {
    e.value = val.value;
    markDirty(d);
  });
  valCell.appendChild(val);

  if (isSecret) {
    const reveal = document.createElement('button');
    reveal.type = 'button';
    reveal.className = 'env-reveal';
    reveal.title = 'Show value';
    reveal.setAttribute('aria-label', 'Show value');
    reveal.innerHTML = '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>';
    reveal.addEventListener('click', () => {
      revealed = !revealed;
      val.type = revealed ? 'text' : 'password';
      reveal.classList.toggle('active', revealed);
      reveal.title = revealed ? 'Hide value' : 'Show value';
      if (revealed) { val.focus(); val.select(); }
    });
    valCell.appendChild(reveal);
  }

  const copy = document.createElement('button');
  copy.type = 'button';
  copy.className = 'env-reveal';
  copy.title = 'Copy value';
  copy.setAttribute('aria-label', 'Copy value');
  copy.innerHTML = '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';
  copy.addEventListener('click', () => copyToClipboard(e.value || '', 'Value copied'));

  const del = document.createElement('button');
  del.type = 'button';
  del.className = 'env-del';
  del.title = 'Remove this variable';
  del.setAttribute('aria-label', 'Remove this variable');
  del.innerHTML = '&times;';
  del.addEventListener('click', () => removeEntry(d, i));

  const actions = document.createElement('div');
  actions.className = 'env-cell env-cell-actions';
  actions.appendChild(copy);
  actions.appendChild(del);

  row.appendChild(keyCell);
  row.appendChild(valCell);
  row.appendChild(actions);
  return row;
}

/**
 * markDirty records that the file changed and schedules a write.
 */
function markDirty(d) {
  d.envDirty = true;
  d.envError = null;
  updateEnvStatus(d);

  if (!autosaveOn()) {
    // Autosave is off: the reader saves by hand.
    drawTabs();
    updateStatus();
    return;
  }
  let t = pending.get(d);
  if (t) clearTimeout(t);
  t = setTimeout(() => {
    pending.delete(d);
    saveEnvDoc(d, { silent: true });
  }, autosaveDelay());
  pending.set(d, t);
}

/**
 * removeEntry drops an assignment from the file.
 */
function removeEntry(d, i) {
  if (!d.envEntries || !d.envEntries[i]) return;
  const removed = d.envEntries.splice(i, 1)[0];
  renderEnvTable(d);
  markDirty(d);
  showToast('i', 'Removed ' + (removed.key || 'variable'));
}

/**
 * addEntry appends a new assignment to the file and puts the cursor in it.
 */
export function addEntry(d) {
  if (!d) return;
  if (!d.envEntries) d.envEntries = [];
  // A new key goes after the last assignment, so it lands with the other
  // values rather than below a trailing comment block.
  let at = d.envEntries.length;
  for (let i = d.envEntries.length - 1; i >= 0; i--) {
    if (d.envEntries[i].type === 'pair') { at = i + 1; break; }
  }
  const entry = { type: 'pair', key: '', value: '', quote: '' };
  d.envEntries.splice(at, 0, entry);
  renderEnvTable(d);
  markDirty(d);

  const row = envRows?.querySelector(`.env-pair[data-idx="${at}"] .env-key-input`);
  row?.focus();
}

/**
 * saveEnvDoc writes the file back to disk.
 */
export async function saveEnvDoc(d, { silent = false } = {}) {
  if (!d || !d.isEnv) return false;
  const t = pending.get(d);
  if (t) { clearTimeout(t); pending.delete(d); }

  d.envSaving = true;
  updateEnvStatus(d);
  try {
    await apiPostJson('/api/env/save', {
      path: d.path,
      entries: d.envEntries || [],
      trailingNewline: d.envTrailingNewline !== false,
    });
    d.envDirty = false;
    d.envSaving = false;
    d.envError = null;
    d.envSavedAt = Date.now();
    updateEnvStatus(d);
    drawTabs();
    updateStatus();
    emit('env:saved', d);
    if (!silent) showToast('✓', 'Saved ' + d.name);
    return true;
  } catch (e) {
    d.envSaving = false;
    d.envError = e.message;
    updateEnvStatus(d);
    showToast('!', 'Failed to save ' + d.name + ': ' + e.message);
    return false;
  }
}

/**
 * updateEnvStatus writes the state of the file in the header: what is
 * pending, what was written, what went wrong.
 */
function updateEnvStatus(d) {
  if (!envStatusEl) return;
  const el = envStatusEl;
  el.classList.remove('saving', 'saved', 'error', 'dirty');

  if (!d || !d.isEnv) { el.textContent = ''; return; }

  if (d.envError) {
    el.classList.add('error');
    el.textContent = 'Not saved: ' + d.envError;
    el.title = d.envError;
    return;
  }
  if (d.envSaving) {
    el.classList.add('saving');
    el.textContent = 'Saving…';
    el.title = '';
    return;
  }
  if (d.envDirty) {
    el.classList.add('dirty');
    el.textContent = autosaveOn() ? 'Unsaved changes' : 'Unsaved changes — press ' + (isMacKey() ? '⌘S' : 'Ctrl+S');
    el.title = '';
    return;
  }
  if (d.envSavedAt) {
    el.classList.add('saved');
    const t = new Date(d.envSavedAt);
    el.textContent = 'Saved ' + t.toTimeString().slice(0, 8);
    el.title = 'Last saved at ' + t.toLocaleTimeString();
    return;
  }
  el.textContent = autosaveOn() ? 'Autosave on' : 'Autosave off';
  el.title = autosaveOn()
    ? 'Changes are written to disk automatically.'
    : 'Changes are written to disk when you save.';
}

const isMacKey = () => /mac|iphone|ipad/i.test(navigator.userAgentData?.platform || navigator.platform || '');

/**
 * setEnvMode switches between the table of variables and the file as plain
 * text.
 */
export function setEnvMode(mode) {
  const d = doc_();
  if (!d || !d.isEnv) return;
  d.envRaw = mode === 'raw';
  syncEditUI();
  syncEnvView();
}

/**
 * initEnvView wires the .env editor up.
 */
export function initEnvView() {
  if (!envViewEl) return;

  btnAdd?.addEventListener('click', () => addEntry(doc_()));
  btnSave?.addEventListener('click', () => saveEnvDoc(doc_()));

  envSwitch?.addEventListener('click', (e) => {
    const b = e.target.closest('[data-env]');
    if (!b) return;
    setEnvMode(b.dataset.env);
  });

  envFilterEl?.addEventListener('input', () => {
    const d = doc_();
    if (d && d.isEnv) renderEnvTable(d);
  });

  // Saving the file as raw text changes it on disk: the table has to be read
  // again the next time it is shown.
  on('file:saved', (d) => {
    if (!d || !d.isEnv) return;
    d.envLoaded = false;
    d.envEntries = null;
    d.envDirty = false;
    d.envSavedAt = Date.now();
  });

  on('tab:activated', () => syncEnvView());
  on('tabs:cleared', () => syncEnvView());
  // The text editor takes over the file, so the .env table steps aside.
  on('edit:mode', () => syncEnvView());
}
