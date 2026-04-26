package api

import "testing"

func TestStatus(t *testing.T) {
	if got := Status(); got != "ready" {
		t.Fatalf("Status() = %q", got)
	}
}
