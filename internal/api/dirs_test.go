package api

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestListDirsRequiresAbsolute(t *testing.T) {
	_, err := ListDirs("relative-path")
	if err == nil {
		t.Fatal("expected error for non-absolute path")
	}
}

func TestUpTargetListing_unixLike(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	up, ok := upTargetForListing(filepath.Join(string(filepath.Separator), "tmp"))
	if !ok || up != string(filepath.Separator) {
		t.Fatalf("got up=%q ok=%v", up, ok)
	}
	_, ok = upTargetForListing(string(filepath.Separator))
	if ok {
		t.Fatal("filesystem root should not have parent")
	}
}
