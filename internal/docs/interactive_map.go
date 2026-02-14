package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ziadkadry99/auto-doc/internal/indexer"
)

type mapNode struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Group    string   `json:"group"`
	Lang     string   `json:"lang"`
	Summary  string   `json:"summary"`
	Purpose  string   `json:"purpose"`
	Funcs    []string `json:"funcs"`
	Types    []string `json:"types"`
	Deps     []string `json:"deps"`
	Size     int      `json:"size"`
	DocLink  string   `json:"docLink"`
}

type mapEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type mapFeature struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Slug  string `json:"slug"`
}

type mapData struct {
	ProjectName string       `json:"projectName"`
	Nodes       []mapNode    `json:"nodes"`
	Edges       []mapEdge    `json:"edges"`
	Features    []mapFeature `json:"features"`
}

var featureColors = []string{
	"#4e79a7", "#f28e2b", "#e15759", "#76b7b2",
	"#59a14f", "#edc948", "#b07aa1", "#ff9da7",
	"#9c755f", "#bab0ac", "#86bcb6", "#8cd17d",
}

// GenerateInteractiveMap creates a self-contained HTML page with a D3.js
// force-directed graph showing files, their feature groups, and dependencies.
func (g *DocGenerator) GenerateInteractiveMap(analyses []indexer.FileAnalysis, features []Feature) error {
	data := buildMapData(analyses, features, filepath.Base(g.OutputDir))

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling map data: %w", err)
	}

	html := strings.Replace(interactiveMapHTML, "/*__GRAPH_DATA__*/null", string(jsonBytes), 1)

	docsDir := filepath.Join(g.OutputDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(docsDir, "interactive-map.html"), []byte(html), 0o644)
}

func buildMapData(analyses []indexer.FileAnalysis, features []Feature, projectName string) mapData {
	// Feature membership: file → feature name.
	fileFeature := make(map[string]string)
	for _, f := range features {
		for _, fp := range f.Files {
			fileFeature[fp] = f.Name
		}
	}

	// Package path → files (for internal edge resolution).
	pkgFiles := make(map[string][]string)
	for _, a := range analyses {
		dir := filepath.ToSlash(filepath.Dir(a.FilePath))
		pkgFiles[dir] = append(pkgFiles[dir], a.FilePath)
	}
	// Sort file lists for deterministic representative selection.
	for pkg := range pkgFiles {
		sort.Strings(pkgFiles[pkg])
	}

	// Build nodes.
	nodes := make([]mapNode, 0, len(analyses))
	for _, a := range analyses {
		label := filepath.Base(a.FilePath)
		if len(a.Classes) > 0 {
			label = a.Classes[0].Name
		}

		funcNames := make([]string, 0, len(a.Functions))
		for _, fn := range a.Functions {
			funcNames = append(funcNames, fn.Name)
		}
		typeNames := make([]string, 0, len(a.Classes))
		for _, c := range a.Classes {
			typeNames = append(typeNames, c.Name)
		}

		// Get deps from structured field, or fall back to parsing from summary.
		depNames := make([]string, 0)
		for _, d := range a.Dependencies {
			depNames = append(depNames, d.Name)
		}
		if len(depNames) == 0 {
			depNames = parseSummaryDeps(a.Summary)
		}

		group := fileFeature[a.FilePath]
		if group == "" {
			group = "Other"
		}

		nodes = append(nodes, mapNode{
			ID:      a.FilePath,
			Label:   label,
			Group:   group,
			Lang:    a.Language,
			Summary: a.Summary,
			Purpose: a.Purpose,
			Funcs:   funcNames,
			Types:   typeNames,
			Deps:    depNames,
			Size:    len(a.Functions) + len(a.Classes),
			DocLink: a.FilePath + ".html",
		})
	}

	// Build internal edges by matching dependency names to package paths.
	// To keep the graph readable:
	// - Only ONE edge per source file → target package (pick first file as representative)
	// - Only cross-feature edges (skip within-feature, they're trivially related)
	allPkgs := make([]string, 0, len(pkgFiles))
	for pkg := range pkgFiles {
		allPkgs = append(allPkgs, pkg)
	}

	seen := make(map[string]bool)
	edges := make([]mapEdge, 0)
	for _, a := range analyses {
		srcFeature := fileFeature[a.FilePath]

		// Get deps from structured field, or parse from summary.
		var depNames []string
		for _, d := range a.Dependencies {
			depNames = append(depNames, d.Name)
		}
		if len(depNames) == 0 {
			depNames = parseSummaryDeps(a.Summary)
		}

		matched := make(map[string]bool) // one edge per target package
		for _, depName := range depNames {
			for _, pkg := range allPkgs {
				if matched[pkg] || !depMatchesPkg(depName, pkg) {
					continue
				}
				// Pick a representative target file in this package
				// that is in a different feature than the source.
				var target string
				for _, t := range pkgFiles[pkg] {
					if t != a.FilePath && fileFeature[t] != srcFeature {
						target = t
						break
					}
				}
				if target == "" {
					continue
				}
				matched[pkg] = true
				key := a.FilePath + "|" + target
				if seen[key] {
					continue
				}
				seen[key] = true
				edges = append(edges, mapEdge{Source: a.FilePath, Target: target})
			}
		}
	}

	// Build feature list with colors.
	feats := make([]mapFeature, 0, len(features))
	for i, f := range features {
		feats = append(feats, mapFeature{
			Name:  f.Name,
			Color: featureColors[i%len(featureColors)],
			Slug:  f.Slug,
		})
	}
	// Add "Other" if any files lack a feature.
	for _, n := range nodes {
		if n.Group == "Other" {
			feats = append(feats, mapFeature{Name: "Other", Color: "#8b949e", Slug: "other"})
			break
		}
	}

	return mapData{ProjectName: projectName, Nodes: nodes, Edges: edges, Features: feats}
}

// parseSummaryDeps extracts dependency names from the summary text.
// Handles the lite tier format where deps appear as:
// "Dependencies: pkg1 (import), pkg2 (api_call), ..."
func parseSummaryDeps(summary string) []string {
	idx := strings.Index(summary, "Dependencies:")
	if idx < 0 {
		return nil
	}
	after := summary[idx+len("Dependencies:"):]
	// Trim at newline or next field marker.
	for _, marker := range []string{"\n", "Functions:", "Classes:", "Key Logic:"} {
		if i := strings.Index(after, marker); i >= 0 {
			after = after[:i]
		}
	}
	var deps []string
	for _, part := range strings.Split(after, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Remove type suffix like " (import)"
		if i := strings.Index(part, " ("); i > 0 {
			part = part[:i]
		}
		deps = append(deps, strings.TrimSpace(part))
	}
	return deps
}

// depMatchesPkg returns true if the dependency name plausibly refers to pkgPath.
func depMatchesPkg(depName, pkgPath string) bool {
	if depName == pkgPath {
		return true
	}
	if strings.HasSuffix(depName, "/"+pkgPath) {
		return true
	}
	base := filepath.Base(pkgPath)
	if base != "." && depName == base {
		return true
	}
	return false
}

const interactiveMapHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Component Map</title>
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
#search{background:var(--bg3);border:1px solid var(--bd);color:var(--tx);padding:5px 10px;border-radius:6px;font-size:13px;width:220px;outline:none}
#search:focus{border-color:var(--ac)}
#stats{font-size:12px;color:var(--tx2);white-space:nowrap}
.btn{background:var(--bg3);border:1px solid var(--bd);color:var(--tx);padding:4px 10px;border-radius:6px;font-size:12px;cursor:pointer}
.btn:hover{background:var(--bd)}
#main{display:flex;height:calc(100vh - 48px)}
#sidebar{width:220px;min-width:220px;background:var(--bg2);border-right:1px solid var(--bd);overflow-y:auto;padding:12px;flex-shrink:0}
.sidebar-hdr{font-size:11px;font-weight:600;text-transform:uppercase;color:var(--tx2);margin-bottom:8px;letter-spacing:.5px}
.feat-item{display:flex;align-items:center;gap:6px;padding:4px 0;cursor:pointer;font-size:13px}
.feat-item input{margin:0;cursor:pointer}
.feat-color{width:10px;height:10px;border-radius:50%;flex-shrink:0}
.feat-name{flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.feat-count{font-size:11px;color:var(--tx2)}
.sidebar-stats{margin-top:16px;padding-top:12px;border-top:1px solid var(--bd);font-size:12px;color:var(--tx2);line-height:1.8}
#graph-container{flex:1;position:relative;overflow:hidden}
#graph-container svg{width:100%;height:100%;display:block}
#info-panel{width:320px;min-width:320px;background:var(--bg2);border-left:1px solid var(--bd);overflow-y:auto;padding:16px;flex-shrink:0;transition:margin-right .2s;position:relative}
#info-panel.hidden{display:none}
#info-close{position:absolute;top:8px;right:8px;background:none;border:none;color:var(--tx2);font-size:20px;cursor:pointer;line-height:1}
#info-close:hover{color:var(--tx)}
.info-label{font-size:16px;font-weight:600;margin-bottom:4px;word-break:break-word}
.info-path{font-size:12px;color:var(--tx2);margin-bottom:12px;word-break:break-all}
.info-badges{display:flex;gap:6px;margin-bottom:12px;flex-wrap:wrap}
.badge{font-size:11px;padding:2px 8px;border-radius:10px;font-weight:500}
.badge-feat{border:1px solid}
.badge-lang{background:var(--bg3);color:var(--tx)}
.info-section{margin-bottom:14px}
.info-section-title{font-size:11px;font-weight:600;text-transform:uppercase;color:var(--tx2);margin-bottom:4px;letter-spacing:.5px}
.info-text{font-size:13px;line-height:1.5;color:var(--tx)}
.info-list{list-style:none;padding:0}
.info-list li{font-size:13px;padding:2px 0;color:var(--tx)}
.info-list li::before{content:"";display:inline-block;width:4px;height:4px;background:var(--ac);border-radius:50%;margin-right:8px;vertical-align:middle}
.conn-link{color:var(--ac);cursor:pointer;font-size:13px;padding:2px 0;display:block}
.conn-link:hover{text-decoration:underline}
.info-doclink{display:inline-block;margin-top:10px;color:var(--ac);text-decoration:none;font-size:13px;font-weight:500}
.info-doclink:hover{text-decoration:underline}
#tooltip{position:fixed;pointer-events:none;background:var(--bg2);border:1px solid var(--bd);border-radius:8px;padding:10px 14px;font-size:12px;max-width:320px;box-shadow:0 4px 12px rgba(0,0,0,.4);z-index:100;transition:opacity .1s}
#tooltip.hidden{opacity:0;visibility:hidden}
.tip-label{font-weight:600;font-size:13px;margin-bottom:2px}
.tip-path{color:var(--tx2);margin-bottom:4px;word-break:break-all}
.tip-summary{line-height:1.4;display:-webkit-box;-webkit-line-clamp:3;-webkit-box-orient:vertical;overflow:hidden}
.edge{stroke:var(--tx2);stroke-opacity:.15;fill:none}
.edge.highlighted{stroke-opacity:.7;stroke-width:1.5px}
.node{cursor:pointer;stroke-width:1.5px}
.node-label{font-size:10px;fill:var(--tx);pointer-events:none;text-anchor:middle;dominant-baseline:central}
.dimmed{opacity:.08!important}
</style>
</head>
<body class="dark">
<div id="toolbar">
 <div class="toolbar-section"><a href="index.html" class="back-link">&#8592; Back to Docs</a><span class="title">Component Map</span></div>
 <div class="toolbar-section"><input type="text" id="search" placeholder="Search files, types..."></div>
 <div class="toolbar-section"><span id="stats"></span><button class="btn" id="btn-fit">Fit</button><button class="btn" id="btn-labels">Labels</button><button class="btn" id="btn-theme">&#9788;</button></div>
</div>
<div id="main">
 <div id="sidebar"><div class="sidebar-hdr">Features</div><div id="feature-list"></div><div class="sidebar-stats" id="sidebar-stats"></div></div>
 <div id="graph-container"><svg id="graph"></svg></div>
 <div id="info-panel" class="hidden"><button id="info-close">&times;</button><div id="info-content"></div></div>
</div>
<div id="tooltip" class="hidden"></div>
<script src="https://d3js.org/d3.v7.min.js"></script>
<script>
(function(){
var data = /*__GRAPH_DATA__*/null;
if(!data||typeof d3==='undefined'){document.getElementById('graph-container').innerHTML='<div style="padding:40px;color:var(--tx2)">Could not load visualization. Ensure internet access for D3.js.</div>';return;}

var showLabels=true, selectedId=null;
var colorMap={};data.features.forEach(function(f){colorMap[f.name]=f.color;});
var slugMap={};data.features.forEach(function(f){slugMap[f.name]=f.slug;});
var nodeMap={};data.nodes.forEach(function(n){nodeMap[n.id]=n;});
var hiddenFeats={};

var svgEl=document.getElementById('graph');
var width=svgEl.clientWidth, height=svgEl.clientHeight;
var svg=d3.select(svgEl);
var container=svg.append('g');

var zoom=d3.zoom().scaleExtent([0.05,10]).on('zoom',function(e){container.attr('transform',e.transform);});
svg.call(zoom);

// Arrow markers
var defs=svg.append('defs');
data.features.forEach(function(f){
 defs.append('marker').attr('id','arr-'+f.slug).attr('viewBox','0 -4 8 8').attr('refX',12).attr('refY',0).attr('markerWidth',6).attr('markerHeight',6).attr('orient','auto')
  .append('path').attr('d','M0,-3L6,0L0,3').attr('fill',f.color).attr('opacity',0.6);
});

// Size scale
var maxSize=d3.max(data.nodes,function(d){return d.size;})||1;
var sizeScale=d3.scaleSqrt().domain([0,maxSize||1]).range([4,14]);

// Initial positions by feature angle
var featAngle={};data.features.forEach(function(f,i){featAngle[f.name]=(i/data.features.length)*2*Math.PI;});
var initR=Math.min(width,height)*0.3;
data.nodes.forEach(function(d){
 var a=featAngle[d.group]||0;
 d.x=width/2+initR*Math.cos(a)+(Math.random()-0.5)*80;
 d.y=height/2+initR*Math.sin(a)+(Math.random()-0.5)*80;
});

// Force simulation
function clusterForce(strength){
 var nodes;
 function force(alpha){
  var cx={},cy={},ct={};
  nodes.forEach(function(d){if(!cx[d.group]){cx[d.group]=0;cy[d.group]=0;ct[d.group]=0;}cx[d.group]+=d.x;cy[d.group]+=d.y;ct[d.group]++;});
  Object.keys(cx).forEach(function(g){cx[g]/=ct[g];cy[g]/=ct[g];});
  nodes.forEach(function(d){var g=d.group;if(cx[g]!==undefined){d.vx+=(cx[g]-d.x)*strength*alpha;d.vy+=(cy[g]-d.y)*strength*alpha;}});
 }
 force.initialize=function(_){nodes=_;};
 return force;
}

var sim=d3.forceSimulation(data.nodes)
 .force('link',d3.forceLink(data.edges).id(function(d){return d.id;}).distance(80).strength(0.3))
 .force('charge',d3.forceManyBody().strength(-180))
 .force('center',d3.forceCenter(width/2,height/2))
 .force('collision',d3.forceCollide().radius(function(d){return sizeScale(d.size)+3;}))
 .force('cluster',clusterForce(0.25))
 .alphaDecay(0.02);

// Draw edges
var edgeG=container.append('g');
var edgeEls=edgeG.selectAll('line').data(data.edges).join('line')
 .attr('class','edge')
 .attr('stroke-width',1)
 .attr('marker-end',function(d){var src=typeof d.source==='object'?d.source:nodeMap[d.source];var sl=src?slugMap[src.group]:'';return sl?'url(#arr-'+sl+')':'';});

// Draw nodes
var nodeG=container.append('g');
var nodeEls=nodeG.selectAll('circle').data(data.nodes).join('circle')
 .attr('class','node')
 .attr('r',function(d){return sizeScale(d.size);})
 .attr('fill',function(d){return colorMap[d.group]||'#8b949e';})
 .attr('stroke',function(d){var c=colorMap[d.group]||'#8b949e';return d3.color(c).darker(0.5).toString();})
 .on('mouseover',onHover).on('mousemove',moveTooltip).on('mouseout',onHoverOut).on('click',onClick)
 .call(d3.drag().on('start',function(e,d){if(!e.active)sim.alphaTarget(0.3).restart();d.fx=d.x;d.fy=d.y;})
  .on('drag',function(e,d){d.fx=e.x;d.fy=e.y;})
  .on('end',function(e,d){if(!e.active)sim.alphaTarget(0);d.fx=null;d.fy=null;}));

// Draw labels
var labelG=container.append('g');
var labelEls=labelG.selectAll('text').data(data.nodes).join('text')
 .attr('class','node-label')
 .text(function(d){return d.label;})
 .attr('dy',function(d){return sizeScale(d.size)+10;});

sim.on('tick',function(){
 edgeEls.attr('x1',function(d){return d.source.x;}).attr('y1',function(d){return d.source.y;}).attr('x2',function(d){return d.target.x;}).attr('y2',function(d){return d.target.y;});
 nodeEls.attr('cx',function(d){return d.x;}).attr('cy',function(d){return d.y;});
 labelEls.attr('x',function(d){return d.x;}).attr('y',function(d){return d.y;});
});

// Tooltip
var tip=document.getElementById('tooltip');
function esc(t){if(!t)return '';var d=document.createElement('div');d.appendChild(document.createTextNode(t));return d.innerHTML;}
function onHover(event,d){
 if(selectedId)return;
 highlightConnected(d);
 var s=d.summary||'';if(s.length>150)s=s.substring(0,150)+'...';
 tip.innerHTML='<div class="tip-label">'+esc(d.label)+'</div><div class="tip-path">'+esc(d.id)+'</div><div class="tip-summary">'+esc(s)+'</div>';
 tip.classList.remove('hidden');
 moveTooltip(event);
}
function moveTooltip(event){
 var x=event.clientX+14,y=event.clientY-14;
 var r=tip.getBoundingClientRect();
 if(x+r.width>window.innerWidth)x=event.clientX-r.width-14;
 if(y+r.height>window.innerHeight)y=event.clientY-r.height-14;
 if(y<0)y=4;
 tip.style.left=x+'px';tip.style.top=y+'px';
}
function onHoverOut(){if(!selectedId){resetHighlight();}tip.classList.add('hidden');}

function highlightConnected(d){
 var conn={};conn[d.id]=true;
 data.edges.forEach(function(e){
  var s=typeof e.source==='object'?e.source.id:e.source;
  var t=typeof e.target==='object'?e.target.id:e.target;
  if(s===d.id)conn[t]=true;if(t===d.id)conn[s]=true;
 });
 nodeEls.classed('dimmed',function(n){return !conn[n.id];});
 edgeEls.classed('dimmed',function(e){var s=typeof e.source==='object'?e.source.id:e.source;var t=typeof e.target==='object'?e.target.id:e.target;return s!==d.id&&t!==d.id;});
 edgeEls.classed('highlighted',function(e){var s=typeof e.source==='object'?e.source.id:e.source;var t=typeof e.target==='object'?e.target.id:e.target;return s===d.id||t===d.id;});
 labelEls.classed('dimmed',function(n){return !conn[n.id];});
}
function resetHighlight(){nodeEls.classed('dimmed',false);edgeEls.classed('dimmed',false).classed('highlighted',false);labelEls.classed('dimmed',false);}

// Click → info panel
function onClick(event,d){
 event.stopPropagation();
 selectedId=d.id;
 highlightConnected(d);
 tip.classList.add('hidden');
 showInfo(d);
}
svg.on('click',function(){selectedId=null;resetHighlight();document.getElementById('info-panel').classList.add('hidden');});

function showInfo(d){
 var h='';
 h+='<div class="info-label">'+esc(d.label)+'</div>';
 h+='<div class="info-path">'+esc(d.id)+'</div>';
 h+='<div class="info-badges">';
 h+='<span class="badge badge-feat" style="color:'+colorMap[d.group]+';border-color:'+colorMap[d.group]+'">'+esc(d.group)+'</span>';
 if(d.lang)h+='<span class="badge badge-lang">'+esc(d.lang)+'</span>';
 h+='</div>';
 if(d.summary){h+='<div class="info-section"><div class="info-section-title">Summary</div><div class="info-text">'+esc(d.summary)+'</div></div>';}
 if(d.purpose&&d.purpose!==d.summary){h+='<div class="info-section"><div class="info-section-title">Purpose</div><div class="info-text">'+esc(d.purpose)+'</div></div>';}
 if(d.types&&d.types.length){h+='<div class="info-section"><div class="info-section-title">Types</div><ul class="info-list">';d.types.forEach(function(t){h+='<li>'+esc(t)+'</li>';});h+='</ul></div>';}
 if(d.funcs&&d.funcs.length){h+='<div class="info-section"><div class="info-section-title">Functions</div><ul class="info-list">';d.funcs.forEach(function(f){h+='<li>'+esc(f)+'</li>';});h+='</ul></div>';}
 // Connections
 var outgoing=[],incoming=[];
 data.edges.forEach(function(e){
  var s=typeof e.source==='object'?e.source.id:e.source;
  var t=typeof e.target==='object'?e.target.id:e.target;
  if(s===d.id&&t!==d.id)outgoing.push(t);
  if(t===d.id&&s!==d.id)incoming.push(s);
 });
 if(outgoing.length){h+='<div class="info-section"><div class="info-section-title">Depends On ('+outgoing.length+')</div>';outgoing.forEach(function(id){var n=nodeMap[id];var lbl=n?n.label:id;h+='<span class="conn-link" data-id="'+esc(id)+'">'+esc(lbl)+' <small style="color:var(--tx2)">'+esc(id)+'</small></span>';});h+='</div>';}
 if(incoming.length){h+='<div class="info-section"><div class="info-section-title">Used By ('+incoming.length+')</div>';incoming.forEach(function(id){var n=nodeMap[id];var lbl=n?n.label:id;h+='<span class="conn-link" data-id="'+esc(id)+'">'+esc(lbl)+' <small style="color:var(--tx2)">'+esc(id)+'</small></span>';});h+='</div>';}
 if(d.deps&&d.deps.length){h+='<div class="info-section"><div class="info-section-title">External Deps</div><ul class="info-list">';d.deps.forEach(function(dep){h+='<li>'+esc(dep)+'</li>';});h+='</ul></div>';}
 h+='<a class="info-doclink" href="'+esc(d.docLink)+'">View Documentation &#8594;</a>';
 var panel=document.getElementById('info-panel');
 document.getElementById('info-content').innerHTML=h;
 panel.classList.remove('hidden');
 // Connection click handlers
 panel.querySelectorAll('.conn-link').forEach(function(el){
  el.addEventListener('click',function(){
   var id=el.getAttribute('data-id');
   var n=data.nodes.find(function(nd){return nd.id===id;});
   if(n){selectedId=n.id;highlightConnected(n);showInfo(n);}
  });
 });
}
document.getElementById('info-close').addEventListener('click',function(){selectedId=null;resetHighlight();document.getElementById('info-panel').classList.add('hidden');});

// Feature sidebar
function buildFeatureList(){
 var el=document.getElementById('feature-list');
 var h='';
 data.features.forEach(function(f){
  var cnt=data.nodes.filter(function(n){return n.group===f.name;}).length;
  h+='<label class="feat-item"><input type="checkbox" checked data-feat="'+esc(f.name)+'">';
  h+='<span class="feat-color" style="background:'+f.color+'"></span>';
  h+='<span class="feat-name" title="'+esc(f.name)+'">'+esc(f.name)+'</span>';
  h+='<span class="feat-count">'+cnt+'</span></label>';
 });
 el.innerHTML=h;
 el.addEventListener('change',function(e){
  if(e.target.type==='checkbox'){
   var feat=e.target.getAttribute('data-feat');
   if(e.target.checked){delete hiddenFeats[feat];}else{hiddenFeats[feat]=true;}
   applyVisibility();
  }
 });
}
function applyVisibility(){
 nodeEls.style('display',function(d){return hiddenFeats[d.group]?'none':null;});
 labelEls.style('display',function(d){return hiddenFeats[d.group]?'none':null;});
 edgeEls.style('display',function(e){
  var s=typeof e.source==='object'?e.source:nodeMap[e.source];
  var t=typeof e.target==='object'?e.target:nodeMap[e.target];
  return (s&&hiddenFeats[s.group])||(t&&hiddenFeats[t.group])?'none':null;
 });
}

// Search
document.getElementById('search').addEventListener('input',function(){
 var q=this.value.toLowerCase().trim();
 if(!q){nodeEls.classed('dimmed',false).attr('r',function(d){return sizeScale(d.size);});labelEls.classed('dimmed',false);edgeEls.classed('dimmed',false);return;}
 var matches={};
 data.nodes.forEach(function(n){
  var hay=(n.id+' '+n.label+' '+(n.types||[]).join(' ')+' '+(n.funcs||[]).join(' ')).toLowerCase();
  if(hay.indexOf(q)>=0)matches[n.id]=true;
 });
 nodeEls.classed('dimmed',function(d){return !matches[d.id];}).attr('r',function(d){return matches[d.id]?sizeScale(d.size)+3:sizeScale(d.size);});
 labelEls.classed('dimmed',function(d){return !matches[d.id];});
 edgeEls.classed('dimmed',function(e){var s=typeof e.source==='object'?e.source.id:e.source;var t=typeof e.target==='object'?e.target.id:e.target;return !matches[s]&&!matches[t];});
});

// Controls
document.getElementById('btn-fit').addEventListener('click',zoomToFit);
document.getElementById('btn-labels').addEventListener('click',function(){showLabels=!showLabels;labelEls.style('display',showLabels?null:'none');});
document.getElementById('btn-theme').addEventListener('click',function(){
 document.body.classList.toggle('light');document.body.classList.toggle('dark');
 this.textContent=document.body.classList.contains('light')?'\u263E':'\u2606';
});

function zoomToFit(){
 var bounds=container.node().getBBox();
 if(bounds.width===0||bounds.height===0)return;
 var fw=svgEl.clientWidth,fh=svgEl.clientHeight;
 var scale=0.85/Math.max(bounds.width/fw,bounds.height/fh);
 if(scale>4)scale=4;
 var tx=fw/2-scale*(bounds.x+bounds.width/2);
 var ty=fh/2-scale*(bounds.y+bounds.height/2);
 svg.transition().duration(750).call(zoom.transform,d3.zoomIdentity.translate(tx,ty).scale(scale));
}

// Stats
function updateStats(){
 document.getElementById('stats').textContent=data.nodes.length+' files \u00B7 '+data.edges.length+' connections';
 document.getElementById('sidebar-stats').innerHTML='<div>'+data.nodes.length+' files</div><div>'+data.edges.length+' connections</div><div>'+data.features.length+' features</div>';
}

// Resize
window.addEventListener('resize',function(){
 width=svgEl.clientWidth;height=svgEl.clientHeight;
 sim.force('center',d3.forceCenter(width/2,height/2));sim.alpha(0.1).restart();
});

// Boot
buildFeatureList();
updateStats();
setTimeout(zoomToFit,2000);
})();
</script>
</body>
</html>`
