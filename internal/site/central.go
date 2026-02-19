package site

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RepoInfo holds information about a registered repository for central site generation.
type RepoInfo struct {
	Name        string
	DisplayName string
	Summary     string
	Status      string
	FileCount   int
	SourceType  string
	DocsDir     string // path to the repo's .autodoc/docs/ directory
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
	// Normalize links and flows before generating.
	g.normalizeData()

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
		b.WriteString("| Service | Status | Files | Type | Summary |\n")
		b.WriteString("|---------|--------|-------|------|---------|\n")
		for _, repo := range g.Repos {
			displayName := repo.DisplayName
			if displayName == "" {
				displayName = repo.Name
			}
			summary := repo.Summary
			if len(summary) > 80 {
				summary = summary[:77] + "..."
			}
			// Link to the repo's docs subdirectory.
			link := fmt.Sprintf("[%s](%s/index.md)", displayName, repo.Name)
			statusBadge := repo.Status
			if statusBadge == "" {
				statusBadge = "unknown"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %d | %s | %s |\n",
				link, statusBadge, repo.FileCount, repo.SourceType, summary))
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
		b.WriteString("| Service | Status | Files | Type | Summary |\n")
		b.WriteString("|---------|--------|-------|------|---------|\n")
		for _, repo := range g.Repos {
			displayName := repo.DisplayName
			if displayName == "" {
				displayName = repo.Name
			}
			summary := repo.Summary
			if len(summary) > 100 {
				summary = summary[:97] + "..."
			}
			b.WriteString(fmt.Sprintf("| **%s** | %s | %d | %s | %s |\n",
				displayName, repo.Status, repo.FileCount, repo.SourceType, summary))
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

		// Deduplicate by from+to pair.
		key := link.FromRepo + "->" + link.ToRepo
		if seen[key] {
			continue
		}
		seen[key] = true
		cleanLinks = append(cleanLinks, link)
	}

	// Filter fan-out false positives: if a single service has outbound links
	// to more than 60% of all other services, those links are likely from
	// shared proto/interface stubs, not real dependencies.
	if len(g.Repos) > 3 {
		threshold := int(float64(len(g.Repos)-1) * 0.6)
		outCount := make(map[string]int)
		for _, link := range cleanLinks {
			outCount[link.FromRepo]++
		}
		var filtered []LinkInfo
		for _, link := range cleanLinks {
			if outCount[link.FromRepo] > threshold {
				continue // skip fan-out links
			}
			filtered = append(filtered, link)
		}
		cleanLinks = filtered
	}

	g.Links = cleanLinks

	// Deduplicate flows by name (keep the one with the most services).
	flowMap := make(map[string]int) // name -> index in deduped list
	var cleanFlows []FlowInfo
	for _, f := range g.Flows {
		if idx, exists := flowMap[f.Name]; exists {
			// Keep the one with more services.
			if len(f.Services) > len(cleanFlows[idx].Services) {
				cleanFlows[idx] = f
			}
			continue
		}
		flowMap[f.Name] = len(cleanFlows)
		cleanFlows = append(cleanFlows, f)
	}
	g.Flows = cleanFlows
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
