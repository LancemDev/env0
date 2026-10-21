# lx8

**The `.env` editor that runs in your browser.**

A project usually has more than one environment file: the one the app runs
on, a local override, a production variant, a template to copy. They are
scattered through the tree, most of them are ignored by git, and every one of
them is a list of assignments that you have to edit as text.

lx8 is a fast, local-first code navigator built around that one job. It
lists every `.env` in the workspace, opens it as a table of keys and values
instead of a wall of text, and writes your changes to disk on its own.

- **Autosave is on by default.** A change reaches the disk a moment after the
  last keystroke. There is no save button to forget.
- **Your formatting survives.** Comments, blank lines, ordering, `export`
  prefixes and the quoting style of every value are kept. A file you never
  touch comes back byte for byte the same.
- **Secrets stay masked.** Values whose names look like credentials are hidden
  until you ask to see them.
- **One binary, no dependencies.** A single static Go binary, ~15 MB, that
  serves the whole interface to your browser. No Electron, no Node, no CGO.
- **< 1 ms cold start.** ~20–30 MB of server RAM, ~100–180 MB including the
  browser tab.

---

## Installation

macOS, Linux and BSD:

```bash
curl -fsSL https://raw.githubusercontent.com/LancemDev/lx8/main/install.sh | sh
```

That drops a single `lx8` binary in `/usr/local/bin` (or `~/.local/bin`) and
prints the address to open. Update an existing install the same way, or:

```bash
lx8 --update
```

Build it yourself instead (Go 1.25+ and Node.js or Bun):

```bash
git clone https://github.com/LancemDev/lx8.git
cd lx8
make build
install -d ~/.local/bin && install lx8 ~/.local/bin/
```

---

## Usage

```bash
# Open the workbench: pick any directory from one page
lx8

# Edit the .env files of the current directory
lx8 .

# A specific directory or file
lx8 ~/workspace/project
lx8 main.go:42

# Remote or headless: bind to a network address
lx8 -host 0.0.0.0 -port 7777 ~/workspace
```

When lx8 binds to `0.0.0.0` it prints a `network` address for every
non-loopback IPv4 address on the machine:

```text
  url:        http://0.0.0.0:7777
  network:    http://10.0.0.15:7777
  network:    http://192.168.1.42:7777
```

Open the address that is reachable from your machine. lx8 never makes a
network call before serving the first request, and it never phones home.

---

## The environment editor

Press `Cmd/Ctrl+Shift+E`, or click the environment button in the sidebar, to
open the **Environment** panel. It lists every `.env` file in the workspace,
including the ones git ignores, with the number of variables each one holds.

Click a file and it opens as a table:

```
KEY                VALUE                                     📋  ✕
────────────────────────────────────────────────────────────────────────
# Application settings
APP_NAME           MyApp                                     📋  ✕
APP_ENV            development                               📋  ✕

# Database
DATABASE_URL       postgres://user@localhost:5432/db          📋  ✕
DATABASE_PASSWORD  •••••••••                                 👁  📋  ✕
```

- **Add variable** appends a new assignment and puts the cursor in it.
- **Enter** in the key box moves to the value box.
- **Filter** narrows the list to the keys and values you type.
- **Raw** shows the file as plain text, and `Cmd/Ctrl+E` edits it there.

### Autosave

Autosave is on by default, and the header says what is happening:
`Saving…`, then `Saved 14:32:07`, or `Unsaved changes` if a write failed.

Turn it off in **Settings → Environment Files**, and the editor waits for
`Cmd/Ctrl+S` instead. The delay before the write is configurable there too.

Writes are atomic: the new content goes to a temporary file in the same
directory and is renamed over the old one, so a crash or a full disk can
never leave a half-written `.env` behind.

### What is preserved

```env
# A comment
export KEY="a value"   # an inline comment
OTHER=plain
```

A file you open and save without changing comes back byte for byte the same.
Only a line you actually changed is written out afresh. A value that has
grown a space, a `#` or a newline is quoted, because other `.env` readers
stop at those characters.

A new `.env` file is created `0600` -- readable only by its owner. A new
`.env.example` is created `0644`, because a template is meant to be shared.

Full details: [docs/features/env-editor.md](docs/features/env-editor.md).

---

## Everything else

lx8 is a complete code navigator, not only an env editor.

- **Review workbench** -- run `lx8` with no arguments to open a launcher for
  directories and pull requests, all on one port.
- **Pull request review** -- review a PR from GitHub, GitLab, Gitea/Forgejo
  or Bitbucket Cloud (`lx8 <pr-url>`), draft inline comments, and merge.
- **Fast navigation** -- fuzzy file search (`Cmd/Ctrl+P`), symbol outline
  (`Cmd/Ctrl+Shift+O`), workspace search (`Cmd/Ctrl+Shift+F`), in milliseconds.
- **Direct file editing** -- edit and save any file in the browser
  (`Cmd/Ctrl+E`, `Cmd/Ctrl+S`).
- **Git and diffs** -- live status, split and unified diffs, stage, commit,
  push and pull from the sidebar.
- **Agent threads** -- anchor a conversation to a code range (`Alt+T`) and
  dispatch it to Claude Code, Cursor Agent, OpenCode, Codex or the harness
  you already use.
- **Rich viewer** -- 14 themes, rendered Markdown with Mermaid, CSV/TSV as a
  table, an image inspector, and optional zero-config LSP.
- **Remote-first** -- run it on a server, a VM or a container and read the
  code in your local browser. Path traversal and DNS rebinding are guarded.

Full feature guide: [docs/features/README.md](docs/features/README.md).

---

## Keyboard shortcuts

| Shortcut | Does |
| :--- | :--- |
| `Cmd/Ctrl+Shift+E` | Environment files |
| `Cmd/Ctrl+P` | Go to file |
| `Cmd/Ctrl+Shift+P` | Command palette |
| `Cmd/Ctrl+Shift+O` | Go to symbol |
| `Cmd/Ctrl+Shift+F` | Search in files |
| `Cmd/Ctrl+E` | Edit the file |
| `Cmd/Ctrl+S` | Save |
| `Cmd/Ctrl+,` | Settings |
| `Cmd/Ctrl+B` | Toggle the sidebar |
| `Cmd/Ctrl+J` | Toggle the inspector |
| `F12` / `Shift+F12` | Definition / references |
| `Alt+T` | Start a thread on the selection |

The full list is in the help sheet, behind the `?` in the status bar.

---

## Configuration

Settings live in `~/.lx8/settings.json` (or
`$XDG_CONFIG_HOME/lx8/settings.json`) and are edited from the settings
modal at `Cmd/Ctrl+`, which writes the file as you change it.

| Setting | Default | Does |
| :--- | :--- | :--- |
| `env.autosave` | `true` | Writes a `.env` file without being asked |
| `env.autosaveDelay` | `600` | Milliseconds to wait after the last change |
| `editor.fontSize` | `13.5` | Font size in the code viewer |
| `workbench.colorTheme` | `dark` | The theme |

---

## Security

lx8 binds to `127.0.0.1` by default. It only ever serves files inside the
workspace it was pointed at, and it checks that on every request.

Anyone who can reach the address can also **edit** the files in that
workspace -- that includes your `.env` files. Expose it only over a trusted
network, and reach it through Tailscale, a VPN or an SSH tunnel rather than
over the open internet.

`lx8 -host 0.0.0.0` prints the network addresses it is reachable on. It
does not make a network call of any kind before serving the first request, and
no telemetry is collected.

---

## Building from source

```bash
git clone https://github.com/LancemDev/lx8.git
cd lx8

make build         # bundle the web assets and build the binary
make test          # run the test suite
make web           # bundle only the frontend
```

The frontend is plain ES modules with no dependencies; the bundler needs
Node.js or Bun. The server needs Go 1.25 or later.

For frontend work, run the server against the sources on disk:

```bash
go run . -dev .
```

---

## Credits

lx8 is built on [px0](https://github.com/px0-ai/px0) by Arpit Bhayani, the
code navigator and review workbench this project grew out of. The MIT licence
and the original copyright are kept in [LICENSE](LICENSE).

---

## Licence

[MIT](LICENSE)
