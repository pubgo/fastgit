package skills

// LocalManager 定义本地 skills 管理能力边界。
// 目标：让上层（copilotcmd / runtime / mcp tool）只依赖接口而不是具体实现。
type LocalManager interface {
	// Discover 扫描 skills 根目录列表，返回可用技能条目与告警信息。
	//
	// 参数：
	//   - skillDirs: skills 根目录列表（例如 ./skills、./.copilot/skills）。
	//
	// 返回：
	//   - []Entry: 已成功解析的技能集合（已去重）。
	//   - []string: 非致命告警（如某个目录不存在、某个 SKILL.md 解析失败）。
	Discover(skillDirs []string) ([]Entry, []string)

	// FindByName 在 Discover 返回的条目中按技能名查找。
	// name 匹配不区分大小写；找不到时返回 error。
	FindByName(entries []Entry, name string) (Entry, error)

	// ReadSkill 读取指定 SKILL.md 原始文本内容。
	// path 应指向具体文件路径，路径为空或读取失败会返回 error。
	ReadSkill(path string) (string, error)

	// CreateSkill 根据输入参数创建技能目录与 SKILL.md 文件。
	// 默认会生成标准模板；若 Force=false 且文件已存在会返回 error。
	CreateSkill(in CreateInput) (Entry, error)

	// BuildTemplate 生成标准 SKILL.md 模板文本（包含 frontmatter 与基础章节）。
	BuildTemplate(name string) string

	// SanitizeName 规范化技能名：小写、空格转中划线，并校验字符合法性。
	// 返回空字符串表示名称非法。
	SanitizeName(name string) string

	// ParseFile 解析单个 SKILL.md 文件，返回结构化结果。
	// fallbackName 用于当前文件缺失可用名称时的兜底值（通常传目录名）。
	ParseFile(path string, fallbackName string) (ParsedSkill, error)

	// ParseContent 解析 SKILL.md 文本内容。
	// 支持：frontmatter 元数据、一级/二级/三级标题提取，以及名称来源判定。
	ParseContent(content, fallbackName string) (ParsedSkill, error)

	// FindSectionContent 按标题路径提取正文内容。
	//
	// 用法：
	//   - headings=[]{"二级标题"}            => 返回该 H2 的内容
	//   - headings=[]{"二级标题","三级标题"} => 返回该 H3 的内容
	FindSectionContent(sections []Section, headings ...string) (string, bool)

	// ExistingDirs 过滤并返回实际存在的目录列表。
	ExistingDirs(candidates []string) []string

	// DirExists 判断路径是否为存在的目录。
	DirExists(path string) bool

	// CompactStringSlice 清理字符串数组（trim + 去空项）。
	CompactStringSlice(in []string) []string
}

// Service 是 LocalManager 的兼容别名（保留旧名称，避免外部调用方破坏）。
// Deprecated: 请优先使用 LocalManager。
type Service = LocalManager

// DefaultLocalManager 是 LocalManager 的默认实现，直接复用当前包内函数。
type DefaultLocalManager struct{}

// NewLocalManager 返回默认 LocalManager 实现。
func NewLocalManager() LocalManager {
	return DefaultLocalManager{}
}

// NewService 返回默认 Service 实现（兼容旧接口名）。
// 上层可以用这个构造默认行为，也可以注入自定义实现用于 mock/替换 runtime。
func NewService() Service {
	return NewLocalManager()
}

func (DefaultLocalManager) Discover(skillDirs []string) ([]Entry, []string) {
	return Discover(skillDirs)
}

func (DefaultLocalManager) FindByName(entries []Entry, name string) (Entry, error) {
	return FindByName(entries, name)
}

func (DefaultLocalManager) ReadSkill(path string) (string, error) {
	return ReadSkill(path)
}

func (DefaultLocalManager) CreateSkill(in CreateInput) (Entry, error) {
	return CreateSkill(in)
}

func (DefaultLocalManager) BuildTemplate(name string) string {
	return BuildTemplate(name)
}

func (DefaultLocalManager) SanitizeName(name string) string {
	return SanitizeName(name)
}

func (DefaultLocalManager) ParseFile(path string, fallbackName string) (ParsedSkill, error) {
	return ParseFile(path, fallbackName)
}

func (DefaultLocalManager) ParseContent(content, fallbackName string) (ParsedSkill, error) {
	return ParseContent(content, fallbackName)
}

func (DefaultLocalManager) FindSectionContent(sections []Section, headings ...string) (string, bool) {
	return FindSectionContent(sections, headings...)
}

func (DefaultLocalManager) ExistingDirs(candidates []string) []string {
	return ExistingDirs(candidates)
}

func (DefaultLocalManager) DirExists(path string) bool {
	return DirExists(path)
}

func (DefaultLocalManager) CompactStringSlice(in []string) []string {
	return CompactStringSlice(in)
}
