package cli

import (
	"os"
	"testing"
)

// A pipe and /dev/null are not terminals, though /dev/null is a character
// device: setup must not ask a question nobody can answer (FH-129).
func TestPipesAndDevNullAreNotTerminals(t *testing.T) {
	t.Parallel()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminal(r) {
		t.Error("a pipe is a terminal")
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	if IsTerminal(null) {
		t.Error("/dev/null is a terminal")
	}
}
