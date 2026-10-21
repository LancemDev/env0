# Environment File Editor

lx8 is built around one job: keeping a project's `.env` files correct without
ever leaving the browser. A `.env` file is a list of assignments, comments and
blank lines, and lx8 edits it as a list rather than as a wall of text.

---

## What counts as a .env file

lx8 recognises a file as an environment file when its name is:

| Name | Recognised |
| :--- | :--- |
| `.env` | yes |
| `.env.local`, `.env.production`, `.env.development` | yes |
| `.env.example`, `.env.sample` | yes |
| `.envrc`, `env.js`, `config.env` | no |

The search walks the whole workspace, up to six directories deep, and skips
dependency and build directories (`node_modules`, `vendor`, `dist`, `target`
and the rest).

**The search ignores `.gitignore` on purpose.** A `.env` file is nearly always
ignored by git, and a tool for editing them would be useless if it could not
see them.

---

## The environment panel

`Cmd/Ctrl+Shift+E` opens the **Environment** panel in the sidebar, or the
third button in the sidebar's view switch does the same.

The panel lists every `.env` file in the workspace with the number of
variables it holds and the directory it lives in. Clicking a file opens it in
the environment editor. The button next to the panel's title creates a new
`.env` file.

---

## The editor

Opening a `.env` file shows it as a table:

- **Key** — the name of the variable. Edit it to rename the variable.
- **Value** — the value. Edit it to change the value.
- **Copy** — copies the value to the clipboard.
- **Remove** — deletes the variable.

**Add variable** puts a new, empty row at the end of the assignments and
puts the cursor in the key box. `Enter` in the key box moves to the value box.

**Filter** narrows the table to the keys and values that match what you type.

### Comments stay where they are

A comment is part of the file, so it keeps its place in the table. The file
still reads the way it was written:

```env
# Application settings
APP_NAME=MyApp

# Database
DATABASE_URL=postgres://user@localhost:5432/db
```

### Secrets stay masked

A value whose name looks like a credential -- `SECRET`, `TOKEN`, `PASSWORD`,
`PASSWD`, `PWD`, `API_KEY`, `ACCESS_KEY`, `PRIVATE_KEY`, `CREDENTIAL` or
`AUTH` -- is masked. The eye button beside it reveals the value, and only
that one.

### Raw view

The **Raw** switch above the editor shows the file as plain text instead of as
a table, and `Cmd/Ctrl+E` edits it directly there. It is the escape hatch for
a file with a shape the table does not describe.

---

## Autosave

**Autosave is on by default.** A change is written to disk a moment after the
last keystroke, so a `.env` file is never left half-edited and there is no
save button to forget.

The header shows what is happening: `Saving…`, then `Saved 14:32:07`, or
`Unsaved changes` if a write failed.

Turn it off in **Settings → Environment Files → Autosave Environment Files**,
and the editor waits for `Cmd/Ctrl+S` instead. The delay before the write is
also configurable there (`Autosave Delay`, 600 ms by default).

Writes are atomic: the new content goes to a temporary file in the same
directory and is renamed over the old one, so a crash or a full disk can never
leave a half-written `.env` behind.

---

## What is preserved

A file you open and save without changing comes back **byte for byte the
same**. lx8 keeps:

- comments, blank lines and the order of everything;
- the `export` prefix on an assignment;
- the quoting style of each value (`"double"`, `'single'` or none);
- the whitespace around `=` and at the end of a line;
- an inline comment after a value.

Only a line you actually changed is written out afresh. A value that has
grown a space, a `#` or a newline is quoted, because other `.env` readers
stop at those characters.

---

## File permissions

A new `.env` file is created `0600` -- readable only by its owner. A new
`.env.example` is created `0644`, because a template is meant to be shared.
An existing file keeps the permissions it already had.

---

## The format

lx8 follows the conventions dotenv, docker-compose and 12-factor apps agree
on:

```env
# A full-line comment
KEY=value                 unquoted
KEY="quoted value"        double quotes; escapes are honoured
KEY='literal $value'      single quotes; everything is literal
export KEY=value          the export prefix
KEY=value # a comment     an inline comment, only after whitespace
```

A `#` inside an unquoted value is part of the value: `KEY=a#b` is the value
`a#b`. A `#` that follows whitespace starts a comment: `KEY=a #b` is the
value `a`.

A value may be written across several lines inside quotes. It is read as one
value, and written back the way it was.

---

## The API

The editor is a thin client over four endpoints. They are ordinary JSON, and
the server owns the format:

| Endpoint | What it does |
| :--- | :--- |
| `GET /api/env/files` | Lists every `.env` file in the workspace |
| `GET /api/env/get?path=` | Reads and parses one file into entries |
| `POST /api/env/save` | Writes a file back, from its entries |
| `POST /api/env/create` | Creates a new `.env` file |

`POST /api/env/save` takes the whole file as entries -- pairs, comments and
blanks -- so the browser never has to reconstruct the text itself.

---

## Related

- [File Editing](file-editing.md) -- the plain text editor for every other file
- [Settings & Configuration](settings-and-configuration.md) -- where the autosave options live
- [File Explorer](file-explorer.md) -- the tree, and how `.gitignore` is shown there
