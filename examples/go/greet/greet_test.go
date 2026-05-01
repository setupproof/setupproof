package greet

import "testing"

func TestMessage(t *testing.T) {
	if got := Message("reader"); got != "hello, reader" {
		t.Fatalf("Message() = %q", got)
	}
}
