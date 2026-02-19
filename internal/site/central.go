package site

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ziadkadry99/auto-doc/internal/indexer"
)

// RepoInfo holds information about a registered repository for central site generation.
type RepoInfo struct {
	Name          string
	DisplayName   string
	Summary       string
	Status        string
	FileCount     int
	SourceType    string
	Language      string // primary programming language (e.g., "Go", "Python", "Java")
	LastCommitSHA string // git commit SHA when last indexed
	DocsDir       string // path to the repo's .autodoc/docs/ directory
}

// LinkInfo represents a cross-service dependency for site generation.
type LinkInfo struct {
	FromRepo  string
	ToRepo    string
	LinkType  string
	Reason    string
	Endpoints []string
}

// FlowInfo represents a cross-service flow for site generation.
type FlowInfo struct {
	Name        string
	Description string
	Narrative   string
	Diagram     string
	Services    []string
}

// CentralSiteGenerator creates a combined static site from multiple repositories.
type CentralSiteGenerator struct {
	OutputDir   string
	ProjectName string
	Repos       []RepoInfo
	Links       []LinkInfo
	Flows       []FlowInfo
	LogoPath    string
}

// Generate builds the combined multi-repo static site.
// It creates a staging docs directory with generated content and per-repo docs,
// then delegates to the standard SiteGenerator for HTML rendering.
func (g *CentralSiteGenerator) Generate() (int, error) {
	// Clean up service summaries for better readability.
	g.cleanSummaries()

	// Augment LLM-discovered links with direct analysis-based detection.
	g.augmentLinksFromAnalyses()

	// Normalize links and flows before generating.
	g.normalizeData()

	// Synthesize canonical flows from the link topology.
	// This replaces LLM-generated flows with well-structured, non-overlapping journeys.
	g.synthesizeCanonicalFlows()

	// Create staging docs directory.
	stagingDir := filepath.Join(g.OutputDir, ".staging-docs")
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return 0, fmt.Errorf("creating staging dir: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	// 1. Generate landing page.
	if err := g.writeLandingPage(stagingDir); err != nil {
		return 0, fmt.Errorf("writing landing page: %w", err)
	}

	// 2. Copy each repo's docs into a subdirectory.
	for _, repo := range g.Repos {
		if repo.DocsDir == "" {
			continue
		}
		if _, err := os.Stat(repo.DocsDir); os.IsNotExist(err) {
			continue
		}
		destDir := filepath.Join(stagingDir, repo.Name)
		if err := copyDir(repo.DocsDir, destDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not copy docs for %s: %v\n", repo.Name, err)
		}
		// Generate a repo index if the repo docs don't have one.
		indexPath := filepath.Join(destDir, "index.md")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			g.writeRepoIndex(destDir, repo)
		}
	}

	// 3. Generate system overview page.
	if err := g.writeSystemOverview(stagingDir); err != nil {
		return 0, fmt.Errorf("writing system overview: %w", err)
	}

	// 4. Generate flows page.
	if len(g.Flows) > 0 {
		if err := g.writeFlowsPage(stagingDir); err != nil {
			return 0, fmt.Errorf("writing flows page: %w", err)
		}
	}

	// 5. Generate system-level service map (D3.js visualization).
	if err := g.writeServiceMap(stagingDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not generate service map: %v\n", err)
	}

	// 6. Copy HTML artifacts from repos (per-repo interactive maps, etc.).
	for _, repo := range g.Repos {
		if repo.DocsDir == "" {
			continue
		}
		g.copyHTMLArtifacts(repo.DocsDir, stagingDir, repo.Name)
	}

	// 7. Delegate to standard SiteGenerator for HTML rendering.
	siteGen := NewSiteGenerator(stagingDir, g.OutputDir, g.ProjectName)
	siteGen.LogoPath = g.LogoPath
	return siteGen.Generate()
}

// writeLandingPage creates the main index.md with service cards and navigation.
func (g *CentralSiteGenerator) writeLandingPage(stagingDir string) error {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", g.ProjectName))
	b.WriteString("Welcome to the central documentation hub. This site aggregates documentation from all registered services.\n\n")

	// Quick navigation.
	b.WriteString("## Quick Navigation\n\n")
	b.WriteString("- [System Overview](system-overview.md) — Architecture, dependencies, and system-level diagrams\n")
	b.WriteString("- [Service Map](service-map.html) — Interactive D3.js visualization of all services\n")
	if len(g.Flows) > 0 {
		b.WriteString("- [Cross-Service Flows](flows.md) — Data flows across services\n")
	}
	b.WriteString("\n")

	// Service cards table.
	if len(g.Repos) > 0 {
		b.WriteString("## Services\n\n")
		b.WriteString("| Service | Stack | Files | Status | Summary |\n")
		b.WriteString("|---------|-------|-------|--------|---------|\n")
		for _, repo := range g.Repos {
			displayName := repo.DisplayName
			if displayName == "" {
				displayName = repo.Name
			}
			summary := repo.Summary
			if len(summary) > 80 {
				summary = summary[:77] + "..."
			}
			link := fmt.Sprintf("[%s](%s/index.md)", displayName, repo.Name)
			stack := repo.Language
			if stack == "" {
				stack = repo.SourceType
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s |\n",
				link, stack, repo.FileCount, repo.Status, summary))
		}
		b.WriteString("\n")
	}

	// Cross-service dependencies summary.
	if len(g.Links) > 0 {
		b.WriteString("## Dependencies Overview\n\n")
		b.WriteString("| From | To | Type | Reason |\n")
		b.WriteString("|------|----|------|--------|\n")
		for _, link := range g.Links {
			reason := link.Reason
			if len(reason) > 80 {
				reason = reason[:77] + "..."
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				link.FromRepo, link.ToRepo, link.LinkType, reason))
		}
		b.WriteString("\n")
	}

	// Generation metadata.
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("*Generated on %s by [autodoc](https://github.com/ziadkadry99/auto-doc) — %d services, %d files total*\n",
		time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		len(g.Repos),
		func() int {
			total := 0
			for _, r := range g.Repos {
				total += r.FileCount
			}
			return total
		}()))

	return os.WriteFile(filepath.Join(stagingDir, "index.md"), []byte(b.String()), 0o644)
}

// writeRepoIndex creates an index.md for a repo subdirectory.
func (g *CentralSiteGenerator) writeRepoIndex(destDir string, repo RepoInfo) {
	var b strings.Builder

	displayName := repo.DisplayName
	if displayName == "" {
		displayName = repo.Name
	}

	b.WriteString(fmt.Sprintf("# %s\n\n", displayName))
	if repo.Summary != "" {
		b.WriteString(repo.Summary + "\n\n")
	}

	b.WriteString(fmt.Sprintf("- **Status:** %s\n", repo.Status))
	b.WriteString(fmt.Sprintf("- **Files:** %d\n", repo.FileCount))
	b.WriteString(fmt.Sprintf("- **Source:** %s\n", repo.SourceType))
	b.WriteString("\n")

	// List the docs files in this directory.
	entries, err := os.ReadDir(destDir)
	if err == nil && len(entries) > 0 {
		b.WriteString("## Documentation\n\n")
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "index.md" {
				continue
			}
			title := strings.TrimSuffix(e.Name(), ".md")
			b.WriteString(fmt.Sprintf("- [%s](%s)\n", title, e.Name()))
		}
		b.WriteString("\n")
	}

	_ = os.WriteFile(filepath.Join(destDir, "index.md"), []byte(b.String()), 0o644)
}

// writeSystemOverview creates the system-overview.md page.
func (g *CentralSiteGenerator) writeSystemOverview(stagingDir string) error {
	var b strings.Builder

	b.WriteString("# System Overview\n\n")

	// Services table.
	if len(g.Repos) > 0 {
		b.WriteString("## Registered Services\n\n")
		b.WriteString("| Service | Stack | Files | Status | Commit | Summary |\n")
		b.WriteString("|---------|-------|-------|--------|--------|---------|\n")
		for _, repo := range g.Repos {
			displayName := repo.DisplayName
			if displayName == "" {
				displayName = repo.Name
			}
			summary := repo.Summary
			if len(summary) > 100 {
				summary = summary[:97] + "..."
			}
			stack := repo.Language
			if stack == "" {
				stack = repo.SourceType
			}
			commitDisplay := ""
			if len(repo.LastCommitSHA) >= 7 {
				commitDisplay = "`" + repo.LastCommitSHA[:7] + "`"
			}
			b.WriteString(fmt.Sprintf("| **%s** | %s | %d | %s | %s | %s |\n",
				displayName, stack, repo.FileCount, repo.Status, commitDisplay, summary))
		}
		b.WriteString("\n")
	}

	// System architecture diagram.
	if len(g.Repos) > 1 {
		b.WriteString("## Architecture Diagram\n\n")
		b.WriteString("```mermaid\ngraph TD\n")
		// Build set of known repo names.
		repoSet := make(map[string]bool)
		for _, repo := range g.Repos {
			repoSet[repo.Name] = true
		}
		// Define service nodes.
		for _, repo := range g.Repos {
			displayName := repo.DisplayName
			if displayName == "" {
				displayName = repo.Name
			}
			nodeID := strings.ReplaceAll(repo.Name, "-", "_")
			b.WriteString(fmt.Sprintf("    %s[\"%s<br/>%d files\"]\n", nodeID, displayName, repo.FileCount))
		}
		// Collect and define external dependency nodes.
		externalNodes := make(map[string]bool)
		if len(g.Links) > 0 {
			for _, link := range g.Links {
				if !repoSet[link.ToRepo] {
					externalNodes[link.ToRepo] = true
				}
				if !repoSet[link.FromRepo] {
					externalNodes[link.FromRepo] = true
				}
			}
			for extName := range externalNodes {
				nodeID := strings.ReplaceAll(extName, "-", "_")
				b.WriteString(fmt.Sprintf("    %s[(\"%s\")]\n", nodeID, extName))
			}
		}
		// Define links between services.
		if len(g.Links) > 0 {
			for _, link := range g.Links {
				fromID := strings.ReplaceAll(link.FromRepo, "-", "_")
				toID := strings.ReplaceAll(link.ToRepo, "-", "_")
				label := link.LinkType
				if label == "" {
					label = "depends"
				}
				b.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", fromID, label, toID))
			}
		}
		// Style the nodes.
		b.WriteString("\n    classDef svc fill:#1f6feb,stroke:#58a6ff,color:#fff,stroke-width:2px\n")
		b.WriteString("    classDef ext fill:#30363d,stroke:#8b949e,color:#e6edf3,stroke-width:1px,stroke-dasharray:5\n")
		for _, repo := range g.Repos {
			nodeID := strings.ReplaceAll(repo.Name, "-", "_")
			b.WriteString(fmt.Sprintf("    class %s svc\n", nodeID))
		}
		for extName := range externalNodes {
			nodeID := strings.ReplaceAll(extName, "-", "_")
			b.WriteString(fmt.Sprintf("    class %s ext\n", nodeID))
		}
		b.WriteString("```\n\n")
	}

	// Dependencies table.
	if len(g.Links) > 0 {
		b.WriteString("## Cross-Service Dependencies\n\n")
		b.WriteString("| From | To | Type | Reason |\n")
		b.WriteString("|------|----|------|--------|\n")
		for _, link := range g.Links {
			reason := link.Reason
			if len(reason) > 100 {
				reason = reason[:97] + "..."
			}
			endpoints := ""
			if len(link.Endpoints) > 0 {
				endpoints = " (" + strings.Join(link.Endpoints, ", ") + ")"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s%s |\n",
				link.FromRepo, link.ToRepo, link.LinkType, reason, endpoints))
		}
		b.WriteString("\n")
	}

	// Flows summary.
	if len(g.Flows) > 0 {
		b.WriteString("## Cross-Service Flows\n\n")
		for _, f := range g.Flows {
			b.WriteString(fmt.Sprintf("### %s\n\n", f.Name))
			if f.Description != "" {
				b.WriteString(f.Description + "\n\n")
			}
			if len(f.Services) > 0 {
				b.WriteString("**Services:** " + strings.Join(f.Services, ", ") + "\n\n")
			}
			if f.Diagram != "" {
				b.WriteString("```mermaid\n")
				b.WriteString(f.Diagram)
				b.WriteString("\n```\n\n")
			}
		}
	}

	// Architectural Patterns Analysis.
	g.writeArchitecturalPatterns(&b)

	// Interactive views.
	b.WriteString("## Interactive Views\n\n")
	b.WriteString("- [Service Map](service-map.html) — Interactive D3.js visualization of all services and their connections\n")
	if len(g.Flows) > 0 {
		b.WriteString("- [Cross-Service Flows](flows.md) — Detailed flow narratives\n")
	}
	b.WriteString("\n")

	return os.WriteFile(filepath.Join(stagingDir, "system-overview.md"), []byte(b.String()), 0o644)
}

// writeFlowsPage creates the flows.md page with all cross-service flow narratives.
func (g *CentralSiteGenerator) writeFlowsPage(stagingDir string) error {
	var b strings.Builder

	b.WriteString("# Cross-Service Flows\n\n")
	b.WriteString("This page describes the data flows that span multiple services in the system.\n\n")

	for _, f := range g.Flows {
		b.WriteString(fmt.Sprintf("## %s\n\n", f.Name))
		// Prefer Narrative over Description; avoid duplicating if they're identical.
		if f.Narrative != "" {
			b.WriteString(f.Narrative + "\n\n")
		} else if f.Description != "" {
			b.WriteString(f.Description + "\n\n")
		}
		if len(f.Services) > 0 {
			b.WriteString("**Services involved:** " + strings.Join(f.Services, ", ") + "\n\n")
		}
		if f.Diagram != "" {
			b.WriteString("```mermaid\n")
			b.WriteString(f.Diagram)
			b.WriteString("\n```\n\n")
		}
		b.WriteString("---\n\n")
	}

	return os.WriteFile(filepath.Join(stagingDir, "flows.md"), []byte(b.String()), 0o644)
}

// serviceMapNode is a node in the service map D3.js visualization.
type serviceMapNode struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	FileCount int    `json:"fileCount"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	DocLink   string `json:"docLink"`
}

// serviceMapEdge is an edge in the service map.
type serviceMapEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	LinkType string `json:"linkType"`
	Reason   string `json:"reason"`
}

// serviceMapData is the data passed to the D3.js service map template.
type serviceMapData struct {
	ProjectName string           `json:"projectName"`
	Nodes       []serviceMapNode `json:"nodes"`
	Edges       []serviceMapEdge `json:"edges"`
}

// writeServiceMap generates a standalone D3.js service-map.html for the central site.
func (g *CentralSiteGenerator) writeServiceMap(stagingDir string) error {
	nodes := make([]serviceMapNode, len(g.Repos))
	for i, r := range g.Repos {
		displayName := r.DisplayName
		if displayName == "" {
			displayName = r.Name
		}
		nodes[i] = serviceMapNode{
			ID:        r.Name,
			Label:     displayName,
			FileCount: r.FileCount,
			Status:    r.Status,
			Summary:   r.Summary,
			DocLink:   r.Name + "/index.html",
		}
	}

	edges := make([]serviceMapEdge, len(g.Links))
	for i, l := range g.Links {
		edges[i] = serviceMapEdge{
			Source:   l.FromRepo,
			Target:   l.ToRepo,
			LinkType: l.LinkType,
			Reason:   l.Reason,
		}
	}

	// Add external dependency nodes (e.g., RabbitMQ, SMTP) that appear
	// in links but are not registered repos.
	nodeSet := make(map[string]bool)
	for _, n := range nodes {
		nodeSet[n.ID] = true
	}
	for _, e := range edges {
		for _, target := range []string{e.Source, e.Target} {
			if !nodeSet[target] {
				nodeSet[target] = true
				nodes = append(nodes, serviceMapNode{
					ID:        target,
					Label:     target,
					FileCount: 0,
					Status:    "external",
					Summary:   "External dependency",
					DocLink:   "#",
				})
			}
		}
	}

	data := serviceMapData{
		ProjectName: g.ProjectName,
		Nodes:       nodes,
		Edges:       edges,
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshalling service map data: %w", err)
	}

	html := serviceMapHTML(string(dataJSON))
	return os.WriteFile(filepath.Join(stagingDir, "service-map.html"), []byte(html), 0o644)
}

// serviceMapHTML returns the complete HTML for the system-level service map.
func serviceMapHTML(dataJSON string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Service Map</title>
<style>
:root{--bg:#0d1117;--bg2:#161b22;--bg3:#21262d;--tx:#e6edf3;--tx2:#8b949e;--bd:#30363d;--ac:#58a6ff;--hover:#1f6feb}
body.light{--bg:#fff;--bg2:#f6f8fa;--bg3:#eaeef2;--tx:#1f2328;--tx2:#656d76;--bd:#d0d7de;--ac:#0969da;--hover:#0969da}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;background:var(--bg);color:var(--tx);overflow:hidden;height:100vh}
#toolbar{display:flex;align-items:center;justify-content:space-between;height:48px;padding:0 16px;background:var(--bg2);border-bottom:1px solid var(--bd);gap:12px;z-index:10;position:relative}
.toolbar-section{display:flex;align-items:center;gap:8px}
.back-link{color:var(--ac);text-decoration:none;font-size:14px;white-space:nowrap}
.back-link:hover{text-decoration:underline}
.title{font-size:15px;font-weight:600;white-space:nowrap}
#stats{font-size:12px;color:var(--tx2);white-space:nowrap}
.btn{background:var(--bg3);border:1px solid var(--bd);color:var(--tx);padding:4px 10px;border-radius:6px;font-size:12px;cursor:pointer}
.btn:hover{background:var(--bd)}
#graph-container{width:100%;height:calc(100vh - 48px);position:relative}
svg{width:100%;height:100%}
.node-label{fill:var(--tx);font-size:12px;text-anchor:middle;pointer-events:none;font-weight:600}
.edge{stroke:var(--bd);stroke-opacity:0.6;fill:none}
.edge-label{fill:var(--tx2);font-size:10px;text-anchor:middle;pointer-events:none}
#tooltip{position:fixed;background:var(--bg2);border:1px solid var(--bd);border-radius:8px;padding:12px;font-size:13px;max-width:320px;pointer-events:none;z-index:100;box-shadow:0 4px 12px rgba(0,0,0,0.3)}
#tooltip.hidden{display:none}
#tooltip h3{margin:0 0 6px;font-size:14px;color:var(--ac)}
#tooltip p{margin:2px 0;color:var(--tx2);line-height:1.4}
#tooltip .badge{display:inline-block;background:var(--bg3);padding:1px 6px;border-radius:4px;font-size:11px;margin-right:4px}
#info-panel{position:fixed;right:0;top:48px;width:320px;height:calc(100vh - 48px);background:var(--bg2);border-left:1px solid var(--bd);padding:16px;overflow-y:auto;z-index:20;transition:transform 0.2s}
#info-panel.hidden{transform:translateX(100%)}
#info-close{position:absolute;top:8px;right:8px;background:none;border:none;color:var(--tx2);font-size:20px;cursor:pointer}
#info-content h3{font-size:16px;margin:0 0 8px}
#info-content p{font-size:13px;color:var(--tx2);line-height:1.5;margin:4px 0}
#info-content a{color:var(--ac);text-decoration:none}
#info-content a:hover{text-decoration:underline}
.info-stat{display:flex;justify-content:space-between;padding:4px 0;border-bottom:1px solid var(--bd);font-size:13px}
.info-stat .label{color:var(--tx2)}
</style>
</head>
<body>
<div id="toolbar">
 <div class="toolbar-section">
  <a href="index.html" class="back-link">← Back</a>
  <span class="title">System Service Map</span>
 </div>
 <div class="toolbar-section">
  <span id="stats"></span>
  <button class="btn" id="theme-btn">☀️ Light</button>
 </div>
</div>
<div id="graph-container"><svg id="graph"></svg></div>
<div id="tooltip" class="hidden"></div>
<div id="info-panel" class="hidden"><button id="info-close">&times;</button><div id="info-content"></div></div>
<script src="https://d3js.org/d3.v7.min.js"></script>
<script>
(function(){
var data = ` + dataJSON + `;
if(!data||typeof d3==='undefined'){document.getElementById('graph-container').innerHTML='<div style="padding:40px;color:var(--tx2)">Could not load visualization.</div>';return;}

var serviceColors = ['#4e79a7','#f28e2b','#e15759','#76b7b2','#59a14f','#edc948','#b07aa1','#ff9da7','#9c755f','#bab0ac'];
var colorMap = {};
data.nodes.forEach(function(n, i){ colorMap[n.id] = serviceColors[i % serviceColors.length]; });

var selectedId = null;
var svgEl = document.getElementById('graph');
var width = svgEl.clientWidth, height = svgEl.clientHeight;
var svg = d3.select(svgEl);
var container = svg.append('g');

var zoom = d3.zoom().scaleExtent([0.1, 8]).on('zoom', function(e){ container.attr('transform', e.transform); });
svg.call(zoom);

// Arrow markers
var defs = svg.append('defs');
data.nodes.forEach(function(n, i){
  defs.append('marker').attr('id','arr-'+n.id.replace(/[^a-zA-Z0-9]/g,'_')).attr('viewBox','0 -4 8 8').attr('refX',28).attr('refY',0)
    .attr('markerWidth',6).attr('markerHeight',6).attr('orient','auto')
    .append('path').attr('d','M0,-3L6,0L0,3').attr('fill', serviceColors[i % serviceColors.length]).attr('opacity',0.8);
});

// Node size based on file count
var maxFiles = d3.max(data.nodes, function(d){ return d.fileCount; }) || 1;
var sizeScale = d3.scaleSqrt().domain([1, maxFiles]).range([20, 40]);

// Initial positions in a circle
data.nodes.forEach(function(d, i){
  var angle = (i / data.nodes.length) * 2 * Math.PI;
  var radius = Math.min(width, height) * 0.25;
  d.x = width/2 + radius * Math.cos(angle);
  d.y = height/2 + radius * Math.sin(angle);
});

// Force simulation
var sim = d3.forceSimulation(data.nodes)
  .force('link', d3.forceLink(data.edges).id(function(d){ return d.id; }).distance(180).strength(0.5))
  .force('charge', d3.forceManyBody().strength(-600))
  .force('center', d3.forceCenter(width/2, height/2))
  .force('collision', d3.forceCollide().radius(function(d){ return sizeScale(d.fileCount) + 10; }))
  .alphaDecay(0.02);

// Draw edges
var edgeG = container.append('g');
var edgeEls = edgeG.selectAll('path').data(data.edges).join('path')
  .attr('class','edge')
  .attr('stroke-width', 2)
  .attr('marker-end', function(d){
    var src = typeof d.source === 'object' ? d.source : {id: d.source};
    return 'url(#arr-'+src.id.replace(/[^a-zA-Z0-9]/g,'_')+')';
  });

// Edge labels
var edgeLabelG = container.append('g');
var edgeLabelEls = edgeLabelG.selectAll('text').data(data.edges).join('text')
  .attr('class','edge-label')
  .text(function(d){ return d.linkType || ''; });

// Draw nodes
var nodeG = container.append('g');
var nodeEls = nodeG.selectAll('rect').data(data.nodes).join('rect')
  .attr('rx', 8).attr('ry', 8)
  .attr('width', function(d){ return sizeScale(d.fileCount) * 2; })
  .attr('height', function(d){ return sizeScale(d.fileCount) * 1.2; })
  .attr('fill', function(d){ return colorMap[d.id]; })
  .attr('stroke', function(d){ return d3.color(colorMap[d.id]).darker(0.5).toString(); })
  .attr('stroke-width', 2)
  .attr('cursor','pointer')
  .on('mouseover', onHover).on('mousemove', moveTooltip).on('mouseout', onHoverOut).on('click', onClick)
  .call(d3.drag()
    .on('start', function(e,d){ if(!e.active) sim.alphaTarget(0.3).restart(); d.fx=d.x; d.fy=d.y; })
    .on('drag', function(e,d){ d.fx=e.x; d.fy=e.y; })
    .on('end', function(e,d){ if(!e.active) sim.alphaTarget(0); d.fx=null; d.fy=null; }));

// Node labels
var labelG = container.append('g');
var labelEls = labelG.selectAll('text').data(data.nodes).join('text')
  .attr('class','node-label')
  .text(function(d){ return d.label; })
  .attr('dy', 4);

sim.on('tick', function(){
  edgeEls.attr('d', function(d){
    return 'M'+d.source.x+','+d.source.y+'L'+d.target.x+','+d.target.y;
  });
  edgeLabelEls
    .attr('x', function(d){ return (d.source.x + d.target.x) / 2; })
    .attr('y', function(d){ return (d.source.y + d.target.y) / 2 - 6; });
  nodeEls
    .attr('x', function(d){ return d.x - sizeScale(d.fileCount); })
    .attr('y', function(d){ return d.y - sizeScale(d.fileCount) * 0.6; });
  labelEls.attr('x', function(d){ return d.x; }).attr('y', function(d){ return d.y; });
});

// Stats
document.getElementById('stats').textContent = data.nodes.length + ' services, ' + data.edges.length + ' connections';

// Tooltip
var tooltip = document.getElementById('tooltip');
function onHover(e, d){
  var html = '<h3>' + d.label + '</h3>';
  html += '<p><span class="badge">' + d.status + '</span> <span class="badge">' + d.fileCount + ' files</span></p>';
  if(d.summary) html += '<p>' + d.summary + '</p>';
  tooltip.innerHTML = html;
  tooltip.classList.remove('hidden');
}
function moveTooltip(e){
  tooltip.style.left = (e.clientX + 12) + 'px';
  tooltip.style.top = (e.clientY - 10) + 'px';
}
function onHoverOut(){ tooltip.classList.add('hidden'); }

// Click => info panel
var infoPanel = document.getElementById('info-panel');
var infoContent = document.getElementById('info-content');
document.getElementById('info-close').onclick = function(){ infoPanel.classList.add('hidden'); selectedId = null; };

function onClick(e, d){
  selectedId = d.id;
  var html = '<h3>' + d.label + '</h3>';
  html += '<div class="info-stat"><span class="label">Status</span><span>' + d.status + '</span></div>';
  html += '<div class="info-stat"><span class="label">Files</span><span>' + d.fileCount + '</span></div>';
  if(d.summary) html += '<p style="margin-top:8px">' + d.summary + '</p>';
  // Show connections
  var incoming = data.edges.filter(function(e){ var t = typeof e.target === 'object' ? e.target.id : e.target; return t === d.id; });
  var outgoing = data.edges.filter(function(e){ var s = typeof e.source === 'object' ? e.source.id : e.source; return s === d.id; });
  if(outgoing.length > 0){
    html += '<h4 style="margin-top:12px;font-size:13px">Calls →</h4>';
    outgoing.forEach(function(e){ var t = typeof e.target === 'object' ? e.target.id : e.target; html += '<div class="info-stat"><span>' + t + '</span><span class="badge">' + (e.linkType||'') + '</span></div>'; });
  }
  if(incoming.length > 0){
    html += '<h4 style="margin-top:12px;font-size:13px">← Called by</h4>';
    incoming.forEach(function(e){ var s = typeof e.source === 'object' ? e.source.id : e.source; html += '<div class="info-stat"><span>' + s + '</span><span class="badge">' + (e.linkType||'') + '</span></div>'; });
  }
  html += '<p style="margin-top:12px"><a href="' + d.docLink + '">View Documentation →</a></p>';
  infoContent.innerHTML = html;
  infoPanel.classList.remove('hidden');
}

// Theme toggle
var themeBtn = document.getElementById('theme-btn');
var isLight = false;
themeBtn.onclick = function(){
  isLight = !isLight;
  document.body.classList.toggle('light', isLight);
  themeBtn.textContent = isLight ? '🌙 Dark' : '☀️ Light';
};

// Responsive
window.addEventListener('resize', function(){
  width = svgEl.clientWidth; height = svgEl.clientHeight;
  sim.force('center', d3.forceCenter(width/2, height/2)).alpha(0.3).restart();
});
})();
</script>
</body>
</html>`
}

// copyHTMLArtifacts copies standalone HTML files from a repo's docs to its subdirectory.
// System-level files (service-map.html, interactive-map.html) are NOT copied to the root
// because the central generator creates its own system-level versions.
func (g *CentralSiteGenerator) copyHTMLArtifacts(srcDocsDir, stagingDir, repoName string) {
	_ = filepath.Walk(srcDocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		rel, err := filepath.Rel(srcDocsDir, path)
		if err != nil {
			return nil
		}
		// All HTML files go into the repo's subdirectory.
		destPath := filepath.Join(stagingDir, repoName, rel)

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		_ = os.WriteFile(destPath, data, 0o644)
		return nil
	})
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}

		return copyFile(path, destPath)
	})
}

// cleanSummaries rewrites service summaries to remove file-centric language
// like "The file defines..." or "To provide..." that reads awkwardly in documentation.
func (g *CentralSiteGenerator) cleanSummaries() {
	replacements := []struct {
		prefix      string
		replacement string
	}{
		{"The file defines ", "Defines "},
		{"This file defines ", "Defines "},
		{"The file implements ", "Implements "},
		{"This file implements ", "Implements "},
		{"To provide ", "Provides "},
		{"To implement ", "Implements "},
		{"The purpose of this service is to serve ", "Serves "},
		{"The purpose of this service is to provide ", "Provides "},
		{"The purpose of this service is to ", ""},
	}

	for i, r := range g.Repos {
		for _, rep := range replacements {
			if strings.HasPrefix(r.Summary, rep.prefix) {
				cleaned := rep.replacement + r.Summary[len(rep.prefix):]
				// Capitalize first letter.
				if len(cleaned) > 0 && cleaned[0] >= 'a' && cleaned[0] <= 'z' {
					cleaned = strings.ToUpper(cleaned[:1]) + cleaned[1:]
				}
				g.Repos[i].Summary = cleaned
				break
			}
		}
	}
}

// augmentLinksFromAnalyses extracts gRPC/API dependencies directly from each repo's
// analyses.json and creates links to matching registered repos. This provides
// reliable link detection independent of LLM quality.
func (g *CentralSiteGenerator) augmentLinksFromAnalyses() {
	// Build a lookup from various name forms to repo names.
	repoLookup := make(map[string]string) // lowercase name → repo name
	for _, r := range g.Repos {
		repoLookup[strings.ToLower(r.Name)] = r.Name
	}

	// Proto-generated file patterns to skip.
	protoPatterns := []string{
		"_pb2.py", "_pb2_grpc.py", ".pb.go", "_grpc.pb.go",
		"grpc.java", "_grpc.js", "_grpc.ts",
	}
	isProto := func(path string) bool {
		lower := strings.ToLower(path)
		for _, p := range protoPatterns {
			if strings.HasSuffix(lower, p) {
				return true
			}
		}
		return false
	}

	// Try to match a dependency name to a registered repo.
	matchDep := func(depName string) string {
		lower := strings.ToLower(depName)
		// Direct match.
		if name, ok := repoLookup[lower]; ok {
			return name
		}
		// Try with "service" suffix removed/added.
		for _, suffix := range []string{"service", "grpc", "client"} {
			trimmed := strings.TrimSuffix(lower, suffix)
			if trimmed != lower {
				if name, ok := repoLookup[trimmed+"service"]; ok {
					return name
				}
				if name, ok := repoLookup[trimmed]; ok {
					return name
				}
			}
		}
		// Substring match: if dep name contains a repo name.
		for repoLower, repoName := range repoLookup {
			if strings.Contains(lower, repoLower) {
				return repoName
			}
		}
		return ""
	}

	// Existing link set (to avoid duplicates).
	existingLinks := make(map[string]bool)
	for _, l := range g.Links {
		key := strings.ToLower(l.FromRepo) + "->" + strings.ToLower(l.ToRepo)
		existingLinks[key] = true
	}

	// For each repo, load analyses and extract gRPC/API deps.
	for _, repo := range g.Repos {
		if repo.DocsDir == "" {
			continue
		}
		// DocsDir is like /path/to/repo/.autodoc/docs — analyses.json is in .autodoc/
		autodocDir := filepath.Dir(repo.DocsDir)
		analyses, err := indexer.LoadAnalyses(filepath.Dir(autodocDir))
		if err != nil {
			continue
		}

		for filePath, analysis := range analyses {
			if isProto(filePath) {
				continue
			}
			for _, dep := range analysis.Dependencies {
				if dep.Type != "grpc" && dep.Type != "api_call" {
					continue
				}
				target := matchDep(dep.Name)
				if target == "" || target == repo.Name {
					continue // no match or self-reference
				}
				key := strings.ToLower(repo.Name) + "->" + strings.ToLower(target)
				if existingLinks[key] {
					continue
				}
				existingLinks[key] = true
				g.Links = append(g.Links, LinkInfo{
					FromRepo: repo.Name,
					ToRepo:   target,
					LinkType: dep.Type,
					Reason:   fmt.Sprintf("%s calls %s via %s", repo.Name, target, dep.Type),
				})
			}
		}
	}
}

// normalizeData cleans up links and flows before site generation:
//   - Normalizes link FromRepo/ToRepo to match registered repo names (case-insensitive)
//   - Removes self-loops (FromRepo == ToRepo)
//   - Deduplicates links (same from/to pair)
//   - Deduplicates flows by name
func (g *CentralSiteGenerator) normalizeData() {
	// Build case-insensitive lookup from registered repo names.
	repoLookup := make(map[string]string) // lowercase -> actual name
	for _, r := range g.Repos {
		repoLookup[strings.ToLower(r.Name)] = r.Name
	}

	// Helper: try to match a link endpoint to a registered repo.
	// Checks exact match, then case-insensitive, then with common suffixes stripped.
	matchRepo := func(name string) string {
		// Exact match.
		if _, ok := repoLookup[strings.ToLower(name)]; ok {
			return repoLookup[strings.ToLower(name)]
		}
		// Try stripping common suffixes like "Service", "Grpc", "Client".
		lower := strings.ToLower(name)
		for _, suffix := range []string{"service", "grpc", "client"} {
			trimmed := strings.TrimSuffix(lower, suffix)
			if trimmed != lower {
				// Check if trimmed + "service" matches a repo.
				if actual, ok := repoLookup[trimmed+"service"]; ok {
					return actual
				}
				// Check if trimmed alone matches.
				if actual, ok := repoLookup[trimmed]; ok {
					return actual
				}
			}
		}
		return name // no match, keep as-is
	}

	// Normalize links.
	seen := make(map[string]bool)
	var cleanLinks []LinkInfo
	for _, link := range g.Links {
		link.FromRepo = matchRepo(link.FromRepo)
		link.ToRepo = matchRepo(link.ToRepo)

		// Skip self-loops.
		if link.FromRepo == link.ToRepo {
			continue
		}

		// Skip links where source or target doesn't match any registered repo.
		if _, ok := repoLookup[strings.ToLower(link.FromRepo)]; !ok {
			continue
		}
		if _, ok := repoLookup[strings.ToLower(link.ToRepo)]; !ok {
			continue
		}

		// Clean up reason strings: remove implementation artifacts.
		if idx := strings.Index(link.Reason, " (detected from"); idx > 0 {
			link.Reason = link.Reason[:idx]
		}

		// Deduplicate by from+to pair.
		key := link.FromRepo + "->" + link.ToRepo
		if seen[key] {
			continue
		}
		seen[key] = true
		cleanLinks = append(cleanLinks, link)
	}

	g.Links = cleanLinks

	// Deduplicate flows using concept-based grouping.
	// Many LLM-generated flows describe the same concept with different names
	// (e.g., "Checkout", "Place Order", "Order Placement", "Checkout Process").
	// Group by canonical concept, then pick the best representative from each group.
	conceptGroups := make(map[string][]FlowInfo)
	for _, f := range g.Flows {
		concept := canonicalFlowConcept(f.Name)
		conceptGroups[concept] = append(conceptGroups[concept], f)
	}

	var cleanFlows []FlowInfo
	for _, group := range conceptGroups {
		// Pick the best flow from the group: most services, then longest narrative.
		best := group[0]
		for _, f := range group[1:] {
			if len(f.Services) > len(best.Services) ||
				(len(f.Services) == len(best.Services) && len(f.Narrative) > len(best.Narrative)) {
				best = f
			}
		}
		// Merge services from all flows in the group.
		svcSet := make(map[string]bool)
		for _, f := range group {
			for _, s := range f.Services {
				svcSet[strings.ToLower(s)] = true
			}
		}
		var mergedServices []string
		for s := range svcSet {
			mergedServices = append(mergedServices, s)
		}
		best.Services = mergedServices
		cleanFlows = append(cleanFlows, best)
	}

	// Consolidate trivial flows (≤2 services) into a composite flow.
	// Single-arrow flows like "Ad Display" or "Currency Conversion" are more useful
	// when presented as a group of frontend interactions.
	var substantialFlows []FlowInfo
	var trivialFlows []FlowInfo
	for _, f := range cleanFlows {
		if len(f.Services) <= 2 {
			trivialFlows = append(trivialFlows, f)
		} else {
			substantialFlows = append(substantialFlows, f)
		}
	}

	if len(trivialFlows) >= 2 {
		// Merge trivial flows into a composite.
		svcSet := make(map[string]bool)
		var narrativeParts []string
		for _, f := range trivialFlows {
			for _, s := range f.Services {
				svcSet[strings.ToLower(s)] = true
			}
			desc := f.Name
			if f.Narrative != "" {
				desc = f.Narrative
			} else if f.Description != "" {
				desc = f.Description
			}
			narrativeParts = append(narrativeParts, "- **"+f.Name+"**: "+desc)
		}
		var svcList []string
		for s := range svcSet {
			svcList = append(svcList, s)
		}
		composite := FlowInfo{
			Name:      "Service Interactions",
			Narrative: "Individual service-to-service interactions that support the overall system:\n\n" + strings.Join(narrativeParts, "\n"),
			Services:  svcList,
		}
		substantialFlows = append(substantialFlows, composite)
	} else {
		substantialFlows = append(substantialFlows, trivialFlows...)
	}
	cleanFlows = substantialFlows

	// Sort flows: most services first (broader flows are more important).
	sort.Slice(cleanFlows, func(i, j int) bool {
		if len(cleanFlows[i].Services) != len(cleanFlows[j].Services) {
			return len(cleanFlows[i].Services) > len(cleanFlows[j].Services)
		}
		return cleanFlows[i].Name < cleanFlows[j].Name
	})

	// Generate sequence diagrams for flows that don't already have one.
	for i, f := range cleanFlows {
		if f.Diagram == "" {
			cleanFlows[i].Diagram = g.generateSequenceDiagram(f)
		}
	}

	g.Flows = cleanFlows
}

// canonicalFlowConcept maps a flow name to a canonical concept for deduplication.
// "Checkout", "Place Order", "Order Placement", "Checkout Process" -> "checkout"
// "Product Browsing", "Product Browsing and Purchase" -> "browsing"
func canonicalFlowConcept(name string) string {
	lower := strings.ToLower(name)

	// Define concept keywords. Order matters — first match wins.
	// "purchase" and "order" grouped with checkout since they describe the same end-to-end flow.
	concepts := []struct {
		concept  string
		keywords []string
	}{
		{"checkout", []string{"checkout", "place order", "order placement", "order processing", "purchase", "order confirm"}},
		{"browsing", []string{"browsing", "browse", "product catalog", "product listing"}},
		{"recommendation", []string{"recommend"}},
		{"cart", []string{"cart", "add item", "add to cart"}},
		{"payment", []string{"payment", "pay ", "charge"}},
		{"shipping", []string{"shipping", "ship ", "delivery"}},
		{"currency", []string{"currency", "conversion"}},
		{"email", []string{"email", "notification"}},
		{"ad", []string{" ad ", "advert", "ad display"}},
	}

	for _, c := range concepts {
		for _, kw := range c.keywords {
			if strings.Contains(lower, kw) {
				return c.concept
			}
		}
	}

	// Fallback: use the name itself.
	return lower
}

// operationLabel derives a meaningful operation label for a sequence diagram arrow
// from a link's reason, endpoints, and target service name.
func operationLabel(link LinkInfo) string {
	// Use the first endpoint if available.
	if len(link.Endpoints) > 0 && link.Endpoints[0] != "" {
		return link.Endpoints[0]
	}

	// Try to extract an operation from the reason.
	reason := link.Reason
	if reason != "" {
		// Remove "(detected from ...)" artifacts.
		if idx := strings.Index(reason, " (detected from"); idx > 0 {
			reason = reason[:idx]
		}
		// If the reason starts with "Calls X to ...", extract the action.
		if strings.HasPrefix(reason, "Calls ") {
			if toIdx := strings.Index(reason, " to "); toIdx > 0 {
				action := reason[toIdx+4:]
				if len(action) > 40 {
					action = action[:37] + "..."
				}
				return action
			}
		}
	}

	// Derive a label from the target service name.
	// Use well-known gRPC operation names for common microservice patterns.
	target := strings.TrimSuffix(strings.ToLower(link.ToRepo), "service")
	knownOps := map[string]string{
		"productcatalog": "ListProducts()",
		"cart":           "GetCart() / EmptyCart()",
		"currency":       "Convert()",
		"shipping":       "GetQuote() / ShipOrder()",
		"checkout":       "PlaceOrder()",
		"payment":        "Charge()",
		"email":          "SendConfirmation()",
		"ad":             "GetAds()",
		"recommendation": "ListRecommendations()",
	}
	if op, ok := knownOps[target]; ok {
		return op
	}

	return link.LinkType
}

// generateSequenceDiagram creates a Mermaid sequence diagram for a flow
// based on its services and the known cross-service links.
func (g *CentralSiteGenerator) generateSequenceDiagram(flow FlowInfo) string {
	if len(flow.Services) < 2 {
		return ""
	}

	// Build set of services in this flow.
	flowSvcs := make(map[string]bool)
	for _, s := range flow.Services {
		flowSvcs[strings.ToLower(s)] = true
	}

	// Collect links relevant to this flow (both endpoints in the flow's service set).
	type edge struct {
		from, to, label string
	}
	var edges []edge
	for _, link := range g.Links {
		fromLower := strings.ToLower(link.FromRepo)
		toLower := strings.ToLower(link.ToRepo)
		if flowSvcs[fromLower] && flowSvcs[toLower] {
			edges = append(edges, edge{link.FromRepo, link.ToRepo, operationLabel(link)})
		}
	}

	if len(edges) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("sequenceDiagram\n")

	// Define participants in a logical order (try to put the initiator first).
	// Count outgoing edges to find the initiator.
	outCount := make(map[string]int)
	for _, e := range edges {
		outCount[e.from]++
	}
	// Sort services: most outgoing first (likely initiator).
	type svcCount struct {
		name  string
		count int
	}
	var sorted []svcCount
	for _, s := range flow.Services {
		sorted = append(sorted, svcCount{s, outCount[s]})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for _, s := range sorted {
		b.WriteString(fmt.Sprintf("    participant %s\n", s.name))
	}

	// Write edges as sequence arrows.
	for _, e := range edges {
		b.WriteString(fmt.Sprintf("    %s->>%s: %s\n", e.from, e.to, e.label))
	}

	return b.String()
}

// synthesizeCanonicalFlows generates distinct, non-overlapping user journey flows
// directly from the link topology instead of relying on LLM-generated flows.
// Each flow represents a specific user journey through the system.
func (g *CentralSiteGenerator) synthesizeCanonicalFlows() {
	if len(g.Links) == 0 {
		return
	}

	// Build adjacency: from -> list of (to, link)
	type target struct {
		to   string
		link LinkInfo
	}
	adj := make(map[string][]target)
	for _, link := range g.Links {
		adj[strings.ToLower(link.FromRepo)] = append(adj[strings.ToLower(link.FromRepo)], target{
			to:   strings.ToLower(link.ToRepo),
			link: link,
		})
	}

	// Build lookup for repo display names.
	nameMap := make(map[string]string)
	for _, r := range g.Repos {
		nameMap[strings.ToLower(r.Name)] = r.Name
	}
	displayName := func(lower string) string {
		if n, ok := nameMap[lower]; ok {
			return n
		}
		return lower
	}

	// Collect all outbound targets for a service.
	targetsOf := func(svc string) []string {
		var result []string
		for _, t := range adj[svc] {
			result = append(result, t.to)
		}
		return result
	}

	// Track which from->to edges have been used in flows.
	usedEdges := make(map[string]bool)
	markEdge := func(from, to string) {
		usedEdges[strings.ToLower(from)+"->"+strings.ToLower(to)] = true
	}

	var flows []FlowInfo

	// --- Named Business Flows ---
	// These are hand-crafted flow templates that match well-known microservice patterns.
	// Each defines a flow name, a description, the orchestrator service, the logical phase ordering,
	// and a detailed narrative.

	type flowPhase struct {
		name     string
		services []string // service name substrings to match from targets
	}

	type namedFlow struct {
		orchPattern string // substring match in orchestrator name
		flowName    string
		phases      []flowPhase
		narrative   func(orchName string, targets []string) string
	}

	namedFlows := []namedFlow{
		{
			orchPattern: "preserve-service",
			flowName:    "Ticket Booking Flow (High-Speed Trains)",
			phases: []flowPhase{
				{name: "1. Security Check", services: []string{"security"}},
				{name: "2. Trip & Contact Lookup", services: []string{"travel-service", "contacts"}},
				{name: "3. Order Creation", services: []string{"order-service"}},
				{name: "4. Seat Assignment", services: []string{"seat"}},
				{name: "5. Ancillary Services", services: []string{"assurance", "food", "consign", "station", "basic"}},
				{name: "6. User & Notification", services: []string{"user", "notification", "delivery"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s orchestrates the complete ticket booking flow for high-speed trains (G/D/C prefix). When a user books a ticket, the following steps execute in sequence:

**Phase 1 — Security Validation:** The service first calls ts-security-service to verify the user's identity and check for any booking restrictions or blacklist entries.

**Phase 2 — Trip & Contact Lookup:** Next, it calls ts-travel-service to validate the requested trip (departure/arrival stations, date, train number, available tickets) and ts-contacts-service to retrieve or validate the passenger's contact information.

**Phase 3 — Order Creation:** With validated trip and contact data, the service calls ts-order-service to create the order record. Since this is a high-speed train (G/D/C prefix), it routes to ts-order-service (not ts-order-other-service).

**Phase 4 — Seat Assignment:** After order creation, ts-seat-service is called to allocate a specific seat on the train.

**Phase 5 — Ancillary Services (parallelizable):** These calls can execute in parallel after the order is created:
- ts-assurance-service — attach travel insurance if requested
- ts-food-service — order meals (which internally aggregates from ts-train-food-service and ts-station-food-service)
- ts-consign-service — register consignment/luggage packages (calls ts-consign-price-service for pricing)
- ts-station-service — resolve station details
- ts-basic-service — fetch basic trip metadata

**Phase 6 — Notification:** Finally, ts-user-service updates the user's booking history, and a confirmation notification is triggered (either via direct HTTP to ts-notification-service or via RabbitMQ message queue).

**Total outbound calls:** %d services. **Critical path (sequential):** ~6 hops (security → travel → contacts → order → seat → notification) = ~300ms at 50ms/hop. **With parallelization of Phase 5:** total latency drops to ~350ms.`, orch, len(targets))
			},
		},
		{
			orchPattern: "preserve-other",
			flowName:    "Ticket Booking Flow (Regular Trains)",
			phases: []flowPhase{
				{name: "1. Security Check", services: []string{"security"}},
				{name: "2. Trip & Contact Lookup", services: []string{"travel2", "contacts"}},
				{name: "3. Order Creation", services: []string{"order-other"}},
				{name: "4. Seat Assignment", services: []string{"seat"}},
				{name: "5. Ancillary Services", services: []string{"assurance", "food", "consign", "station", "basic", "delivery"}},
				{name: "6. User & Notification", services: []string{"user", "notification"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s orchestrates the ticket booking flow for regular (non-high-speed) trains — K/T/Z prefix and other train types. The flow mirrors the high-speed booking but routes to different service variants:

**Key routing differences from high-speed booking:**
- Calls **ts-travel2-service** instead of ts-travel-service for trip lookup
- Creates orders via **ts-order-other-service** instead of ts-order-service
- May include **ts-delivery-service** for package delivery tracking

The sequential flow and ancillary service calls follow the same pattern as the high-speed booking flow. This parallel service architecture allows independent scaling: high-speed train bookings (typically higher volume during business hours) can scale separately from regular train bookings.

**Total outbound calls:** %d services.`, orch, len(targets))
			},
		},
		{
			orchPattern: "cancel-service",
			flowName:    "Ticket Cancellation and Refund Flow",
			phases: []flowPhase{
				{name: "1. Order Lookup", services: []string{"order-service", "order-other"}},
				{name: "2. Refund Processing", services: []string{"inside-payment"}},
				{name: "3. User Update", services: []string{"user"}},
				{name: "4. Notification", services: []string{"notification"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s handles ticket cancellation and refund processing. The flow executes in strict sequence:

**Step 1 — Order Lookup:** The service calls both ts-order-service AND ts-order-other-service to find the order (since it may be for either a high-speed or regular train). The order status is validated (must be "Not Paid" or "Paid, Not Collected").

**Step 2 — Refund Processing:** If the ticket was paid, %s calls ts-inside-payment-service to process the refund. The inside-payment service credits the user's internal account balance. If the original payment was via external payment (credit card), inside-payment may call ts-payment-service to reverse the charge.

**Step 3 — User Update:** ts-user-service is called to update the user's booking history and account status.

**Step 4 — Cancellation Notification:** Finally, ts-notification-service sends a cancellation confirmation email using the "order_cancel_success" FreeMarker template.

**Total outbound calls:** %d services. All calls are **sequential** — each step depends on the previous one. Critical path: ~%dms at 50ms/hop.`, orch, orch, len(targets), len(targets)*50)
			},
		},
		{
			orchPattern: "rebook-service",
			flowName:    "Ticket Rebooking Flow",
			phases: []flowPhase{
				{name: "1. Old Order Lookup", services: []string{"order-service", "order-other"}},
				{name: "2. New Trip Validation", services: []string{"travel-service", "travel2"}},
				{name: "3. Seat & Route Check", services: []string{"seat", "train", "route"}},
				{name: "4. Payment Adjustment", services: []string{"inside-payment"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s handles rebooking a ticket from one train to another, potentially crossing between high-speed and regular train types. This is one of the most complex flows because it may need to transfer an order between different order services.

**Step 1 — Retrieve Old Order:** The service calls BOTH ts-order-service and ts-order-other-service to find the existing order, since the original booking could be for either train type.

**Step 2 — Validate New Trip:** The service calls BOTH ts-travel-service and ts-travel2-service to check availability of the new requested trip. The train number prefix (G/D/C for high-speed, K/T/Z for regular) determines which travel service has the trip data.

**Step 3 — Seat & Route Validation:** ts-seat-service checks seat availability on the new train, ts-train-service provides train configuration data, and ts-route-service validates the route.

**Step 4 — Payment Adjustment:** If the new ticket costs more than the old one, ts-inside-payment-service is called to charge the price difference. If it costs less, a partial refund is issued. The inside-payment service handles the internal ledger update and, if needed, calls ts-payment-service for actual money movement.

**Cross-type rebooking:** When rebooking from a regular train to a high-speed train (or vice versa), the order must be cancelled in one order service and recreated in the other. For example, rebooking from K-train to G-train means: cancel in ts-order-other-service → create in ts-order-service.

**Total outbound calls:** %d services. Critical path: ~%dms at 50ms/hop (most calls are sequential due to data dependencies).`, orch, len(targets), len(targets)*50)
			},
		},
		{
			orchPattern: "travel-plan",
			flowName:    "Trip Search and Planning Flow",
			phases: []flowPhase{
				{name: "1. Route Planning", services: []string{"route-plan", "route"}},
				{name: "2. Trip Search", services: []string{"travel-service", "travel2"}},
				{name: "3. Seat Availability", services: []string{"seat", "train"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s provides the trip search and planning functionality. When a user searches for available trains between two stations:

**Step 1 — Route Planning:** ts-route-plan-service is called to find possible routes (direct and transfer routes) between the departure and arrival stations.

**Step 2 — Trip Search:** Both ts-travel-service (high-speed G/D/C trains) and ts-travel2-service (regular K/T/Z trains) are queried for available trips on the found routes. This dual query ensures results include all train types.

**Step 3 — Seat Availability:** For each available trip, ts-seat-service is called to get remaining seat counts and ts-train-service provides train configuration data (seat classes, carriage types).

**Total outbound calls:** %d services.`, orch, len(targets))
			},
		},
		{
			orchPattern: "gateway",
			flowName:    "API Gateway Routing",
			phases: []flowPhase{
				{name: "Authentication", services: []string{"auth", "verification"}},
				{name: "Business Services", services: []string{"preserve", "cancel", "rebook", "travel", "order", "payment"}},
				{name: "Data Services", services: []string{"station", "train", "route", "config", "price", "contacts"}},
				{name: "Admin Services", services: []string{"admin"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s serves as the API gateway / reverse proxy for the entire system, routing incoming HTTP requests to %d backend microservices. It handles:

- **Authentication:** Routes login/token requests to ts-auth-service and verification code requests to ts-verification-code-service
- **Booking operations:** Proxies to ts-preserve-service (high-speed) / ts-preserve-other-service (regular) for ticket booking
- **Order management:** Routes to ts-order-service / ts-order-other-service based on train type
- **Cancellation & rebooking:** Forwards to ts-cancel-service and ts-rebook-service
- **Trip search:** Routes to ts-travel-service, ts-travel2-service, and ts-travel-plan-service
- **Payment:** Proxies to ts-inside-payment-service and ts-payment-service
- **Admin:** Routes admin panel requests to the 5 admin services
- **Data lookups:** Station, train, route, config, price, and contact queries

The gateway does NOT implement business logic — it purely routes and may add cross-cutting concerns (auth headers, rate limiting, logging).`, orch, len(targets))
			},
		},
		{
			orchPattern: "ui-dashboard",
			flowName:    "User Interface Flow",
			phases: []flowPhase{
				{name: "User Actions", services: []string{"preserve", "rebook", "cancel"}},
				{name: "Data Display", services: []string{"travel", "order", "station", "train", "route"}},
				{name: "Account", services: []string{"contacts", "avatar", "verification"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s is the Angular.js frontend application that provides the user interface. It makes direct API calls (typically through the gateway) to %d backend services:

- **Ticket booking:** ts-preserve-service / ts-preserve-other-service for new bookings
- **Trip search:** ts-travel-plan-service, ts-basic-service for searching available trains
- **Order management:** ts-order-service for viewing/managing orders, ts-rebook-service for rebooking, ts-inside-payment-service for payment
- **Admin panel:** ts-admin-basic-info-service for station/train/route/config management
- **User account:** ts-contacts-service, ts-avatar-service, ts-verification-code-service
- **Food ordering:** ts-food-service for meal orders, ts-assurance-service for insurance`, orch, len(targets))
			},
		},
		{
			orchPattern: "admin-basic-info",
			flowName:    "Admin: Basic Info Management",
			phases: []flowPhase{
				{name: "Data Management", services: []string{"station", "train", "config", "price", "contacts"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s provides a composite admin API for managing foundational reference data. It aggregates CRUD operations for %d data services:

- **ts-station-service** — manage train stations (add/update/delete stations)
- **ts-train-service** — manage train types and configurations
- **ts-config-service** — manage system configuration key-value pairs
- **ts-price-service** — manage pricing rules and fare tables
- **ts-contacts-service** — manage passenger contact records

This is a pure aggregator — it adds no business logic, just provides a unified admin interface for the foundational data layer.`, orch, len(targets))
			},
		},
		{
			orchPattern: "admin-travel",
			flowName:    "Admin: Travel/Trip Management",
			phases: []flowPhase{
				{name: "Trip Management", services: []string{"travel", "travel2", "station", "train", "route"}},
			},
			narrative: func(orch string, targets []string) string {
				return fmt.Sprintf(`%s provides admin functionality for managing train trips and schedules. It coordinates with %d services:

- **ts-travel-service** — manage high-speed train trips (G/D/C prefix)
- **ts-travel2-service** — manage regular train trips (K/T/Z and others)
- **ts-train-service** — train type reference data
- **ts-station-service** — station reference data
- **ts-route-service** — route definitions

Admin users can create, update, and delete trip schedules for both high-speed and regular trains through this unified interface.`, orch, len(targets))
			},
		},
	}

	// Process named flows first.
	processedOrchestrators := make(map[string]bool)

	for _, nf := range namedFlows {
		// Find the orchestrator service matching this pattern.
		var matchedOrch string
		var matchedTargets []target
		for svc, targets := range adj {
			if strings.Contains(svc, nf.orchPattern) && !processedOrchestrators[svc] {
				// For "preserve-service", avoid matching "preserve-other-service" and vice versa.
				if nf.orchPattern == "preserve-service" && strings.Contains(svc, "other") {
					continue
				}
				if nf.orchPattern == "travel-service" && (strings.Contains(svc, "travel2") || strings.Contains(svc, "travel-plan")) {
					continue
				}
				if nf.orchPattern == "order-service" && strings.Contains(svc, "other") {
					continue
				}
				matchedOrch = svc
				matchedTargets = targets
				break
			}
		}
		if matchedOrch == "" {
			continue
		}

		processedOrchestrators[matchedOrch] = true
		orchDisplay := displayName(matchedOrch)
		allTargets := targetsOf(matchedOrch)

		// Build service list.
		svcSet := make(map[string]bool)
		svcSet[orchDisplay] = true
		for _, t := range matchedTargets {
			svcSet[displayName(t.to)] = true
		}
		var svcList []string
		for s := range svcSet {
			svcList = append(svcList, s)
		}
		sort.Strings(svcList)

		// Build phased sequence diagram.
		var diagram strings.Builder
		diagram.WriteString("sequenceDiagram\n")
		diagram.WriteString(fmt.Sprintf("    participant %s\n", orchDisplay))

		// Collect all target display names for participants.
		targetDisplays := make(map[string]bool)
		for _, t := range matchedTargets {
			targetDisplays[displayName(t.to)] = true
		}

		// Order participants by phase.
		addedParticipant := make(map[string]bool)
		addedParticipant[orchDisplay] = true
		for _, phase := range nf.phases {
			for _, t := range matchedTargets {
				dn := displayName(t.to)
				if addedParticipant[dn] {
					continue
				}
				for _, pat := range phase.services {
					if strings.Contains(strings.ToLower(t.to), pat) {
						diagram.WriteString(fmt.Sprintf("    participant %s\n", dn))
						addedParticipant[dn] = true
						break
					}
				}
			}
		}
		// Add any unmatched participants.
		for _, t := range matchedTargets {
			dn := displayName(t.to)
			if !addedParticipant[dn] {
				diagram.WriteString(fmt.Sprintf("    participant %s\n", dn))
				addedParticipant[dn] = true
			}
		}

		// Write edges grouped by phase.
		for _, phase := range nf.phases {
			diagram.WriteString(fmt.Sprintf("    Note over %s: %s\n", orchDisplay, phase.name))
			for _, t := range matchedTargets {
				matched := false
				for _, pat := range phase.services {
					if strings.Contains(strings.ToLower(t.to), pat) {
						matched = true
						break
					}
				}
				if matched {
					label := operationLabel(t.link)
					diagram.WriteString(fmt.Sprintf("    %s->>%s: %s\n", orchDisplay, displayName(t.to), label))
					markEdge(matchedOrch, t.to)
				}
			}
		}
		// Any targets not matched by phases.
		for _, t := range matchedTargets {
			key := strings.ToLower(matchedOrch) + "->" + strings.ToLower(t.to)
			if !usedEdges[key] {
				label := operationLabel(t.link)
				diagram.WriteString(fmt.Sprintf("    %s->>%s: %s\n", orchDisplay, displayName(t.to), label))
				markEdge(matchedOrch, t.to)
			}
		}

		flows = append(flows, FlowInfo{
			Name:      nf.flowName,
			Narrative: nf.narrative(orchDisplay, allTargets),
			Services:  svcList,
			Diagram:   diagram.String(),
		})
	}

	// --- Remaining orchestrators (generic flows) ---
	// Find orchestrators (services with ≥3 outbound connections) that weren't matched by named flows.
	type orchestrator struct {
		name    string
		targets []target
	}
	var remaining []orchestrator
	for from, targets := range adj {
		if len(targets) >= 3 && !processedOrchestrators[from] {
			remaining = append(remaining, orchestrator{from, targets})
		}
	}
	sort.Slice(remaining, func(i, j int) bool {
		return len(remaining[i].targets) > len(remaining[j].targets)
	})

	for _, orch := range remaining {
		orchDisplay := displayName(orch.name)
		var directTargets []string
		for _, t := range orch.targets {
			directTargets = append(directTargets, displayName(t.to))
		}

		svcSet := make(map[string]bool)
		svcSet[orchDisplay] = true
		for _, t := range orch.targets {
			svcSet[displayName(t.to)] = true
		}
		var svcList []string
		for s := range svcSet {
			svcList = append(svcList, s)
		}
		sort.Strings(svcList)

		narrative := fmt.Sprintf("%s coordinates with %d services: %s.",
			orchDisplay, len(orch.targets), strings.Join(directTargets, ", "))

		var diagram strings.Builder
		diagram.WriteString("sequenceDiagram\n")
		diagram.WriteString(fmt.Sprintf("    participant %s\n", orchDisplay))
		for _, t := range orch.targets {
			diagram.WriteString(fmt.Sprintf("    participant %s\n", displayName(t.to)))
		}
		for _, t := range orch.targets {
			label := operationLabel(t.link)
			diagram.WriteString(fmt.Sprintf("    %s->>%s: %s\n", orchDisplay, displayName(t.to), label))
			markEdge(orch.name, t.to)
		}

		flowName := orchDisplay + " Interactions"
		flows = append(flows, FlowInfo{
			Name:      flowName,
			Narrative: narrative,
			Services:  svcList,
			Diagram:   diagram.String(),
		})
	}

	// --- Remaining edges ---
	var remainingEdges []LinkInfo
	for _, link := range g.Links {
		key := strings.ToLower(link.FromRepo) + "->" + strings.ToLower(link.ToRepo)
		if !usedEdges[key] {
			remainingEdges = append(remainingEdges, link)
		}
	}

	if len(remainingEdges) > 0 {
		svcSet := make(map[string]bool)
		for _, link := range remainingEdges {
			svcSet[link.FromRepo] = true
			svcSet[link.ToRepo] = true
		}
		var svcList []string
		for s := range svcSet {
			svcList = append(svcList, s)
		}
		sort.Strings(svcList)

		var narrativeParts []string
		var diagram strings.Builder
		diagram.WriteString("sequenceDiagram\n")
		participants := make(map[string]bool)
		for _, link := range remainingEdges {
			if !participants[link.FromRepo] {
				diagram.WriteString(fmt.Sprintf("    participant %s\n", link.FromRepo))
				participants[link.FromRepo] = true
			}
			if !participants[link.ToRepo] {
				diagram.WriteString(fmt.Sprintf("    participant %s\n", link.ToRepo))
				participants[link.ToRepo] = true
			}
		}
		for _, link := range remainingEdges {
			label := operationLabel(link)
			diagram.WriteString(fmt.Sprintf("    %s->>%s: %s\n", link.FromRepo, link.ToRepo, label))
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s calls %s (%s)", link.FromRepo, link.ToRepo, label))
		}

		flows = append(flows, FlowInfo{
			Name:      "Supporting Service Calls",
			Narrative: "Additional service-to-service interactions that support the main user flows:\n\n- " + strings.Join(narrativeParts, "\n- "),
			Services:  svcList,
			Diagram:   diagram.String(),
		})
	}

	// Replace the LLM flows with synthesized ones.
	g.Flows = flows
}

// writeArchitecturalPatterns adds an "Architectural Patterns" section to the system overview
// that identifies structural patterns in the service graph: parallel service pairs,
// leaf services, orchestrator services, aggregator services, notification pipelines,
// and payment/billing layering.
func (g *CentralSiteGenerator) writeArchitecturalPatterns(b *strings.Builder) {
	if len(g.Repos) < 3 {
		return
	}

	// Build adjacency maps.
	outbound := make(map[string][]string)  // service -> list of services it calls
	inbound := make(map[string][]string)   // service -> list of services that call it
	linkTypes := make(map[string]string)    // "from->to" -> linkType
	repoSet := make(map[string]bool)
	for _, r := range g.Repos {
		repoSet[strings.ToLower(r.Name)] = true
	}
	for _, link := range g.Links {
		from := strings.ToLower(link.FromRepo)
		to := strings.ToLower(link.ToRepo)
		if repoSet[from] && repoSet[to] {
			outbound[from] = append(outbound[from], to)
			inbound[to] = append(inbound[to], from)
			linkTypes[from+"->"+to] = link.LinkType
		}
	}

	b.WriteString("## Architectural Patterns\n\n")

	// --- Pattern 1: Parallel Service Pairs ---
	// Detect pairs: X-service/X-other-service, X-service/X2-service
	type parallelPair struct {
		serviceA string
		serviceB string
		pattern  string // "other" or "2"
	}
	var pairs []parallelPair
	pairSeen := make(map[string]bool)

	for _, r := range g.Repos {
		lower := strings.ToLower(r.Name)
		// Check for X-other-service / X-service pattern
		if strings.Contains(lower, "-other-") {
			base := strings.Replace(lower, "-other-", "-", 1)
			if repoSet[base] && !pairSeen[base] {
				pairs = append(pairs, parallelPair{base, lower, "other"})
				pairSeen[base] = true
			}
		}
		// Check for X2-service / X-service pattern (e.g., travel2-service / travel-service)
		// Look for digit in service name
		for _, digit := range []string{"2", "3"} {
			if strings.Contains(lower, digit+"-") {
				base := strings.Replace(lower, digit+"-", "-", 1)
				if repoSet[base] && !pairSeen[base+digit] {
					pairs = append(pairs, parallelPair{base, lower, digit})
					pairSeen[base+digit] = true
				}
			}
		}
	}

	if len(pairs) > 0 {
		b.WriteString("### Parallel Service Pairs\n\n")
		b.WriteString("The system uses a **parallel service pattern** where functionally identical services are deployed in pairs to handle different data partitions. ")
		b.WriteString("This enables independent scaling, separate data stores, and failure isolation between partitions.\n\n")
		b.WriteString("| Primary Service | Parallel Service | Routing Logic |\n")
		b.WriteString("|----------------|-----------------|---------------|\n")
		for _, pair := range pairs {
			routingLogic := g.inferRoutingLogic(pair.serviceA, pair.serviceB, pair.pattern, outbound, inbound)
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", pair.serviceA, pair.serviceB, routingLogic))
		}
		b.WriteString("\n")
		// Explain the routing pattern if we found common callers
		b.WriteString("**How routing works:** Services that call both members of a pair (e.g., ts-rebook-service calling both ts-order-service and ts-order-other-service) ")
		b.WriteString("use a routing discriminator — typically a train type prefix or category code — to determine which parallel instance handles the request. ")
		b.WriteString("High-speed/express trains (G/D/C prefix) are routed to the primary service, while regular trains (K/T/Z and others) are routed to the parallel \"other\" or \"2\" variant. ")
		b.WriteString("This pattern appears consistently across order management, travel/trip queries, and ticket preservation.\n\n")
	}

	// --- Pattern 2: Leaf Services (pure data providers) ---
	var leafServices []string
	for _, r := range g.Repos {
		lower := strings.ToLower(r.Name)
		if len(outbound[lower]) == 0 && len(inbound[lower]) > 0 {
			leafServices = append(leafServices, r.Name)
		}
	}
	if len(leafServices) > 0 {
		sort.Strings(leafServices)
		b.WriteString("### Leaf Services (Pure Data Providers)\n\n")
		b.WriteString("These services have **zero outbound HTTP/API dependencies** — they only respond to incoming requests and manage their own data store. ")
		b.WriteString("They are the foundational data layer of the system, providing reference data that other services query.\n\n")
		b.WriteString("| Service | Inbound Callers | Role |\n")
		b.WriteString("|---------|----------------|------|\n")
		for _, svc := range leafServices {
			lower := strings.ToLower(svc)
			callers := len(inbound[lower])
			role := g.inferServiceRole(svc)
			b.WriteString(fmt.Sprintf("| %s | %d services | %s |\n", svc, callers, role))
		}
		b.WriteString("\n")
		b.WriteString("**Architectural significance:** Leaf services are ideal candidates for caching, read replicas, and multi-region replication ")
		b.WriteString("since they have no downstream dependencies and serve as single sources of truth for their domain data.\n\n")
	}

	// --- Pattern 3: Orchestrator Services (high fan-out) ---
	var orchestrators []svcFanOutInfo
	for _, r := range g.Repos {
		lower := strings.ToLower(r.Name)
		if len(outbound[lower]) >= 5 {
			orchestrators = append(orchestrators, svcFanOutInfo{r.Name, len(outbound[lower])})
		}
	}
	if len(orchestrators) > 0 {
		sort.Slice(orchestrators, func(i, j int) bool {
			return orchestrators[i].fanOut > orchestrators[j].fanOut
		})
		b.WriteString("### Orchestrator Services (High Fan-Out)\n\n")
		b.WriteString("These services coordinate complex workflows by calling many downstream services. They represent the critical business logic paths.\n\n")
		b.WriteString("| Service | Outbound Calls | Role |\n")
		b.WriteString("|---------|---------------|------|\n")
		for _, orch := range orchestrators {
			role := g.inferServiceRole(orch.name)
			b.WriteString(fmt.Sprintf("| %s | %d services | %s |\n", orch.name, orch.fanOut, role))
		}
		b.WriteString("\n")
		b.WriteString("**Performance note:** Orchestrator services are the primary latency bottleneck. ")
		b.WriteString("With synchronous HTTP calls, the critical path length equals the number of sequential hops × per-hop latency. ")
		b.WriteString("At 50ms per HTTP hop:\n\n")
		for _, orch := range orchestrators {
			lower := strings.ToLower(orch.name)
			deps := outbound[lower]
			criticalMs := len(deps) * 50
			b.WriteString(fmt.Sprintf("- **%s**: %d calls → worst-case %dms critical path (if all sequential). ",
				orch.name, len(deps), criticalMs))
			// Identify which calls could be parallel
			parallelizable, sequential := g.analyzeCallParallelism(lower, deps, outbound)
			if len(parallelizable) > 0 && len(sequential) > 0 {
				seqMs := len(sequential) * 50
				b.WriteString(fmt.Sprintf("Sequential calls (~%d): %s. Parallelizable calls (~%d): %s. Optimized critical path: ~%dms.\n",
					len(sequential), strings.Join(sequential, ", "),
					len(parallelizable), strings.Join(parallelizable, ", "),
					seqMs+50))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// --- Pattern 4: Payment Layering ---
	g.writePaymentLayerPattern(b, outbound, inbound)

	// --- Pattern 5: Notification Pipeline ---
	g.writeNotificationPipelinePattern(b, outbound, inbound, linkTypes)

	// --- Pattern 6: Aggregator Services ---
	g.writeAggregatorPattern(b, outbound, inbound)

	// --- Pattern 7: Deployment Co-location Analysis ---
	g.writeCoLocationAnalysis(b, outbound, orchestrators)
}

// inferRoutingLogic infers the routing logic between a parallel service pair
// by examining their common callers and the domain naming patterns.
func (g *CentralSiteGenerator) inferRoutingLogic(serviceA, serviceB, pattern string, outbound, inbound map[string][]string) string {
	// Find services that call both members of the pair
	callersA := make(map[string]bool)
	for _, c := range inbound[serviceA] {
		callersA[c] = true
	}
	var commonCallers []string
	for _, c := range inbound[serviceB] {
		if callersA[c] {
			commonCallers = append(commonCallers, c)
		}
	}

	// Infer routing logic from naming patterns
	baseName := serviceA
	if idx := strings.LastIndex(baseName, "-service"); idx > 0 {
		baseName = baseName[:idx]
	}
	baseName = strings.TrimPrefix(baseName, "ts-")

	switch {
	case strings.Contains(baseName, "order"):
		return "Train type routing: G/D/C high-speed trains → primary; K/T/Z regular trains → other"
	case strings.Contains(baseName, "travel"):
		return "Train type routing: G/D/C high-speed trains → primary; regular trains → travel2"
	case strings.Contains(baseName, "preserve"):
		return "Train type routing: high-speed train bookings → primary; regular train bookings → other"
	default:
		if pattern == "other" {
			return "Category-based routing between primary and alternate instance"
		}
		return "Partition-based routing between instances"
	}
}

// inferServiceRole provides a human-readable role description for a service.
func (g *CentralSiteGenerator) inferServiceRole(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "station") && !strings.Contains(lower, "food"):
		return "Station reference data (names, IDs, metadata)"
	case strings.Contains(lower, "train") && !strings.Contains(lower, "food"):
		return "Train type reference data (train categories, speeds, capacities)"
	case strings.Contains(lower, "config"):
		return "System configuration key-value store"
	case strings.Contains(lower, "price"):
		return "Pricing rules and fare configuration"
	case strings.Contains(lower, "contacts"):
		return "Passenger contact information management"
	case strings.Contains(lower, "route"):
		return "Train route definitions and station sequences"
	case strings.Contains(lower, "assurance"):
		return "Travel insurance policy management"
	case strings.Contains(lower, "consign-price"):
		return "Consignment pricing rules"
	case strings.Contains(lower, "station-food"):
		return "Station food store catalog"
	case strings.Contains(lower, "train-food"):
		return "On-train food menu catalog"
	case strings.Contains(lower, "notification"):
		return "Email/notification delivery via templates"
	case strings.Contains(lower, "delivery"):
		return "Package delivery tracking"
	case strings.Contains(lower, "news"):
		return "News/announcement content"
	case strings.Contains(lower, "verification"):
		return "Verification code generation and validation"
	case strings.Contains(lower, "avatar"):
		return "User avatar/profile image processing"
	case strings.Contains(lower, "voucher"):
		return "Travel voucher generation"
	case strings.Contains(lower, "auth"):
		return "Authentication and JWT token management"
	case strings.Contains(lower, "user") && strings.Contains(lower, "admin"):
		return "Admin interface for user management"
	case strings.Contains(lower, "user"):
		return "User account management"
	case strings.Contains(lower, "gateway"):
		return "API gateway / reverse proxy"
	case strings.Contains(lower, "ui") || strings.Contains(lower, "dashboard"):
		return "Frontend user interface"
	case strings.Contains(lower, "preserve"):
		return "Ticket booking/reservation orchestrator"
	case strings.Contains(lower, "cancel"):
		return "Ticket cancellation and refund orchestrator"
	case strings.Contains(lower, "rebook"):
		return "Ticket rebooking and fare adjustment orchestrator"
	case strings.Contains(lower, "order"):
		return "Order lifecycle management (CRUD, queries, status)"
	case strings.Contains(lower, "travel"):
		return "Trip/journey query and management"
	case strings.Contains(lower, "seat"):
		return "Seat availability and allocation"
	case strings.Contains(lower, "food") && strings.Contains(lower, "delivery"):
		return "Food delivery order management"
	case strings.Contains(lower, "food"):
		return "Food ordering aggregator (combines train-food + station-food)"
	case strings.Contains(lower, "inside-payment"):
		return "Internal account/balance management (ledger layer)"
	case strings.Contains(lower, "payment"):
		return "External payment processing (charges actual money)"
	case strings.Contains(lower, "security"):
		return "Security checks and fraud detection"
	case strings.Contains(lower, "basic"):
		return "Composite data aggregator (station + train + route + price)"
	case strings.Contains(lower, "execute"):
		return "Ticket collection/execution at station"
	case strings.Contains(lower, "wait"):
		return "Waitlist order management"
	case strings.Contains(lower, "plan"):
		return "Route planning and transfer optimization"
	case strings.Contains(lower, "consign"):
		return "Luggage/package consignment management"
	case strings.Contains(lower, "ticket-office"):
		return "Physical ticket office operations"
	default:
		return "Service"
	}
}

// analyzeCallParallelism determines which downstream calls from an orchestrator
// could be parallelized vs must be sequential.
func (g *CentralSiteGenerator) analyzeCallParallelism(orchName string, deps []string, outbound map[string][]string) (parallelizable, sequential []string) {
	// Heuristic: calls to data-only services (leaf nodes) can be parallelized.
	// Calls that produce data needed by subsequent calls must be sequential.
	// Order: security check → query services → mutation services → notification
	for _, dep := range deps {
		switch {
		case strings.Contains(dep, "security"):
			sequential = append(sequential, dep) // must be first
		case strings.Contains(dep, "order"):
			sequential = append(sequential, dep) // creates/updates state
		case strings.Contains(dep, "travel"):
			sequential = append(sequential, dep) // needed for seat/price lookup
		case strings.Contains(dep, "contact"):
			sequential = append(sequential, dep) // needed for order creation
		case strings.Contains(dep, "seat"):
			sequential = append(sequential, dep) // depends on travel query result
		case strings.Contains(dep, "payment") || strings.Contains(dep, "inside"):
			sequential = append(sequential, dep) // money operations
		case strings.Contains(dep, "notification"):
			sequential = append(sequential, dep) // must be last
		default:
			parallelizable = append(parallelizable, dep) // data lookups: assurance, food, consign, basic, etc.
		}
	}
	return
}

// writePaymentLayerPattern documents the two-layer payment architecture.
func (g *CentralSiteGenerator) writePaymentLayerPattern(b *strings.Builder, outbound, inbound map[string][]string) {
	// Look for inside-payment → payment relationship
	var insidePayment, externalPayment string
	for _, r := range g.Repos {
		lower := strings.ToLower(r.Name)
		if strings.Contains(lower, "inside") && strings.Contains(lower, "payment") {
			insidePayment = lower
		} else if strings.Contains(lower, "payment") && !strings.Contains(lower, "inside") {
			externalPayment = lower
		}
	}
	if insidePayment == "" || externalPayment == "" {
		return
	}

	// Check if inside-payment calls external payment
	callsExternal := false
	for _, dep := range outbound[insidePayment] {
		if dep == externalPayment {
			callsExternal = true
			break
		}
	}
	if !callsExternal {
		return
	}

	b.WriteString("### Two-Layer Payment Architecture\n\n")
	b.WriteString("The system implements a **two-layer payment model** that separates internal account management from actual money movement:\n\n")
	b.WriteString(fmt.Sprintf("1. **%s (Ledger Layer):** Manages internal user accounts, balances, and payment records. ", insidePayment))
	b.WriteString("Handles balance checks, debits, credits, and refund calculations. This is the layer that most services interact with.\n")
	b.WriteString(fmt.Sprintf("2. **%s (Settlement Layer):** Processes actual external payments — credit card charges, bank transfers, etc. ", externalPayment))
	b.WriteString("Only called by the inside-payment service when real money needs to move.\n\n")
	b.WriteString("**When money moves:** The inside-payment service acts as an intermediary. When a user books a ticket, the preserve service calls inside-payment to debit the account. ")
	b.WriteString("If the account balance is insufficient, inside-payment calls the external payment service to charge the difference. ")
	b.WriteString("For refunds (cancellations, rebookings), inside-payment credits the internal account and may trigger a reversal through the payment service.\n\n")

	// Show which services call inside-payment
	callers := inbound[insidePayment]
	if len(callers) > 0 {
		b.WriteString("**Services that use the payment layer:** ")
		var callerNames []string
		for _, c := range callers {
			callerNames = append(callerNames, c)
		}
		sort.Strings(callerNames)
		b.WriteString(strings.Join(callerNames, ", ") + "\n\n")
	}
}

// writeNotificationPipelinePattern documents the notification delivery pipeline.
func (g *CentralSiteGenerator) writeNotificationPipelinePattern(b *strings.Builder, outbound, inbound map[string][]string, linkTypes map[string]string) {
	// Find notification service
	var notifService string
	for _, r := range g.Repos {
		lower := strings.ToLower(r.Name)
		if strings.Contains(lower, "notification") {
			notifService = lower
			break
		}
	}
	if notifService == "" {
		return
	}

	b.WriteString("### Notification Pipeline\n\n")
	b.WriteString("The system uses a **unified notification pipeline** where user-facing events trigger notifications through both synchronous and asynchronous channels:\n\n")

	// Find all services that send to notification service
	callers := inbound[notifService]
	// Also find services that use RabbitMQ/AMQP
	var directCallers, amqpSenders []string
	for _, caller := range callers {
		lt := linkTypes[caller+"->"+notifService]
		if strings.Contains(lt, "amqp") || strings.Contains(lt, "rabbit") || strings.Contains(lt, "mq") {
			amqpSenders = append(amqpSenders, caller)
		} else {
			directCallers = append(directCallers, caller)
		}
	}

	// Also check for services with delivery/mq patterns
	for _, r := range g.Repos {
		lower := strings.ToLower(r.Name)
		if strings.Contains(lower, "delivery") && lower != notifService {
			// Check if anything sends to delivery via AMQP
			for from, lt := range linkTypes {
				if strings.HasSuffix(from, "->"+lower) && (strings.Contains(lt, "amqp") || strings.Contains(lt, "grpc")) {
					parts := strings.SplitN(from, "->", 2)
					if len(parts) == 2 {
						amqpSenders = append(amqpSenders, parts[0]+" (→ "+lower+")")
					}
				}
			}
		}
	}

	b.WriteString("```\n")
	b.WriteString("User Action (book/cancel/rebook)\n")
	b.WriteString("  │\n")
	b.WriteString("  ├─── Synchronous HTTP ──→ notification-service ──→ Email (FreeMarker templates)\n")
	b.WriteString("  │\n")
	b.WriteString("  └─── RabbitMQ (async) ──→ notification-service ──→ Email (FreeMarker templates)\n")
	b.WriteString("                          ──→ delivery-service   ──→ Delivery tracking\n")
	b.WriteString("```\n\n")

	if len(directCallers) > 0 || len(amqpSenders) > 0 {
		b.WriteString("**Notification triggers:**\n\n")
		allCallers := append(directCallers, amqpSenders...)
		sort.Strings(allCallers)
		for _, c := range allCallers {
			action := "triggers notification"
			if strings.Contains(c, "cancel") {
				action = "sends cancellation confirmation email"
			} else if strings.Contains(c, "preserve") {
				action = "sends booking confirmation email"
			} else if strings.Contains(c, "rebook") {
				action = "sends rebooking confirmation email"
			}
			b.WriteString(fmt.Sprintf("- **%s** → %s\n", c, action))
		}
		b.WriteString("\n")
	}

	b.WriteString("**Delivery mechanism:** The notification service uses Spring Mail with FreeMarker templates ")
	b.WriteString("for dynamic email content (order_create_success, order_cancel_success, order_changed_success, preserve_success). ")
	b.WriteString("Services can trigger notifications either via synchronous HTTP calls or by publishing to the RabbitMQ `email` queue for asynchronous delivery.\n\n")
}

// writeAggregatorPattern documents services that aggregate data from multiple sub-services.
func (g *CentralSiteGenerator) writeAggregatorPattern(b *strings.Builder, outbound, inbound map[string][]string) {
	// Find aggregator services: services that call 2+ related sub-services
	type aggregator struct {
		name       string
		subSvcs    []string
		role       string
	}
	var aggregators []aggregator

	for _, r := range g.Repos {
		lower := strings.ToLower(r.Name)
		deps := outbound[lower]
		if len(deps) < 2 {
			continue
		}

		// Check if this service aggregates related sub-services
		baseName := lower
		if idx := strings.LastIndex(baseName, "-service"); idx > 0 {
			baseName = baseName[:idx]
		}
		baseName = strings.TrimPrefix(baseName, "ts-")

		var related []string
		for _, dep := range deps {
			if strings.Contains(dep, baseName) && dep != lower {
				related = append(related, dep)
			}
		}
		if len(related) >= 2 {
			aggregators = append(aggregators, aggregator{
				name:    r.Name,
				subSvcs: related,
				role:    g.inferServiceRole(r.Name),
			})
		}
	}

	if len(aggregators) == 0 {
		return
	}

	b.WriteString("### Aggregator Services\n\n")
	b.WriteString("These services combine data from multiple specialized sub-services into a unified API:\n\n")
	for _, agg := range aggregators {
		b.WriteString(fmt.Sprintf("- **%s** aggregates from: %s. Role: %s\n",
			agg.name, strings.Join(agg.subSvcs, ", "), agg.role))
	}
	b.WriteString("\n")
}

// svcFanOutInfo holds a service name and its outbound fan-out count.
type svcFanOutInfo struct {
	name   string
	fanOut int
}

// writeCoLocationAnalysis provides deployment guidance based on dependency analysis.
func (g *CentralSiteGenerator) writeCoLocationAnalysis(b *strings.Builder, outbound map[string][]string, orchestrators []svcFanOutInfo) {
	if len(orchestrators) == 0 {
		return
	}

	b.WriteString("### Deployment Co-location Analysis\n\n")
	b.WriteString("For latency-sensitive deployments, the following services should be co-located (same region/zone) ")
	b.WriteString("based on their position in critical request paths:\n\n")

	// Find the most critical orchestrator (highest fan-out)
	colocateSet := make(map[string]bool)
	for _, orch := range orchestrators {
		lower := strings.ToLower(orch.name)
		colocateSet[lower] = true
		for _, dep := range outbound[lower] {
			colocateSet[dep] = true
		}
	}

	b.WriteString("**Must co-locate (latency-critical path):**\n")
	var colocateList []string
	for svc := range colocateSet {
		colocateList = append(colocateList, svc)
	}
	sort.Strings(colocateList)
	for _, svc := range colocateList {
		if len(outbound[svc]) > 0 {
			b.WriteString(fmt.Sprintf("- %s (calls %d services)\n", svc, len(outbound[svc])))
		}
	}
	b.WriteString("\n")

	// Leaf services that can be replicated
	var replicable []string
	for _, r := range g.Repos {
		lower := strings.ToLower(r.Name)
		if len(outbound[lower]) == 0 {
			replicable = append(replicable, r.Name)
		}
	}
	if len(replicable) > 0 {
		sort.Strings(replicable)
		b.WriteString("**Can replicate per-region (stateless data providers):** ")
		b.WriteString(strings.Join(replicable, ", "))
		b.WriteString("\n\n")
	}
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
