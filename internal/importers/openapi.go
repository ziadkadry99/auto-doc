package importers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseOpenAPI parses an OpenAPI specification (JSON or YAML) and extracts endpoints.
func ParseOpenAPI(content string) ([]OpenAPIEndpoint, error) {
	var spec map[string]interface{}

	// Try JSON first, then YAML.
	if err := json.Unmarshal([]byte(content), &spec); err != nil {
		if err := yaml.Unmarshal([]byte(content), &spec); err != nil {
			return nil, fmt.Errorf("parsing OpenAPI spec: not valid JSON or YAML")
		}
	}

	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no paths found in OpenAPI spec")
	}

	var endpoints []OpenAPIEndpoint
	methods := []string{"get", "post", "put", "patch", "delete", "head", "options"}

	// Sort paths for deterministic output.
	pathKeys := make([]string, 0, len(paths))
	for k := range paths {
		pathKeys = append(pathKeys, k)
	}
	sort.Strings(pathKeys)

	for _, path := range pathKeys {
		pathItem, ok := paths[path].(map[string]interface{})
		if !ok {
			continue
		}

		for _, method := range methods {
			op, ok := pathItem[method].(map[string]interface{})
			if !ok {
				continue
			}

			ep := OpenAPIEndpoint{
				Path:   path,
				Method: strings.ToUpper(method),
			}

			if s, ok := op["summary"].(string); ok {
				ep.Summary = s
			}
			if d, ok := op["description"].(string); ok {
				ep.Description = d
			}

			// Extract parameters.
			if params, ok := op["parameters"].([]interface{}); ok {
				for _, p := range params {
					if pm, ok := p.(map[string]interface{}); ok {
						name, _ := pm["name"].(string)
						in, _ := pm["in"].(string)
						if name != "" {
							ep.Parameters = append(ep.Parameters, fmt.Sprintf("%s (in %s)", name, in))
						}
					}
				}
			}

			// Extract responses.
			if responses, ok := op["responses"].(map[string]interface{}); ok {
				ep.Responses = make(map[string]string)
				for code, resp := range responses {
					if rm, ok := resp.(map[string]interface{}); ok {
						desc, _ := rm["description"].(string)
						ep.Responses[code] = desc
					}
				}
			}

			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, nil
}

// FormatEndpointsMarkdown formats OpenAPI endpoints as a markdown table.
func FormatEndpointsMarkdown(endpoints []OpenAPIEndpoint) string {
	if len(endpoints) == 0 {
		return "No endpoints found."
	}

	var sb strings.Builder
	sb.WriteString("| Method | Path | Summary |\n")
	sb.WriteString("|--------|------|---------|\n")

	for _, ep := range endpoints {
		summary := ep.Summary
		if summary == "" {
			summary = ep.Description
		}
		if len(summary) > 80 {
			summary = summary[:77] + "..."
		}
		fmt.Fprintf(&sb, "| %s | `%s` | %s |\n", ep.Method, ep.Path, summary)
	}

	return sb.String()
}
