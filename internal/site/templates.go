package site

// pageTemplate is the Go html/template for each documentation page.
const pageTemplate = `<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}} — {{.ProjectName}}</title>
  <link rel="stylesheet" href="{{.BasePath}}style.css">
  <script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
</head>
<body>
  <nav class="sidebar" id="sidebar">
    <div class="sidebar-header">
      {{if .LogoFile}}<a href="{{.BasePath}}index.html" class="sidebar-logo-link"><img src="{{.BasePath}}{{.LogoFile}}" alt="{{.ProjectName}}" class="sidebar-logo"></a>{{end}}
      <h2 class="project-title">{{.ProjectName}}</h2>
      <input type="text" id="search-input" placeholder="Search docs..." autocomplete="off">
    </div>
    <div class="sidebar-tree" id="sidebar-tree">
      {{.TreeHTML}}
    </div>
  </nav>
  <div class="sidebar-overlay" id="sidebar-overlay"></div>
  <main class="content">
    <div class="top-bar">
      <button class="menu-toggle" id="menu-toggle" aria-label="Toggle sidebar">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/>
        </svg>
      </button>
      <div class="ai-search-bar">
        <svg class="ai-search-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
        <input type="text" id="ai-search-input" placeholder="Ask about this codebase..." autocomplete="off">
        <span class="ai-search-hint">Enter</span>
      </div>
      <button class="theme-toggle" id="theme-toggle" aria-label="Toggle theme">
        <svg class="sun-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
        </svg>
        <svg class="moon-icon" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
        </svg>
      </button>
    </div>
    <div class="ai-search-results" id="ai-search-results"></div>
    <article class="page-content">
      {{.Content}}
    </article>
  </main>
  <script src="{{.BasePath}}script.js"></script>
</body>
</html>`

// cssContent is the full CSS for the documentation site.
const cssContent = `/* ============ CSS Variables ============ */
:root {
  --bg: #ffffff;
  --bg-secondary: #f8f9fa;
  --bg-sidebar: #f1f3f5;
  --text: #212529;
  --text-secondary: #495057;
  --text-muted: #868e96;
  --border: #dee2e6;
  --accent: #228be6;
  --accent-hover: #1c7ed6;
  --accent-light: #e7f5ff;
  --code-bg: #f1f3f5;
  --code-border: #e9ecef;
  --link: #228be6;
  --sidebar-width: 280px;
  --content-max-width: 900px;
  --table-stripe: #f8f9fa;
  --search-bg: #ffffff;
  --shadow: 0 1px 3px rgba(0,0,0,0.08);
  --shadow-lg: 0 4px 12px rgba(0,0,0,0.1);
}

[data-theme="dark"] {
  --bg: #1a1b26;
  --bg-secondary: #1f2030;
  --bg-sidebar: #16171f;
  --text: #c0caf5;
  --text-secondary: #a9b1d6;
  --text-muted: #565f89;
  --border: #292e42;
  --accent: #7aa2f7;
  --accent-hover: #89b4fa;
  --accent-light: #1a1b2e;
  --code-bg: #1f2030;
  --code-border: #292e42;
  --link: #7aa2f7;
  --sidebar-width: 280px;
  --table-stripe: #1f2030;
  --search-bg: #1f2030;
  --shadow: 0 1px 3px rgba(0,0,0,0.3);
  --shadow-lg: 0 4px 12px rgba(0,0,0,0.4);
}

@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --bg: #1a1b26;
    --bg-secondary: #1f2030;
    --bg-sidebar: #16171f;
    --text: #c0caf5;
    --text-secondary: #a9b1d6;
    --text-muted: #565f89;
    --border: #292e42;
    --accent: #7aa2f7;
    --accent-hover: #89b4fa;
    --accent-light: #1a1b2e;
    --code-bg: #1f2030;
    --code-border: #292e42;
    --link: #7aa2f7;
    --table-stripe: #1f2030;
    --search-bg: #1f2030;
    --shadow: 0 1px 3px rgba(0,0,0,0.3);
    --shadow-lg: 0 4px 12px rgba(0,0,0,0.4);
  }
}

/* ============ Reset & Base ============ */
*, *::before, *::after {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

html {
  font-size: 16px;
  scroll-behavior: smooth;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  color: var(--text);
  background: var(--bg);
  line-height: 1.7;
  display: flex;
  min-height: 100vh;
}

/* ============ Sidebar ============ */
.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border);
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  overflow-y: auto;
  z-index: 100;
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  padding: 20px 16px 12px;
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  background: var(--bg-sidebar);
  z-index: 1;
}

.sidebar-logo-link {
  display: block;
  text-align: center;
  margin-bottom: 8px;
}

.sidebar-logo {
  max-width: 100px;
  max-height: 100px;
  border-radius: 8px;
}

.project-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--accent);
  margin-bottom: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

#search-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  font-size: 0.85rem;
  background: var(--search-bg);
  color: var(--text);
  outline: none;
  transition: border-color 0.2s;
}

#search-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-light);
}

.sidebar-tree {
  padding: 8px 0;
  flex: 1;
  overflow-y: auto;
}

.sidebar-tree ul {
  list-style: none;
  padding-left: 0;
  margin: 0;
}

.sidebar-tree ul ul {
  padding-left: 16px;
}

.sidebar-tree li {
  margin: 0;
}

.sidebar-tree .dir > .dir-toggle {
  display: block;
  padding: 4px 16px;
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-secondary);
  cursor: pointer;
  user-select: none;
  position: relative;
}

.sidebar-tree .dir > .dir-toggle::before {
  content: "\25B6";
  display: inline-block;
  margin-right: 6px;
  font-size: 0.6rem;
  transition: transform 0.15s;
  vertical-align: middle;
}

.sidebar-tree .dir.expanded > .dir-toggle::before {
  transform: rotate(90deg);
}

.sidebar-tree .dir > ul {
  display: none;
}

.sidebar-tree .dir.expanded > ul {
  display: block;
}

.sidebar-tree .file a {
  display: block;
  padding: 3px 16px 3px 22px;
  font-size: 0.82rem;
  color: var(--text-muted);
  text-decoration: none;
  border-radius: 4px;
  transition: background 0.15s, color 0.15s;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sidebar-tree .file a:hover {
  background: var(--accent-light);
  color: var(--accent);
}

.sidebar-tree .file a.active {
  background: var(--accent-light);
  color: var(--accent);
  font-weight: 600;
}

/* ============ Overlay (mobile) ============ */
.sidebar-overlay {
  display: none;
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.4);
  z-index: 99;
}

.sidebar-overlay.visible {
  display: block;
}

/* ============ Main Content ============ */
.content {
  margin-left: var(--sidebar-width);
  flex: 1;
  min-width: 0;
}

.top-bar {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  padding: 8px 24px;
  border-bottom: 1px solid var(--border);
  background: var(--bg);
  position: sticky;
  top: 0;
  z-index: 50;
}

.menu-toggle {
  display: none;
  background: none;
  border: none;
  color: var(--text);
  cursor: pointer;
  padding: 4px;
  margin-right: auto;
}

.theme-toggle {
  background: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  cursor: pointer;
  padding: 6px 8px;
  display: flex;
  align-items: center;
  transition: background 0.2s;
}

.theme-toggle:hover {
  background: var(--bg-secondary);
}

[data-theme="dark"] .sun-icon { display: inline; }
[data-theme="dark"] .moon-icon { display: none; }
[data-theme="light"] .sun-icon { display: none; }
[data-theme="light"] .moon-icon { display: inline; }

.page-content {
  max-width: var(--content-max-width);
  margin: 0 auto;
  padding: 32px 40px 64px;
}

/* ============ Typography ============ */
.page-content h1 {
  font-size: 2rem;
  font-weight: 700;
  margin: 0 0 16px;
  padding-bottom: 8px;
  border-bottom: 2px solid var(--border);
  color: var(--text);
}

.page-content h2 {
  font-size: 1.5rem;
  font-weight: 600;
  margin: 32px 0 12px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border);
  color: var(--text);
}

.page-content h3 {
  font-size: 1.2rem;
  font-weight: 600;
  margin: 24px 0 8px;
  color: var(--text);
}

.page-content h4 {
  font-size: 1.05rem;
  font-weight: 600;
  margin: 20px 0 6px;
  color: var(--text-secondary);
}

.page-content p {
  margin: 0 0 16px;
}

.page-content a {
  color: var(--link);
  text-decoration: none;
}

.page-content a:hover {
  text-decoration: underline;
}

.page-content ul, .page-content ol {
  margin: 0 0 16px;
  padding-left: 24px;
}

.page-content li {
  margin-bottom: 4px;
}

.page-content hr {
  border: none;
  border-top: 1px solid var(--border);
  margin: 24px 0;
}

.page-content blockquote {
  border-left: 4px solid var(--accent);
  padding: 8px 16px;
  margin: 0 0 16px;
  background: var(--bg-secondary);
  color: var(--text-secondary);
  border-radius: 0 4px 4px 0;
}

/* ============ Code ============ */
.page-content code {
  font-family: "JetBrains Mono", "Fira Code", "SF Mono", Consolas, monospace;
  font-size: 0.88em;
  background: var(--code-bg);
  padding: 2px 6px;
  border-radius: 4px;
  border: 1px solid var(--code-border);
}

.page-content pre {
  margin: 0 0 16px;
  border-radius: 8px;
  border: 1px solid var(--code-border);
  overflow-x: auto;
  position: relative;
}

.page-content pre code {
  display: block;
  padding: 16px;
  border: none;
  background: var(--code-bg);
  font-size: 0.85rem;
  line-height: 1.6;
}

.copy-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px 8px;
  font-size: 0.75rem;
  opacity: 0;
  transition: opacity 0.2s;
}

.page-content pre:hover .copy-btn {
  opacity: 1;
}

.copy-btn:hover {
  color: var(--accent);
  border-color: var(--accent);
}

/* ============ Tables ============ */
.page-content table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  margin: 0 0 16px;
  font-size: 0.88rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow: hidden;
}

.page-content thead th {
  background: var(--bg-secondary);
  font-weight: 600;
  text-align: left;
  padding: 10px 14px;
  border-bottom: 2px solid var(--border);
  white-space: nowrap;
}

.page-content tbody td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
  line-height: 1.6;
}

.page-content tbody td:first-child {
  font-weight: 500;
}

.page-content tbody td:last-child {
  color: var(--text-secondary);
}

.page-content tbody tr:nth-child(even) {
  background: var(--table-stripe);
}

.page-content tbody tr:hover {
  background: var(--accent-light);
}

/* ============ File Metadata (reformatted from LLM output) ============ */
.file-meta {
  margin: 8px 0 16px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.meta-badge {
  display: inline-block;
  font-size: 0.78rem;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 12px;
  letter-spacing: 0.02em;
}

.lang-badge {
  background: var(--accent-light);
  color: var(--accent);
  border: 1px solid var(--accent);
}

.file-summary {
  font-size: 1rem;
  line-height: 1.7;
  margin: 0 0 16px;
}

.file-purpose {
  margin: 0 0 16px;
  padding: 12px 16px;
  background: var(--bg-secondary);
  border-left: 3px solid var(--accent);
  border-radius: 0 6px 6px 0;
  font-size: 0.9rem;
  line-height: 1.6;
  color: var(--text-secondary);
}

.file-purpose strong {
  color: var(--text);
}

.file-deps {
  margin: 0 0 16px;
  font-size: 0.88rem;
}

.file-deps strong {
  display: block;
  margin-bottom: 8px;
  font-size: 0.82rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.dep-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.dep-tag {
  display: inline-block;
  font-size: 0.78rem;
  font-family: "JetBrains Mono", "Fira Code", "SF Mono", Consolas, monospace;
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--code-bg);
  border: 1px solid var(--code-border);
  color: var(--text-secondary);
}

.dep-tag.dep-api {
  border-color: var(--accent);
  color: var(--accent);
}

/* ============ AI Search ============ */
.ai-search-bar {
  flex: 1;
  max-width: 520px;
  margin: 0 16px;
  position: relative;
  display: flex;
  align-items: center;
}

.ai-search-icon {
  position: absolute;
  left: 12px;
  color: var(--text-muted);
  pointer-events: none;
}

#ai-search-input {
  width: 100%;
  padding: 8px 60px 8px 36px;
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 0.88rem;
  background: var(--bg-secondary);
  color: var(--text);
  outline: none;
  transition: border-color 0.2s, box-shadow 0.2s;
}

#ai-search-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-light);
}

#ai-search-input::placeholder {
  color: var(--text-muted);
}

.ai-search-hint {
  position: absolute;
  right: 10px;
  font-size: 0.7rem;
  color: var(--text-muted);
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 1px 6px;
  pointer-events: none;
}

.ai-search-results {
  display: none;
  max-width: var(--content-max-width);
  margin: 0 auto;
  padding: 16px 40px 0;
}

.ai-search-results.visible {
  display: block;
}

.ai-results-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.ai-results-header h3 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
  margin: 0;
}

.ai-results-close {
  background: none;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px 10px;
  font-size: 0.78rem;
  transition: color 0.15s, border-color 0.15s;
}

.ai-results-close:hover {
  color: var(--text);
  border-color: var(--text-muted);
}

.ai-result-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 14px 18px;
  margin-bottom: 10px;
  transition: border-color 0.15s, box-shadow 0.15s;
  cursor: pointer;
  text-decoration: none;
  display: block;
  color: inherit;
}

.ai-result-card:hover {
  border-color: var(--accent);
  box-shadow: var(--shadow);
  text-decoration: none;
  color: inherit;
}

.ai-result-top {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  flex-wrap: wrap;
}

.ai-result-path {
  font-size: 0.82rem;
  font-family: "JetBrains Mono", "Fira Code", "SF Mono", Consolas, monospace;
  color: var(--accent);
  font-weight: 500;
}

.ai-result-badge {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 1px 7px;
  border-radius: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.ai-result-badge.type-file { background: #dbeafe; color: #1d4ed8; }
.ai-result-badge.type-function { background: #dcfce7; color: #166534; }
.ai-result-badge.type-class { background: #fef3c7; color: #92400e; }
.ai-result-badge.type-architecture { background: #ede9fe; color: #5b21b6; }
.ai-result-badge.type-module { background: #fce7f3; color: #9d174d; }

[data-theme="dark"] .ai-result-badge.type-file { background: #1e3a5f; color: #93c5fd; }
[data-theme="dark"] .ai-result-badge.type-function { background: #14532d; color: #86efac; }
[data-theme="dark"] .ai-result-badge.type-class { background: #451a03; color: #fcd34d; }
[data-theme="dark"] .ai-result-badge.type-architecture { background: #2e1065; color: #c4b5fd; }
[data-theme="dark"] .ai-result-badge.type-module { background: #500724; color: #f9a8d4; }

.ai-result-score {
  font-size: 0.72rem;
  color: var(--text-muted);
  margin-left: auto;
}

.ai-result-content {
  font-size: 0.85rem;
  line-height: 1.6;
  color: var(--text-secondary);
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}

.ai-result-symbol {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 4px;
}

.ai-search-loading {
  text-align: center;
  padding: 24px;
  color: var(--text-muted);
  font-size: 0.88rem;
}

.ai-search-loading .spinner {
  display: inline-block;
  width: 18px;
  height: 18px;
  border: 2px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
  vertical-align: middle;
  margin-right: 8px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.ai-search-empty {
  text-align: center;
  padding: 24px;
  color: var(--text-muted);
  font-size: 0.88rem;
}

.ai-search-error {
  text-align: center;
  padding: 16px;
  color: #e03131;
  background: #fff5f5;
  border: 1px solid #ffc9c9;
  border-radius: 8px;
  font-size: 0.85rem;
}

[data-theme="dark"] .ai-search-error {
  background: #2c1010;
  border-color: #5c2020;
  color: #ffa8a8;
}

/* ============ AI Answer Panel ============ */
.ai-answer {
  background: var(--accent-light);
  border: 1px solid var(--accent);
  border-left: 4px solid var(--accent);
  border-radius: 8px;
  padding: 16px 20px;
  margin-bottom: 16px;
}

.ai-answer-label {
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--accent);
  margin-bottom: 8px;
}

.ai-answer-content {
  font-size: 0.9rem;
  line-height: 1.7;
  color: var(--text);
}

.ai-answer-content code {
  font-family: "JetBrains Mono", "Fira Code", "SF Mono", Consolas, monospace;
  font-size: 0.85em;
  background: var(--code-bg);
  padding: 1px 5px;
  border-radius: 3px;
  border: 1px solid var(--code-border);
}

/* ============ Mermaid Diagrams ============ */
.mermaid {
  text-align: center;
  margin: 16px 0;
  padding: 16px;
  background: var(--bg-secondary);
  border-radius: 8px;
  border: 1px solid var(--border);
  overflow: hidden;
  position: relative;
  cursor: grab;
}

.mermaid.panning {
  cursor: grabbing;
}

.mermaid-controls {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  gap: 4px;
  z-index: 10;
}

.mermaid-controls button {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-muted);
  cursor: pointer;
  width: 28px;
  height: 28px;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s, border-color 0.15s;
}

.mermaid-controls button:hover {
  color: var(--accent);
  border-color: var(--accent);
}

/* ============ Architecture Diagrams (JSON→SVG) ============ */
.arch-diagram {
  text-align: center;
  margin: 16px 0;
  padding: 16px;
  background: var(--bg-secondary);
  border-radius: 8px;
  border: 1px solid var(--border);
  overflow: hidden;
  position: relative;
  cursor: grab;
  min-height: 120px;
}

.arch-diagram.panning {
  cursor: grabbing;
}

.arch-diagram svg {
  width: 100%;
  max-height: 600px;
}

.arch-diagram-controls {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  gap: 4px;
  z-index: 10;
}

.arch-diagram-controls button {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text-muted);
  cursor: pointer;
  width: 28px;
  height: 28px;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s, border-color 0.15s;
}

.arch-diagram-controls button:hover {
  color: var(--accent);
  border-color: var(--accent);
}

/* Fullscreen overlay for architecture diagrams */
.arch-fullscreen-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}

.arch-fullscreen-overlay .arch-fullscreen-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
  gap: 6px;
  flex-shrink: 0;
}

.arch-fullscreen-overlay .arch-fullscreen-toolbar button {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-muted);
  cursor: pointer;
  padding: 6px 10px;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s, border-color 0.15s;
}

.arch-fullscreen-overlay .arch-fullscreen-toolbar button:hover {
  color: var(--accent);
  border-color: var(--accent);
}

.arch-fullscreen-overlay .arch-fullscreen-body {
  flex: 1;
  overflow: hidden;
  position: relative;
  cursor: grab;
}

.arch-fullscreen-overlay .arch-fullscreen-body.panning {
  cursor: grabbing;
}

.arch-fullscreen-overlay .arch-fullscreen-body svg {
  width: 100%;
  height: 100%;
}

/* ============ Responsive ============ */
@media (max-width: 768px) {
  .sidebar {
    transform: translateX(-100%);
    transition: transform 0.3s;
    box-shadow: none;
  }

  .sidebar.open {
    transform: translateX(0);
    box-shadow: var(--shadow-lg);
  }

  .content {
    margin-left: 0;
  }

  .menu-toggle {
    display: block;
  }

  .page-content {
    padding: 24px 16px 48px;
  }
}

/* ============ Search highlight ============ */
.search-match {
  background: var(--accent-light);
  border-radius: 2px;
}

.sidebar-tree .file.hidden,
.sidebar-tree .dir.hidden {
  display: none;
}

/* ============ Home link ============ */
.sidebar-tree .home-link a {
  display: block;
  padding: 6px 16px;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--accent);
  text-decoration: none;
  border-bottom: 1px solid var(--border);
  margin-bottom: 4px;
}

.sidebar-tree .home-link a:hover,
.sidebar-tree .home-link a.active {
  background: var(--accent-light);
}

/* ============ Scrollbar ============ */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: 3px;
}

::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

* {
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}
`

// jsContent is the JavaScript for search, sidebar, theme, and mermaid.
const jsContent = `(function() {
  "use strict";

  var html = document.documentElement;
  var sidebarTree = document.getElementById("sidebar-tree");

  // ===== Theme toggle =====
  var themeToggle = document.getElementById("theme-toggle");

  function getStoredTheme() {
    try { return localStorage.getItem("autodoc-theme"); } catch(e) { return null; }
  }

  function setTheme(theme) {
    html.setAttribute("data-theme", theme);
    try { localStorage.setItem("autodoc-theme", theme); } catch(e) {}
    // Re-render architecture diagrams with new theme colors.
    if (typeof renderArchDiagrams === "function") { renderArchDiagrams(); }
    // Re-render Mermaid diagrams with the new theme
    if (typeof mermaid !== "undefined") {
      mermaid.initialize({ startOnLoad: false, theme: theme === "dark" ? "dark" : "default", securityLevel: "loose", maxEdges: 2000, flowchart: { htmlLabels: true } });
      var reRenderPromises = [];
      document.querySelectorAll(".mermaid").forEach(function(el, idx) {
        var src = el.getAttribute("data-source");
        if (src) {
          el.removeAttribute("data-processed");
          el.removeAttribute("data-panzoom");
          var oldControls = el.querySelector(".mermaid-controls");
          if (oldControls) oldControls.remove();
          reRenderPromises.push(
            mermaid.render("mermaid-theme-" + idx, src).then(function(result) {
              el.innerHTML = result.svg;
            })
          );
        }
      });
      Promise.all(reRenderPromises).then(function() {
        if (typeof setupMermaidPanZoom === "function") {
          setupMermaidPanZoom();
        }
      });
    }
  }

  var stored = getStoredTheme();
  if (stored) {
    setTheme(stored);
  } else if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
    setTheme("dark");
  }

  if (themeToggle) {
    themeToggle.addEventListener("click", function() {
      var current = html.getAttribute("data-theme") || "light";
      setTheme(current === "dark" ? "light" : "dark");
    });
  }

  // ===== Sidebar toggle (mobile) =====
  var menuToggle = document.getElementById("menu-toggle");
  var sidebar = document.getElementById("sidebar");
  var overlay = document.getElementById("sidebar-overlay");

  function toggleSidebar() {
    sidebar.classList.toggle("open");
    overlay.classList.toggle("visible");
  }

  if (menuToggle) menuToggle.addEventListener("click", toggleSidebar);
  if (overlay) overlay.addEventListener("click", toggleSidebar);

  // ===== Directory tree toggle =====
  document.querySelectorAll(".dir-toggle").forEach(function(toggle) {
    toggle.addEventListener("click", function() {
      this.parentElement.classList.toggle("expanded");
    });
  });

  // ===== Sidebar file filter (with search-index.json) =====
  var searchInput = document.getElementById("search-input");
  var searchIndex = null;

  // Load search index asynchronously
  (function loadSearchIndex() {
    var basePath = document.querySelector("link[rel=stylesheet]");
    var base = "";
    if (basePath) {
      var href = basePath.getAttribute("href");
      base = href.replace("style.css", "");
    }
    fetch(base + "search-index.json")
      .then(function(r) { return r.json(); })
      .then(function(data) { searchIndex = data; })
      .catch(function() { searchIndex = null; });
  })();

  if (searchInput && sidebarTree) {
    // Remember original expanded state
    var originalExpanded = [];
    sidebarTree.querySelectorAll(".dir").forEach(function(dir) {
      if (dir.classList.contains("expanded")) originalExpanded.push(dir);
    });

    searchInput.addEventListener("input", function() {
      var query = this.value.toLowerCase().trim();
      var items = sidebarTree.querySelectorAll("li");

      if (query === "") {
        items.forEach(function(item) { item.classList.remove("hidden"); });
        // Restore original collapse state
        sidebarTree.querySelectorAll(".dir").forEach(function(dir) {
          if (originalExpanded.indexOf(dir) !== -1) {
            dir.classList.add("expanded");
          } else {
            dir.classList.remove("expanded");
          }
        });
        return;
      }

      // Build set of matching paths from search index
      var matchingPaths = new Set();
      if (searchIndex) {
        searchIndex.forEach(function(entry) {
          var haystack = (entry.title + " " + entry.summary + " " + entry.content + " " + entry.path).toLowerCase();
          if (haystack.indexOf(query) !== -1) {
            matchingPaths.add(entry.path);
          }
        });
      }

      // First pass: mark files
      sidebarTree.querySelectorAll(".file").forEach(function(item) {
        var link = item.querySelector("a");
        if (!link) return;
        var text = link.textContent.toLowerCase();
        var href = link.getAttribute("href").toLowerCase();
        // Extract just the relative path for search index matching
        var relPath = href.replace(/^(\.\.\/)*/g, "");

        var match = text.indexOf(query) !== -1 || href.indexOf(query) !== -1 || matchingPaths.has(relPath);
        item.classList.toggle("hidden", !match);
      });

      // Second pass: show/hide directories based on visible children
      Array.from(sidebarTree.querySelectorAll(".dir")).reverse().forEach(function(dir) {
        var hasVisible = dir.querySelectorAll("li.file:not(.hidden)").length > 0;
        dir.classList.toggle("hidden", !hasVisible);
        if (hasVisible) dir.classList.add("expanded");
      });
    });
  }

  // ===== AI Search =====
  var aiSearchInput = document.getElementById("ai-search-input");
  var aiResultsPanel = document.getElementById("ai-search-results");

  function getBasePath() {
    var link = document.querySelector("link[rel=stylesheet]");
    if (link) {
      return link.getAttribute("href").replace("style.css", "");
    }
    return "";
  }

  function escapeHtml(str) {
    var div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
  }

  function filePathToDocUrl(filePath, basePath) {
    // Convert source file path (e.g. "internal/config/config.go") to doc page URL
    return basePath + filePath + ".html";
  }

  function formatAnswerHtml(text) {
    // Convert backtick-wrapped text to <code> elements.
    return escapeHtml(text).replace(/` + "`" + `([^` + "`" + `]+)` + "`" + `/g, '<code>$1</code>');
  }

  function showAIResults(query, results, answer) {
    var base = getBasePath();
    var html = '<div class="ai-results-header">' +
      '<h3>Results for "' + escapeHtml(query) + '"</h3>' +
      '<button class="ai-results-close" id="ai-results-close">Close</button>' +
      '</div>';

    if (answer) {
      html += '<div class="ai-answer">';
      html += '<div class="ai-answer-label">AI Answer</div>';
      html += '<div class="ai-answer-content">' + formatAnswerHtml(answer) + '</div>';
      html += '</div>';
    }

    if (results.length === 0 && !answer) {
      html += '<div class="ai-search-empty">No relevant results found. Try rephrasing your question.</div>';
    } else {
      results.forEach(function(r) {
        var url = filePathToDocUrl(r.file_path, base);
        var badgeClass = "type-" + (r.type || "file");
        html += '<a class="ai-result-card" href="' + escapeHtml(url) + '">';
        html += '<div class="ai-result-top">';
        html += '<span class="ai-result-path">' + escapeHtml(r.file_path) + '</span>';
        if (r.type) {
          html += '<span class="ai-result-badge ' + badgeClass + '">' + escapeHtml(r.type) + '</span>';
        }
        if (r.language) {
          html += '<span class="ai-result-badge type-file">' + escapeHtml(r.language) + '</span>';
        }
        html += '<span class="ai-result-score">' + Math.round(r.similarity * 100) + '% match</span>';
        html += '</div>';
        if (r.symbol) {
          html += '<div class="ai-result-symbol">' + escapeHtml(r.symbol) + '</div>';
        }
        html += '<div class="ai-result-content">' + escapeHtml(r.content) + '</div>';
        html += '</a>';
      });
    }

    aiResultsPanel.innerHTML = html;
    aiResultsPanel.classList.add("visible");

    document.getElementById("ai-results-close").addEventListener("click", function() {
      aiResultsPanel.classList.remove("visible");
      aiResultsPanel.innerHTML = "";
    });
  }

  function showAILoading() {
    aiResultsPanel.innerHTML = '<div class="ai-search-loading"><span class="spinner"></span>Searching and generating answer...</div>';
    aiResultsPanel.classList.add("visible");
  }

  function showAIError(msg) {
    aiResultsPanel.innerHTML = '<div class="ai-results-header"><h3>Search</h3>' +
      '<button class="ai-results-close" id="ai-results-close">Close</button></div>' +
      '<div class="ai-search-error">' + escapeHtml(msg) + '</div>';
    aiResultsPanel.classList.add("visible");
    document.getElementById("ai-results-close").addEventListener("click", function() {
      aiResultsPanel.classList.remove("visible");
      aiResultsPanel.innerHTML = "";
    });
  }

  function localSearch(query) {
    if (!searchIndex) return [];
    var terms = query.toLowerCase().split(/\s+/);
    var scored = [];
    searchIndex.forEach(function(entry) {
      var text = ((entry.title || "") + " " + (entry.summary || "") + " " + (entry.content || "")).toLowerCase();
      var hits = 0;
      terms.forEach(function(t) { if (text.indexOf(t) >= 0) hits++; });
      if (hits > 0) {
        scored.push({
          file_path: entry.path.replace(/\.html$/, "").replace(/\.md$/, ""),
          type: "file",
          similarity: hits / terms.length,
          content: entry.summary || entry.title || ""
        });
      }
    });
    scored.sort(function(a, b) { return b.similarity - a.similarity; });
    return scored.slice(0, 10);
  }

  if (aiSearchInput) {
    aiSearchInput.addEventListener("keydown", function(e) {
      if (e.key !== "Enter") return;
      var query = this.value.trim();
      if (!query) return;

      showAILoading();

      fetch("/api/search", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query: query, limit: 10 })
      })
      .then(function(r) {
        var ct = r.headers.get("content-type") || "";
        if (!r.ok || ct.indexOf("application/json") === -1) {
          throw new Error("_fallback_");
        }
        return r.json();
      })
      .then(function(data) {
        var results = Array.isArray(data) ? data : (data.results || []);
        var answer = data.answer || "";
        showAIResults(query, results, answer);
      })
      .catch(function(err) {
        if (err.message === "_fallback_" || err instanceof SyntaxError) {
          var local = localSearch(query);
          if (local.length > 0) {
            showAIResults(query, local, "");
          } else {
            showAIError("No results found for your query.");
          }
        } else {
          showAIError(err.message || "Search failed.");
        }
      });
    });
  }

  // ===== Copy buttons for code blocks =====
  document.querySelectorAll("pre").forEach(function(pre) {
    var btn = document.createElement("button");
    btn.className = "copy-btn";
    btn.textContent = "Copy";
    btn.addEventListener("click", function() {
      var code = pre.querySelector("code");
      if (code) {
        navigator.clipboard.writeText(code.textContent).then(function() {
          btn.textContent = "Copied!";
          setTimeout(function() { btn.textContent = "Copy"; }, 2000);
        });
      }
    });
    pre.style.position = "relative";
    pre.appendChild(btn);
  });

  // ===== Architecture diagram JSON→SVG renderer =====
  function escSvg(s) {
    return String(s).replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/"/g,"&quot;");
  }

  // Build SVG string from parsed diagram data and resolved CSS colors.
  function buildArchSvg(nodes, edges, colors, markerID) {
    if (nodes.length === 0) return "";

    // Group nodes by group field.
    var groupOrder = [];
    var groupMap = {};
    nodes.forEach(function(n) {
      var g = n.group || "";
      if (!groupMap[g]) { groupMap[g] = []; groupOrder.push(g); }
      groupMap[g].push(n);
    });

    // Compute dynamic node width: fit the longest label with padding.
    var maxLabelLen = 0;
    nodes.forEach(function(n) {
      if (n.label && n.label.length > maxLabelLen) maxLabelLen = n.label.length;
    });
    var nodeW = Math.max(160, Math.min(300, maxLabelLen * 8.5 + 32));
    var nodeH = 54;
    var padX = 40, padY = 60;
    var groupPadX = 20, groupLabelH = 24, groupPadBottom = 14;

    var nodePos = {};
    var y = 24;
    var groupRects = [];

    groupOrder.forEach(function(gName) {
      var gNodes = groupMap[gName];
      var startY = y;
      if (gName) y += groupLabelH;
      gNodes.forEach(function(n, i) {
        nodePos[n.id] = { x: i * (nodeW + padX) + nodeW / 2, y: y + nodeH / 2, w: nodeW, h: nodeH };
      });
      if (gName) {
        var totalW = gNodes.length * nodeW + (gNodes.length - 1) * padX;
        groupRects.push({ name: gName, y: startY, h: y + nodeH + groupPadBottom - startY, nodesW: totalW });
      }
      y += nodeH + padY;
    });

    // Compute SVG width from widest row.
    var maxRowW = 0;
    groupOrder.forEach(function(gName) {
      var gNodes = groupMap[gName];
      var w = gNodes.length * nodeW + (gNodes.length - 1) * padX;
      if (w > maxRowW) maxRowW = w;
    });
    var svgW = maxRowW + groupPadX * 4;
    var svgH = y;

    // Center each row horizontally.
    groupOrder.forEach(function(gName) {
      var gNodes = groupMap[gName];
      var totalW = gNodes.length * nodeW + (gNodes.length - 1) * padX;
      var offsetX = (svgW - totalW) / 2;
      gNodes.forEach(function(n, i) {
        nodePos[n.id].x = offsetX + i * (nodeW + padX) + nodeW / 2;
      });
    });
    groupRects.forEach(function(gr) {
      gr.x = groupPadX;
      gr.w = svgW - groupPadX * 2;
    });

    var mid = markerID || "ah";
    var svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ' + svgW + ' ' + svgH + '">';
    svg += '<defs><marker id="' + mid + '" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">';
    svg += '<polygon points="0 0,8 3,0 6" fill="' + colors.muted + '"/></marker></defs>';

    // Group backgrounds.
    groupRects.forEach(function(gr) {
      svg += '<rect x="' + gr.x + '" y="' + gr.y + '" width="' + gr.w + '" height="' + gr.h + '" rx="8" fill="none" stroke="' + colors.border + '" stroke-dasharray="4"/>';
      svg += '<text x="' + (gr.x + 14) + '" y="' + (gr.y + 17) + '" font-size="11" font-weight="600" fill="' + colors.muted + '" font-family="sans-serif">' + escSvg(gr.name) + '</text>';
    });

    // Edges.
    edges.forEach(function(e) {
      var from = nodePos[e.from];
      var to = nodePos[e.to];
      if (!from || !to) return;
      var x1 = from.x, y1 = from.y + from.h / 2;
      var x2 = to.x, y2 = to.y - to.h / 2;
      if (y2 < y1) { y1 = from.y - from.h / 2; y2 = to.y + to.h / 2; }
      var my = (y1 + y2) / 2;
      svg += '<path d="M' + x1 + ',' + y1 + ' C' + x1 + ',' + my + ' ' + x2 + ',' + my + ' ' + x2 + ',' + y2 + '" fill="none" stroke="' + colors.muted + '" stroke-width="1.5" marker-end="url(#' + mid + ')"/>';
      if (e.label) {
        svg += '<text x="' + ((x1 + x2) / 2) + '" y="' + (my - 4) + '" font-size="10" fill="' + colors.muted + '" text-anchor="middle" font-family="sans-serif">' + escSvg(e.label) + '</text>';
      }
    });

    // Nodes.
    nodes.forEach(function(n) {
      var p = nodePos[n.id];
      if (!p) return;
      var x = p.x - p.w / 2, ny = p.y - p.h / 2;
      svg += '<rect x="' + x + '" y="' + ny + '" width="' + p.w + '" height="' + p.h + '" rx="6" fill="' + colors.bg + '" stroke="' + colors.accent + '" stroke-width="1.5"/>';
      var textY = n.desc ? p.y - 6 : p.y + 4;
      svg += '<text x="' + p.x + '" y="' + textY + '" font-size="13" font-weight="600" fill="' + colors.text + '" text-anchor="middle" font-family="sans-serif">' + escSvg(n.label) + '</text>';
      if (n.desc) {
        var desc = n.desc.length > 38 ? n.desc.substring(0, 37) + "\u2026" : n.desc;
        svg += '<text x="' + p.x + '" y="' + (p.y + 11) + '" font-size="10" fill="' + colors.muted + '" text-anchor="middle" font-family="sans-serif">' + escSvg(desc) + '</text>';
      }
    });

    svg += '</svg>';
    return svg;
  }

  function getArchColors() {
    var cs = getComputedStyle(document.documentElement);
    return {
      bg: cs.getPropertyValue("--bg").trim() || "#fff",
      border: cs.getPropertyValue("--border").trim() || "#dee2e6",
      accent: cs.getPropertyValue("--accent").trim() || "#228be6",
      text: cs.getPropertyValue("--text").trim() || "#212529",
      muted: cs.getPropertyValue("--text-muted").trim() || "#868e96"
    };
  }

  function setupArchPanZoom(container) {
    var svg = container.querySelector("svg");
    if (!svg) return;
    svg.style.maxWidth = "none";
    var state = { scale: 1, panX: 0, panY: 0, dragging: false, startX: 0, startY: 0 };
    function apply() {
      svg.style.transform = "translate(" + state.panX + "px," + state.panY + "px) scale(" + state.scale + ")";
      svg.style.transformOrigin = "0 0";
    }
    function reset() { state.scale = 1; state.panX = 0; state.panY = 0; apply(); }
    container.addEventListener("wheel", function(e) {
      e.preventDefault();
      var d = e.deltaY > 0 ? 0.9 : 1.1;
      var ns = Math.max(0.1, Math.min(10, state.scale * d));
      var r = container.getBoundingClientRect();
      var cx = e.clientX - r.left, cy = e.clientY - r.top;
      state.panX = cx - (cx - state.panX) * (ns / state.scale);
      state.panY = cy - (cy - state.panY) * (ns / state.scale);
      state.scale = ns;
      apply();
    }, { passive: false });
    container.addEventListener("mousedown", function(e) {
      if (e.button !== 0) return;
      state.dragging = true; state.startX = e.clientX - state.panX; state.startY = e.clientY - state.panY;
      container.classList.add("panning"); e.preventDefault();
    });
    document.addEventListener("mousemove", function(e) {
      if (!state.dragging) return;
      state.panX = e.clientX - state.startX; state.panY = e.clientY - state.startY; apply();
    });
    document.addEventListener("mouseup", function() {
      if (state.dragging) { state.dragging = false; container.classList.remove("panning"); }
    });
    container.addEventListener("dblclick", function(e) { e.preventDefault(); reset(); });
    container.addEventListener("touchstart", function(e) {
      if (e.touches.length === 1) {
        state.dragging = true; state.startX = e.touches[0].clientX - state.panX; state.startY = e.touches[0].clientY - state.panY;
        container.classList.add("panning");
      }
    }, { passive: true });
    container.addEventListener("touchmove", function(e) {
      if (!state.dragging || e.touches.length !== 1) return;
      e.preventDefault(); state.panX = e.touches[0].clientX - state.startX; state.panY = e.touches[0].clientY - state.startY; apply();
    }, { passive: false });
    container.addEventListener("touchend", function() { state.dragging = false; container.classList.remove("panning"); });
    return { apply: apply, reset: reset, state: state };
  }

  function renderArchDiagrams() {
    document.querySelectorAll(".arch-diagram").forEach(function(container, idx) {
      var raw = container.getAttribute("data-graph");
      if (!raw) return;
      var data;
      try { data = JSON.parse(raw); } catch(e) {
        container.innerHTML = '<p style="color:var(--text-muted);">Diagram data invalid</p>';
        return;
      }
      var nodes = data.nodes || [];
      var edges = data.edges || [];
      if (nodes.length === 0) return;

      var colors = getArchColors();
      var svgStr = buildArchSvg(nodes, edges, colors, "ah-" + idx);
      container.innerHTML = svgStr;

      // Pan/zoom on the inline diagram.
      var pz = setupArchPanZoom(container);

      // Add controls: zoom in, zoom out, reset, fullscreen.
      var ctrl = document.createElement("div"); ctrl.className = "arch-diagram-controls";
      ctrl.innerHTML = '<button title="Zoom in">+</button><button title="Zoom out">&minus;</button><button title="Reset">&#8634;</button><button title="Full screen">&#x26F6;</button>';
      var btns = ctrl.querySelectorAll("button");
      btns[0].addEventListener("click", function(e) { e.stopPropagation(); if (pz) { pz.state.scale = Math.min(10, pz.state.scale * 1.25); pz.apply(); } });
      btns[1].addEventListener("click", function(e) { e.stopPropagation(); if (pz) { pz.state.scale = Math.max(0.1, pz.state.scale * 0.8); pz.apply(); } });
      btns[2].addEventListener("click", function(e) { e.stopPropagation(); if (pz) pz.reset(); });
      btns[3].addEventListener("click", function(e) {
        e.stopPropagation();
        openArchFullscreen(nodes, edges, idx);
      });
      container.appendChild(ctrl);
    });
  }

  function openArchFullscreen(nodes, edges, idx) {
    // Remove any existing overlay.
    var existing = document.getElementById("arch-fullscreen");
    if (existing) existing.remove();

    var colors = getArchColors();
    var svgStr = buildArchSvg(nodes, edges, colors, "ah-fs-" + idx);

    var overlay = document.createElement("div");
    overlay.id = "arch-fullscreen";
    overlay.className = "arch-fullscreen-overlay";

    var toolbar = document.createElement("div");
    toolbar.className = "arch-fullscreen-toolbar";
    toolbar.innerHTML = '<button title="Zoom in">+</button><button title="Zoom out">&minus;</button><button title="Reset view">&#8634;</button><button title="Close">&#10005; Close</button>';

    var body = document.createElement("div");
    body.className = "arch-fullscreen-body";
    body.innerHTML = svgStr;

    overlay.appendChild(toolbar);
    overlay.appendChild(body);
    document.body.appendChild(overlay);

    // Pan/zoom on fullscreen body.
    var pz = setupArchPanZoom(body);

    var tbtns = toolbar.querySelectorAll("button");
    tbtns[0].addEventListener("click", function() { if (pz) { pz.state.scale = Math.min(10, pz.state.scale * 1.25); pz.apply(); } });
    tbtns[1].addEventListener("click", function() { if (pz) { pz.state.scale = Math.max(0.1, pz.state.scale * 0.8); pz.apply(); } });
    tbtns[2].addEventListener("click", function() { if (pz) pz.reset(); });
    tbtns[3].addEventListener("click", function() { overlay.remove(); });

    // Escape key closes.
    function onKey(e) { if (e.key === "Escape") { overlay.remove(); document.removeEventListener("keydown", onKey); } }
    document.addEventListener("keydown", onKey);
  }

  // Render arch diagrams on load.
  renderArchDiagrams();

  // ===== Mermaid pan/zoom =====
  function setupMermaidPanZoom() {
    document.querySelectorAll(".mermaid").forEach(function(container) {
      var svg = container.querySelector("svg");
      if (!svg || container.getAttribute("data-panzoom") === "true") return;
      container.setAttribute("data-panzoom", "true");

      // Ensure SVG has a viewBox for transform-based zoom/pan.
      if (!svg.getAttribute("viewBox")) {
        var bbox = svg.getBBox();
        var w = parseFloat(svg.getAttribute("width")) || bbox.width + bbox.x;
        var h = parseFloat(svg.getAttribute("height")) || bbox.height + bbox.y;
        svg.setAttribute("viewBox", "0 0 " + w + " " + h);
      }
      svg.setAttribute("width", "100%");
      svg.setAttribute("height", "100%");
      svg.style.maxWidth = "none";

      var state = { scale: 1, panX: 0, panY: 0, dragging: false, startX: 0, startY: 0 };

      function applyTransform() {
        svg.style.transform = "translate(" + state.panX + "px, " + state.panY + "px) scale(" + state.scale + ")";
        svg.style.transformOrigin = "0 0";
      }

      function resetView() {
        state.scale = 1;
        state.panX = 0;
        state.panY = 0;
        applyTransform();
      }

      // Mouse wheel → zoom
      container.addEventListener("wheel", function(e) {
        e.preventDefault();
        var delta = e.deltaY > 0 ? 0.9 : 1.1;
        var newScale = Math.max(0.1, Math.min(10, state.scale * delta));
        // Zoom towards cursor position
        var rect = container.getBoundingClientRect();
        var cx = e.clientX - rect.left;
        var cy = e.clientY - rect.top;
        state.panX = cx - (cx - state.panX) * (newScale / state.scale);
        state.panY = cy - (cy - state.panY) * (newScale / state.scale);
        state.scale = newScale;
        applyTransform();
      }, { passive: false });

      // Mouse drag → pan
      container.addEventListener("mousedown", function(e) {
        if (e.button !== 0) return;
        state.dragging = true;
        state.startX = e.clientX - state.panX;
        state.startY = e.clientY - state.panY;
        container.classList.add("panning");
        e.preventDefault();
      });

      document.addEventListener("mousemove", function(e) {
        if (!state.dragging) return;
        state.panX = e.clientX - state.startX;
        state.panY = e.clientY - state.startY;
        applyTransform();
      });

      document.addEventListener("mouseup", function() {
        if (state.dragging) {
          state.dragging = false;
          container.classList.remove("panning");
        }
      });

      // Double-click → reset
      container.addEventListener("dblclick", function(e) {
        e.preventDefault();
        resetView();
      });

      // Touch support
      container.addEventListener("touchstart", function(e) {
        if (e.touches.length === 1) {
          state.dragging = true;
          state.startX = e.touches[0].clientX - state.panX;
          state.startY = e.touches[0].clientY - state.panY;
          container.classList.add("panning");
        }
      }, { passive: true });

      container.addEventListener("touchmove", function(e) {
        if (!state.dragging || e.touches.length !== 1) return;
        e.preventDefault();
        state.panX = e.touches[0].clientX - state.startX;
        state.panY = e.touches[0].clientY - state.startY;
        applyTransform();
      }, { passive: false });

      container.addEventListener("touchend", function() {
        state.dragging = false;
        container.classList.remove("panning");
      });

      // Add zoom controls
      var controls = document.createElement("div");
      controls.className = "mermaid-controls";
      controls.innerHTML = '<button title="Zoom in">+</button><button title="Zoom out">&minus;</button><button title="Reset">&#8634;</button>';
      var btns = controls.querySelectorAll("button");
      btns[0].addEventListener("click", function(e) {
        e.stopPropagation();
        state.scale = Math.min(10, state.scale * 1.25);
        applyTransform();
      });
      btns[1].addEventListener("click", function(e) {
        e.stopPropagation();
        state.scale = Math.max(0.1, state.scale * 0.8);
        applyTransform();
      });
      btns[2].addEventListener("click", function(e) {
        e.stopPropagation();
        resetView();
      });
      container.appendChild(controls);
    });
  }

  // ===== Mermaid initialization =====
  if (typeof mermaid !== "undefined") {
    var isDark = html.getAttribute("data-theme") === "dark";
    mermaid.initialize({
      startOnLoad: false,
      theme: isDark ? "dark" : "default",
      securityLevel: "loose",
      maxEdges: 2000,
      flowchart: { htmlLabels: true }
    });
    // Convert <pre><code class="language-mermaid"> to rendered mermaid diagrams.
    // Use mermaid.render() with the source text directly as a string to avoid
    // any DOM HTML encoding/decoding issues with arrows (-->) and entities.
    var mermaidBlocks = document.querySelectorAll("pre > code.language-mermaid");
    var renderPromises = [];
    mermaidBlocks.forEach(function(code, idx) {
      var pre = code.parentElement;
      var source = code.textContent;
      var div = document.createElement("div");
      div.className = "mermaid";
      div.setAttribute("data-source", source);
      pre.parentElement.replaceChild(div, pre);
      renderPromises.push(
        mermaid.render("mermaid-diagram-" + idx, source).then(function(result) {
          div.innerHTML = result.svg;
        }).catch(function(err) {
          div.innerHTML = '<pre style="color:red;">Mermaid error: ' + err.message + '</pre>';
        })
      );
    });
    // Set up pan/zoom after all diagrams render.
    Promise.all(renderPromises).then(function() {
      if (typeof setupMermaidPanZoom === "function") {
        setupMermaidPanZoom();
      }
    });
  }
})();
`
