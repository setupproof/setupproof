package greeter

import "testing"

func TestGreet(t *testing.T) {
	got := Greet("world")
	want := "hello, world"
	if got != want {
		t.Fatalf("Greet(world) = %q, want %q", got, want)
	}
}
