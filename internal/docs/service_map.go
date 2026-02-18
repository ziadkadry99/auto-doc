package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type serviceMapNode struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Group       string   `json:"group"`
	Summary     string   `json:"summary"`
	FileCount   int      `json:"fileCount"`
	Language    string   `json:"language"`
	EntryPoints []string `json:"entryPoints"`
	Team        string   `json:"team"`
	DocLink     string   `json:"docLink"`
	SourceType  string   `json:"sourceType"`
}

type serviceMapEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Label  string `json:"label"`
}

type serviceMapData struct {
	ProjectName string           `json:"projectName"`
	Nodes       []serviceMapNode `json:"nodes"`
	Edges       []serviceMapEdge `json:"edges"`
	LinkTypes   []linkTypeInfo   `json:"linkTypes"`
}

type linkTypeInfo struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

var linkTypeColors = map[string]string{
	"http":  "#4e79a7",
	"grpc":  "#f28e2b",
	"kafka": "#e15759",
	"amqp":  "#76b7b2",
	"sns":   "#59a14f",
	"sqs":   "#edc948",
}

// GenerateServiceMap creates a self-contained HTML page with a D3.js
// force-directed graph showing services and their cross-service dependencies.
func GenerateServiceMap(outputDir string, repos []ServiceInfo, links []ServiceLinkInfo, projectName string) error {
	data := buildServiceMapData(repos, links, projectName)

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling service map data: %w", err)
	}

	html := strings.Replace(serviceMapHTML, "/*__SERVICE_MAP_DATA__*/null", string(jsonBytes), 1)

	docsDir := filepath.Join(outputDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(docsDir, "service-map.html"), []byte(html), 0o644)
}

func buildServiceMapData(repos []ServiceInfo, links []ServiceLinkInfo, projectName string) serviceMapData {
	nodes := make([]serviceMapNode, 0, len(repos))
	for _, repo := range repos {
		label := repo.DisplayName
		if label == "" {
			label = repo.Name
		}
		nodes = append(nodes, serviceMapNode{
			ID:         repo.Name,
			Label:      label,
			Group:      repo.SourceType,
			Summary:    repo.Summary,
			FileCount:  repo.FileCount,
			SourceType: repo.SourceType,
			DocLink:    repo.Name + "/index.html",
		})
	}

	edges := make([]serviceMapEdge, 0, len(links))
	seenTypes := make(map[string]bool)
	for _, link := range links {
		edges = append(edges, serviceMapEdge{
			Source: link.FromRepo,
			Target: link.ToRepo,
			Type:   link.LinkType,
			Label:  link.Reason,
		})
		seenTypes[link.LinkType] = true
	}

	var linkTypes []linkTypeInfo
	for t := range seenTypes {
		color := linkTypeColors[t]
		if color == "" {
			color = "#bab0ac"
		}
		linkTypes = append(linkTypes, linkTypeInfo{Name: t, Color: color})
	}

	return serviceMapData{
		ProjectName: projectName,
		Nodes:       nodes,
		Edges:       edges,
		LinkTypes:   linkTypes,
	}
}

const serviceMapHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Service Map</title>
<script src="https://d3js.org/d3.v7.min.js"></script>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0d1117; color: #c9d1d9; overflow: hidden; }
#container { display: flex; height: 100vh; }
#sidebar { width: 320px; background: #161b22; border-right: 1px solid #30363d; overflow-y: auto; padding: 16px; flex-shrink: 0; }
#graph { flex: 1; position: relative; }
svg { width: 100%; height: 100%; }
h1 { font-size: 1.2em; margin-bottom: 12px; color: #f0f6fc; }
h2 { font-size: 1em; margin: 16px 0 8px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.05em; }
.search-box { width: 100%; padding: 8px 12px; background: #0d1117; border: 1px solid #30363d; border-radius: 6px; color: #c9d1d9; font-size: 14px; margin-bottom: 12px; }
.search-box:focus { outline: none; border-color: #58a6ff; }
.legend-item { display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px; }
.legend-line { width: 24px; height: 3px; flex-shrink: 0; border-radius: 2px; }
.service-detail { background: #0d1117; border: 1px solid #30363d; border-radius: 8px; padding: 12px; margin-top: 12px; display: none; }
.service-detail.active { display: block; }
.service-detail h3 { font-size: 1em; color: #f0f6fc; margin-bottom: 8px; }
.service-detail p { font-size: 13px; margin-bottom: 6px; line-height: 1.5; }
.service-detail .badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; background: #21262d; border: 1px solid #30363d; margin-right: 4px; }
.node circle { cursor: pointer; stroke-width: 2px; }
.node text { font-size: 12px; fill: #c9d1d9; pointer-events: none; text-anchor: middle; }
.link { stroke-opacity: 0.4; fill: none; }
.link.highlighted { stroke-opacity: 1; stroke-width: 3px; }
.stats { font-size: 12px; color: #8b949e; margin-bottom: 12px; }
</style>
</head>
<body>
<div id="container">
<div id="sidebar">
<h1 id="project-name">Service Map</h1>
<div class="stats" id="stats"></div>
<input type="text" class="search-box" id="search" placeholder="Search services...">
<h2>Link Types</h2>
<div id="legend"></div>
<div class="service-detail" id="detail">
<h3 id="detail-name"></h3>
<p id="detail-summary"></p>
<p id="detail-meta"></p>
<p id="detail-deps"></p>
</div>
</div>
<div id="graph">
<svg id="svg"></svg>
</div>
</div>
<script>
const DATA = /*__SERVICE_MAP_DATA__*/null;
if (!DATA) { document.body.innerHTML = '<p style="padding:40px">No service map data available.</p>'; }
else { renderServiceMap(DATA); }

function renderServiceMap(data) {
  document.getElementById('project-name').textContent = data.projectName + ' — Service Map';
  document.getElementById('stats').textContent = data.nodes.length + ' services, ' + data.edges.length + ' connections';

  const legend = document.getElementById('legend');
  data.linkTypes.forEach(lt => {
    const item = document.createElement('div');
    item.className = 'legend-item';
    item.innerHTML = '<div class="legend-line" style="background:' + lt.color + '"></div><span>' + lt.name.toUpperCase() + '</span>';
    legend.appendChild(item);
  });

  const svg = d3.select('#svg');
  const width = document.getElementById('graph').clientWidth;
  const height = document.getElementById('graph').clientHeight;

  const g = svg.append('g');
  svg.call(d3.zoom().scaleExtent([0.2, 5]).on('zoom', (e) => g.attr('transform', e.transform)));

  const linkColorMap = {};
  data.linkTypes.forEach(lt => linkColorMap[lt.name] = lt.color);

  const link = g.append('g').selectAll('line')
    .data(data.edges).enter().append('line')
    .attr('class', 'link')
    .attr('stroke', d => linkColorMap[d.type] || '#30363d')
    .attr('stroke-width', 2)
    .attr('stroke-dasharray', d => (d.type === 'kafka' || d.type === 'amqp') ? '5,5' : null);

  const nodeRadius = d => Math.max(20, Math.min(40, 15 + Math.sqrt(d.fileCount || 1) * 2));

  const node = g.append('g').selectAll('g')
    .data(data.nodes).enter().append('g')
    .attr('class', 'node')
    .call(d3.drag()
      .on('start', (e, d) => { if (!e.active) sim.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y; })
      .on('drag', (e, d) => { d.fx = e.x; d.fy = e.y; })
      .on('end', (e, d) => { if (!e.active) sim.alphaTarget(0); d.fx = null; d.fy = null; })
    );

  node.append('circle').attr('r', nodeRadius).attr('fill', '#238636').attr('stroke', '#30363d');
  node.append('text').attr('dy', d => nodeRadius(d) + 16).text(d => d.label);

  const sim = d3.forceSimulation(data.nodes)
    .force('link', d3.forceLink(data.edges).id(d => d.id).distance(150))
    .force('charge', d3.forceManyBody().strength(-400))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('collision', d3.forceCollide().radius(d => nodeRadius(d) + 10))
    .on('tick', () => {
      link.attr('x1', d => d.source.x).attr('y1', d => d.source.y).attr('x2', d => d.target.x).attr('y2', d => d.target.y);
      node.attr('transform', d => 'translate(' + d.x + ',' + d.y + ')');
    });

  node.on('click', (e, d) => {
    const detail = document.getElementById('detail');
    detail.className = 'service-detail active';
    document.getElementById('detail-name').textContent = d.label;
    document.getElementById('detail-summary').textContent = d.summary || 'No summary available';
    document.getElementById('detail-meta').innerHTML = '<span class="badge">' + (d.fileCount || 0) + ' files</span><span class="badge">' + (d.sourceType || 'local') + '</span>';
    const deps = data.edges.filter(e => e.source.id === d.id || e.target.id === d.id);
    if (deps.length > 0) {
      document.getElementById('detail-deps').innerHTML = '<strong>Connections:</strong><br>' +
        deps.map(e => { const other = e.source.id === d.id ? e.target.label : e.source.label; const dir = e.source.id === d.id ? '\u2192' : '\u2190'; return dir + ' ' + other + ' (' + e.type + ')'; }).join('<br>');
    } else { document.getElementById('detail-deps').textContent = 'No connections'; }
    link.classed('highlighted', l => l.source.id === d.id || l.target.id === d.id);
  });

  document.getElementById('search').addEventListener('input', function() {
    const q = this.value.toLowerCase();
    node.style('opacity', d => !q || d.label.toLowerCase().includes(q) || (d.summary || '').toLowerCase().includes(q) ? 1 : 0.15);
    link.style('opacity', l => { if (!q) return 1; return l.source.label.toLowerCase().includes(q) || l.target.label.toLowerCase().includes(q) ? 1 : 0.05; });
  });
}
</script>
</body>
</html>`
