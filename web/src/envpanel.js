// web/src/envpanel.js
/**
 * The .env file list.
 *
 * A project usually has more than one .env file: the one the app runs on, a
 * local override, a production variant, a template to copy. They are spread
 * through the tree, and most of them are ignored by git, so the file tree is
 * a poor place to find them. This panel lists every .env file in the
 * workspace and opens it in the .env editor.
 */
import { $, esc, S, api } from './state.js';
import { showToast } from './ui.js';
import { on } from './bus.js';
import { openFile } from './tabs.js';
import { promptNewItem } from './newitem.js';

const sideEl = $('#side');
const panelEl = $('#envpanel');
const envListEl = $('#envpanel-list');
const btnEnv = $('#btn-env');
const btnNew = $('#envpanel-new');
const countEl = $('#envpanel-count');

let open = false;

/**
 * isEnvPanelOpen reports whether the .env list is showing in the sidebar.
 */
export const isEnvPanelOpen = () => open;

/**
 * refreshEnvFiles reads the list of .env files and draws it.
 */
export async function refreshEnvFiles() {
  if (!envListEl) return;
  let files = [];
  try {
    const j = await api('/api/env/files');
    files = j.files || [];
  } catch (e) {
    envListEl.replaceChildren();
    const err = document.createElement('div');
    err.className = 'envpanel-empty';
    err.textContent = 'Could not read the .env files: ' + e.message;
    envListEl.appendChild(err);
    return;
  }

  envListEl.replaceChildren();

  if (countEl) {
    countEl.textContent = files.length === 1 ? '1 file' : files.length + ' files';
  }

  if (!files.length) {
    const empty = document.createElement('div');
    empty.className = 'envpanel-empty';
    empty.textContent = 'No .env files in this workspace yet.';
    envListEl.appendChild(empty);
    return;
  }

  for (const f of files) {
    const row = document.createElement('button');
    row.type = 'button';
    row.className = 'envpanel-row';
    row.title = f.path;

    const name = document.createElement('span');
    name.className = 'envpanel-name';
    name.textContent = f.name;

    // The directory only tells the reader something when the file is not at
    // the root of the workspace.
    const dir = document.createElement('span');
    dir.className = 'envpanel-dir';
    dir.textContent = f.dir || '';

    const count = document.createElement('span');
    count.className = 'envpanel-keys';
    count.textContent = f.keys === 1 ? '1 key' : f.keys + ' keys';

    row.appendChild(name);
    row.appendChild(dir);
    row.appendChild(count);
    row.addEventListener('click', () => openFile(f.path));
    envListEl.appendChild(row);
  }
}

/**
 * showEnvPanel puts the .env list in the sidebar, taking the place of the
 * file tree.
 */
export function showEnvPanel() {
  if (!sideEl || !panelEl) return;
  open = true;
  sideEl.classList.add('env-mode');
  panelEl.hidden = false;
  btnEnv?.classList.add('active');
  $('#btn-files')?.classList.remove('active');
  $('#btn-changed')?.classList.remove('active');
  refreshEnvFiles();
}

/**
 * hideEnvPanel gives the sidebar back to the file tree.
 */
export function hideEnvPanel() {
  if (!sideEl || !panelEl) return;
  open = false;
  sideEl.classList.remove('env-mode');
  panelEl.hidden = true;
  btnEnv?.classList.remove('active');
}

/**
 * toggleEnvPanel shows the .env list, or gives the sidebar back to the file
 * tree when it is already showing.
 */
export function toggleEnvPanel() {
  if (open) hideEnvPanel();
  else showEnvPanel();
}

/**
 * createEnvFile asks for a name and makes the new .env file.
 */
function createEnvFile() {
  promptNewItem('env');
}

/**
 * initEnvPanel wires the .env list up.
 */
export function initEnvPanel() {
  if (!panelEl) return;

  btnEnv?.addEventListener('click', () => {
    toggleEnvPanel();
  });
  btnNew?.addEventListener('click', createEnvFile);

  // The tree took the sidebar back: step aside.
  on('sidebar:mode', () => {
    if (open) hideEnvPanel();
  });

  // A file saved elsewhere may have created a new .env file.
  on('file:saved', () => {
    if (open) refreshEnvFiles();
  });
  on('env:saved', () => {
    if (open) refreshEnvFiles();
  });
  on('file:created', () => {
    if (open) refreshEnvFiles();
  });
}
