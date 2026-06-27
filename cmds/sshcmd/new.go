package sshcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pubgo/redant"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type sshLoginOptions struct {
	ConfigPath string `yaml:"-" json:"-"`
	IP         string `yaml:"ip" json:"ip"`
	Username   string `yaml:"username" json:"username"`
	APIUser    string `yaml:"api_username" json:"api_username"`
	Password   string `yaml:"password" json:"password"`
	SSHUser    string `yaml:"ssh_user" json:"ssh_user"`
	SSHPort    string `yaml:"ssh_port" json:"ssh_port"`
	SecondPwd  string `yaml:"second_pass" json:"second_pass"`
	APIURL     string `yaml:"passwd_api" json:"passwd_api"`
	Token      string `yaml:"token" json:"token"`
	Timeout    string `yaml:"timeout" json:"timeout"`
}

func New() *redant.Command {
	flags := sshLoginOptions{}
	configArg := ""

	return &redant.Command{
		Use:   "ssh-login [config-file]",
		Short: "ssh login with secondary verification",
		Args: redant.ArgSet{
			{Name: "config-file", Description: "config file path (yaml/json)", Value: redant.StringOf(&configArg)},
		},
		Options: []redant.Option{
			{Flag: "config", Value: redant.StringOf(&flags.ConfigPath), Description: "config file path (yaml/json)"},
			{Flag: "ip", Value: redant.StringOf(&flags.IP), Description: "device ip"},
			{Flag: "username", Value: redant.StringOf(&flags.Username), Description: "ssh username (alias of --ssh-user)"},
			{Flag: "api-username", Value: redant.StringOf(&flags.APIUser), Description: "device api username for LAPI"},
			{Flag: "password", Value: redant.StringOf(&flags.Password), Description: "device password"},
			{Flag: "ssh-user", Value: redant.StringOf(&flags.SSHUser), Description: "ssh username"},
			{Flag: "ssh-port", Value: redant.StringOf(&flags.SSHPort), Description: "ssh port"},
			{Flag: "second-pass", Value: redant.StringOf(&flags.SecondPwd), Description: "manual second password"},
			{Flag: "passwd-api", Value: redant.StringOf(&flags.APIURL), Description: "second password api url"},
			{Flag: "token", Value: redant.StringOf(&flags.Token), Description: "second password api token"},
			{Flag: "timeout", Value: redant.StringOf(&flags.Timeout), Description: "expect timeout in seconds"},
		},
		Handler: func(ctx context.Context, _ *redant.Invocation) error {
			cliValues := flags
			effective := sshLoginOptions{}
			configPath := strings.TrimSpace(flags.ConfigPath)
			if strings.TrimSpace(configArg) != "" {
				configPath = strings.TrimSpace(configArg)
			}

			if configPath != "" {
				cfg, err := loadConfig(configPath)
				if err != nil {
					return fmt.Errorf("load config file failed: %w", err)
				}
				mergeFromConfig(&effective, cfg)
			}
			mergeFromFlags(&effective, cliValues)

			if strings.TrimSpace(effective.IP) == "" || strings.TrimSpace(effective.Password) == "" {
				return fmt.Errorf("--ip --password are required")
			}

			if strings.TrimSpace(effective.Username) != "" {
				effective.SSHUser = strings.TrimSpace(effective.Username)
			}
			if strings.TrimSpace(effective.SSHUser) == "" {
				return fmt.Errorf("--ssh-user (or --username) is required")
			}

			sshPort, err := parsePositiveInt(effective.SSHPort)
			if err != nil {
				return fmt.Errorf("invalid --ssh-port: %w", err)
			}
			waitSec, err := parsePositiveInt(effective.Timeout)
			if err != nil {
				return fmt.Errorf("invalid --timeout: %w", err)
			}

			second := strings.TrimSpace(effective.SecondPwd)
			if second == "" {
				if strings.TrimSpace(effective.APIUser) == "" || strings.TrimSpace(effective.APIURL) == "" || strings.TrimSpace(effective.Token) == "" {
					return fmt.Errorf("when --second-pass is empty, --api-username --passwd-api --token are required")
				}
				var err error
				second, err = fetchSecondPassword(ctx, effective.IP, effective.APIUser, effective.Password, effective.APIURL, effective.Token)
				if err != nil {
					return err
				}
			}

			return loginSSH(effective.IP, sshPort, effective.SSHUser, effective.Password, second, time.Duration(waitSec)*time.Second)
		},
	}
}

func loadConfig(path string) (sshLoginOptions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sshLoginOptions{}, err
	}

	cfg := sshLoginOptions{}
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return sshLoginOptions{}, err
	}
	return cfg, nil
}

func mergeFromConfig(dst *sshLoginOptions, cfg sshLoginOptions) {
	if strings.TrimSpace(cfg.IP) != "" {
		dst.IP = cfg.IP
	}
	if strings.TrimSpace(cfg.Username) != "" {
		dst.Username = cfg.Username
	}
	if strings.TrimSpace(cfg.APIUser) != "" {
		dst.APIUser = cfg.APIUser
	}
	if strings.TrimSpace(cfg.Password) != "" {
		dst.Password = cfg.Password
	}
	if strings.TrimSpace(cfg.SSHUser) != "" {
		dst.SSHUser = cfg.SSHUser
	}
	if strings.TrimSpace(cfg.SSHPort) != "" {
		dst.SSHPort = cfg.SSHPort
	}
	if strings.TrimSpace(cfg.SecondPwd) != "" {
		dst.SecondPwd = cfg.SecondPwd
	}
	if strings.TrimSpace(cfg.APIURL) != "" {
		dst.APIURL = cfg.APIURL
	}
	if strings.TrimSpace(cfg.Token) != "" {
		dst.Token = cfg.Token
	}
	if strings.TrimSpace(cfg.Timeout) != "" {
		dst.Timeout = cfg.Timeout
	}
}

func mergeFromFlags(dst *sshLoginOptions, cli sshLoginOptions) {
	if strings.TrimSpace(cli.IP) != "" {
		dst.IP = cli.IP
	}
	if strings.TrimSpace(cli.Username) != "" {
		dst.Username = cli.Username
	}
	if strings.TrimSpace(cli.APIUser) != "" {
		dst.APIUser = cli.APIUser
	}
	if strings.TrimSpace(cli.Password) != "" {
		dst.Password = cli.Password
	}
	if strings.TrimSpace(cli.SSHUser) != "" {
		dst.SSHUser = cli.SSHUser
	}
	if strings.TrimSpace(cli.SSHPort) != "" {
		dst.SSHPort = cli.SSHPort
	}
	if strings.TrimSpace(cli.SecondPwd) != "" {
		dst.SecondPwd = cli.SecondPwd
	}
	if strings.TrimSpace(cli.APIURL) != "" {
		dst.APIURL = cli.APIURL
	}
	if strings.TrimSpace(cli.Token) != "" {
		dst.Token = cli.Token
	}
	if strings.TrimSpace(cli.Timeout) != "" {
		dst.Timeout = cli.Timeout
	}
}

func parsePositiveInt(v string) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, fmt.Errorf("value is empty")
	}
	val, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	if val <= 0 {
		return 0, fmt.Errorf("must be > 0")
	}
	return val, nil
}

func fetchSecondPassword(ctx context.Context, ip, username, password, apiURL, token string) (string, error) {
	_, err := curlDigest(ctx, "PUT", fmt.Sprintf("http://%s/LAPI/V1.0/Network/SSH", ip), username, password, `{"Enabled": 1}`)
	if err != nil {
		return "", fmt.Errorf("enable ssh failed: %w", err)
	}

	factorBody, err := curlDigest(ctx, "GET", fmt.Sprintf("http://%s/LAPI/V1.0/System/Security/Shell/Factor", ip), username, password, "")
	if err != nil {
		return "", fmt.Errorf("get factor failed: %w", err)
	}

	deviceBody, err := curlDigest(ctx, "GET", fmt.Sprintf("http://%s/LAPI/V1.0/System/DeviceInfo", ip), username, password, "")
	if err != nil {
		return "", fmt.Errorf("get device info failed: %w", err)
	}

	randomFactor := jsonGet(factorBody, "Response", "Data", "RandomFactor")
	if randomFactor == "" {
		randomFactor = jsonFindString(factorBody, "RandomFactor")
	}
	serialNumber := jsonGet(deviceBody, "Response", "Data", "SerialNumber")
	if serialNumber == "" {
		serialNumber = jsonFindString(deviceBody, "SerialNumber")
	}
	if randomFactor == "" || serialNumber == "" {
		return "", fmt.Errorf("factor or serial number is empty (factor=%q serial=%q factorBody=%s deviceBody=%s)", randomFactor, serialNumber, clip(string(factorBody), 240), clip(string(deviceBody), 240))
	}

	v := url.Values{}
	v.Set("token", token)
	v.Set("sn", serialNumber)
	v.Set("factor", randomFactor)
	v.Set("date", time.Now().Format("20060102"))
	out, err := curl(ctx, "GET", apiURL+"?"+v.Encode())
	if err != nil {
		return "", fmt.Errorf("get second password failed: %w", err)
	}

	pass := strings.TrimSpace(string(out))
	if pass == "" {
		return "", fmt.Errorf("second password is empty")
	}

	return pass, nil
}

func curlDigest(ctx context.Context, method, reqURL, user, pass, body string) ([]byte, error) {
	args := []string{"-sS", "-L", "--digest", "-u", fmt.Sprintf("%s:%s", user, pass), "-X", method, reqURL}
	if body != "" {
		args = append(args, "-H", "Content-Type: application/json", "-d", body)
	}
	return curlExec(ctx, args...)
}

func curl(ctx context.Context, method, reqURL string) ([]byte, error) {
	return curlExec(ctx, "-sS", "-L", "-X", method, reqURL)
}

func curlExec(ctx context.Context, args ...string) ([]byte, error) {
	fmt.Printf("[ssh-login] curl: %s\n", buildCurlLog(args...))
	out, err := exec.CommandContext(ctx, "curl", args...).CombinedOutput()
	fmt.Printf("[ssh-login] curl response: %s\n", clip(string(out), 600))
	if err != nil {
		return nil, fmt.Errorf("curl failed: %w, output: %s", err, string(out))
	}
	return out, nil
}

func buildCurlLog(args ...string) string {
	masked := make([]string, 0, len(args)+1)
	masked = append(masked, "curl")
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-u" && i+1 < len(args) {
			masked = append(masked, arg)
			masked = append(masked, maskBasicAuth(args[i+1]))
			i++
			continue
		}
		masked = append(masked, arg)
	}
	return strings.Join(masked, " ")
}

func maskBasicAuth(s string) string {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return "***"
	}
	if strings.TrimSpace(parts[0]) == "" {
		return "***:***"
	}
	return parts[0] + ":***"
}

func jsonGet(data []byte, path ...string) string {
	var root map[string]any
	if json.Unmarshal(data, &root) != nil {
		return ""
	}
	var cur any = root
	for _, p := range path {
		n, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = n[p]
		if !ok {
			return ""
		}
	}
	if s, ok := cur.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func jsonFindString(data []byte, targetKey string) string {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return ""
	}

	var walk func(v any) string
	walk = func(v any) string {
		switch n := v.(type) {
		case map[string]any:
			if val, ok := n[targetKey]; ok {
				if s, ok := val.(string); ok {
					return strings.TrimSpace(s)
				}
			}
			for _, child := range n {
				if got := walk(child); got != "" {
					return got
				}
			}
		case []any:
			for _, child := range n {
				if got := walk(child); got != "" {
					return got
				}
			}
		}
		return ""
	}

	return walk(root)
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func loginSSH(ip string, port int, user, password, secondPass string, timeout time.Duration) error {
	stdinFD := int(os.Stdin.Fd())
	var restoreState *term.State
	if term.IsTerminal(stdinFD) {
		var err error
		restoreState, err = term.MakeRaw(stdinFD)
		if err != nil {
			return fmt.Errorf("set terminal raw mode failed: %w", err)
		}
		defer func() {
			_ = term.Restore(stdinFD, restoreState)
		}()
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	})
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	sess, err := conn.NewSession()
	if err != nil {
		return err
	}
	defer func() { _ = sess.Close() }()

	rows, cols := getTerminalSize(stdinFD)
	if err = sess.RequestPty("xterm-256color", rows, cols, ssh.TerminalModes{ssh.ECHO: 1}); err != nil {
		return err
	}
	_ = sess.Setenv("TERM", "xterm-256color")
	_ = sess.Setenv("COLORTERM", "truecolor")
	stopWinSync := watchWindowSize(sess, stdinFD)
	defer stopWinSync()

	in, _ := sess.StdinPipe()
	out, _ := sess.StdoutPipe()
	errOut, _ := sess.StderrPipe()
	w := &watch{ch: make(chan string, 256)}
	go w.copy(out)
	go w.copy(errOut)

	if err = sess.Shell(); err != nil {
		return err
	}
	if err = w.wait(timeout, "#", "$", ">"); err != nil {
		return err
	}
	_, _ = io.WriteString(in, "login\n")
	if err = w.wait(timeout, "Username:", "username:"); err != nil {
		return err
	}
	_, _ = io.WriteString(in, "zhimakaimen\x11\x17\n")
	if err = w.wait(timeout, "asswd:", "password:", "Password:"); err != nil {
		return err
	}
	_, _ = io.WriteString(in, secondPass+"\n")

	go func() { _, _ = io.Copy(in, os.Stdin) }()
	return sess.Wait()
}

func watchWindowSize(sess *ssh.Session, fd int) func() {
	if fd <= 0 || !term.IsTerminal(fd) {
		return func() {}
	}

	if width, height, err := term.GetSize(fd); err == nil {
		_ = sess.WindowChange(height, width)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ch:
				if width, height, err := term.GetSize(fd); err == nil {
					_ = sess.WindowChange(height, width)
				}
			}
		}
	}()

	return func() {
		signal.Stop(ch)
		close(done)
	}
}

func getTerminalSize(fd int) (rows int, cols int) {
	rows, cols = 40, 120
	if fd <= 0 || !term.IsTerminal(fd) {
		return rows, cols
	}

	width, height, err := term.GetSize(fd)
	if err != nil {
		return rows, cols
	}
	if height > 0 {
		rows = height
	}
	if width > 0 {
		cols = width
	}
	return rows, cols
}

type watch struct {
	ch   chan string
	mu   sync.Mutex
	hist string
}

func (w *watch) copy(r io.Reader) {
	b := make([]byte, 1024)
	for {
		n, err := r.Read(b)
		if n > 0 {
			s := string(b[:n])
			_, _ = os.Stdout.Write(b[:n])
			w.mu.Lock()
			w.hist += s
			if len(w.hist) > 4096 {
				w.hist = w.hist[len(w.hist)-4096:]
			}
			w.mu.Unlock()
			select {
			case w.ch <- s:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

func (w *watch) wait(timeout time.Duration, keys ...string) error {
	t := time.NewTimer(timeout)
	defer t.Stop()
	for {
		w.mu.Lock()
		h := w.hist
		w.mu.Unlock()
		for _, k := range keys {
			if strings.Contains(h, k) {
				return nil
			}
		}
		select {
		case <-t.C:
			return fmt.Errorf("wait prompt timeout: %s", strings.Join(keys, ","))
		case <-w.ch:
		}
	}
}
