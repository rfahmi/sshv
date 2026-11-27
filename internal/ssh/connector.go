package ssh

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
)

// NewConnectCmd returns a Bubble Tea command that launches an interactive SSH session.
// If hasPassword is true, it spawns ssh inside a PTY and auto-fills the password once.
func NewConnectCmd(alias, password string, hasPassword bool, onDone func(error) tea.Msg) tea.Cmd {
	if !hasPassword || password == "" {
		cmd := exec.Command("ssh", alias)
		return tea.ExecProcess(cmd, onDone)
	}
	return tea.Exec(&ptySSHCommand{
		alias:    alias,
		password: password,
	}, onDone)
}

type ptySSHCommand struct {
	alias    string
	password string
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
}

func (p *ptySSHCommand) SetStdin(r io.Reader)  { p.stdin = r }
func (p *ptySSHCommand) SetStdout(w io.Writer) { p.stdout = w }
func (p *ptySSHCommand) SetStderr(w io.Writer) { p.stderr = w }

func (p *ptySSHCommand) Run() error {
	cmd := exec.Command("ssh", p.alias)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	// Keep PTY window size synced with iTerm2.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)
	go func() {
		for range sigCh {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	sigCh <- syscall.SIGWINCH

	inFd := int(os.Stdin.Fd())
	if term.IsTerminal(inFd) {
		oldState, err := term.MakeRaw(inFd)
		if err == nil {
			defer func() { _ = term.Restore(inFd, oldState) }()
		}
	}

	out := p.stdout
	if out == nil {
		out = os.Stdout
	}
	in := p.stdin
	if in == nil {
		in = os.Stdin
	}

	// Use cancelreader so stdin forwarding stops cleanly before Bubble Tea resumes.
	cr, crErr := cancelreader.NewReader(in)
	var inputReader io.Reader = in
	if crErr == nil {
		inputReader = cr
	}

	stdinDone := make(chan struct{})
	go func() {
		defer close(stdinDone)
		_, _ = io.Copy(ptmx, inputReader)
	}()

	var once sync.Once
	var tail []byte
	buf := make([]byte, 4096)
	for {
		n, readErr := ptmx.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = out.Write(chunk)

			tail = append(tail, chunk...)
			if len(tail) > 256 {
				tail = tail[len(tail)-256:]
			}
			if containsPasswordPrompt(tail) {
				once.Do(func() {
					_, _ = ptmx.Write([]byte(p.password + "\n"))
				})
			}
		}
		if readErr != nil {
			break
		}
	}

	if cr != nil {
		cr.Cancel()
		<-stdinDone
		_ = cr.Close()
	}

	return cmd.Wait()
}

func containsPasswordPrompt(b []byte) bool {
	lower := bytes.ToLower(b)
	return bytes.Contains(lower, []byte("password:"))
}
