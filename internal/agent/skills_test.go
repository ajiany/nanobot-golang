package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir, name, content string) {
	t.Helper()
	os.MkdirAll(dir, 0755)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSkillsEmpty(t *testing.T) {
	dir := t.TempDir()
	l := NewSkillsLoader(dir)
	if skills := l.LoadAll(); len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkillWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	writeSkill(t, skillsDir, "web-search.md", `---
name: web-search
description: Search the web for information
always: false
---

# Web Search Skill
actual content here
`)

	l := NewSkillsLoader(dir)
	skills := l.LoadAll()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	s := skills[0]
	if s.Meta.Name != "web-search" {
		t.Errorf("expected name %q, got %q", "web-search", s.Meta.Name)
	}
	if s.Meta.Description != "Search the web for information" {
		t.Errorf("unexpected description: %q", s.Meta.Description)
	}
	if s.Meta.Always {
		t.Error("expected always=false")
	}
	if !strings.Contains(s.Content, "actual content here") {
		t.Errorf("expected content body, got %q", s.Content)
	}
}

func TestGetAlwaysSkills(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	writeSkill(t, skillsDir, "always-skill.md", `---
name: always-skill
description: Always injected
always: true
---

Always skill content
`)
	writeSkill(t, skillsDir, "optional-skill.md", `---
name: optional-skill
description: Optional
always: false
---

Optional content
`)

	l := NewSkillsLoader(dir)
	out := l.GetAlwaysSkills()
	if !strings.Contains(out, "Always skill content") {
		t.Errorf("expected always skill content, got %q", out)
	}
	if strings.Contains(out, "Optional content") {
		t.Error("optional skill should not appear in always skills")
	}
}

func TestBuildSkillsSummary(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	writeSkill(t, skillsDir, "code-review.md", `---
name: code-review
description: Review code for issues
always: false
---

Code review content
`)

	l := NewSkillsLoader(dir)
	out := l.BuildSkillsSummary()
	if !strings.Contains(out, "<available_skills>") {
		t.Error("expected <available_skills> tag")
	}
	if !strings.Contains(out, `name="code-review"`) {
		t.Error("expected skill name attribute")
	}
	if !strings.Contains(out, "Review code for issues") {
		t.Error("expected skill description")
	}
}

func TestRequirementsCheck(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	writeSkill(t, skillsDir, "needs-fake.md", `---
name: needs-fake
description: Requires a nonexistent binary
always: false
requires:
  - __nonexistent_binary_xyz__
---

Content
`)

	l := NewSkillsLoader(dir)
	skills := l.LoadAll()
	if len(skills) != 0 {
		t.Errorf("expected skill to be skipped due to missing requirement, got %d skills", len(skills))
	}
}
