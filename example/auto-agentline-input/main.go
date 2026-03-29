package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

type PTYRunner struct {
	cmd  *exec.Cmd
	pty  *os.File
	mu   sync.Mutex
	buf  bytes.Buffer
	done chan struct{}
	wmu  sync.Mutex
	werr error
	wset bool
}

func NewPTYRunner(cmd *exec.Cmd) *PTYRunner {
	return &PTYRunner{cmd: cmd, done: make(chan struct{})}
}

func (r *PTYRunner) Start() error {
	if r == nil || r.cmd == nil {
		return fmt.Errorf("runner/cmd 不能为空")
	}
	f, err := pty.Start(r.cmd)
	if err != nil {
		return err
	}
	r.pty = f
	return nil
}

func (r *PTYRunner) AttachInteractive() error {
	if r == nil || r.pty == nil {
		return fmt.Errorf("pty 未启动")
	}

	inFD := int(os.Stdin.Fd())
	if !term.IsTerminal(inFD) {
		go func() { _, _ = io.Copy(r.pty, os.Stdin) }()
		_, err := io.Copy(os.Stdout, r.pty)
		if errors.Is(err, syscall.EIO) {
			return nil
		}
		return err
	}

	oldState, err := term.MakeRaw(inFD)
	if err != nil {
		return fmt.Errorf("切换 raw 模式失败: %w", err)
	}
	defer func() { _ = term.Restore(inFD, oldState) }()

	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	defer signal.Stop(resizeCh)

	_ = pty.InheritSize(os.Stdin, r.pty)
	go func() {
		for range resizeCh {
			_ = pty.InheritSize(os.Stdin, r.pty)
		}
	}()
	resizeCh <- syscall.SIGWINCH

	go func() {
		_, _ = io.Copy(r.pty, os.Stdin)
	}()

	_, err = io.Copy(os.Stdout, r.pty)
	if errors.Is(err, syscall.EIO) {
		return nil
	}
	return err
}

func (r *PTYRunner) AttachCapture(stdout io.Writer) {
	if r == nil || r.pty == nil {
		return
	}
	if stdout == nil {
		stdout = os.Stdout
	}

	go func() {
		defer close(r.done)
		buf := make([]byte, 4096)
		for {
			n, err := r.pty.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				r.mu.Lock()
				_, _ = r.buf.Write(chunk)
				r.mu.Unlock()
				_, _ = stdout.Write(chunk)
			}
			if err != nil {
				return
			}
		}
	}()

}

func (r *PTYRunner) SendLine(line string) error {
	if r == nil || r.pty == nil {
		return fmt.Errorf("pty 未启动")
	}
	_, err := io.WriteString(r.pty, strings.TrimRight(line, "\n")+"\n")
	return err
}

func (r *PTYRunner) ExpectContains(substr string, timeout time.Duration) error {
	if r == nil {
		return fmt.Errorf("runner 不能为空")
	}
	substr = strings.TrimSpace(substr)
	if substr == "" {
		return nil
	}

	deadline := time.Now().Add(timeout)
	for {
		r.mu.Lock()
		text := r.buf.String()
		r.mu.Unlock()
		if strings.Contains(text, substr) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("expect timeout: %q", substr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (r *PTYRunner) Close() error {
	if r == nil {
		return nil
	}
	if r.pty != nil {
		_ = r.pty.Close()
	}
	select {
	case <-r.done:
	default:
	}
	return nil
}

func (r *PTYRunner) Wait() error {
	if r == nil || r.cmd == nil {
		return nil
	}
	r.wmu.Lock()
	if r.wset {
		err := r.werr
		r.wmu.Unlock()
		return err
	}
	r.wmu.Unlock()

	err := r.cmd.Wait()

	r.wmu.Lock()
	r.werr = err
	r.wset = true
	r.wmu.Unlock()
	return err
}

func main() {
	command := flag.String("cmd", "copilot", "要封装的命令（建议 interactive 命令）")
	script := flag.String("script", "", "自动输入脚本，使用 \\n 分隔；默认空表示不自动发送")
	pipeStdin := flag.Bool("stdin", true, "将当前终端输入透传给子进程（交互模式建议开启）")
	stepDelay := flag.Duration("step-delay", 300*time.Millisecond, "每条输入之间的间隔")
	timeout := flag.Duration("timeout", 0, "整体超时；0 表示不设置超时")
	expect := flag.String("expect", "", "发送脚本前等待输出包含该文本（可选）")
	flag.Parse()

	ctx := context.Background()
	cancel := func() {}
	if timeout != nil && *timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), *timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc", strings.TrimSpace(*command))
	runner := NewPTYRunner(cmd)
	if err := runner.Start(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "start pty failed: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = runner.Close() }()

	interactiveMode := *pipeStdin
	if interactiveMode && strings.TrimSpace(*expect) != "" {
		_, _ = fmt.Fprintln(os.Stderr, "warn: interactive 模式下忽略 -expect（建议配合 -script 使用固定步骤）")
	}

	if !interactiveMode {
		runner.AttachCapture(os.Stdout)
	}

	if !interactiveMode && strings.TrimSpace(*expect) != "" {
		if err := runner.ExpectContains(*expect, 8*time.Second); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "expect failed: %v\n", err)
			os.Exit(1)
		}
	}

	steps := strings.Split(strings.ReplaceAll(*script, "\\r\\n", "\\n"), "\\n")
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		if err := runner.SendLine(step); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "send failed: %v\n", err)
			os.Exit(1)
		}
		time.Sleep(*stepDelay)
	}

	if interactiveMode {
		if err := runner.AttachInteractive(); err != nil {
			if ctx.Err() != nil {
				_, _ = fmt.Fprintf(os.Stderr, "command timeout/canceled: %v\n", ctx.Err())
				os.Exit(1)
			}
			_, _ = fmt.Fprintf(os.Stderr, "interactive attach failed: %v\n", err)
			os.Exit(1)
		}
	}

	if err := runner.Wait(); err != nil {
		if ctx.Err() != nil {
			_, _ = fmt.Fprintf(os.Stderr, "command timeout/canceled: %v\n", ctx.Err())
			os.Exit(1)
		}
		_, _ = fmt.Fprintf(os.Stderr, "command exited with error: %v\n", err)
		os.Exit(1)
	}
}
