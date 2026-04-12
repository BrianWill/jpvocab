package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	// port is in the dynamic/private range (49152–65535) to avoid conflicts
	// with registered services.
	port   = 49200
	dbPath = "jpvocab.db"
)

var (
	isDesktopApp          bool
	webviewUserDataPath   string
	clearWebviewCacheFlag string
)

func defaultWebviewUserDataPath() (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheRoot, "jpvocab", "wails-webview2"), nil
}

func clearPendingWebviewCache() error {
	if clearWebviewCacheFlag == "" || webviewUserDataPath == "" {
		return nil
	}
	if _, err := os.Stat(clearWebviewCacheFlag); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.RemoveAll(webviewUserDataPath); err != nil {
		return err
	}
	if err := os.Remove(clearWebviewCacheFlag); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func main() {
	serverOnly := flag.Bool("server-only", false, "run the web server without opening the Wails desktop window")
	skipLargeStories := flag.Bool("skip-large-seed-stories", false, "skip large stories during DB seeding")
	flag.Parse()
	if os.Getenv("SERVER_ONLY") == "true" {
		*serverOnly = true
	}
	if os.Getenv("SKIP_LARGE_SEED_STORIES") == "true" {
		*skipLargeStories = true
	}
	skipLargeSeedStories = *skipLargeStories

	var err error
	webviewUserDataPath, err = defaultWebviewUserDataPath()
	if err != nil {
		log.Fatal(err)
	}
	clearWebviewCacheFlag = webviewUserDataPath + ".clear"
	if err := clearPendingWebviewCache(); err != nil {
		log.Printf("warning: unable to clear pending webview cache: %v", err)
	}

	initDictAsync() // decompress jdict.db.gz in background; overlaps with tokenizer + DB init

	initTokenizer()

	db := initDB(dbPath)
	defer db.Close()

	log.Printf("jpvocab backend running on http://localhost:%d", port)

	if *serverOnly {
		// Blocking: run the web server on the main goroutine with no GUI.
		serverInit(db)
		return
	}

	// Run the web server in the background; serverInit blocks on ListenAndServe.
	go serverInit(db)

	// Give the web server a moment to bind before the webview makes its first request.
	time.Sleep(200 * time.Millisecond)

	// Launch the Wails v3 app. We don't use the Go<->JS bridge; the window simply
	// loads our locally-served welcome page directly.
	app := application.New(application.Options{
		Name: "jpvocab",
		Windows: application.WindowsOptions{
			WebviewUserDataPath: webviewUserDataPath,
		},
	})
	isDesktopApp = true

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "jpvocab",
		Width:  1280,
		Height: 800,
		URL:    fmt.Sprintf("http://localhost:%d/welcome", port),
		// ZoomControlEnabled allows Ctrl+scroll to reach the page's JS wheel
		// handler (in app_nav.html) rather than being consumed by WebView2.
		ZoomControlEnabled: true,
		DevToolsEnabled:    true,
		KeyBindings: map[string]func(application.Window){
			// Standard browser reload shortcuts; disabled by default in Wails
			// because AreBrowserAcceleratorKeysEnabled is false.
			"ctrl+r":       func(w application.Window) { w.Reload() },
			"ctrl+shift+r": func(w application.Window) { w.ForceReload() },
		},
	})

	if err := app.Run(); err != nil {
		log.Fatal("Wails:", err)
	}
}
