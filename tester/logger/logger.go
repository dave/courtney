package logger

import (
	"bytes"
	"io"
)

// Log returns a buffer and two Writers. Data written to the writers is
// combined and stored in the buffer. If verbose = true, it is also written to
// the two provided Writers.
func Log(verbose bool, stdout io.Writer, stderr io.Writer) (log *bytes.Buffer, loggedStdout io.Writer, loggedStderr io.Writer) {
	log = &bytes.Buffer{}
	if verbose {
		loggedStdout = MultiWriter(stdout, log)
		loggedStderr = MultiWriter(stderr, log)
	} else {
		loggedStdout = log
		loggedStderr = log
	}
	return
}

// MultiWriter creates a writer that duplicates its writes to all the
// provided writers, similar to the Unix tee(1) command.
func MultiWriter(primary io.Writer, writers ...io.Writer) io.Writer {
	w := make([]io.Writer, len(writers))
	copy(w, writers)
	return &multiWriter{primary: primary, writers: w}
}

type multiWriter struct {
	primary io.Writer
	writers []io.Writer
}

// Write writes to the writers.
func (t *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		w.Write(p)
	}
	return t.primary.Write(p)
}
