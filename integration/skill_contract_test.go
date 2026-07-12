package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSopInitSkillDelegatesMechanicalWorkToVersionedSopctl(t *testing.T) {
	repoRoot := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(repoRoot, "skills", "sop-init", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	contents := string(data)
	for _, required := range []string{
		"../../rules/STANDARD.md",
		"../../rules/schemas/profile.schema.json",
		".sop/profile.json",
		"sopctl diff --project-root <repo> --profile <temporary-profile.json>",
		"预览阶段不得先覆盖",
		"sopctl render --project-root <repo> --profile <temporary-profile.json>",
		"本次实际 diff",
		"同一事务",
		"sopctl check",
		"找不到兼容的 `sopctl`",
	} {
		if !strings.Contains(contents, required) {
			t.Errorf("sop-init is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"/Users/sunchongsheng",
		"SOP_HOME",
		"copy `master/`",
		"master/SLOTS.md",
	} {
		if strings.Contains(contents, forbidden) {
			t.Errorf("sop-init still contains checkout/manual-generation coupling %q", forbidden)
		}
	}
}

func TestSopAuditSkillStartsWithMechanicalCheckWithoutCheckoutCoupling(t *testing.T) {
	repoRoot := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(repoRoot, "skills", "sop-audit", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	contents := string(data)
	for _, required := range []string{
		"../../rules/STANDARD.md",
		"sopctl check",
		"默认只读",
		"主线覆盖闸",
		"机械错误",
		"P1：",
		"P0",
		"高频入口要对账",
		"STANDARD / master 权威锚点 → 项目实际落点",
		"近似不等于覆盖",
		`"severity"`,
	} {
		if !strings.Contains(contents, required) {
			t.Errorf("sop-audit is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"/Users/sunchongsheng",
		"SOP_HOME",
		"$SOP_HOME/master/",
		"slot-masked",
	} {
		if strings.Contains(contents, forbidden) {
			t.Errorf("sop-audit still contains checkout/manual-template coupling %q", forbidden)
		}
	}
}

func TestSopRunSkillKeepsTheRuntimeLoopClosed(t *testing.T) {
	repoRoot := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(repoRoot, "skills", "sop-run", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	contents := string(data)
	for _, required := range []string{
		"sopctl task start", "sopctl task continue", "sopctl task status", "任务胶囊",
		"不得手工创建 worktree", "测试 → 独立 review → 修复", "done", "waiting", "running",
		"不得阅读整份运行时设计来决定下一步", "远端副作用",
	} {
		if !strings.Contains(contents, required) {
			t.Errorf("sop-run is missing %q", required)
		}
	}
	if len(data) > 4096 {
		t.Errorf("sop-run is %d bytes, want at most 4096", len(data))
	}
}
