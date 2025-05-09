package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	utils "github.com/gxmmx/compage-go/utils"
)

// Helper to capture stdout and stderr
// This is not thread-safe and should not be used in parallel tests
func capOutErr(f func()) (string, string) {
	// Save original file descriptors
	origOut := os.Stdout
	origErr := os.Stderr
	// Create pipes
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	// Redirect
	os.Stdout = wOut
	os.Stderr = wErr
	// Run the function
	f()
	// Close pipes
	wOut.Close()
	wErr.Close()
	// Read output
	outBytes, _ := io.ReadAll(rOut)
	errBytes, _ := io.ReadAll(rErr)
	// Restore original file descriptors
	os.Stdout = origOut
	os.Stderr = origErr
	return string(outBytes), string(errBytes)
}

func TestNew(t *testing.T) {
	t.Run("should use defaults for out and err", func(t *testing.T) {
		appname := utils.AppNameFromBin()
		sout, serr := capOutErr(func() {
			ctrl := New()
			log := ctrl.GetLogger()
			log.Info("testinginfo123")
			log.Error("testingerror123")
		})
		if !strings.Contains(sout, appname) {
			t.Errorf("expected log to contain app name '%s', got '%s'", appname, sout)
		}
		if !strings.Contains(sout, "testinginfo123") {
			t.Errorf("expected log to contain 'testinginfo123', got '%s'", sout)
		}
		if !strings.Contains(serr, "testingerror123") {
			t.Errorf("expected log to contain 'testingerror123', got '%s'", serr)
		}
	})
	t.Run("should not print info on level warn", func(t *testing.T) {
		sout, serr := capOutErr(func() {
			ctrl := New(WithLevel("warn"))
			log := ctrl.GetLogger()
			log.Info("testingdebugmessage")
		})
		if sout != "" {
			t.Errorf("expected no standard output, got '%s'", sout)
		}
		if serr != "" {
			t.Errorf("expected no error output, got '%s'", serr)
		}
	})
}

func TestOptions(t *testing.T) {
	t.Run("should set options", func(t *testing.T) {
		sout, serr := capOutErr(func() {
			ctrl := New(
				WithName("testname"),
				WithUnit("testunit"),
				WithClass("testclass"),
			)
			log := ctrl.GetLogger()
			log.WithGroup("some group").Info("testinginfo123")
		})
		if !strings.Contains(sout, "testname") {
			t.Errorf("expected log to contain name 'testname', got '%s'", sout)
		}
		if !strings.Contains(sout, "testunit") {
			t.Errorf("expected log to contain unit 'testunit', got '%s'", sout)
		}
		if !strings.Contains(sout, "testclass") {
			t.Errorf("expected log to contain class 'testclass', got '%s'", sout)
		}
		if serr != "" {
			t.Errorf("expected no error output, got '%s'", serr)
		}
	})
}

func TestSetLevel(t *testing.T) {
	t.Run("should set level to debug", func(t *testing.T) {
		sout, serr := capOutErr(func() {
			ctrl := New()
			ctrl.SetLevel("debug")
			log := ctrl.GetLogger()
			log.Debug("testingdebugmessage")
			ctrl.SetLevel("error")
			log.Info("thisshouldnotprinaterror")
			ctrl.SetLevel("invalidloglevelshoulddefaulttoinfo")
			log.Info("thisshouldprintatinfo")
		})
		if !strings.Contains(sout, "testingdebugmessage") {
			t.Errorf("expected log to contain 'testingdebugmessage', got '%s'", sout)
		}
		if strings.Contains(sout, "thisshouldnotprinaterror") {
			t.Error("expected log to not contain 'thisshouldnotprint'")
		}
		if !strings.Contains(sout, "thisshouldprintatinfo") {
			t.Error("expected log to contain 'thisshouldprintatinfo'")
		}
		if serr != "" {
			t.Errorf("expected no error output, got '%s'", serr)
		}
	})
}

func TestServiceHandler(t *testing.T) {
	t.Run("should set service handler", func(t *testing.T) {
		sout, _ := capOutErr(func() {
			ctrl := New(WithService(NewExampleLoggerService()))
			log := ctrl.GetLogger()
			log.WithGroup("testgroup").Info("testinginfo123", slog.String("testkey", "testvalue"))
		})
		if !strings.Contains(sout, "ExampleLoggerService:") {
			t.Errorf("expected log to contain service name 'ExampleLoggerService:', got '%s'", sout)
		}
	})
}

func TestClassAndUnit(t *testing.T) {
	t.Run("should set class and unit", func(t *testing.T) {
		sout, _ := capOutErr(func() {
			ctrl := New(WithClass("defaulttestclass"), WithUnit("defaulttestunit"))
			log := ctrl.GetLogger()
			log.Info("testinginfo123", slog.String("unit", "overrideunit"), slog.String("class", "overrideclass"))
		})
		if !strings.Contains(sout, "overrideunit") {
			t.Errorf("expected log to contain class 'overrideunit', got '%s'", sout)
		}
		if !strings.Contains(sout, "overrideclass") {
			t.Errorf("expected log to contain unit 'overrideclass', got '%s'", sout)
		}
		if strings.Contains(sout, "defaulttestclass") {
			t.Errorf("expected log to not contain class 'defaulttestclass', got '%s'", sout)
		}
		if strings.Contains(sout, "defaulttestunit") {
			t.Errorf("expected log to not contain unit 'defaulttestunit', got '%s'", sout)
		}
	})
}
