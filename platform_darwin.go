package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

// platformInit opens the app in the default browser after the server is ready.
// On macOS the binary is typically packaged as a .app bundle (see scripts/build-macos-app.sh).
func platformInit(port string) {
	url := "http://localhost:" + port
	waitForServer(url)
	if err := exec.Command("open", url).Start(); err != nil {
		fmt.Printf("V1FS Scanner ready → %s\n", url)
	}
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
