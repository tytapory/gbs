package logger

import (
	"bytes"
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitializeLoggers(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temporary log file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	InitializeLoggers("debug", tmpFile.Name())

	assert.NotNil(t, l.debugLogger, "Debug logger should be initialized")
	assert.NotNil(t, l.infoLogger, "Info logger should be initialized")
	assert.NotNil(t, l.warnLogger, "Warn logger should be initialized")
	assert.NotNil(t, l.errorLogger, "Error logger should be initialized")
	assert.NotNil(t, l.fatalLogger, "Fatal logger should be initialized")
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer

	l.debugLogger = log.New(&buf, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	l.infoLogger = log.New(&buf, "INFO: ", log.Ldate|log.Ltime)
	l.warnLogger = log.New(&buf, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	l.errorLogger = log.New(&buf, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	l.level = DEBUG

	Debug("Debug message")
	assert.Contains(t, buf.String(), "DEBUG: ", "Debug message not logged correctly")
	assert.Contains(t, buf.String(), "Debug message", "Debug message content is incorrect")
	buf.Reset()

	Info("Info message")
	assert.Contains(t, buf.String(), "INFO: ", "Info message not logged correctly")
	assert.Contains(t, buf.String(), "Info message", "Info message content is incorrect")
	buf.Reset()

	Warn("Warn message")
	assert.Contains(t, buf.String(), "WARN: ", "Warn message not logged correctly")
	assert.Contains(t, buf.String(), "Warn message", "Warn message content is incorrect")
	buf.Reset()

	Error("Error message")
	assert.Contains(t, buf.String(), "ERROR: ", "Error message not logged correctly")
	assert.Contains(t, buf.String(), "Error message", "Error message content is incorrect")
	buf.Reset()
}

func TestSetLogLevel(t *testing.T) {
	var buf bytes.Buffer

	l.debugLogger = log.New(&buf, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	l.infoLogger = log.New(&buf, "INFO: ", log.Ldate|log.Ltime)
	l.warnLogger = log.New(&buf, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	l.errorLogger = log.New(&buf, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	l.level = WARN

	Debug("Debug message")
	Info("Info message")
	assert.Empty(t, buf.String(), "Debug and Info messages should not be logged on WARN level")

	Warn("Warn message")
	assert.Contains(t, buf.String(), "WARN: ", "Warn message should be logged")
	buf.Reset()

	Error("Error message")
	assert.Contains(t, buf.String(), "ERROR: ", "Error message should be logged")
}

func TestFallbackToStdout(t *testing.T) {
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	InitializeLoggers("info", "/non-existent-dir/log.log")

	Info("Test stdout fallback")

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = originalStdout

	assert.Contains(t, string(out), "Can't open/create file")
}
