package skills

// Manager 聚合本地与远程两类管理能力。
// 上层只依赖 Manager，可在不同 runtime 中替换具体实现。
type Manager interface {
	Local() LocalManager
	Remote() RemoteManager
}

// DefaultManager 是默认管理器：本地能力使用当前模块实现，远程能力默认占位实现。
type DefaultManager struct {
	local  LocalManager
	remote RemoteManager
}

// NewManager 构建默认聚合管理器。
func NewManager() Manager {
	return DefaultManager{
		local:  NewLocalManager(),
		remote: NewUnsupportedRemoteManager(),
	}
}

func (m DefaultManager) Local() LocalManager {
	return m.local
}

func (m DefaultManager) Remote() RemoteManager {
	return m.remote
}
