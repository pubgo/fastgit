package acpclient

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	acp "github.com/coder/acp-go-sdk"
)

// fsHandler 实现 ACP client 侧 fs/read_text_file、fs/write_text_file,
// 所有路径限定在 root(repo cwd)内,越界返回错误。
type fsHandler struct{ root string }

func (f *fsHandler) read(params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	path, err := f.resolve(params.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}

	content := string(data)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if params.Line != nil && *params.Line > 0 {
			start = clamp(*params.Line-1, 0, len(lines))
		}
		end := len(lines)
		if params.Limit != nil && *params.Limit > 0 && start+*params.Limit < end {
			end = start + *params.Limit
		}
		if start > end {
			start = end
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return acp.ReadTextFileResponse{Content: content}, nil
}

func (f *fsHandler) write(params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	path, err := f.resolve(params.Path)
	if err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o644); err != nil {
			return acp.WriteTextFileResponse{}, err
		}
	}
	if err := os.WriteFile(path, []byte(params.Content), 0o644); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	return acp.WriteTextFileResponse{}, nil
}

func (f *fsHandler) resolve(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("fs: empty path")
	}
	root, err := filepath.Abs(f.root)
	if err != nil {
		return "", err
	}
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fs: path %q outside sandbox %q", raw, root)
	}
	return path, nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
