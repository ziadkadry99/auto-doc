package site

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// FileTree represents a node in the documentation file tree.
type FileTree struct {
	Name     string
	Title    string // Human-readable display name (extracted from markdown H1 or formatted from name).
	Path     string // For files: full relative path. For dirs: directory path (e.g., "internal/config").
	IsDir    bool
	Children []*FileTree
}

// BuildTree constructs a FileTree from a list of relative file paths.
// titleMap is an optional map of relative path -> display title (from markdown H1 headings).
func BuildTree(paths []string, titleMap map[string]string) *FileTree {
	root := &FileTree{Name: "docs", IsDir: true}

	for _, p := range paths {
		p = filepath.ToSlash(p)
		parts := strings.Split(p, "/")
		current := root
		for i, part := range parts {
			isLast := i == len(parts)-1
			found := false
			for _, child := range current.Children {
				if child.Name == part {
					current = child
					found = true
					break
				}
			}
			if !found {
				node := &FileTree{
					Name:  part,
					IsDir: !isLast,
				}
				if isLast {
					node.Path = p
					if titleMap != nil {
						if title, ok := titleMap[p]; ok {
							node.Title = title
						}
					}
				} else {
					// Store the directory path.
					node.Path = strings.Join(parts[:i+1], "/")
					node.Title = formatDirName(part)
				}
				current.Children = append(current.Children, node)
				current = node
			}
		}
	}

	sortTree(root)
	return root
}

// sortTree recursively sorts tree children: directories first, then files, alphabetically.
func sortTree(node *FileTree) {
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	for _, child := range node.Children {
		if child.IsDir {
			sortTree(child)
		}
	}
}

// ToHTML renders the file tree as nested <ul><li> HTML for the sidebar.
// basePath is the relative prefix to get back to root (e.g., "../" for a page one level deep).
func (t *FileTree) ToHTML(activePath, basePath string) string {
	activeAncestors := computeActiveAncestors(activePath)

	var b strings.Builder
	// Home link at top.
	homeActive := ""
	if activePath == "index.md" {
		homeActive = ` class="active"`
	}
	fmt.Fprintf(&b, `<ul><li class="file home-link"><a href="%sindex.html"%s>Home</a></li></ul>`+"\n", basePath, homeActive)

	renderChildren(&b, t, activePath, basePath, activeAncestors)
	return b.String()
}

// computeActiveAncestors returns the set of directory paths that are ancestors of activePath.
// For "internal/config/config.go.md" it returns {"internal", "internal/config"}.
func computeActiveAncestors(activePath string) map[string]bool {
	ancestors := make(map[string]bool)
	activePath = filepath.ToSlash(activePath)
	parts := strings.Split(activePath, "/")
	for i := 1; i < len(parts); i++ {
		ancestors[strings.Join(parts[:i], "/")] = true
	}
	return ancestors
}

func renderChildren(b *strings.Builder, node *FileTree, activePath, basePath string, activeAncestors map[string]bool) {
	if len(node.Children) == 0 {
		return
	}
	b.WriteString("<ul>\n")
	for _, child := range node.Children {
		if child.IsDir {
			expanded := ""
			if activeAncestors[child.Path] {
				expanded = "expanded"
			}
			dirLabel := child.Title
			if dirLabel == "" {
				dirLabel = child.Name
			}
			fmt.Fprintf(b, `<li class="dir %s"><span class="dir-toggle">%s</span>`+"\n", expanded, dirLabel)
			renderChildren(b, child, activePath, basePath, activeAncestors)
			b.WriteString("</li>\n")
		} else {
			if child.Path == "index.md" {
				continue
			}
			htmlPath := basePath + mdPathToHTML(child.Path)
			displayName := child.Title
			if displayName == "" {
				displayName = cleanDisplayName(child.Name)
			}
			activeClass := ""
			if child.Path == activePath {
				activeClass = ` class="active"`
			}
			fmt.Fprintf(b, `<li class="file"><a href="%s"%s>%s</a></li>`+"\n", htmlPath, activeClass, displayName)
		}
	}
	b.WriteString("</ul>\n")
}

// mdPathToHTML converts a markdown path to its HTML equivalent.
func mdPathToHTML(p string) string {
	if strings.HasSuffix(p, ".md") {
		return strings.TrimSuffix(p, ".md") + ".html"
	}
	return p
}

// cleanDisplayName strips the .md extension and shows a cleaner file name.
func cleanDisplayName(name string) string {
	name = strings.TrimSuffix(name, ".md")
	return name
}

// formatDirName converts a directory name to a human-readable display name.
// Source directories (cmd, internal, etc.) are capitalized; multi-word slugs are title-cased.
func formatDirName(name string) string {
	// Title-case each word separated by hyphens or underscores.
	words := strings.FieldsFunc(name, func(c rune) bool {
		return c == '-' || c == '_'
	})
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
