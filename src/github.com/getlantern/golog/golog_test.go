package golog

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	expectedLog      = "myprefix: golog_test.go:([0-9]+) Hello world\nmyprefix: golog_test.go:([0-9]+) Hello 5\n"
	expectedTraceLog = "myprefix: golog_test.go:([0-9]+) Hello world\nmyprefix: golog_test.go:([0-9]+) Hello 5\nmyprefix: golog_test.go:([0-9]+) Gravy\nmyprefix: golog_test.go:([0-9]+) TraceWriter closed due to unexpected error: EOF\n"
	expectedStdLog   = "myprefix: golog_test.go:([0-9]+) Hello world\nmyprefix: golog_test.go:([0-9]+) Hello 5\n"
)

func expected(severity string, log string) *regexp.Regexp {
	return regexp.MustCompile(severitize(severity, log))
}

func severitize(severity string, log string) string {
	return strings.Replace(log, "myprefix", severity+" myprefix", 4)
}

func TestDebug(t *testing.T) {
	out := newBuffer()
	SetOutputs(ioutil.Discard, out)
	l := LoggerFor("myprefix")
	l.Debug("Hello world")
	l.Debugf("Hello %d", 5)
	assert.Regexp(t, expected("DEBUG", expectedLog), string(out.Bytes()))
}

func TestError(t *testing.T) {
	out := newBuffer()
	SetOutputs(out, ioutil.Discard)
	l := LoggerFor("myprefix")
	l.Error("Hello world")
	l.Errorf("Hello %d", 5)

	assert.Regexp(t, expected("ERROR", expectedLog), string(out.Bytes()))
}

func TestTraceEnabled(t *testing.T) {
	originalTrace := os.Getenv("TRACE")
	err := os.Setenv("TRACE", "true")
	if err != nil {
		t.Fatalf("Unable to set trace to true")
	}
	defer func() {
		if err := os.Setenv("TRACE", originalTrace); err != nil {
			t.Fatalf("Unable to set TRACE environment variable: %v", err)
		}
	}()

	out := newBuffer()
	SetOutputs(ioutil.Discard, out)
	l := LoggerFor("myprefix")
	l.Trace("Hello world")
	l.Tracef("Hello %d", 5)
	tw := l.TraceOut()
	if _, err := tw.Write([]byte("Gravy\n")); err != nil {
		t.Fatalf("Unable to write: %v", err)
	}
	if err := tw.(io.Closer).Close(); err != nil {
		t.Fatalf("Unable to close: %v", err)
	}

	// Give trace writer a moment to catch up
	time.Sleep(50 * time.Millisecond)
	assert.Regexp(t, severitize("TRACE", expectedTraceLog), string(out.Bytes()))
}

func TestTraceDisabled(t *testing.T) {
	originalTrace := os.Getenv("TRACE")
	err := os.Setenv("TRACE", "false")
	if err != nil {
		t.Fatalf("Unable to set trace to false")
	}
	defer func() {
		if err := os.Setenv("TRACE", originalTrace); err != nil {
			t.Fatalf("Unable to set TRACE environment variable: %v", err)
		}
	}()

	out := newBuffer()
	SetOutputs(ioutil.Discard, out)
	l := LoggerFor("myprefix")
	l.Trace("Hello world")
	l.Tracef("Hello %d", 5)
	if _, err := l.TraceOut().Write([]byte("Gravy\n")); err != nil {
		t.Fatalf("Unable to write: %v", err)
	}

	// Give trace writer a moment to catch up
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, "", string(out.Bytes()), "Nothing should have been logged")
}

func TestAsStdLogger(t *testing.T) {
	out := newBuffer()
	SetOutputs(out, ioutil.Discard)
	l := LoggerFor("myprefix")
	stdlog := l.AsStdLogger()
	stdlog.Print("Hello world")
	stdlog.Printf("Hello %d", 5)
	assert.Regexp(t, severitize("ERROR", expectedStdLog), string(out.Bytes()))
}

func newBuffer() *synchronizedbuffer {
	return &synchronizedbuffer{orig: &bytes.Buffer{}}
}

type synchronizedbuffer struct {
	orig  *bytes.Buffer
	mutex sync.RWMutex
}

func (buf *synchronizedbuffer) Write(p []byte) (int, error) {
	buf.mutex.Lock()
	defer buf.mutex.Unlock()
	return buf.orig.Write(p)
}

func (buf *synchronizedbuffer) Bytes() []byte {
	buf.mutex.RLock()
	defer buf.mutex.RUnlock()
	return buf.orig.Bytes()
}
