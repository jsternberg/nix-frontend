package greeter

import "testing"

func TestGreet(t *testing.T) {
	got := Greet("Go")
	want := "Hello, Go!"
	if got != want {
		t.Errorf("Greet(%q) = %q, want %q", "Go", got, want)
	}
}
