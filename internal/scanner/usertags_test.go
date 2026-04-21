package scanner

import (
	"reflect"
	"testing"
)

func TestParseUserScanTags(t *testing.T) {
	got := ParseUserScanTags([]string{"  a ", "b", "a", "", "c"})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if len(ParseUserScanTags(make([]string, maxUserScanTags+5))) != maxUserScanTags {
		t.Fatal("expected max tag count")
	}
}
