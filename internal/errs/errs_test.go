package errs

import (
	"errors"
	"testing"
)

func TestWrapKeepsUnsupported(t *testing.T) {
	u := New(Unsupported, "x")
	if got := Wrap(Internal, u, "wrapped").Code; got != Unsupported {
		t.Fatalf("got %s", got)
	}
	if got := Wrap(Internal, Wrap(Internal, u, "inner"), "outer").Code; got != Unsupported {
		t.Fatalf("nested got %s", got)
	}
	if got := Wrap(Internal, errors.New("plain"), "wrapped").Code; got != Internal {
		t.Fatalf("plain got %s", got)
	}
}
