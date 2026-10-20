package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// The .env API is deliberately small: list the files, read one, write one.
// Everything else the editor does -- adding a key, renaming it, changing a
// value, dropping a line -- happens in the browser and comes back as a whole
// file. The server owns the format, the browser owns the editing.

// envSaveRequest carries a whole .env file back to disk.
type envSaveRequest struct {
	Path string `json:"path"`
	// Entries is the file as the editor holds it: pairs, comments and blanks.
	Entries []envEntry `json:"entries"`
	// TrailingNewline records whether the file ended with a newline.
	TrailingNewline *bool `json:"trailingNewline"`
}

// envCreateRequest asks for a new .env file.
type envCreateRequest struct {
	Path string `json:"path"`
}

// handleEnvFiles lists every .env file in the workspace.
func (s *Server) handleEnvFiles(w http.ResponseWriter, r *http.Request) {
	root := s.ix.Root()
	files := findEnvFiles(root)
	for i := range files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(files[i].Path)))
		if err != nil {
			continue
		}
		files[i].Keys = countEnvKeys(string(data))
	}
	writeJSON(w, map[string]any{"files": files})
}

// handleEnvGet reads and parses one .env file.
func (s *Server) handleEnvGet(w http.ResponseWriter, r *http.Request) {
	abs, rel, ok := s.safePath(r.URL.Query().Get("path"))
	if !ok || rel == "" {
		fail(w, http.StatusBadRequest, "bad path")
		return
	}
	if !isEnvFileName(filepath.Base(rel)) {
		fail(w, http.StatusBadRequest, "not an env file")
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	entries, trailing := parseEnv(string(data))
	writeJSON(w, map[string]any{
		"path":            rel,
		"entries":         entries,
		"trailingNewline": trailing,
	})
}

// handleEnvSave writes a .env file back to disk.
//
// The write is atomic: the new content goes to a temporary file in the same
// directory and is renamed over the old one, so a crash or a full disk can
// never leave a half-written .env behind.
func (s *Server) handleEnvSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req envSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	abs, rel, ok := s.safePath(req.Path)
	if !ok || rel == "" {
		fail(w, http.StatusBadRequest, "bad path")
		return
	}
	if !isEnvFileName(filepath.Base(rel)) {
		fail(w, http.StatusBadRequest, "not an env file")
		return
	}

	trailing := true
	if req.TrailingNewline != nil {
		trailing = *req.TrailingNewline
	}
	content := renderEnv(req.Entries, trailing)

	mode := os.FileMode(0o600)
	if st, err := os.Stat(abs); err == nil {
		mode = st.Mode().Perm()
	} else if !strings.HasSuffix(rel, ".env") {
		// A template like .env.example is meant to be shared.
		mode = 0o644
	}

	if err := writeFileAtomicMode(abs, []byte(content), mode); err != nil {
		fail(w, http.StatusInternalServerError, "failed to write file: "+err.Error())
		return
	}

	// The file on disk changed, so anything the index cached for it is stale.
	Evict(abs)
	if s.gitWatcher != nil {
		s.gitWatcher.Trigger()
	}
	go s.ix.Build()

	writeJSON(w, map[string]any{
		"ok":   true,
		"path": rel,
		"size": len(content),
	})
}

// handleEnvCreate creates an empty .env file, with a short header so it is
// obvious what it is the first time it is opened.
func (s *Server) handleEnvCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req envCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	abs, rel, ok := s.safePath(req.Path)
	if !ok || rel == "" {
		fail(w, http.StatusBadRequest, "bad path")
		return
	}
	if !isEnvFileName(filepath.Base(rel)) {
		fail(w, http.StatusBadRequest, "not an env file")
		return
	}
	if _, err := os.Stat(abs); err == nil {
		fail(w, http.StatusConflict, "file already exists")
		return
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		fail(w, http.StatusInternalServerError, "failed to create directory: "+err.Error())
		return
	}

	mode := os.FileMode(0o600)
	if !strings.HasSuffix(rel, ".env") {
		mode = 0o644
	}
	header := fmt.Sprintf("# %s\n", filepath.Base(rel))
	if err := writeFileAtomicMode(abs, []byte(header), mode); err != nil {
		fail(w, http.StatusInternalServerError, "failed to write file: "+err.Error())
		return
	}

	Evict(abs)
	if s.gitWatcher != nil {
		s.gitWatcher.Trigger()
	}
	go s.ix.Build()

	writeJSON(w, map[string]any{"ok": true, "path": rel})
}

// writeFileAtomicMode writes data to path through a temporary file in the
// same directory, then renames it over the target.
func writeFileAtomicMode(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".env0-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
