package main

import (
	"net/http"
	"os"
	"os/exec"
	"time"
)

// platformInit waits for the server and opens it in the best available browser.
// The binary is built with -ldflags="-H windowsgui" so no console window appears.
// Launch order: Edge app-mode → Chrome app-mode → default browser.
// App-mode strips the browser chrome (address bar, tabs) giving a native-app feel.
func platformInit(port string) {
	url := "http://localhost:" + port
	waitForServer(url)
	openWindowsBrowser(url)
}

func openWindowsBrowser(url string) {
	// Edge is always available on Windows 10/11
	edgePaths := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, p := range edgePaths {
		if _, err := os.Stat(p); err == nil {
			exec.Command(p, "--app="+url, "--new-window").Start()
			return
		}
	}

	// Chrome as fallback
	chromePaths := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	}
	for _, p := range chromePaths {
		if _, err := os.Stat(p); err == nil {
			exec.Command(p, "--app="+url, "--new-window").Start()
			return
		}
	}

	// Last resort: system default browser
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
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
