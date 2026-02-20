package indexer

import (
	"fmt"

	"github.com/ziadkadry99/auto-doc/internal/config"
	"github.com/ziadkadry99/auto-doc/internal/llm"
)

const systemPrompt = `You are a senior software engineer performing a code review. Analyze the provided source code file and return a structured JSON response. Be precise and factual. Do not invent details that are not present in the code.`

const litePromptTemplate = `Analyze this %s file and return a JSON object with exactly these fields:

{
  "skip": false,
  "summary": "2-3 sentence summary of what this file does. Include concrete values like port numbers, routes, and service names.",
  "purpose": "One sentence describing the file's role in the project",
  "dependencies": [{"name": "package or service name", "type": "import|api_call|grpc|database|event"}]
}

Set "skip" to true if this file is NOT relevant to understanding the project's architecture or functionality — for example: .gitignore, lock files, generated code, changelog entries, license files, CI configs, editor configs, or other boilerplate that adds no insight. When skip is true, you can leave the other fields empty.

IMPORTANT for configuration files (YAML, .properties, .env, JSON config, TOML): In the summary, list the actual configured values (topic names, URLs, ports, feature flags) — do NOT just describe them generically.

Do NOT list shell commands (ls, cd, mkdir, cp) as dependencies.

File path: %s

` + "```%s\n%s\n```"

const normalPromptTemplate = `Analyze this %s file and return a JSON object with exactly these fields:

{
  "skip": false,
  "summary": "2-3 sentence summary of what this file does. IMPORTANT: Include specific concrete values found in the code such as port numbers (e.g. 'listens on port 8080'), HTTP routes (e.g. 'exposes /cart, /checkout'), environment variable names (e.g. 'reads REDIS_ADDR'), gRPC service names, and database connection details.",
  "purpose": "One sentence describing the file's role in the project",
  "functions": [
    {
      "name": "function name",
      "signature": "full function signature",
      "summary": "What this function does. Include specific values: ports, routes, env vars, service addresses.",
      "parameters": [{"name": "param", "type": "type", "description": "what it is"}],
      "returns": "return type and meaning",
      "line_start": 0,
      "line_end": 0
    }
  ],
  "classes": [
    {
      "name": "class/struct/interface name",
      "summary": "What this type represents",
      "methods": [],
      "fields": [{"name": "field", "type": "type", "description": "what it stores"}],
      "line_start": 0,
      "line_end": 0
    }
  ],
  "dependencies": [{"name": "package or service name", "type": "import|api_call|grpc|database|event"}],
  "key_logic": ["Description of important algorithm or business logic. Include specific values: validation rules, port numbers, route paths, config keys."]
}

Set "skip" to true if this file is NOT relevant to understanding the project's architecture or functionality — for example: .gitignore, lock files, generated code, changelog entries, license files, CI configs, editor configs, or other boilerplate that adds no insight. When skip is true, you can leave the other fields empty.

IMPORTANT for configuration files (YAML, .properties, .env, JSON config, TOML):
- In key_logic, list EVERY important key-value pair with its actual configured value — e.g. "topics.pcch = product-event-emitter-pcch-choice-id-transactional-v2-avro"
- Do NOT summarize config values. The actual values (topic names, URLs, ports, feature flags, connection strings) are the most useful content for search.
- Group related config entries together in a single key_logic entry if they belong to the same section.

IMPORTANT for dependencies:
- Use type "grpc" for gRPC service calls (e.g. ProductCatalogService, CurrencyService)
- Use type "api_call" for HTTP/REST API calls to other services
- Use type "database" for database connections (Redis, PostgreSQL, etc.)
- Use type "import" for library/package imports
- Do NOT list shell commands (ls, cd, mkdir, cp, echo) as dependencies

Omit empty arrays. Set line numbers to 0 if unknown.

File path: %s

` + "```%s\n%s\n```"

const maxPromptTemplate = `Perform a thorough analysis of this %s file and return a JSON object with exactly these fields:

{
  "skip": false,
  "summary": "Detailed 3-5 sentence summary of what this file does. IMPORTANT: Include specific concrete values found in the code such as port numbers, HTTP routes, environment variable names, gRPC service names, and database connection details.",
  "purpose": "Detailed description of the file's role, responsibilities, and how it fits in the project",
  "functions": [
    {
      "name": "function name",
      "signature": "full function signature",
      "summary": "Detailed description including edge cases and error handling. Include specific values: ports, routes, env vars.",
      "parameters": [{"name": "param", "type": "type", "description": "detailed description"}],
      "returns": "return type, meaning, and possible error conditions",
      "line_start": 0,
      "line_end": 0
    }
  ],
  "classes": [
    {
      "name": "class/struct/interface name",
      "summary": "Detailed description including design patterns and responsibilities",
      "methods": [],
      "fields": [{"name": "field", "type": "type", "description": "detailed purpose"}],
      "line_start": 0,
      "line_end": 0
    }
  ],
  "dependencies": [{"name": "package or service name", "type": "import|api_call|grpc|database|event"}],
  "key_logic": [
    "Detailed description of each important algorithm, business rule, error handling pattern, or cross-reference to other modules. Include concrete values: validation rules, port numbers, route paths, config keys."
  ]
}

IMPORTANT for dependencies:
- Use type "grpc" for gRPC service calls (e.g. ProductCatalogService, CurrencyService)
- Use type "api_call" for HTTP/REST API calls to other services
- Use type "database" for database connections (Redis, PostgreSQL, etc.)
- Use type "import" for library/package imports
- Do NOT list shell commands (ls, cd, mkdir, cp, echo) as dependencies

IMPORTANT for configuration files (YAML, .properties, .env, JSON config, TOML):
- In key_logic, list EVERY important key-value pair with its actual configured value — e.g. "topics.pcch = product-event-emitter-pcch-choice-id-transactional-v2-avro"
- Do NOT summarize config values. The actual values (topic names, URLs, ports, feature flags, connection strings) are the most useful content for search.
- Group related config entries together in a single key_logic entry if they belong to the same section.

Set "skip" to true if this file is NOT relevant to understanding the project's architecture or functionality — for example: .gitignore, lock files, generated code, changelog entries, license files, CI configs, editor configs, or other boilerplate that adds no insight. When skip is true, you can leave the other fields empty.

Include all functions, methods, types, and significant constants. Document error handling patterns and edge cases. Note any cross-references to other files or modules. Omit empty arrays. Set line numbers to 0 if unknown.

File path: %s

` + "```%s\n%s\n```"

const fallbackPromptTemplate = `Summarize this source code file in 2-3 sentences. Return JSON: {"summary": "...", "purpose": "..."}

File path: %s

` + "```\n%s\n```"

// buildMessages constructs the LLM messages for analyzing a file.
func buildMessages(tier config.QualityTier, filePath string, content string, language string) []llm.Message {
	var userPrompt string
	switch tier {
	case config.QualityMax:
		userPrompt = fmt.Sprintf(maxPromptTemplate, language, filePath, language, content)
	case config.QualityNormal:
		userPrompt = fmt.Sprintf(normalPromptTemplate, language, filePath, language, content)
	default:
		userPrompt = fmt.Sprintf(litePromptTemplate, language, filePath, language, content)
	}

	return []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: userPrompt},
	}
}

// buildFallbackMessages constructs a simpler prompt for retry after parse failure.
func buildFallbackMessages(filePath string, content string) []llm.Message {
	userPrompt := fmt.Sprintf(fallbackPromptTemplate, filePath, content)
	return []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: userPrompt},
	}
}
