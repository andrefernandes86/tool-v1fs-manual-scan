package main

import "fmt"

// platformInit prints the server URL to stdout.
// On Linux the binary is headless; point any browser at the printed URL.
// When running inside Docker the URL is exposed via the mapped host port.
func platformInit(port string) {
	fmt.Printf("\nV1FS Scanner ready → http://localhost:%s\n\n", port)
}
