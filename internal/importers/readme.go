package importers

import (
	"regexp"
	"strings"
)

var headingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// ParseReadme extracts structured sections from a README markdown file.
func ParseReadme(content string) []ReadmeSection {
	lines := strings.Split(content, "\n")
	var sections []ReadmeSection
	var current *ReadmeSection

	for _, line := range lines {
		if m := headingRegex.FindStringSubmatch(line); m != nil {
			// Save previous section.
			if current != nil {
				current.Content = strings.TrimSpace(current.Content)
				sections = append(sections, *current)
			}
			current = &ReadmeSection{
				Heading: m[2],
				Level:   len(m[1]),
			}
		} else if current != nil {
			current.Content += line + "\n"
		}
	}

	// Save last section.
	if current != nil {
		current.Content = strings.TrimSpace(current.Content)
		sections = append(sections, *current)
	}

	return sections
}

// ExtractReadmeDescription extracts the project description from a README.
// It looks for common patterns like the first paragraph after the title,
// or a "Description" / "About" section.
func ExtractReadmeDescription(content string) string {
	sections := ParseReadme(content)
	if len(sections) == 0 {
		// No headings found; return first paragraph.
		paragraphs := strings.SplitN(strings.TrimSpace(content), "\n\n", 2)
		if len(paragraphs) > 0 {
			return strings.TrimSpace(paragraphs[0])
		}
		return ""
	}

	// Look for a description/about section first.
	for _, s := range sections {
		lower := strings.ToLower(s.Heading)
		if lower == "description" || lower == "about" || lower == "overview" {
			return s.Content
		}
	}

	// Fall back to first section content.
	if sections[0].Content != "" {
		return sections[0].Content
	}

	// Try second section if first is empty (title-only heading).
	if len(sections) > 1 {
		return sections[1].Content
	}

	return ""
}
