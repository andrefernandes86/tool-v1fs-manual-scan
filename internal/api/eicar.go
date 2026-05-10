package api

// eicarProbe assembles the standard EICAR test-file bytes at call time.
// The signature is intentionally split across two variables so no complete
// AV pattern exists as a static string in the compiled binary.
// The bytes are only joined immediately before submission to the scanner.
func eicarProbe() []byte {
	p1 := "X5O!P%@AP[4\\PZX54(P^)7CC)7}"
	p2 := "$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"
	return []byte(p1 + p2)
}
