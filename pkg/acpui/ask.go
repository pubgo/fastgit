// Package acpui 提供 ACP 权限请求的终端交互实现(tap 确认)。
package acpui

import (
	"context"
	"fmt"
	"strings"

	acp "github.com/coder/acp-go-sdk"
	"github.com/yarlson/tap"

	"github.com/pubgo/fastgit/pkg/acpclient"
)

// Permission 按模式名构造权限策略: ask(默认,交互)|auto(自动放行)|deny(自动拒绝)。
func Permission(mode string) (acpclient.PermissionHandler, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "ask":
		return PermissionAsk(), nil
	case "auto":
		return acpclient.PermissionAllowOnce, nil
	case "deny":
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid permission mode %q, want ask|auto|deny", mode)
	}
}

// PermissionAsk 返回一个交互权限策略:每次 agent 请求权限时列出候选项,
// 由用户选择;选择"拒绝"时取消请求。
func PermissionAsk() acpclient.PermissionHandler {
	return acpclient.PermissionInteractive(func(title string, options []acp.PermissionOption) string {
		const denyValue = "__deny__"
		items := make([]tap.SelectOption[string], 0, len(options)+1)
		for _, opt := range options {
			label := opt.Name
			if label == "" {
				label = string(opt.OptionId)
			}
			items = append(items, tap.SelectOption[string]{
				Label: label + " (" + string(opt.Kind) + ")",
				Value: string(opt.OptionId),
			})
		}
		items = append(items, tap.SelectOption[string]{Label: "拒绝该请求", Value: denyValue})

		selected := tap.Select[string](context.Background(), tap.SelectOptions[string]{
			Message: "acp agent 请求权限: " + title,
			Options: items,
		})
		if selected == denyValue {
			return ""
		}
		return selected
	})
}
