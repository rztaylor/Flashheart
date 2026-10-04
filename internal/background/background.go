package background

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// handshakeFD is the descriptor number of the first entry in exec.Cmd.ExtraFiles.
const handshakeFD = 3

// maxHandshakeBytes bounds the single handshake line read from the child.
const maxHandshakeBytes = 16 << 10

// Options describes the child process to start.
type Options struct {
	Executable string
	Args       []string
	// Env is the child's environment; nil inherits the launcher's.
	Env     []string
	Timeout time.Duration
}

// Report is the child's startup outcome. ManualURL is a short-lived secret.
type Report struct {
	Address      string `json:"address"`
	ManualURL    string `json:"manualURL,omitempty"`
	BrowserError string `json:"browserError,omitempty"`
}

type message struct {
	Report
	Error string `json:"error,omitempty"`
}

// Start launches the child detached from the caller's terminal session and
// waits until it reports ready, reports an error, exits, or times out. On
// success the child keeps running after the caller exits.
func Start(ctx context.Context, options Options) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return Report{}, fmt.Errorf("create startup pipe: %w", err)
	}
	defer reader.Close()

	command := exec.Command(options.Executable, options.Args...)
	command.Env = options.Env
	command.ExtraFiles = []*os.File{writer}
	command.SysProcAttr = detachedAttributes()
	// Nil standard streams are connected to the null device.
	startErr := command.Start()
	writer.Close()
	if startErr != nil {
		return Report{}, fmt.Errorf("start background server: %w", startErr)
	}

	type result struct {
		line []byte
		err  error
	}
	lines := make(chan result, 1)
	go func() {
		line, err := bufio.NewReader(io.LimitReader(reader, maxHandshakeBytes)).ReadBytes('\n')
		lines <- result{line: line, err: err}
	}()

	timer := time.NewTimer(options.Timeout)
	defer timer.Stop()
	select {
	case received := <-lines:
		if received.err != nil {
			waitErr := command.Wait()
			if waitErr == nil {
				return Report{}, errors.New("the background server exited during startup")
			}
			return Report{}, fmt.Errorf("the background server exited during startup (%v)", waitErr)
		}
		var decoded message
		if err := json.Unmarshal(received.line, &decoded); err != nil {
			stop(command)
			return Report{}, fmt.Errorf("the background server sent an unreadable startup report: %w", err)
		}
		if decoded.Error != "" {
			stop(command)
			return Report{}, errors.New(decoded.Error)
		}
		if err := command.Process.Release(); err != nil {
			return Report{}, fmt.Errorf("release background server: %w", err)
		}
		return decoded.Report, nil
	case <-timer.C:
		stop(command)
		return Report{}, fmt.Errorf("the background server did not start within %s", options.Timeout)
	case <-ctx.Done():
		stop(command)
		return Report{}, ctx.Err()
	}
}

func stop(command *exec.Cmd) {
	_ = command.Process.Kill()
	_ = command.Wait()
}

// Handshake is the child's side of the startup report. Only the first Ready
// or Fail is sent; later calls do nothing.
type Handshake struct {
	mu   sync.Mutex
	file *os.File
	sent bool
}

// OpenHandshake opens the descriptor inherited from Start.
func OpenHandshake() (*Handshake, error) {
	return openHandshake(os.NewFile(handshakeFD, "flashheart-handshake"))
}

func openHandshake(file *os.File) (*Handshake, error) {
	if file == nil {
		return nil, errors.New("no startup handshake descriptor")
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		return nil, errors.New("no startup handshake descriptor; serve --background-child is started by flashheart itself")
	}
	// Keep the descriptor out of processes the child starts, such as the
	// browser opener.
	closeOnExec(file)
	return &Handshake{file: file}, nil
}

// Ready reports a successful start.
func (h *Handshake) Ready(report Report) error {
	return h.send(message{Report: report})
}

// Fail reports a startup failure.
func (h *Handshake) Fail(err error) error {
	return h.send(message{Error: err.Error()})
}

func (h *Handshake) send(value message) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sent {
		return nil
	}
	h.sent = true
	data, err := json.Marshal(value)
	if err != nil {
		h.file.Close()
		return err
	}
	_, writeErr := h.file.Write(append(data, '\n'))
	return errors.Join(writeErr, h.file.Close())
}
