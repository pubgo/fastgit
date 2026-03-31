package skills

import (
	"context"
	"fmt"
)

// UnsupportedRemoteManager 是默认远程管理占位实现。
// 当项目尚未接入远程存储（HTTP/Git/MCP）时，所有远程操作会返回清晰错误。
type UnsupportedRemoteManager struct{}

func NewUnsupportedRemoteManager() RemoteManager {
	return UnsupportedRemoteManager{}
}

func (UnsupportedRemoteManager) List(ctx context.Context, namespace string) ([]RemoteEntry, error) {
	_ = ctx
	_ = namespace
	return nil, errRemoteNotConfigured("List")
}

func (UnsupportedRemoteManager) Get(ctx context.Context, namespace, name string) (RemoteSkill, error) {
	_ = ctx
	_ = namespace
	_ = name
	return RemoteSkill{}, errRemoteNotConfigured("Get")
}

func (UnsupportedRemoteManager) Upsert(ctx context.Context, skill RemoteSkill) (RemoteEntry, error) {
	_ = ctx
	_ = skill
	return RemoteEntry{}, errRemoteNotConfigured("Upsert")
}

func (UnsupportedRemoteManager) Delete(ctx context.Context, namespace, name string) error {
	_ = ctx
	_ = namespace
	_ = name
	return errRemoteNotConfigured("Delete")
}

func (UnsupportedRemoteManager) Pull(ctx context.Context, req PullRequest) (Entry, error) {
	_ = ctx
	_ = req
	return Entry{}, errRemoteNotConfigured("Pull")
}

func (UnsupportedRemoteManager) Push(ctx context.Context, req PushRequest) (RemoteEntry, error) {
	_ = ctx
	_ = req
	return RemoteEntry{}, errRemoteNotConfigured("Push")
}

func errRemoteNotConfigured(method string) error {
	return fmt.Errorf("skills remote manager not configured: %s", method)
}
