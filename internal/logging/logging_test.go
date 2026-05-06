package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, true)

	// After Init with debug=true, IsDebug should return true
	if !IsDebug() {
		t.Error("IsDebug() = false, want true after Init with debug=true")
	}

	// Debug messages should appear
	Debug("test debug message", "key", "value")
	if !strings.Contains(buf.String(), "test debug message") {
		t.Errorf("Debug message not found in output: %s", buf.String())
	}
}

func TestSetDebug(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, false)

	if IsDebug() {
		t.Error("IsDebug() = true, want false after Init with debug=false")
	}

	// Debug should be silent when debug=false
	Debug("should not appear")
	if buf.String() != "" {
		t.Errorf("Debug output should be empty, got: %s", buf.String())
	}

	// Enable debug
	SetDebug(true)
	if !IsDebug() {
		t.Error("IsDebug() = false after SetDebug(true)")
	}

	// Now debug should appear
	Debug("now visible")
	if !strings.Contains(buf.String(), "now visible") {
		t.Errorf("Debug message should appear after SetDebug(true): %s", buf.String())
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, true)

	tests := []struct {
		name    string
		logFunc func(string, ...any)
		msg     string
	}{
		{"Debug", Debug, "debug message"},
		{"Info", Info, "info message"},
		{"Warn", Warn, "warn message"},
		{"Error", Error, "error message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.logFunc(tt.msg, "key", "value")
			if !strings.Contains(buf.String(), tt.msg) {
				t.Errorf("%s message not found: %s", tt.name, buf.String())
			}
		})
	}
}

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, true)

	Info("test message", "count", 42, "enabled", true)

	output := buf.String()
	// Should be JSON
	if !strings.Contains(output, `"msg"`) {
		t.Errorf("Output should contain JSON msg field: %s", output)
	}
	if !strings.Contains(output, `"count"`) {
		t.Errorf("Output should contain JSON count field: %s", output)
	}
	if !strings.Contains(output, `"enabled"`) {
		t.Errorf("Output should contain JSON enabled field: %s", output)
	}
}

func TestConcurrentLogging(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, true)

	// Concurrent logging should not race
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) {
			Info("concurrent message", "n", n)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestLevelThreshold(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, false) // debug=false, level=Warn

	// Debug and Info should be silent
	Debug("debug silent")
	Info("info silent")
	if buf.String() != "" {
		t.Errorf("Debug/Info should be silent at Warn level: %s", buf.String())
	}

	// Warn and Error should appear
	Warn("warn visible")
	if !strings.Contains(buf.String(), "warn visible") {
		t.Errorf("Warn should appear: %s", buf.String())
	}

	buf.Reset()
	Error("error visible")
	if !strings.Contains(buf.String(), "error visible") {
		t.Errorf("Error should appear: %s", buf.String())
	}
}

func TestSpan(t *testing.T) {
	var buf bytes.Buffer
	Init(&buf, true)

	end := Span("operation", "key", "value")
	output := buf.String()
	if !strings.Contains(output, `"operation"`) {
		t.Errorf("Span should log operation: %s", output)
	}
	end()
	// After end, there should be duration logged
	output = buf.String()
	if !strings.Contains(output, `"duration_ms"`) {
		t.Errorf("Span end should log duration_ms: %s", output)
	}
}

func BenchmarkDebug(b *testing.B) {
	var buf bytes.Buffer
	Init(&buf, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Debug("benchmark message", "n", i)
	}
}

func BenchmarkInfo(b *testing.B) {
	var buf bytes.Buffer
	Init(&buf, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Info("benchmark message", "n", i)
	}
}

func BenchmarkSpan(b *testing.B) {
	var buf bytes.Buffer
	Init(&buf, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		end := Span("benchmark span", "n", i)
		end()
	}
}