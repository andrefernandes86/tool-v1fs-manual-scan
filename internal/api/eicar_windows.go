package api

// eicarProbe returns nil on Windows so no EICAR-related bytes are compiled
// into the binary. All callers check for nil and skip the malware-detection
// step, falling back to a plain connectivity check instead.
func eicarProbe() []byte { return nil }
