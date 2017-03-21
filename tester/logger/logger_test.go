package logger

import (
	"bytes"
	"testing"

	"os"
)

func TestLogger(t *testing.T) {
	_, o, e := Log(false, os.Stdout, os.Stderr)
	_, ok := o.(*bytes.Buffer)
	if !ok {
		t.Fatal("Stdout is not a *Buffer")
	}
	_, ok = e.(*bytes.Buffer)
	if !ok {
		t.Fatal("Stderr is not a *Buffer")
	}

	_, o, e = Log(true, os.Stdout, os.Stderr)
	_, ok = o.(*multiWriter)
	if !ok {
		t.Fatal("Stdout is not a *multiWriter")
	}
	_, ok = e.(*multiWriter)
	if !ok {
		t.Fatal("Stderr is not a *multiWriter")
	}
}

func TestMultiWriter(t *testing.T) {
	var p, w1, w2 []byte
	pb := bytes.NewBuffer(p)
	w1b := bytes.NewBuffer(w1)
	w2b := bytes.NewBuffer(w2)
	mw := MultiWriter(pb, w1b, w2b)
	mw.Write([]byte("a"))
	mw.Write([]byte("b"))
	if pb.String() != "ab" {
		t.Fatalf("pb expected 'ab', got '%s'", pb.String())
	}
	if w1b.String() != "ab" {
		t.Fatalf("w1b expected 'ab', got '%s'", pb.String())
	}
	if w2b.String() != "ab" {
		t.Fatalf("w2b expected 'ab', got '%s'", pb.String())
	}
}
