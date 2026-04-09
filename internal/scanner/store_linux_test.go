//go:build linux

package scanner

import "testing"

func TestLinuxVirtualSkipPath_onlyRootVirtualTrees(t *testing.T) {
	cases := []struct {
		path   string
		skip bool
	}{
		{"/dev/null", true},
		{"/proc/self/status", true},
		{"/sys/class", true},
		{"/run/lock", true},
		{"/home/user/myproject/dev/main.go", false},
		{"/opt/foo/sys/config.yml", false},
		{"/var/lib/app/run/server.pid", false},
	}
	for _, c := range cases {
		if got := linuxVirtualSkipPath(c.path); got != c.skip {
			t.Errorf("%q: got %v want %v", c.path, got, c.skip)
		}
	}
}
