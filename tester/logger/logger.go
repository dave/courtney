package logger

import (
	"bytes"
	"io"
)

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

func (t *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		w.Write(p)
	}
	return t.primary.Write(p)
}
