package workflow

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validCommandName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Command is a reusable prompt template loaded from .sky/commands/<name>.md.
type Command struct {
	Name         string
	Description  string
	ArgumentHint string
	Prompt       string // The markdown body (everything after frontmatter)
}

// LoadCommand loads a command from the repo's .sky/commands/ directory.
//
// Deprecated: prefer LoadCommandFromRoots for multi-tier support.
func LoadCommand(repoDir, name string) (*Command, error) {
	return LoadCommandFromRoots(Roots{Repo: filepath.Join(repoDir, ".sky")}, name)
}

// LoadCommandFromRoots loads a command file by name, searching each enabled tier
// in precedence order (Repo → Workspace → User). Returns an error if the command
// is not found in any tier.
func LoadCommandFromRoots(roots Roots, name string) (*Command, error) {
	if !validCommandName.MatchString(name) {
		return nil, fmt.Errorf("invalid command name %q: must match %s", name, validCommandName.String())
	}
	for _, t := range roots.ordered() {
		if t.path == "" {
			continue
		}
		base := filepath.Join(t.path, "commands")
		path := filepath.Join(base, name+".md")
		cleanBase := filepath.Clean(base) + string(filepath.Separator)
		if !strings.HasPrefix(filepath.Clean(path), cleanBase) {
			return nil, fmt.Errorf("command path escapes commands dir: %q", name)
		}
		cmd, err := parseCommandFile(path, name)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		return cmd, nil
	}
	return nil, fmt.Errorf("command %q not found", name)
}

func parseCommandFile(path, name string) (*Command, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	cmd := &Command{Name: name}
	content := string(data)

	// Parse YAML frontmatter (between --- delimiters)
	if strings.HasPrefix(content, "---") {
		scanner := bufio.NewScanner(strings.NewReader(content[3:]))
		var frontmatter []string
		foundEnd := false
		for scanner.Scan() {
			line := scanner.Text()
			if line == "---" {
				foundEnd = true
				break
			}
			frontmatter = append(frontmatter, line)
		}
		if foundEnd {
			// Parse simple key: value from frontmatter
			for _, line := range frontmatter {
				if k, v, ok := parseKV(line); ok {
					switch k {
					case "description":
						cmd.Description = v
					case "argument-hint":
						cmd.ArgumentHint = v
					}
				}
			}
			// Body is everything after the closing ---
			rest := content[3:] // skip first ---
			idx := strings.Index(rest, "---")
			if idx >= 0 {
				cmd.Prompt = strings.TrimSpace(rest[idx+3:])
			}
		} else {
			cmd.Prompt = strings.TrimSpace(content)
		}
	} else {
		cmd.Prompt = strings.TrimSpace(content)
	}

	return cmd, nil
}

func parseKV(line string) (key, value string, ok bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}
