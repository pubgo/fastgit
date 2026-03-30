package copilotcmd

import (
	"context"
	"fmt"
	"strings"

	agentlinemodule "github.com/pubgo/fastgit/pkg/agentline"
	skillsmodule "github.com/pubgo/fastgit/pkg/skills"
	"github.com/pubgo/redant"
)

func newSkillsCmd(profileName, profileFile *string, cliSkillDirs *[]string) *redant.Command {
	var (
		skillName string
		skillDir  string
		force     bool
	)

	root := &redant.Command{
		Use:      "skills",
		Short:    "管理 skills（查看、创建、加载）",
		Metadata: agentlinemodule.AgentCommandMetadata(),
	}

	listCmd := &redant.Command{
		Use:      "list",
		Short:    "列出当前可发现的 skills",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			dirs, err := resolveSkillDirs(*profileName, *profileFile, *cliSkillDirs)
			if err != nil {
				return err
			}
			entries, warns := skillsmodule.Discover(dirs)
			if len(entries) == 0 {
				_, _ = fmt.Fprintln(inv.Stdout, "未发现任何 skills")
				for _, w := range warns {
					_, _ = fmt.Fprintf(inv.Stdout, "warn: %s\n", w)
				}
				return nil
			}
			for _, s := range entries {
				desc := strings.TrimSpace(s.Description)
				if desc != "" {
					_, _ = fmt.Fprintf(inv.Stdout, "- %s\t%s\t(source=%s)\t%s\n", s.Name, s.Path, strings.TrimSpace(s.Source), desc)
					continue
				}
				_, _ = fmt.Fprintf(inv.Stdout, "- %s\t%s\t(source=%s)\n", s.Name, s.Path, strings.TrimSpace(s.Source))
			}
			for _, w := range warns {
				_, _ = fmt.Fprintf(inv.Stdout, "warn: %s\n", w)
			}
			return nil
		},
	}

	showCmd := &redant.Command{
		Use:      "show",
		Short:    "查看指定 skill 内容",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "name", Description: "skill 名称（目录名）", Value: redant.StringOf(&skillName), Required: true},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			dirs, err := resolveSkillDirs(*profileName, *profileFile, *cliSkillDirs)
			if err != nil {
				return err
			}
			entries, _ := skillsmodule.Discover(dirs)
			target, err := skillsmodule.FindByName(entries, skillName)
			if err != nil {
				return err
			}
			content, err := skillsmodule.ReadSkill(target.Path)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(inv.Stdout, content)
			return nil
		},
	}

	createCmd := &redant.Command{
		Use:      "create",
		Short:    "创建新的 skill 模板",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Options: redant.OptionSet{
			{Flag: "name", Description: "skill 名称（目录名）", Value: redant.StringOf(&skillName), Required: true},
			{Flag: "dir", Description: "目标 skills 根目录", Value: redant.StringOf(&skillDir), Default: "./skills"},
			{Flag: "force", Description: "覆盖已存在的 SKILL.md", Value: redant.BoolOf(&force), Default: "false"},
		},
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			name := skillsmodule.SanitizeName(skillName)
			if name == "" {
				return fmt.Errorf("invalid --name")
			}
			entry, err := skillsmodule.CreateSkill(skillsmodule.CreateInput{
				Name:    name,
				BaseDir: strings.TrimSpace(skillDir),
				Force:   force,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(inv.Stdout, "skill created: %s\n", entry.Path)
			return nil
		},
	}

	loadCmd := &redant.Command{
		Use:      "load",
		Short:    "展示本次会话将加载的 skills 目录与结果",
		Metadata: agentlinemodule.AgentCommandMetadata(),
		Handler: func(ctx context.Context, inv *redant.Invocation) error {
			_ = ctx
			dirs, err := resolveSkillDirs(*profileName, *profileFile, *cliSkillDirs)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(inv.Stdout, "resolved skill directories:")
			for _, d := range dirs {
				_, _ = fmt.Fprintf(inv.Stdout, "- %s\n", d)
			}
			entries, warns := skillsmodule.Discover(dirs)
			_, _ = fmt.Fprintf(inv.Stdout, "discovered skills: %d\n", len(entries))
			for _, s := range entries {
				_, _ = fmt.Fprintf(inv.Stdout, "- %s\n", s.Name)
			}
			for _, w := range warns {
				_, _ = fmt.Fprintf(inv.Stdout, "warn: %s\n", w)
			}
			return nil
		},
	}

	root.Children = []*redant.Command{listCmd, showCmd, createCmd, loadCmd}
	return root
}

func resolveSkillDirs(profileName, profileFile string, cliSkillDirs []string) ([]string, error) {
	resolved, err := resolveCopilotOptions(resolveCopilotInput{
		ProfileName:           strings.TrimSpace(profileName),
		ProfileFile:           strings.TrimSpace(profileFile),
		SkillDirs:             cliSkillDirs,
		SystemMessageMode:     "append",
		EnableInfiniteSession: true,
	})
	if err != nil {
		return nil, err
	}

	dirs := skillsmodule.CompactStringSlice(resolved.Advanced.SkillDirectories)
	if len(dirs) > 0 {
		return dirs, nil
	}

	fallback := skillsmodule.ExistingDirs([]string{"./skills", "./.copilot/skills"})
	return skillsmodule.CompactStringSlice(fallback), nil
}
