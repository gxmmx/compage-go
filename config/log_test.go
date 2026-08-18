package config

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestFlushLogDrainsInitializationDiagnostics(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	c, err := Load[testConfig]()
	if err != nil {
		t.Fatal(err)
	}
	c.FlushLog(logger)
	if !contains(output.String(), "config load completed") || !contains(output.String(), "config sources discovered") {
		t.Fatalf("diagnostic output = %q", output.String())
	}
	output.Reset()
	c.FlushLog(logger)
	if output.Len() != 0 {
		t.Fatalf("FlushLog did not drain records: %q", output.String())
	}
}

func TestFlushLogReportsOverflow(t *testing.T) {
	var buffer logBuffer
	for i := 0; i < maxLogRecords+1; i++ {
		buffer.add(newLogRecord(slog.LevelDebug, "record"))
	}
	var output bytes.Buffer
	buffer.flush(slog.New(slog.NewTextHandler(&output, nil)))
	if !contains(output.String(), "config initialization diagnostics discarded") {
		t.Fatalf("overflow warning missing from diagnostic output = %q", output.String())
	}
}
