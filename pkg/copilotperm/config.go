package copilotperm

import (
	"strings"
	"sync"

	"github.com/pubgo/funk/v2/config"
	"github.com/pubgo/fastgit/configs"
)

type globalConfig struct {
	Copilot struct {
		PermissionMode string `yaml:"permission_mode"`
	} `yaml:"copilot"`
}

var loadGlobalMode = sync.OnceValue(func() string {
	cfgPath := configs.GetConfigPath()
	if strings.TrimSpace(cfgPath) == "" {
		return ""
	}
	result, err := config.LoadFromPath[globalConfig](cfgPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(result.T.Copilot.PermissionMode)
})
