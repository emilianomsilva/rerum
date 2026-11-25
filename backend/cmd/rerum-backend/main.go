package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"rerum/backend/pkg/backup"
	"rerum/backend/pkg/httpapi"
	"rerum/backend/pkg/migrations"
	"rerum/backend/pkg/storage/sqlite"
)

func main() {
	dbPath := flag.String("db", "data/rerum.sqlite", "Caminho do banco SQLite")
	migrationsDir := flag.String("migrations", "", "Diretório alternativo de migrações SQL")
	receiptsDir := flag.String("receipts", "", "Diretório de recibos (padrão: <dir do banco>/recibos)")
	listen := flag.String("listen", "127.0.0.1:0", "Endereço HTTP local")
	backupFull := flag.String("backup-full", "", "Cria um backup completo no caminho informado e encerra")
	restore := flag.String("restore", "", "Restaura o backup completo informado e encerra")
	flag.Parse()

	if *receiptsDir == "" {
		*receiptsDir = filepath.Join(filepath.Dir(*dbPath), "recibos")
	}

	if *restore != "" {
		currentSchema, err := migrations.MaxVersion(*migrationsDir)
		if err != nil {
			log.Fatalf("ler versão das migrações: %v", err)
		}
		safety, err := backup.Restore(context.Background(), *restore, *dbPath, *receiptsDir, currentSchema)
		if err != nil {
			log.Fatalf("restaurar backup: %v", err)
		}
		fmt.Printf("RERUM_RESTORE_OK %s\n", safety)
		return
	}

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o700); err != nil {
		log.Fatalf("criar diretório do banco: %v", err)
	}
	db, err := sql.Open("sqlite3", *dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		log.Fatalf("abrir banco: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)

	if *backupFull != "" {
		if err := backup.Create(context.Background(), db, *receiptsDir, *backupFull); err != nil {
			log.Fatalf("criar backup completo: %v", err)
		}
		fmt.Printf("RERUM_BACKUP_OK %s\n", *backupFull)
		return
	}

	if err := migrations.Apply(context.Background(), db, *migrationsDir); err != nil {
		log.Fatalf("aplicar migrações: %v", err)
	}

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("abrir listener: %v", err)
	}
	api := &httpapi.Server{Store: &sqlite.Store{DB: db, ReceiptsDir: *receiptsDir}}
	server := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second}
	// The desktop launcher reads this line to discover the ephemeral local port.
	fmt.Printf("RERUM_BACKEND_READY %s\n", listener.Addr().String())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("servidor Go: %v", err)
	}
}
