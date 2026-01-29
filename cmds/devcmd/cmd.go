package devcmd

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pubgo/funk/v2/log"
	"github.com/pubgo/funk/v2/pathutil"
	"github.com/pubgo/funk/v2/recovery"
	"github.com/pubgo/redant"
	"gopkg.in/yaml.v3"
)

//go:embed static/index.html
var indexHTML string

type DevConfig struct {
	WebPort  int             `yaml:"web_port" json:"web_port"` // Web 管理界面端口
	Services []ServiceConfig `yaml:"services" json:"services"` // 服务列表
}

type ServiceConfig struct {
	Name        string   `yaml:"name" json:"name"`                   // 服务名称
	Port        int      `yaml:"port" json:"port"`                   // 服务运行端口（可选）
	WatchDirs   []string `yaml:"watch_dirs" json:"watch_dirs"`       // 监控的目录
	WatchExts   []string `yaml:"watch_exts" json:"watch_exts"`       // 监控的文件扩展名
	IgnoreDirs  []string `yaml:"ignore_dirs" json:"ignore_dirs"`     // 忽略的目录
	BuildCmd    string   `yaml:"build_cmd" json:"build_cmd"`         // 构建命令
	RunCmd      string   `yaml:"run_cmd" json:"run_cmd"`             // 运行命令
	RunArgs     []string `yaml:"run_args" json:"run_args"`           // 运行参数
	Delay       int      `yaml:"delay" json:"delay"`                 // 重启延迟（毫秒）
	LogMaxLines int      `yaml:"log_max_lines" json:"log_max_lines"` // 日志最大行数
	Enabled     bool     `yaml:"enabled" json:"enabled"`             // 是否启用
}

type DevServer struct {
	name        string
	config      *ServiceConfig
	watcher     *fsnotify.Watcher
	cmd         *exec.Cmd
	mu          sync.RWMutex
	logs        []LogEntry
	logsMu      sync.RWMutex
	restartCh   chan struct{}
	stopCh      chan struct{}
	lastRestart time.Time
	status      string
}

type DevManager struct {
	webPort    int
	servers    map[string]*DevServer
	serversMu  sync.RWMutex
	configPath string
	httpServer *http.Server
	config     *DevConfig
	configMu   sync.RWMutex
}

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

func New() *redant.Command {
	var flags = new(struct {
		port   string
		config string
	})

	app := &redant.Command{
		Use:   "dev",
		Short: "开发模式：文件监控、自动重启、Web 配置界面（支持多服务）",
		Options: []redant.Option{
			{
				Flag:        "port",
				Description: "Web 管理界面端口，默认 8080",
				Value:       redant.StringOf(&flags.port),
			},
			{
				Flag:        "config",
				Description: "配置文件路径，默认 .dev.yaml",
				Value:       redant.StringOf(&flags.config),
			},
		},
		Handler: func(ctx context.Context, i *redant.Invocation) (gErr error) {
			defer recovery.Exit()

			configPath := flags.config
			if configPath == "" {
				configPath = ".dev.yaml"
			}

			// 加载或创建配置
			cfg := loadOrCreateConfig(configPath)

			// 如果指定了端口，覆盖配置
			if flags.port != "" {
				if port, err := strconv.Atoi(flags.port); err == nil && port > 0 {
					cfg.WebPort = port
				}
			}

			// 创建服务管理器
			manager := NewDevManager(cfg, configPath)

			// 启动管理器
			if err := manager.Start(ctx); err != nil {
				return fmt.Errorf("启动开发服务器失败: %w", err)
			}

			return nil
		},
	}

	return app
}

func loadOrCreateConfig(path string) *DevConfig {
	cfg := &DevConfig{
		WebPort: 8080,
		Services: []ServiceConfig{
			{
				Name:        "default",
				WatchDirs:   []string{"."},
				WatchExts:   []string{".go"},
				IgnoreDirs:  []string{".git", "vendor", "node_modules", "tmp", ".air"},
				BuildCmd:    "go build -o tmp/main .",
				RunCmd:      "./tmp/main",
				RunArgs:     []string{},
				Delay:       500,
				LogMaxLines: 1000,
				Enabled:     true,
			},
		},
	}

	if pathutil.IsExist(path) {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				log.Warn().Err(err).Msg("配置文件解析失败，使用默认配置")
			}
		}
	} else {
		// 创建默认配置文件
		data, err := yaml.Marshal(cfg)
		if err == nil {
			if err := os.WriteFile(path, data, 0644); err == nil {
				log.Info().Msgf("已创建默认配置文件: %s", path)
			}
		}
	}

	return cfg
}

func NewDevManager(cfg *DevConfig, configPath string) *DevManager {
	manager := &DevManager{
		webPort:    cfg.WebPort,
		servers:    make(map[string]*DevServer),
		configPath: configPath,
		config:     cfg,
	}

	// 为每个启用的服务创建服务器实例
	for _, svcCfg := range cfg.Services {
		if svcCfg.Enabled {
			server := NewDevServer(svcCfg.Name, &svcCfg)
			manager.servers[svcCfg.Name] = server
		}
	}

	return manager
}

func NewDevServer(name string, cfg *ServiceConfig) *DevServer {
	return &DevServer{
		name:      name,
		config:    cfg,
		restartCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		status:    "stopped",
		logs:      make([]LogEntry, 0, cfg.LogMaxLines),
	}
}

func (m *DevManager) Start(ctx context.Context) error {
	// 启动所有服务
	m.serversMu.RLock()
	for name, server := range m.servers {
		go func(s *DevServer, n string) {
			if err := s.Start(ctx); err != nil {
				log.Error().Err(err).Msgf("服务 %s 启动失败", n)
			}
		}(server, name)
	}
	m.serversMu.RUnlock()

	// 启动 Web 管理界面
	go m.startWebServer()

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Info().Msg("收到中断信号，正在关闭...")
	case <-ctx.Done():
		log.Info().Msg("上下文已取消，正在关闭...")
	}

	return m.Stop()
}

func (m *DevManager) Stop() error {
	m.serversMu.RLock()
	defer m.serversMu.RUnlock()

	// 停止所有服务
	for _, server := range m.servers {
		server.Stop()
	}

	// 停止 Web 服务器
	if m.httpServer != nil {
		m.httpServer.Shutdown(context.Background())
	}

	return nil
}

func (s *DevServer) Start(ctx context.Context) error {
	// 创建文件监控
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监控失败: %w", err)
	}
	s.watcher = watcher

	// 添加监控目录
	if err := s.addWatchDirs(); err != nil {
		log.Warn().Err(err).Msgf("服务 %s 添加监控目录失败", s.name)
	}

	// 启动文件监控
	go s.watchFiles()

	// 启动进程管理
	go s.manageProcess()

	// 初始启动
	s.restartCh <- struct{}{}

	// 等待停止信号
	<-s.stopCh
	return nil
}

func (s *DevServer) Stop() error {
	close(s.stopCh)

	if s.watcher != nil {
		s.watcher.Close()
	}

	s.stopProcess()

	return nil
}

func (s *DevServer) watchFiles() {
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			// 检查文件扩展名
			if !s.shouldWatch(event.Name) {
				continue
			}

			// 处理目录创建事件，添加监控
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					// 检查是否应该忽略
					shouldIgnore := false
					for _, ignoreDir := range s.config.IgnoreDirs {
						if strings.Contains(event.Name, ignoreDir) {
							shouldIgnore = true
							break
						}
					}
					if !shouldIgnore {
						s.watcher.Add(event.Name)
					}
				}
			}

			// 忽略某些事件
			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Remove == fsnotify.Remove {
				s.addLog("info", fmt.Sprintf("文件变更: %s (%s)", event.Name, event.Op))
				s.scheduleRestart()
			}

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			s.addLog("error", fmt.Sprintf("文件监控错误: %v", err))

		case <-s.stopCh:
			return
		}
	}
}

func (s *DevServer) addWatchDirs() error {
	for _, dir := range s.config.WatchDirs {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 忽略错误，继续遍历
			}

			// 检查是否应该忽略
			for _, ignoreDir := range s.config.IgnoreDirs {
				if strings.Contains(path, ignoreDir) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			// 只监控目录
			if info.IsDir() {
				if err := s.watcher.Add(path); err != nil {
					log.Debug().Err(err).Msgf("添加监控目录失败: %s", path)
				}
			}

			return nil
		}); err != nil {
			return fmt.Errorf("遍历目录失败 %s: %w", dir, err)
		}
	}
	return nil
}

func (s *DevServer) shouldWatch(path string) bool {
	// 检查扩展名
	ext := filepath.Ext(path)
	matched := false
	for _, watchExt := range s.config.WatchExts {
		if ext == watchExt || watchExt == ".*" {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}

	// 检查忽略目录
	for _, ignoreDir := range s.config.IgnoreDirs {
		if strings.Contains(path, ignoreDir) {
			return false
		}
	}

	return true
}

func (s *DevServer) scheduleRestart() {
	// 防抖：延迟重启
	time.Sleep(time.Duration(s.config.Delay) * time.Millisecond)

	select {
	case s.restartCh <- struct{}{}:
	default:
		// 如果通道已满，跳过
	}
}

func (s *DevServer) manageProcess() {
	for {
		select {
		case <-s.restartCh:
			s.restartProcess()

		case <-s.stopCh:
			return
		}
	}
}

func (s *DevServer) restartProcess() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 停止现有进程
	s.stopProcess()

	// 构建
	if s.config.BuildCmd != "" {
		s.addLog("info", fmt.Sprintf("构建中: %s", s.config.BuildCmd))
		buildCmd := exec.Command("sh", "-c", s.config.BuildCmd)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			s.addLog("error", fmt.Sprintf("构建失败: %v", err))
			s.status = "build_failed"
			return
		}
		s.addLog("info", "构建成功")
	}

	// 启动新进程
	if s.config.RunCmd != "" {
		runCmdStr := s.config.RunCmd
		if len(s.config.RunArgs) > 0 {
			runCmdStr += " " + strings.Join(s.config.RunArgs, " ")
		}
		s.addLog("info", fmt.Sprintf("启动: %s", runCmdStr))

		// 解析命令和参数
		parts := strings.Fields(runCmdStr)
		if len(parts) == 0 {
			s.addLog("error", "运行命令为空")
			return
		}

		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Stdout = s
		cmd.Stderr = s
		s.cmd = cmd

		if err := cmd.Start(); err != nil {
			s.addLog("error", fmt.Sprintf("启动失败: %v", err))
			s.status = "start_failed"
			return
		}

		s.status = "running"
		s.lastRestart = time.Now()

		// 监控进程退出
		go func() {
			err := cmd.Wait()
			s.mu.Lock()
			defer s.mu.Unlock()
			if err != nil {
				s.addLog("error", fmt.Sprintf("进程退出: %v", err))
			} else {
				s.addLog("info", "进程正常退出")
			}
			s.status = "stopped"
		}()
	}
}

func (s *DevServer) stopProcess() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.addLog("info", "停止进程")
		s.cmd.Process.Kill()
		s.cmd.Wait()
		s.cmd = nil
	}
	s.status = "stopped"
}

func (s *DevServer) Write(p []byte) (n int, err error) {
	s.addLog("output", string(p))
	return len(p), nil
}

func (s *DevServer) addLog(level, message string) {
	s.logsMu.Lock()
	defer s.logsMu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	}

	s.logs = append(s.logs, entry)

	// 限制日志数量
	if len(s.logs) > s.config.LogMaxLines {
		s.logs = s.logs[len(s.logs)-s.config.LogMaxLines:]
	}
}

func (m *DevManager) startWebServer() {
	mux := http.NewServeMux()

	// 静态文件（内嵌 HTML）
	mux.HandleFunc("/", m.handleIndex)
	mux.HandleFunc("/api/services", m.handleServices)
	mux.HandleFunc("/api/service", m.handleService) // 创建/更新/删除服务
	mux.HandleFunc("/api/config", m.handleConfig)
	mux.HandleFunc("/api/config/save", m.handleSaveConfig) // 保存配置到文件
	mux.HandleFunc("/api/logs", m.handleLogs)
	mux.HandleFunc("/api/status", m.handleStatus)
	mux.HandleFunc("/api/restart", m.handleRestart)
	mux.HandleFunc("/api/stop", m.handleStop)

	addr := fmt.Sprintf(":%d", m.webPort)
	log.Info().Msgf("Web 管理界面启动在 http://localhost%s", addr)

	m.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := m.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Web 服务器错误")
	}
}

func (m *DevManager) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

func (m *DevManager) handleServices(w http.ResponseWriter, r *http.Request) {
	m.serversMu.RLock()
	defer m.serversMu.RUnlock()

	services := make([]map[string]interface{}, 0, len(m.servers))
	for name, server := range m.servers {
		server.mu.RLock()
		status := server.status
		lastRestart := server.lastRestart
		server.mu.RUnlock()

		services = append(services, map[string]interface{}{
			"name":         name,
			"status":       status,
			"last_restart": lastRestart,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(services)
}

func (m *DevManager) getServer(name string) *DevServer {
	m.serversMu.RLock()
	defer m.serversMu.RUnlock()
	return m.servers[name]
}

func (m *DevManager) handleConfig(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "缺少 service 参数", http.StatusBadRequest)
		return
	}

	s := m.getServer(serviceName)
	if s == nil {
		http.Error(w, "服务不存在", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		s.mu.RLock()
		cfg := s.config
		s.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)

	case "POST", "PUT":
		var cfg ServiceConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		oldConfig := s.config
		s.config = &cfg
		s.mu.Unlock()

		// 如果监控目录发生变化，重新加载监控
		if s.watcher != nil {
			dirsChanged := false
			if len(oldConfig.WatchDirs) != len(cfg.WatchDirs) {
				dirsChanged = true
			} else {
				for i, dir := range oldConfig.WatchDirs {
					if i >= len(cfg.WatchDirs) || dir != cfg.WatchDirs[i] {
						dirsChanged = true
						break
					}
				}
			}

			if dirsChanged {
				// 重新添加监控目录
				go func() {
					if err := s.addWatchDirs(); err != nil {
						s.addLog("error", fmt.Sprintf("重新加载监控目录失败: %v", err))
					} else {
						s.addLog("info", "监控目录已重新加载")
					}
				}()
			}
		}

		s.addLog("info", "配置已更新")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *DevManager) handleLogs(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "缺少 service 参数", http.StatusBadRequest)
		return
	}

	s := m.getServer(serviceName)
	if s == nil {
		http.Error(w, "服务不存在", http.StatusNotFound)
		return
	}

	s.logsMu.RLock()
	logs := make([]LogEntry, len(s.logs))
	copy(logs, s.logs)
	s.logsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (m *DevManager) handleStatus(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "缺少 service 参数", http.StatusBadRequest)
		return
	}

	s := m.getServer(serviceName)
	if s == nil {
		http.Error(w, "服务不存在", http.StatusNotFound)
		return
	}

	s.mu.RLock()
	status := s.status
	lastRestart := s.lastRestart
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       status,
		"last_restart": lastRestart,
	})
}

func (m *DevManager) handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "缺少 service 参数", http.StatusBadRequest)
		return
	}

	s := m.getServer(serviceName)
	if s == nil {
		http.Error(w, "服务不存在", http.StatusNotFound)
		return
	}

	s.restartCh <- struct{}{}
	s.addLog("info", "手动重启请求")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *DevManager) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		http.Error(w, "缺少 service 参数", http.StatusBadRequest)
		return
	}

	s := m.getServer(serviceName)
	if s == nil {
		http.Error(w, "服务不存在", http.StatusNotFound)
		return
	}

	s.stopProcess()
	s.addLog("info", "手动停止请求")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleService 处理服务的创建、更新、删除
func (m *DevManager) handleService(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST": // 创建新服务
		var cfg ServiceConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if cfg.Name == "" {
			http.Error(w, "服务名称不能为空", http.StatusBadRequest)
			return
		}

		m.configMu.Lock()
		// 检查服务是否已存在
		for i, svc := range m.config.Services {
			if svc.Name == cfg.Name {
				// 更新现有服务
				m.config.Services[i] = cfg
				m.configMu.Unlock()

				// 更新或创建服务器实例
				m.updateOrCreateServer(&cfg)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "服务已更新"})
				return
			}
		}

		// 添加新服务
		m.config.Services = append(m.config.Services, cfg)
		m.configMu.Unlock()

		// 创建服务器实例
		m.updateOrCreateServer(&cfg)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "服务已创建"})

	case "DELETE": // 删除服务
		serviceName := r.URL.Query().Get("name")
		if serviceName == "" {
			http.Error(w, "缺少 name 参数", http.StatusBadRequest)
			return
		}

		m.configMu.Lock()
		found := false
		for i, svc := range m.config.Services {
			if svc.Name == serviceName {
				m.config.Services = append(m.config.Services[:i], m.config.Services[i+1:]...)
				found = true
				break
			}
		}
		m.configMu.Unlock()

		if !found {
			http.Error(w, "服务不存在", http.StatusNotFound)
			return
		}

		// 停止并删除服务器实例
		m.serversMu.Lock()
		if server, exists := m.servers[serviceName]; exists {
			server.Stop()
			delete(m.servers, serviceName)
		}
		m.serversMu.Unlock()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "服务已删除"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// updateOrCreateServer 更新或创建服务器实例
func (m *DevManager) updateOrCreateServer(cfg *ServiceConfig) {
	m.serversMu.Lock()
	defer m.serversMu.Unlock()

	if server, exists := m.servers[cfg.Name]; exists {
		// 更新现有服务器配置
		server.mu.Lock()
		server.config = cfg
		server.mu.Unlock()

		// 如果服务被禁用，停止它
		if !cfg.Enabled {
			server.Stop()
		}
	} else if cfg.Enabled {
		// 创建新服务器实例
		server := NewDevServer(cfg.Name, cfg)
		m.servers[cfg.Name] = server
		// 启动服务
		if cfg.Enabled {
			go func() {
				ctx := context.Background()
				if err := server.Start(ctx); err != nil {
					log.Error().Err(err).Msgf("服务 %s 启动失败", cfg.Name)
				}
			}()
		}
	}
}

// handleSaveConfig 保存配置到文件
func (m *DevManager) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	m.configMu.RLock()
	cfg := *m.config
	m.configMu.RUnlock()

	// 将配置保存到文件
	data, err := yaml.Marshal(cfg)
	if err != nil {
		http.Error(w, fmt.Sprintf("序列化配置失败: %v", err), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		http.Error(w, fmt.Sprintf("保存配置文件失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "配置已保存到文件"})
}
