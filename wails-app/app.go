package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"rerum/backend/pkg/backup"
	"rerum/backend/pkg/httpapi"
	"rerum/backend/pkg/migrations"
	"rerum/backend/pkg/storage/sqlite"
)

// App owns the in-process Rerum backend (or optional external RERUM_BACKEND_URL)
// and exposes a small, typed bridge to the Svelte frontend.
type App struct {
	ctx context.Context

	mu          sync.Mutex
	db          *sql.DB
	handler     http.Handler
	baseURL     string
	external    bool
	dbPath      string
	receiptsDir string
	startErr    string
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := a.startBackend(); err != nil {
		a.mu.Lock()
		a.startErr = err.Error()
		a.mu.Unlock()
	}
}

func (a *App) shutdown(_ context.Context) {
	a.stopBackend()
}

func (a *App) startBackend() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.startBackendLocked()
}

func (a *App) startBackendLocked() error {
	if a.handler != nil || a.baseURL != "" {
		return nil
	}
	if value := strings.TrimSpace(os.Getenv("RERUM_BACKEND_URL")); value != "" {
		a.baseURL = strings.TrimRight(value, "/")
		a.external = true
		a.startErr = ""
		return nil
	}
	dbPath := strings.TrimSpace(os.Getenv("RERUM_DB_PATH"))
	if dbPath == "" {
		dataDir, err := userDataDir()
		if err != nil {
			return err
		}
		dbPath = filepath.Join(dataDir, databaseName)
	}
	if absolute, err := filepath.Abs(dbPath); err == nil {
		dbPath = absolute
	}
	a.dbPath = dbPath
	a.receiptsDir = filepath.Join(filepath.Dir(dbPath), "recibos")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return fmt.Errorf("criar diretório do banco: %w", err)
	}
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}
	db.SetMaxOpenConns(8)

	if err := migrations.Apply(context.Background(), db, ""); err != nil {
		_ = db.Close()
		return fmt.Errorf("aplicar migrações: %w", err)
	}

	api := &httpapi.Server{Store: &sqlite.Store{DB: db, ReceiptsDir: a.receiptsDir}}
	a.db = db
	a.handler = api.Handler()
	a.baseURL = "in-process://rerum"
	a.startErr = ""
	return nil
}

func (a *App) stopBackend() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
	a.handler = nil
	a.baseURL = ""
}

// BackendReady is called by the frontend on startup.
func (a *App) BackendReady() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.handler != nil || a.baseURL != "" {
		return a.baseURL, nil
	}
	if a.startErr != "" {
		return "", errors.New(a.startErr)
	}
	return "", errors.New("backend ainda não está pronto")
}

// BackendInfo exposes read-only diagnostics for the settings/about screen.
func (a *App) BackendInfo() map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	return map[string]any{
		"base_url": a.baseURL,
		"db_path":  a.dbPath,
		"managed":  !a.external,
		"root":     filepath.Dir(a.dbPath),
	}
}

func (a *App) httpClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

func (a *App) doRequest(method, path, body string) (int, []byte, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	a.mu.Lock()
	handler := a.handler
	base := a.baseURL
	a.mu.Unlock()

	if handler != nil {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(strings.ToUpper(method), path, reader)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code, rec.Body.Bytes(), nil
	}

	if base == "" {
		return 0, nil, errors.New("backend não está conectado")
	}
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequest(strings.ToUpper(method), base+path, reader)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.httpClient().Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("falha de comunicação com o backend Go: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	return response.StatusCode, payload, err
}

// BackendRequest dispatches a call to the in-process backend (or external HTTP if configured).
func (a *App) BackendRequest(method, path, body string) (string, error) {
	status, payload, err := a.doRequest(method, path, body)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		message := strings.TrimSpace(string(payload))
		if message == "" {
			message = http.StatusText(status)
		}
		return "", fmt.Errorf("HTTP %d: %s", status, message)
	}
	return string(payload), nil
}

func filtersFor(name string) []runtime.FileFilter {
	extension := strings.ToLower(filepath.Ext(name))
	switch extension {
	case ".zip":
		return []runtime.FileFilter{{DisplayName: "Backup completo do Rerum (*.zip)", Pattern: "*.zip"}}
	case ".xlsx":
		return []runtime.FileFilter{{DisplayName: "Planilhas Excel (*.xlsx)", Pattern: "*.xlsx"}}
	case ".sqlite":
		return []runtime.FileFilter{{DisplayName: "SQLite (*.sqlite)", Pattern: "*.sqlite"}}
	case ".csv":
		return []runtime.FileFilter{{DisplayName: "CSV (*.csv)", Pattern: "*.csv"}}
	case ".pdf":
		return []runtime.FileFilter{{DisplayName: "Documento PDF (*.pdf)", Pattern: "*.pdf"}}
	default:
		return []runtime.FileFilter{{DisplayName: "Todos os arquivos (*.*)", Pattern: "*.*"}}
	}
}

// SaveTextFile prompts the user for a destination path and writes UTF-8 text.
func (a *App) SaveTextFile(suggestedName, content string) (string, error) {
	destination, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Salvar arquivo",
		DefaultFilename: suggestedName,
		Filters:         filtersFor(suggestedName),
	})
	if err != nil || destination == "" {
		return "", err
	}
	if err := os.WriteFile(destination, []byte(content), 0o600); err != nil {
		return "", err
	}
	return destination, nil
}

// SaveBackendFile downloads a binary endpoint (PDF receipt, zip/sqlite backup)
// and writes it to a path chosen by the user.
func (a *App) SaveBackendFile(path, suggestedName string) (string, error) {
	destination, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Salvar arquivo",
		DefaultFilename: suggestedName,
		Filters:         filtersFor(suggestedName),
	})
	if err != nil || destination == "" {
		return "", err
	}
	status, payload, err := a.doRequest("GET", path, "")
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", status, strings.TrimSpace(string(payload)))
	}
	if err := os.WriteFile(destination, payload, 0o600); err != nil {
		return "", err
	}
	return destination, nil
}

// OpenFile opens a native file chooser and returns the selected path (or "").
func (a *App) OpenFile(title, pattern string) (string, error) {
	var filters []runtime.FileFilter
	if strings.TrimSpace(pattern) != "" {
		filters = append(filters, runtime.FileFilter{DisplayName: pattern, Pattern: pattern})
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
}

// OpenExternal delegates a URL or local path to the OS default handler.
func (a *App) OpenExternal(target string) error {
	runtime.BrowserOpenURL(a.ctx, target)
	return nil
}

// Clipboard copies text to the desktop clipboard.
func (a *App) Clipboard(text string) error {
	return runtime.ClipboardSetText(a.ctx, text)
}

// RestoreBackup stops the in-process DB, restores a full backup zip, and reopens.
func (a *App) RestoreBackup(archive string) (string, error) {
	if strings.TrimSpace(archive) == "" {
		return "", errors.New("arquivo de backup não informado")
	}
	a.mu.Lock()
	external := a.external
	dbPath := a.dbPath
	receiptsDir := a.receiptsDir
	a.mu.Unlock()
	if external {
		return "", errors.New("restauração indisponível quando conectado a um backend externo")
	}
	a.stopBackend()
	currentSchema, err := migrations.MaxVersion("")
	if err != nil {
		_ = a.startBackend()
		return "", fmt.Errorf("ler versão das migrações: %w", err)
	}
	safety, err := backup.Restore(context.Background(), archive, dbPath, receiptsDir, currentSchema)
	if err != nil {
		_ = a.startBackend()
		return "", err
	}
	if err := a.startBackend(); err != nil {
		return "", err
	}
	return safety, nil
}
