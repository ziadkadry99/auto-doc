package docs

import (
	"fmt"
	"regexp"
	"strings"
)

// sanitizeMermaid fixes common syntax issues in LLM-generated Mermaid diagrams.
// Used by the central site system diagram which still renders via Mermaid.
func sanitizeMermaid(diagram string) string {
	var out []string
	hasHeader := false
	subgraphDepth := 0
	for _, rawLine := range strings.Split(diagram, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "graph ") || strings.HasPrefix(line, "flowchart "):
			if !hasHeader {
				out = append(out, line)
				hasHeader = true
			}
		case strings.HasPrefix(line, "%%"):
			out = append(out, line)
		case strings.HasPrefix(line, "subgraph "):
			out = append(out, line)
			subgraphDepth++
		case line == "end" || line == "en":
			if subgraphDepth > 0 {
				out = append(out, "end")
				subgraphDepth--
			}
		case strings.HasPrefix(line, "classDef ") || strings.HasPrefix(line, "class ") || strings.HasPrefix(line, "style "):
			out = append(out, line)
		default:
			if fixed := sanitizeMermaidLine(rawLine); fixed != "" {
				out = append(out, fixed)
			}
		}
	}
	for subgraphDepth > 0 {
		out = append(out, "end")
		subgraphDepth--
	}
	if !hasHeader {
		out = append([]string{"graph TD"}, out...)
	}
	return strings.Join(out, "\n")
}

// mermaidNodeDef matches a node definition: ID["label"] or ID[label]
var mermaidNodeDef = regexp.MustCompile(`^(\s*)(\S+?)(\[.*)$`)

// mermaidArrow matches an arrow line: ID --> ID or ID -->|label| ID
var mermaidArrow = regexp.MustCompile(`^(\s*)(\S+?)(\s*-->.*)$`)

// mermaidArrowTarget matches the target node ID in an arrow (after --> or -->|...|)
var mermaidArrowTarget = regexp.MustCompile(`(-->(?:\|[^|]*\|)?\s*)(\S+)(.*)$`)

// sanitizeMermaidID replaces characters that are invalid in Mermaid node IDs.
func sanitizeMermaidID(id string) string {
	replacer := strings.NewReplacer(
		"&", "_",
		"#", "_",
		"@", "_",
		"!", "_",
		"?", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"{", "_",
		"}", "_",
		"<", "_",
		">", "_",
		";", "_",
		",", "_",
		"'", "_",
		"\"", "_",
	)
	return replacer.Replace(id)
}

func sanitizeMermaidLine(line string) string {
	trimmed := strings.TrimSpace(line)

	if trimmed == "" || strings.HasPrefix(trimmed, "graph ") ||
		strings.HasPrefix(trimmed, "%%") || trimmed == "end" ||
		strings.HasPrefix(trimmed, "subgraph ") ||
		strings.HasPrefix(trimmed, "flowchart ") ||
		strings.HasPrefix(trimmed, "classDef ") ||
		strings.HasPrefix(trimmed, "class ") ||
		strings.HasPrefix(trimmed, "style ") {
		return line
	}

	if m := mermaidArrow.FindStringSubmatch(line); m != nil {
		indent, rawSource, rest := m[1], m[2], m[3]
		sourceID, sourceLabel, sourceClass := parseNodeRef(rawSource)
		tm := mermaidArrowTarget.FindStringSubmatch(rest)
		if tm == nil {
			return ""
		}
		arrow := strings.TrimSpace(tm[1])
		rawTarget := strings.TrimSpace(tm[2] + tm[3])
		targetID, targetLabel, targetClass := parseNodeRef(rawTarget)

		var buf strings.Builder
		buf.WriteString(indent)
		buf.WriteString(sanitizeMermaidID(sourceID))
		if sourceLabel != "" {
			fmt.Fprintf(&buf, `["%s"]`, escapeMermaidLabel(sourceLabel))
		}
		buf.WriteString(sourceClass)
		fmt.Fprintf(&buf, " %s ", arrow)
		buf.WriteString(sanitizeMermaidID(targetID))
		if targetLabel != "" {
			fmt.Fprintf(&buf, `["%s"]`, escapeMermaidLabel(targetLabel))
		}
		buf.WriteString(targetClass)
		return buf.String()
	}

	if m := mermaidNodeDef.FindStringSubmatch(line); m != nil {
		indent, id, rest := m[1], m[2], m[3]
		normalized, ok := normalizeNodeRest(rest)
		if !ok {
			return ""
		}
		return indent + sanitizeMermaidID(id) + normalized
	}

	return ""
}

func parseNodeRef(s string) (id, label, class string) {
	s = strings.TrimSpace(s)
	bracketDepth := 0
	classIdx := -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
		}
		if bracketDepth == 0 && i+3 <= len(s) && s[i:i+3] == ":::" {
			classIdx = i
			break
		}
	}
	if classIdx >= 0 {
		class = s[classIdx:]
		if spIdx := strings.IndexByte(class, ' '); spIdx >= 0 {
			class = class[:spIdx]
		}
		s = s[:classIdx]
	}
	bracketIdx := strings.Index(s, "[")
	if bracketIdx < 0 {
		return s, "", class
	}
	id = s[:bracketIdx]
	rest := s[bracketIdx:]
	depth := 0
	end := -1
	for i, r := range rest {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return id, "", class
	}
	label = rest[1:end]
	if len(label) >= 2 && strings.HasPrefix(label, "\"") && strings.HasSuffix(label, "\"") {
		label = label[1 : len(label)-1]
	}
	return id, label, class
}

func normalizeNodeRest(rest string) (string, bool) {
	start := strings.Index(rest, "[")
	if start < 0 {
		return "", false
	}
	depth := 0
	end := -1
	for i, r := range rest[start:] {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = start + i
				break
			}
		}
	}
	if end < 0 {
		return "", false
	}
	label := strings.TrimSpace(rest[start+1 : end])
	if len(label) >= 2 && strings.HasPrefix(label, "\"") && strings.HasSuffix(label, "\"") {
		label = strings.TrimPrefix(strings.TrimSuffix(label, "\""), "\"")
	}
	suffix := strings.TrimSpace(rest[end+1:])
	if strings.HasPrefix(suffix, ":::") {
		parts := strings.Fields(suffix)
		if len(parts) > 0 {
			suffix = parts[0]
		} else {
			suffix = ""
		}
	} else {
		suffix = ""
	}
	return fmt.Sprintf("[\"%s\"]%s", escapeMermaidLabel(label), suffix), true
}

func escapeMermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "#quot;")
	s = strings.ReplaceAll(s, "(", "#lpar;")
	s = strings.ReplaceAll(s, ")", "#rpar;")
	s = strings.ReplaceAll(s, "[", "#lsqb;")
	s = strings.ReplaceAll(s, "]", "#rsqb;")
	s = strings.ReplaceAll(s, "{", "#lbrace;")
	s = strings.ReplaceAll(s, "}", "#rbrace;")
	s = strings.ReplaceAll(s, "<", "#lt;")
	s = strings.ReplaceAll(s, ">", "#gt;")
	return s
}
