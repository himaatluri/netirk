package helpers

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestAppName_Constant(t *testing.T) {
	// Test that AppName constant is not empty
	if AppName == "" {
		t.Error("AppName constant should not be empty")
	}

	// Test that AppName contains expected ASCII art characters
	// The banner is ASCII art representation of "netirk"
	if !strings.Contains(AppName, "_") || !strings.Contains(AppName, "|") {
		t.Error("AppName should contain ASCII art characters")
	}

	// Test that AppName is ASCII art (contains underscores and pipes)
	if !strings.Contains(AppName, "_") || !strings.Contains(AppName, "|") {
		t.Error("AppName should contain ASCII art characters")
	}

	// Test that AppName has multiple lines
	lines := strings.Split(AppName, "\n")
	if len(lines) < 3 {
		t.Error("AppName should have multiple lines for ASCII art")
	}
}

func TestGreetBanner(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call GreetBanner
	GreetBanner()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Test that output contains the banner (ASCII art)
	if !strings.Contains(output, "_") || !strings.Contains(output, "|") {
		t.Error("GreetBanner output should contain ASCII art")
	}

	// Test that output starts with newline
	if !strings.HasPrefix(output, "\n") {
		t.Error("GreetBanner output should start with newline")
	}

	// Test that output ends with double newline
	if !strings.HasSuffix(output, "\n\n") {
		t.Error("GreetBanner output should end with double newline")
	}

	// Test that output contains ASCII art characters
	if !strings.Contains(output, "_") || !strings.Contains(output, "|") {
		t.Error("GreetBanner output should contain ASCII art")
	}
}

func TestGreetBanner_OutputFormat(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call GreetBanner
	GreetBanner()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Test that output has expected structure: \n + AppName + \n\n
	expectedOutput := "\n" + AppName + "\n\n"
	if output != expectedOutput {
		t.Errorf("GreetBanner output doesn't match expected format.\nExpected: %q\nGot: %q", expectedOutput, output)
	}
}

func TestGreetBanner_MultipleCallsConsistent(t *testing.T) {
	// Capture first call
	oldStdout := os.Stdout
	r1, w1, _ := os.Pipe()
	os.Stdout = w1
	GreetBanner()
	w1.Close()
	os.Stdout = oldStdout

	var buf1 bytes.Buffer
	io.Copy(&buf1, r1)
	output1 := buf1.String()

	// Capture second call
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	GreetBanner()
	w2.Close()
	os.Stdout = oldStdout

	var buf2 bytes.Buffer
	io.Copy(&buf2, r2)
	output2 := buf2.String()

	// Test that multiple calls produce identical output
	if output1 != output2 {
		t.Error("Multiple calls to GreetBanner should produce identical output")
	}
}

func BenchmarkGreetBanner(b *testing.B) {
	// Redirect stdout to discard output during benchmark
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GreetBanner()
	}
}