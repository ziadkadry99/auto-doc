package importers

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	adrDirs    = []string{"docs/adr", "docs/decisions", "adr", "doc/adr", "docs/architecture/decisions"}
	adrFileRe  = regexp.MustCompile(`^(\d+)[-_](.+)\.md$`)
	adrStatusRe = regexp.MustCompile(`(?i)^##?\s*status\s*$`)
)

// DetectADRDirectory finds the ADR directory in a project root.
func DetectADRDirectory(projectRoot string) string {
	for _, dir := range adrDirs {
		path := filepath.Join(projectRoot, dir)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

// ParseADRFile parses a single ADR markdown file.
func ParseADRFile(content, filePath string) *ADR {
	adr := &ADR{FilePath: filePath}

	// Extract number and title from filename.
	base := filepath.Base(filePath)
	if m := adrFileRe.FindStringSubmatch(base); m != nil {
		adr.Number, _ = strconv.Atoi(m[1])
		adr.Title = strings.ReplaceAll(m[2], "-", " ")
		adr.Title = strings.ReplaceAll(adr.Title, "_", " ")
	}

	// Parse sections.
	sections := parseADRSections(content)

	// Override title from heading if found.
	if t, ok := sections["title"]; ok && t != "" {
		adr.Title = t
	}

	adr.Status = normalizeStatus(sections["status"])
	adr.Context = sections["context"]
	adr.Decision = sections["decision"]
	adr.Consequences = sections["consequences"]
	adr.Date = sections["date"]

	return adr
}

// ParseADRDirectory reads all ADR files in a directory.
func ParseADRDirectory(dir string) ([]ADR, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var adrs []ADR
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		adr := ParseADRFile(string(content), filepath.Join(dir, entry.Name()))
		if adr.Title != "" {
			adrs = append(adrs, *adr)
		}
	}

	return adrs, nil
}

func parseADRSections(content string) map[string]string {
	sections := map[string]string{}
	lines := strings.Split(content, "\n")

	var currentSection string
	var currentContent strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this is a heading.
		if strings.HasPrefix(trimmed, "#") {
			// Save previous section.
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(currentContent.String())
			}

			heading := strings.TrimLeft(trimmed, "#")
			heading = strings.TrimSpace(heading)
			sectionKey := normalizeSection(heading)
			currentSection = sectionKey
			currentContent.Reset()
			// For title sections, store the heading text as the value.
			if sectionKey == "title" {
				currentContent.WriteString(heading)
				currentContent.WriteString("\n")
			}
			continue
		}

		if currentSection != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Save last section.
	if currentSection != "" {
		sections[currentSection] = strings.TrimSpace(currentContent.String())
	}

	return sections
}

func normalizeSection(heading string) string {
	lower := strings.ToLower(heading)
	switch {
	case strings.Contains(lower, "status"):
		return "status"
	case strings.Contains(lower, "context"):
		return "context"
	case strings.Contains(lower, "decision"):
		return "decision"
	case strings.Contains(lower, "consequence"):
		return "consequences"
	case strings.Contains(lower, "date"):
		return "date"
	default:
		return "title"
	}
}

func normalizeStatus(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(lower, "accepted"):
		return "accepted"
	case strings.Contains(lower, "proposed"):
		return "proposed"
	case strings.Contains(lower, "deprecated"):
		return "deprecated"
	case strings.Contains(lower, "superseded"):
		return "superseded"
	case strings.Contains(lower, "rejected"):
		return "rejected"
	default:
		if lower == "" {
			return "unknown"
		}
		return lower
	}
}
