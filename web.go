package main

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed web
var webEmbed embed.FS

// WebFS is the embedded web assets rooted at the web/ subdirectory.
var WebFS fs.FS

func init() {
	var err error
	WebFS, err = fs.Sub(webEmbed, "web")
	if err != nil {
		log.Fatal("init WebFS:", err)
	}
}
