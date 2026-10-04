package background

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const helperEnv = "FLASHHEART_BACKGROUND_TEST_HELPER"

// TestMain lets the test binary act as the detached child.
func TestMain(m *testing.M) {
	if mode := os.Getenv(helperEnv); mode != "" {
		os.Exit(runHelper(mode))
	}
	os.Exit(m.Run())
}

func runHelper(mode string) int {
	switch mode {
	case "crash":
		return 3
	case "hang":
		time.Sleep(time.Minute)
		return 0
	}
	handshake, err := OpenHandshake()
	if err != nil {
		return 4
	}
	switch mode {
	case "ready":
		_ = handshake.Ready(Report{Address: "127.0.0.1:4321"})
		_ = handshake.Ready(Report{Address: "ignored"})
		// Keep running after the handshake, like the real server.
		time.Sleep(200 * time.Millisecond)
	case "manual":
		_ = handshake.Ready(Report{Address: "127.0.0.1:4321", ManualURL: "http://ss-1.localhost:4321/#secret", BrowserError: "no opener"})
	case "fail":
		_ = handshake.Fail(errors.New("read config: bad theme"))
		_ = handshake.Ready(Report{Address: "ignored"})
		return 1
	case "stdout":
		os.Stdout.WriteString("stray output\n")
		_ = handshake.Ready(Report{Address: "127.0.0.1:1"})
	}
	return 0
}

func startHelper(t *testing.T, mode string, timeout time.Duration) (Report, error) {
	t.Helper()
	return Start(context.Background(), Options{
		Executable: os.Args[0],
		Args:       []string{"-test.run=^$"},
		Env:        append(os.Environ(), helperEnv+"="+mode),
		Timeout:    timeout,
	})
}

func TestStartReturnsTheChildsReadyReport(t *testing.T) {
	t.Parallel()

	started := time.Now()
	report, err := startHelper(t, "ready", 10*time.Second)
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	if report != (Report{Address: "127.0.0.1:4321"}) {
		t.Errorf("report = %+v", report)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("Start waited %s; it must return once the child reports ready", elapsed)
	}
}

func TestStartCarriesTheManualURL(t *testing.T) {
	t.Parallel()

	report, err := startHelper(t, "manual", 10*time.Second)
	if err != nil {
		t.Fatalf("Start(): %v", err)
	}
	want := Report{Address: "127.0.0.1:4321", ManualURL: "http://ss-1.localhost:4321/#secret", BrowserError: "no opener"}
	if report != want {
		t.Errorf("report = %+v, want %+v", report, want)
	}
}

func TestStartReturnsTheChildsStartupError(t *testing.T) {
	t.Parallel()

	_, err := startHelper(t, "fail", 10*time.Second)
	if err == nil || err.Error() != "read config: bad theme" {
		t.Errorf("err = %v, want the child's error", err)
	}
}

func TestStartReportsAChildThatExitsWithoutHandshake(t *testing.T) {
	t.Parallel()

	_, err := startHelper(t, "crash", 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), "exited during startup") || !strings.Contains(err.Error(), "exit status 3") {
		t.Errorf("err = %v", err)
	}
}

func TestStartStopsAChildThatNeverReports(t *testing.T) {
	t.Parallel()

	started := time.Now()
	_, err := startHelper(t, "hang", 500*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not start within") {
		t.Errorf("err = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Errorf("Start took %s after its timeout", elapsed)
	}
}

func TestChildOutputDoesNotReachTheLauncher(t *testing.T) {
	t.Parallel()

	// The child's stdout is discarded; only the handshake is read.
	report, err := startHelper(t, "stdout", 10*time.Second)
	if err != nil || report.Address != "127.0.0.1:1" {
		t.Errorf("report=%+v err=%v", report, err)
	}
}

func TestStartRejectsMissingExecutable(t *testing.T) {
	t.Parallel()

	_, err := Start(context.Background(), Options{Executable: "/nonexistent/flashheart", Timeout: time.Second})
	if err == nil {
		t.Fatal("Start() succeeded with a missing executable")
	}
}

func TestOpenHandshakeFailsOutsideABackgroundChild(t *testing.T) {
	t.Parallel()

	if _, err := openHandshake(nil); err == nil {
		t.Fatal("openHandshake(nil) succeeded")
	}
}
