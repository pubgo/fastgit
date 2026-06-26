package skills

import "context"

// RemoteManager 定义远程 skills 管理能力（远端仓库/服务的增删查改与同步）。
// 该文件仅承载“接口与数据结构设计”，不包含具体实现。
type RemoteManager interface {
	// List 列出命名空间下的远程技能摘要。
	List(ctx context.Context, namespace string) ([]RemoteEntry, error)
	// Get 获取远程技能完整内容。
	Get(ctx context.Context, namespace, name string) (RemoteSkill, error)
	// Upsert 创建或更新远程技能。
	Upsert(ctx context.Context, skill RemoteSkill) (RemoteEntry, error)
	// Delete 删除远程技能。
	Delete(ctx context.Context, namespace, name string) error
	// Pull 从远端拉取技能并落地到本地。
	Pull(ctx context.Context, req PullRequest) (Entry, error)
	// Push 将本地技能推送到远端。
	Push(ctx context.Context, req PushRequest) (RemoteEntry, error)
}

// RemoteEntry 表示远程技能条目的摘要信息。
type RemoteEntry struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// RemoteSkill 表示远程技能的完整实体。
type RemoteSkill struct {
	RemoteEntry
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PullRequest 描述从远端拉取并落地到本地目录的请求。
type PullRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	TargetDir string `json:"targetDir"`
	Force     bool   `json:"force"`
}

// PushRequest 描述将本地 skill 推送到远端的请求。
type PushRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Path      string `json:"path"`
}
