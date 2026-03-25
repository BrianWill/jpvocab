package main

import (
	"log"
)

const (
	port               = 1338
	dbPath             = "jpvocab.db"
	defaultDrillTarget = 8
)

func main() {
	initTokenizer()

	db := initDB(dbPath)
	defer db.Close()

	log.Printf("jpvocab backend running on http://localhost:%d", port)
	log.Printf("Admin UI: http://localhost:%d/admin", port)

	serverInit(db)
}
