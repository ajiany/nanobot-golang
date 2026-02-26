package agent

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SkillMeta holds parsed frontmatter from a SKILL.md file.
type SkillMeta struct {
	Name        string
	Description string
	Always      bool
	Requires    []string
}

// SkillsLoader scans workspace and builtin skills directories.
type SkillsLoader struct {
	workspaceSkillsDir string
}

func NewSkillsLoader(workspace string) *SkillsLoader {
	return &SkillsLoader{workspaceSkillsDir: filepath.Join(workspace, "skills")}
}

// LoadedSkill holds a parsed skill.
type LoadedSkill struct {
	Meta    SkillMeta
	Content string
	Path    string
}

// LoadAll scans the skills directory and returns all valid skills.
func (l *SkillsLoader) LoadAll() []LoadedSkill {
	entries, err := os.ReadDir(l.workspaceSkillsDir)
	if err != nil {
		return nil
	}

	var skills []LoadedSkill
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(l.workspaceSkillsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		meta, content, ok := parseFrontmatter(string(data))
		if !ok {
			continue
		}
		if !checkRequirements(meta.Requires) {
			log.Printf("skill %q skipped: missing requirements %v", meta.Name, meta.Requires)
			continue
		}
		skills = append(skills, LoadedSkill{Meta: meta, Content: content, Path: path})
	}
	return skills
}

// GetAlwaysSkills returns full content of skills with always=true.
func (l *SkillsLoader) GetAlwaysSkills() string {
	var parts []string
	for _, s := range l.LoadAll() {
		if s.Meta.Always {
			parts = append(parts, s.Content)
		}
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// BuildSkillsSummary returns XML summary of non-always skills.
func (l *SkillsLoader) BuildSkillsSummary() string {
	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, s := range l.LoadAll() {
		if !s.Meta.Always {
			sb.WriteString(fmt.Sprintf("<skill name=%q>%s</skill>\n", s.Meta.Name, s.Meta.Description))
		}
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

// parseFrontmatter splits YAML frontmatter from content.
// Returns (meta, content, ok).
func parseFrontmatter(raw string) (SkillMeta, string, bool) {
	// Must start with ---
	if !strings.HasPrefix(raw, "---") {
		return SkillMeta{}, "", false
	}
	// Find closing ---
	rest := raw[3:]
	// skip optional newline after opening ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	idx := strings.Index(rest, "---")
	if idx < 0 {
		return SkillMeta{}, "", false
	}
	frontmatter := rest[:idx]
	content := strings.TrimPrefix(rest[idx+3:], "\n")

	meta := parseMeta(frontmatter)
	return meta, content, true
}

func parseMeta(fm string) SkillMeta {
	var meta SkillMeta
	lines := strings.Split(fm, "\n")
	inRequires := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// List item under requires:
		if inRequires {
			if strings.HasPrefix(trimmed, "-") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				if val != "" {
					meta.Requires = append(meta.Requires, val)
				}
				continue
			}
			// Not a list item — fall through to normal key: value parsing
			inRequires = false
		}

		kv := strings.SplitN(trimmed, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch key {
		case "name":
			meta.Name = val
		case "description":
			meta.Description = val
		case "always":
			meta.Always = val == "true"
		case "requires":
			if val == "" {
				inRequires = true
			} else {
				// comma-separated inline
				for _, r := range strings.Split(val, ",") {
					r = strings.TrimSpace(r)
					if r != "" {
						meta.Requires = append(meta.Requires, r)
					}
				}
			}
		}
	}
	return meta
}

// checkRequirements returns true if all required commands are available.
func checkRequirements(requires []string) bool {
	for _, cmd := range requires {
		if _, err := exec.LookPath(cmd); err != nil {
			return false
		}
	}
	return true
}
