package docscmd

import (
	"os"
	"strings"
	"testing"
)

func TestEnsureDocumentationScaffoldCreatesTemplates(t *testing.T) {
	repo := t.TempDir()

	result, err := ensureDocumentationScaffold(repo, scaffoldOptions{})
	if err != nil {
		t.Fatalf("ensureDocumentationScaffold() error = %v", err)
	}
	if len(result.Created) == 0 {
		t.Fatalf("expected documentation scaffold to create files")
	}

	paths := buildPaths(repo)
	assertFileContains(t, paths.DocumentationPromptFile, "name: documentation")
	assertFileContains(t, paths.DocumentationPromptFile, "你是当前仓库的文档维护助手")
	assertFileContains(t, paths.DocumentationRulesFile, "name: 文档专项规范")
	assertFileContains(t, paths.DocumentationRulesFile, "适用于仓库文档写作与维护")
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("expected %s to contain %q, got:\n%s", path, want, string(content))
	}
}
