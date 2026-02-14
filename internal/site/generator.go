package site

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// SiteGenerator converts markdown documentation into a static HTML site.
type SiteGenerator struct {
	DocsDir     string
	OutputDir   string
	ProjectName string
}

// NewSiteGenerator creates a SiteGenerator with the given directories.
func NewSiteGenerator(docsDir, outputDir, projectName string) *SiteGenerator {
	return &SiteGenerator{
		DocsDir:     docsDir,
		OutputDir:   outputDir,
		ProjectName: projectName,
	}
}

// pageData holds the data passed to the HTML template for each page.
type pageData struct {
	Title       string
	ProjectName string
	Content     template.HTML
	TreeHTML    template.HTML
	BasePath    string
}

// Generate builds the full static site from markdown files. Returns the number of pages generated.
func (g *SiteGenerator) Generate() (int, error) {
	// Collect all markdown file paths.
	var mdPaths []string
	err := filepath.Walk(g.DocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			rel, err := filepath.Rel(g.DocsDir, path)
			if err != nil {
				return err
			}
			mdPaths = append(mdPaths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walking docs dir: %w", err)
	}

	if len(mdPaths) == 0 {
		return 0, fmt.Errorf("no markdown files found in %s", g.DocsDir)
	}

	// Build title map from H1 headings for sidebar display names.
	titleMap := make(map[string]string)
	for _, relPath := range mdPaths {
		srcPath := filepath.Join(g.DocsDir, filepath.FromSlash(relPath))
		content, err := os.ReadFile(srcPath)
		if err == nil {
			title := extractTitle(string(content), relPath)
			titleMap[relPath] = title
		}
	}

	// Build file tree for sidebar navigation.
	tree := BuildTree(mdPaths, titleMap)

	// Build and write search index.
	searchEntries, err := BuildSearchIndex(g.DocsDir)
	if err != nil {
		return 0, fmt.Errorf("building search index: %w", err)
	}

	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return 0, err
	}

	if err := WriteSearchIndex(searchEntries, filepath.Join(g.OutputDir, "search-index.json")); err != nil {
		return 0, fmt.Errorf("writing search index: %w", err)
	}

	// Write static assets.
	if err := os.WriteFile(filepath.Join(g.OutputDir, "style.css"), []byte(cssContent), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(g.OutputDir, "script.js"), []byte(jsContent), 0o644); err != nil {
		return 0, err
	}

	// Initialize goldmark with extensions.
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	// Parse page template.
	tmpl, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return 0, fmt.Errorf("parsing page template: %w", err)
	}

	// Render each markdown file to HTML.
	for _, relPath := range mdPaths {
		if err := g.renderPage(md, tmpl, tree, relPath); err != nil {
			return 0, fmt.Errorf("rendering %s: %w", relPath, err)
		}
	}

	// Copy any standalone HTML files (e.g., interactive map) directly to output.
	_ = filepath.Walk(g.DocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		rel, err := filepath.Rel(g.DocsDir, path)
		if err != nil {
			return nil
		}
		outPath := filepath.Join(g.OutputDir, filepath.ToSlash(rel))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		_ = os.WriteFile(outPath, data, 0o644)
		return nil
	})

	return len(mdPaths), nil
}

// renderPage converts a single markdown file to an HTML page.
func (g *SiteGenerator) renderPage(md goldmark.Markdown, tmpl *template.Template, tree *FileTree, relPath string) error {
	srcPath := filepath.Join(g.DocsDir, filepath.FromSlash(relPath))
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// Convert markdown to HTML.
	var htmlBuf bytes.Buffer
	if err := md.Convert(content, &htmlBuf); err != nil {
		return fmt.Errorf("converting markdown: %w", err)
	}

	// Post-process the rendered HTML.
	htmlContent := htmlBuf.String()
	htmlContent = postProcessMermaid(htmlContent)
	htmlContent = rewriteMDLinks(htmlContent)
	htmlContent = reformatMetadataSummary(htmlContent)
	htmlContent = removeEmptySections(htmlContent)
	htmlContent = removeBrokenLinks(htmlContent, g.DocsDir)
	htmlContent = cleanTableSummaries(htmlContent)

	// Determine output path.
	htmlRelPath := mdPathToHTML(relPath)
	outPath := filepath.Join(g.OutputDir, filepath.FromSlash(htmlRelPath))

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	// Compute base path for CSS/JS references.
	depth := strings.Count(htmlRelPath, "/")
	basePath := ""
	for i := 0; i < depth; i++ {
		basePath += "../"
	}

	// Extract title from first heading or use filename.
	title := extractTitle(string(content), relPath)

	// Build tree HTML with active path highlighting.
	treeHTML := tree.ToHTML(relPath, basePath)

	data := pageData{
		Title:       title,
		ProjectName: g.ProjectName,
		Content:     template.HTML(htmlContent),
		TreeHTML:    template.HTML(treeHTML),
		BasePath:    basePath,
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// extractTitle pulls the first # heading from markdown content, or falls back to the filename.
func extractTitle(content, relPath string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return strings.TrimSuffix(filepath.Base(relPath), ".md")
}

// postProcessMermaid converts <pre><code class="language-mermaid">...</code></pre>
// blocks into <div class="mermaid">...</div> for Mermaid.js rendering.
func postProcessMermaid(html string) string {
	// Look for code blocks with language-mermaid class.
	const openTag = `<pre><code class="language-mermaid">`
	const closeTag = `</code></pre>`

	for {
		idx := strings.Index(html, openTag)
		if idx == -1 {
			break
		}
		endIdx := strings.Index(html[idx:], closeTag)
		if endIdx == -1 {
			break
		}
		endIdx += idx

		mermaidContent := html[idx+len(openTag) : endIdx]
		replacement := `<div class="mermaid">` + mermaidContent + `</div>`
		html = html[:idx] + replacement + html[endIdx+len(closeTag):]
	}

	return html
}

// rewriteMDLinks changes .md links in HTML content to .html links.
func rewriteMDLinks(content string) string {
	// Rewrite href="...*.md" to href="...*.html"
	result := content
	search := `.md"`
	replace := `.html"`
	result = strings.ReplaceAll(result, search, replace)

	search = `.md#`
	replace = `.html#`
	result = strings.ReplaceAll(result, search, replace)

	return result
}

// reformatMetadataSummary transforms the verbose "File: ... Language: ... Summary: ... Purpose: ... Dependencies: ..."
// blocks that the lite-tier LLM produces into nicely formatted HTML with metadata badges and clean sections.
func reformatMetadataSummary(htmlContent string) string {
	// The lite tier produces paragraphs like:
	// <p>File: path Language: Go Summary: text Purpose: text Dependencies: list</p>
	// We need to detect these and reformat them into structured HTML.

	const pOpen = "<p>"
	const pClose = "</p>"

	var result strings.Builder
	remaining := htmlContent

	for {
		idx := strings.Index(remaining, pOpen)
		if idx == -1 {
			result.WriteString(remaining)
			break
		}
		pCloseIdx := strings.Index(remaining[idx:], pClose)
		if pCloseIdx == -1 {
			result.WriteString(remaining)
			break
		}
		pCloseIdx += idx

		content := remaining[idx+len(pOpen) : pCloseIdx]

		// Check if this paragraph has the verbose metadata pattern
		if strings.Contains(content, "File:") && strings.Contains(content, "Summary:") && strings.Contains(content, "Language:") {
			formatted := formatMetadataBlock(content)
			result.WriteString(remaining[:idx])
			result.WriteString(formatted)
			remaining = remaining[pCloseIdx+len(pClose):]
		} else {
			result.WriteString(remaining[:pCloseIdx+len(pClose)])
			remaining = remaining[pCloseIdx+len(pClose):]
		}
	}

	return result.String()
}

// formatMetadataBlock transforms a verbose metadata string into structured HTML.
func formatMetadataBlock(text string) string {
	fields := parseMetadataFields(text)

	var b strings.Builder

	// Language badge
	if lang := fields["Language"]; lang != "" {
		b.WriteString(fmt.Sprintf(`<div class="file-meta"><span class="meta-badge lang-badge">%s</span></div>`+"\n", lang))
	}

	// Summary as main description
	if summary := fields["Summary"]; summary != "" {
		b.WriteString(fmt.Sprintf(`<p class="file-summary">%s</p>`+"\n", summary))
	}

	// Purpose as secondary info
	if purpose := fields["Purpose"]; purpose != "" {
		b.WriteString(fmt.Sprintf(`<div class="file-purpose"><strong>Purpose:</strong> %s</div>`+"\n", purpose))
	}

	// Dependencies as tag list
	if deps := fields["Dependencies"]; deps != "" {
		depList := strings.Split(deps, ",")
		b.WriteString(`<div class="file-deps"><strong>Dependencies</strong><div class="dep-tags">`)
		for _, dep := range depList {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			// Clean up "(import)" / "(api_call)" suffixes for display
			name := dep
			depType := ""
			if parenIdx := strings.Index(dep, " ("); parenIdx > 0 {
				name = dep[:parenIdx]
				depType = strings.Trim(dep[parenIdx+1:], " ()")
			}
			typeClass := "dep-import"
			if depType == "api_call" {
				typeClass = "dep-api"
			}
			b.WriteString(fmt.Sprintf(`<span class="dep-tag %s">%s</span>`, typeClass, name))
		}
		b.WriteString(`</div></div>` + "\n")
	}

	return b.String()
}

// parseMetadataFields extracts key-value pairs from the verbose metadata string.
func parseMetadataFields(text string) map[string]string {
	fields := make(map[string]string)
	keys := []string{"File", "Language", "Summary", "Purpose", "Dependencies"}

	for i, key := range keys {
		marker := key + ":"
		idx := strings.Index(text, marker)
		if idx == -1 {
			continue
		}
		after := text[idx+len(marker):]

		// Find where this field ends (at the next field marker or end of text)
		end := len(after)
		for _, nextKey := range keys[i+1:] {
			nextMarker := nextKey + ":"
			if j := strings.Index(after, nextMarker); j > 0 && j < end {
				end = j
			}
		}
		fields[key] = strings.TrimSpace(after[:end])
	}

	return fields
}

// removeEmptySections removes headings that have no content after them
// (e.g., empty "Table of Contents" sections from lite tier).
func removeEmptySections(htmlContent string) string {
	// Remove <h2 id="...">Table of Contents</h2> followed by empty content
	// Pattern: h2 tag, then only whitespace/empty elements until next h2 or end
	lines := strings.Split(htmlContent, "\n")
	var result []string
	skip := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this is a heading we might want to skip
		if strings.Contains(trimmed, "Table of Contents</h2>") {
			// Look ahead to see if there's actual content before next heading
			hasContent := false
			for j := i + 1; j < len(lines); j++ {
				nextTrimmed := strings.TrimSpace(lines[j])
				if nextTrimmed == "" {
					continue
				}
				if strings.HasPrefix(nextTrimmed, "<h") {
					break
				}
				// Check for actual content (not just empty lists or whitespace)
				stripped := strings.ReplaceAll(nextTrimmed, "<p>", "")
				stripped = strings.ReplaceAll(stripped, "</p>", "")
				stripped = strings.TrimSpace(stripped)
				if stripped != "" {
					hasContent = true
					break
				}
			}
			if !hasContent {
				skip = true
				continue
			}
		}

		// If we're skipping empty section content, stop at next heading
		if skip {
			if strings.HasPrefix(trimmed, "<h") && !strings.Contains(trimmed, "Table of Contents") {
				skip = false
			} else if trimmed == "" || trimmed == "<p></p>" {
				continue
			} else if !strings.HasPrefix(trimmed, "<h") {
				skip = false
			} else {
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// removeBrokenLinks removes list items and sections that link to non-existent files.
func removeBrokenLinks(htmlContent, docsDir string) string {
	// Remove <li> elements containing links to files that don't exist in docsDir.
	// This catches the "Quick Links" section pointing to architecture.html when architecture.md doesn't exist.
	lines := strings.Split(htmlContent, "\n")
	var result []string
	skipQuickLinks := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for "Quick Links" heading — if all its links are broken, remove the whole section
		if strings.Contains(trimmed, "Quick Links</h2>") {
			// Look ahead to see if any links are valid
			hasValidLink := false
			for j := i + 1; j < len(lines); j++ {
				nextTrimmed := strings.TrimSpace(lines[j])
				if strings.HasPrefix(nextTrimmed, "<h") && !strings.Contains(nextTrimmed, "Quick Links") {
					break
				}
				// Check if any href points to an existing file
				if strings.Contains(nextTrimmed, "href=") {
					hrefStart := strings.Index(nextTrimmed, `href="`)
					if hrefStart >= 0 {
						hrefStart += 6
						hrefEnd := strings.Index(nextTrimmed[hrefStart:], `"`)
						if hrefEnd > 0 {
							href := nextTrimmed[hrefStart : hrefStart+hrefEnd]
							// Convert .html back to .md to check existence
							mdHref := strings.TrimSuffix(href, ".html") + ".md"
							if _, err := os.Stat(filepath.Join(docsDir, mdHref)); err == nil {
								hasValidLink = true
							}
						}
					}
				}
			}
			if !hasValidLink {
				skipQuickLinks = true
				continue
			}
		}

		if skipQuickLinks {
			if strings.HasPrefix(trimmed, "<h") && !strings.Contains(trimmed, "Quick Links") {
				skipQuickLinks = false
			} else {
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// cleanTableSummaries extracts just the Summary sentence from verbose metadata in table cells.
func cleanTableSummaries(htmlContent string) string {
	const tdOpen = "<td>"
	const tdClose = "</td>"

	var result strings.Builder
	remaining := htmlContent

	for {
		idx := strings.Index(remaining, tdOpen)
		if idx == -1 {
			result.WriteString(remaining)
			break
		}
		endIdx := strings.Index(remaining[idx:], tdClose)
		if endIdx == -1 {
			result.WriteString(remaining)
			break
		}
		endIdx += idx

		cellContent := remaining[idx+len(tdOpen) : endIdx]

		if strings.Contains(cellContent, "Summary:") && strings.Contains(cellContent, "Language:") {
			fields := parseMetadataFields(cellContent)
			summary := fields["Summary"]
			if summary == "" {
				summary = fields["Purpose"]
			}
			if summary == "" {
				summary = strings.TrimSpace(cellContent)
				if len(summary) > 200 {
					summary = summary[:200] + "..."
				}
			}
			result.WriteString(remaining[:idx])
			result.WriteString(tdOpen)
			result.WriteString(summary)
			result.WriteString(tdClose)
			remaining = remaining[endIdx+len(tdClose):]
		} else {
			result.WriteString(remaining[:endIdx+len(tdClose)])
			remaining = remaining[endIdx+len(tdClose):]
		}
	}

	return result.String()
}
