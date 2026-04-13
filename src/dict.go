package main

import (
	"compress/gzip"
	"database/sql"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const (
	dictGzName = "jdict.db.gz"
	dictDBName = "jdict.db"
)

// dictReady is closed when jdict.db is confirmed present and usable.
// Any code that needs the dictionary should select/receive on this channel.
var dictReady = make(chan struct{})

var (
	dictOpenOnce sync.Once
	dictOpenDB   *sql.DB
	dictOpenErr  error
)

func resolveDictPath(name string) string {
	candidates := []string{
		name,
		filepath.Join("dict", name),
		filepath.Join("..", "dict", name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join("..", "dict", name)
}

// initDictAsync checks whether jdict.db already exists; if not, decompresses
// jdict.db.gz in a background goroutine and closes dictReady when done.
// Safe to call multiple times — only the first call does work.
func initDictAsync() {
	go func() {
		dictDBPath := resolveDictPath(dictDBName)
		dictGzPath := resolveDictPath(dictGzName)
		if _, err := os.Stat(dictDBPath); err == nil {
			// Already decompressed from a prior launch.
			close(dictReady)
			return
		}

		if _, err := os.Stat(dictGzPath); err != nil {
			log.Printf("dict: %s not found, dictionary will be unavailable", dictGzPath)
			close(dictReady) // still close so callers don't block forever
			return
		}

		log.Println("dict: decompressing jdict.db in background...")
		if err := decompressGz(dictGzPath, dictDBPath); err != nil {
			log.Printf("dict: decompression failed: %v", err)
			// Leave dictReady open so callers can detect unavailability via
			// a timeout/select rather than blocking forever.
			return
		}

		log.Println("dict: ready")
		close(dictReady)
	}()
}

// dictIsReady reports whether the dictionary DB is available right now,
// without blocking. Useful for handlers that can degrade gracefully.
func dictIsReady() bool {
	select {
	case <-dictReady:
		return true
	default:
		return false
	}
}

// decompressGz decompresses src (.gz) into dst, writing to a temp file first
// and renaming atomically so a crash mid-write never leaves a corrupt dst.
func decompressGz(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}

	gz, err := gzip.NewReader(in)
	if err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}

	if _, err := io.Copy(out, gz); err != nil {
		gz.Close()
		out.Close()
		os.Remove(tmp)
		return err
	}
	gz.Close()
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, dst)
}

func openDictDB() (*sql.DB, error) {
	dictOpenOnce.Do(func() {
		dictPath := resolveDictPath(dictDBName)
		if _, err := os.Stat(dictPath); err != nil {
			dictOpenErr = err
			return
		}
		db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dictPath)+"?mode=ro")
		if err != nil {
			dictOpenErr = err
			return
		}
		db.SetMaxOpenConns(4)
		if err := db.Ping(); err != nil {
			db.Close()
			dictOpenErr = err
			return
		}
		dictOpenDB = db
	})
	return dictOpenDB, dictOpenErr
}
