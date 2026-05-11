package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

// platformInit opens the app in a browser after the server is ready.
// Safari's HTTPS-Only mode blocks plain HTTP, so we prefer Chrome, Firefox,
// or Edge. Falls back to the system default and prints the URL as a safety net.
func platformInit(port string) {
	url := "http://localhost:" + port
	waitForServer(url)

	// Browsers that handle plain HTTP without HTTPS-Only restrictions
	browsers := []string{
		"Google Chrome",
		"Firefox",
		"Brave Browser",
		"Microsoft Edge",
		"Chromium",
	}
	for _, b := range browsers {
		if exec.Command("open", "-a", b, url).Run() == nil {
			return
		}
	}

	// Last resort: system default (Safari etc.) — print URL in case it's blocked
	fmt.Printf("\nV1FS Scanner ready -> %s\n", url)
	fmt.Println("If Safari shows an HTTPS error, open the URL above in Chrome or Firefox.")
	exec.Command("open", url).Start()
}

// waitForServer polls until the HTTP server responds or the timeout expires.
func waitForServer(url string) {
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
	}
}
