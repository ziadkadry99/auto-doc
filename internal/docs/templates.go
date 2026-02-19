package docs

const fileDocTemplate = `# {{ .FilePath }}

{{ if .Summary }}## Summary

{{ .Summary }}
{{ end }}
{{ if .Purpose }}## Purpose

{{ .Purpose }}
{{ end }}
## Table of Contents
{{ if .Functions }}
### Functions
{{ range .Functions }}
- [{{ .Name }}](#{{ anchorize .Name }})
{{- end }}
{{ end }}
{{ if .Classes }}
### Types
{{ range .Classes }}
- [{{ .Name }}](#{{ anchorize .Name }})
{{- end }}
{{ end }}
{{ if .Functions }}## Functions
{{ range .Functions }}
### {{ .Name }}

` + "```" + `
{{ .Signature }}
` + "```" + `

{{ .Summary }}
{{ if .Parameters }}
**Parameters:**

| Name | Type | Description |
|------|------|-------------|
{{ range .Parameters }}| {{ .Name }} | {{ code .Type }} | {{ .Description }} |
{{ end }}
{{- end }}
{{ if .Returns }}**Returns:** {{ .Returns }}
{{ end }}
{{ if and .LineStart .LineEnd }}*Lines: {{ .LineStart }}-{{ .LineEnd }}*
{{ end }}
---
{{ end }}
{{ end }}
{{ if .Classes }}## Types
{{ range .Classes }}
### {{ .Name }}

{{ .Summary }}
{{ if .Fields }}
**Fields:**

| Name | Type | Description |
|------|------|-------------|
{{ range .Fields }}| {{ .Name }} | {{ code .Type }} | {{ .Description }} |
{{ end }}
{{- end }}
{{ if .Methods }}
**Methods:**
{{ range .Methods }}
#### {{ .Name }}

` + "```" + `
{{ .Signature }}
` + "```" + `

{{ .Summary }}
{{ end }}
{{- end }}
{{ if and .LineStart .LineEnd }}*Lines: {{ .LineStart }}-{{ .LineEnd }}*
{{ end }}
---
{{ end }}
{{ end }}
{{ if .Dependencies }}## Dependencies

| Name | Type |
|------|------|
{{ range .Dependencies }}| {{ .Name }} | {{ .Type }} |
{{ end }}
{{- end }}
{{ if .KeyLogic }}## Key Business Logic

{{ range .KeyLogic }}- {{ . }}
{{ end }}
{{- end }}
`

const architectureTemplate = `# Architecture Overview

## Overview

{{ .Overview }}

{{ if .Languages }}## Languages and Services

{{ .Languages }}
{{ end }}
{{ if .Components }}## Components
{{ range .Components }}
### {{ .Name }}

{{ .Description }}
{{ end }}
{{- end }}
{{ if .ServiceDependencies }}## Service Dependencies

{{ .ServiceDependencies }}
{{ end }}
{{ if .CriticalPath }}## Critical Path and Failure Analysis

{{ .CriticalPath }}
{{ end }}
{{ if .EntryPoints }}## Entry Points

| Name | Type | Description |
|------|------|-------------|
{{ range .EntryPoints }}| {{ .Name }} | {{ .Type }} | {{ .Description }} |
{{ end }}
{{- end }}
{{ if .ExitPoints }}## Exit Points

| Name | Type | Description |
|------|------|-------------|
{{ range .ExitPoints }}| {{ .Name }} | {{ .Type }} | {{ .Description }} |
{{ end }}
{{- end }}
{{ if .DataFlow }}## Data Flow

{{ .DataFlow }}
{{ end }}
{{ if .DesignPatterns }}## Design Patterns

{{ range .DesignPatterns }}- {{ . }}
{{ end }}
{{- end }}
{{ if .ArchDiagram }}## Architecture Diagram

<div class="arch-diagram" data-graph='{{ jsonattr .ArchDiagram }}'></div>
{{ end }}
{{ if .DepDiagram }}## Dependency Diagram

<div class="arch-diagram" data-graph='{{ jsonattr .DepDiagram }}'></div>
{{ end }}
`

const indexTemplate = `# {{ .ProjectName }} — Documentation

{{ if .Summary }}{{ .Summary }}
{{ end }}
## Files

| File | Summary |
|------|---------|
{{ range .Files }}| [{{ .FilePath }}]({{ mdlink .FilePath }}) | {{ oneline .Summary }} |
{{ end }}
{{ if .QuickLinks }}## Quick Links

{{ range .QuickLinks }}- [{{ .Label }}]({{ .Href }})
{{ end }}
{{- end }}
`

const enhancedIndexTemplate = `# {{ .ProjectName }} — Documentation

{{ if .ProjectOverview }}## Overview

{{ .ProjectOverview }}
{{ end }}
{{ if .EntryPoints }}## Entry Points

| Name | Type | Description |
|------|------|-------------|
{{ range .EntryPoints }}| {{ .Name }} | {{ .Type }} | {{ .Description }} |
{{ end }}
{{- end }}
{{ if .ExitPoints }}## Exit Points

| Name | Type | Description |
|------|------|-------------|
{{ range .ExitPoints }}| {{ .Name }} | {{ .Type }} | {{ .Description }} |
{{ end }}
{{- end }}
{{ if .Usages }}## Usage Examples
{{ range .Usages }}
### {{ .Title }}

` + "```" + `
{{ .Command }}
` + "```" + `

{{ .Description }}
{{ end }}
{{- end }}
{{ if .ArchDiagram }}## Architecture

<div class="arch-diagram" data-graph='{{ jsonattr .ArchDiagram }}'></div>
{{ end }}
## Component Map

<iframe src="interactive-map.html" style="width:100%;height:600px;border:1px solid #30363d;border-radius:8px;" loading="lazy"></iframe>

<p><a href="interactive-map.html">Open full screen →</a></p>

{{ if .Features }}## Features

| Feature | Description |
|---------|-------------|
{{ range .Features }}| [{{ .Name }}](features/{{ .Slug }}.md) | {{ oneline .Description }} |
{{ end }}
{{- end }}
{{ if .DepDiagram }}## Dependency Graph

<div class="arch-diagram" data-graph='{{ jsonattr .DepDiagram }}'></div>
{{ end }}
## Quick Links

- [Architecture](architecture.md)
`

const featureTemplate = `# {{ .Feature.Name }}

{{ if .Feature.DetailedDescription }}{{ .Feature.DetailedDescription }}{{ else }}{{ .Feature.Description }}{{ end }}

## Files

| File | Summary |
|------|---------|
{{ range .Analyses }}| [{{ .FilePath }}](../{{ mdlink .FilePath }}) | {{ oneline .Summary }} |
{{ end }}
[Back to Home](../index.md)
`
