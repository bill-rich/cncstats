// swagger2openapi converts docs/swagger.json to docs/openapi3.json and docs/openapi3.yaml.
// Run after `swag init` to keep OpenAPI v3 docs in sync.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	data, err := os.ReadFile("docs/swagger.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read docs/swagger.json: %v\n", err)
		os.Exit(1)
	}

	var swagger map[string]interface{}
	if err := json.Unmarshal(data, &swagger); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse swagger.json: %v\n", err)
		os.Exit(1)
	}

	openapi := convert(swagger)

	// Write JSON
	jsonOut, err := json.MarshalIndent(openapi, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal OpenAPI JSON: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("docs/openapi3.json", jsonOut, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write openapi3.json: %v\n", err)
		os.Exit(1)
	}

	// Write YAML (simple serializer — no dependency needed for this structure)
	yamlOut := toYAML(openapi, 0)
	if err := os.WriteFile("docs/openapi3.yaml", []byte(yamlOut), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write openapi3.yaml: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated docs/openapi3.json and docs/openapi3.yaml")
}

func convert(swagger map[string]interface{}) map[string]interface{} {
	info := swagger["info"]

	// Build server URL from host + basePath
	host, _ := swagger["host"].(string)
	basePath, _ := swagger["basePath"].(string)
	if host == "" {
		host = "localhost:8080"
	}
	if basePath == "" {
		basePath = "/"
	}
	serverURL := "http://" + host + basePath

	openapi := map[string]interface{}{
		"openapi": "3.0.3",
		"info":    info,
		"servers": []interface{}{
			map[string]interface{}{"url": serverURL},
		},
	}

	// Convert paths
	if paths, ok := swagger["paths"].(map[string]interface{}); ok {
		openapi["paths"] = convertPaths(paths)
	}

	// Convert definitions → components/schemas
	if defs, ok := swagger["definitions"].(map[string]interface{}); ok {
		schemas := make(map[string]interface{})
		for name, schema := range defs {
			schemas[name] = convertSchema(schema)
		}
		openapi["components"] = map[string]interface{}{
			"schemas": schemas,
		}
	}

	return openapi
}

func convertPaths(paths map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for path, methods := range paths {
		methodMap, ok := methods.(map[string]interface{})
		if !ok {
			continue
		}
		converted := make(map[string]interface{})
		for method, op := range methodMap {
			converted[method] = convertOperation(op)
		}
		result[path] = converted
	}
	return result
}

func convertOperation(op interface{}) map[string]interface{} {
	opMap, ok := op.(map[string]interface{})
	if !ok {
		return nil
	}

	result := make(map[string]interface{})

	// Copy simple fields
	for _, key := range []string{"summary", "description", "tags", "operationId"} {
		if v, ok := opMap[key]; ok {
			result[key] = v
		}
	}

	consumes := getStringSlice(opMap["consumes"])
	params, _ := opMap["parameters"].([]interface{})

	// Separate query/header/path params from body/formData params
	var queryHeaderParams []interface{}
	var bodyParam map[string]interface{}
	var formDataParams []map[string]interface{}

	for _, p := range params {
		param, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		in, _ := param["in"].(string)
		switch in {
		case "body":
			bodyParam = param
		case "formData":
			formDataParams = append(formDataParams, param)
		default: // query, header, path
			converted := map[string]interface{}{
				"name": param["name"],
				"in":   param["in"],
			}
			if d, ok := param["description"]; ok {
				converted["description"] = d
			}
			if r, ok := param["required"]; ok {
				converted["required"] = r
			}
			// Convert type to schema
			schema := map[string]interface{}{}
			if t, ok := param["type"]; ok {
				schema["type"] = t
			}
			if f, ok := param["format"]; ok {
				schema["format"] = f
			}
			if e, ok := param["enum"]; ok {
				schema["enum"] = e
			}
			converted["schema"] = schema
			queryHeaderParams = append(queryHeaderParams, converted)
		}
	}

	if len(queryHeaderParams) > 0 {
		result["parameters"] = queryHeaderParams
	}

	// Convert body/formData to requestBody
	if bodyParam != nil {
		contentType := "application/json"
		if len(consumes) > 0 {
			contentType = consumes[0]
		}
		schema := convertRef(bodyParam["schema"])
		result["requestBody"] = map[string]interface{}{
			"required": true,
			"content": map[string]interface{}{
				contentType: map[string]interface{}{
					"schema": schema,
				},
			},
		}
	} else if len(formDataParams) > 0 {
		properties := make(map[string]interface{})
		var required []interface{}
		for _, fp := range formDataParams {
			name, _ := fp["name"].(string)
			prop := make(map[string]interface{})
			if t, _ := fp["type"].(string); t == "file" {
				prop["type"] = "string"
				prop["format"] = "binary"
			} else if t != "" {
				prop["type"] = t
			}
			if d, ok := fp["description"]; ok {
				prop["description"] = d
			}
			properties[name] = prop
			if r, ok := fp["required"].(bool); ok && r {
				required = append(required, name)
			}
		}
		schema := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		result["requestBody"] = map[string]interface{}{
			"required": true,
			"content": map[string]interface{}{
				"multipart/form-data": map[string]interface{}{
					"schema": schema,
				},
			},
		}
	} else if len(consumes) > 0 && consumes[0] == "application/octet-stream" {
		// Raw binary body (like stats upload)
		result["requestBody"] = map[string]interface{}{
			"required": true,
			"content": map[string]interface{}{
				"application/octet-stream": map[string]interface{}{
					"schema": map[string]interface{}{
						"type":   "string",
						"format": "binary",
					},
				},
			},
		}
	}

	// Convert responses
	if responses, ok := opMap["responses"].(map[string]interface{}); ok {
		convertedResponses := make(map[string]interface{})
		for code, resp := range responses {
			respMap, ok := resp.(map[string]interface{})
			if !ok {
				continue
			}
			converted := map[string]interface{}{
				"description": respMap["description"],
			}
			if schema, ok := respMap["schema"]; ok {
				converted["content"] = map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": convertRef(schema),
					},
				}
			}
			convertedResponses[code] = converted
		}
		result["responses"] = convertedResponses
	}

	return result
}

func convertSchema(schema interface{}) interface{} {
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return schema
	}

	result := make(map[string]interface{})
	for k, v := range schemaMap {
		switch k {
		case "$ref":
			result["$ref"] = convertRefPath(v.(string))
		case "properties":
			if props, ok := v.(map[string]interface{}); ok {
				converted := make(map[string]interface{})
				for name, prop := range props {
					converted[name] = convertSchema(prop)
				}
				result["properties"] = converted
			}
		case "items":
			result["items"] = convertSchema(v)
		case "additionalProperties":
			result["additionalProperties"] = convertSchema(v)
		case "allOf":
			if arr, ok := v.([]interface{}); ok {
				var converted []interface{}
				for _, item := range arr {
					converted = append(converted, convertSchema(item))
				}
				result["allOf"] = converted
			}
		default:
			result[k] = v
		}
	}
	return result
}

func convertRef(schema interface{}) interface{} {
	if schema == nil {
		return nil
	}
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return schema
	}
	result := make(map[string]interface{})
	for k, v := range schemaMap {
		if k == "$ref" {
			result["$ref"] = convertRefPath(v.(string))
		} else {
			result[k] = v
		}
	}
	return result
}

func convertRefPath(ref string) string {
	// #/definitions/Foo → #/components/schemas/Foo
	return strings.Replace(ref, "#/definitions/", "#/components/schemas/", 1)
}

func getStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// toYAML is a minimal YAML serializer for JSON-compatible structures.
func toYAML(v interface{}, indent int) string {
	prefix := strings.Repeat("  ", indent)
	switch val := v.(type) {
	case map[string]interface{}:
		if len(val) == 0 {
			return "{}\n"
		}
		var b strings.Builder
		// Sort keys for deterministic output
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := val[k]
			switch child.(type) {
			case map[string]interface{}, []interface{}:
				b.WriteString(prefix + k + ":\n")
				b.WriteString(toYAML(child, indent+1))
			default:
				b.WriteString(prefix + k + ": " + yamlScalar(child) + "\n")
			}
		}
		return b.String()
	case []interface{}:
		if len(val) == 0 {
			return prefix + "[]\n"
		}
		var b strings.Builder
		for _, item := range val {
			switch item.(type) {
			case map[string]interface{}:
				lines := toYAML(item, indent+1)
				// First line gets "- " prefix
				trimmed := strings.TrimPrefix(lines, strings.Repeat("  ", indent+1))
				b.WriteString(prefix + "- " + trimmed)
			default:
				b.WriteString(prefix + "- " + yamlScalar(item) + "\n")
			}
		}
		return b.String()
	default:
		return prefix + yamlScalar(v) + "\n"
	}
}

func yamlScalar(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%g", val)
	case string:
		// Quote if it contains special chars or could be parsed as non-string
		if val == "" || val == "true" || val == "false" || val == "null" ||
			strings.ContainsAny(val, ":#{}[]|>&*!%@`'\",\n") {
			j, _ := json.Marshal(val)
			return string(j)
		}
		return val
	default:
		j, _ := json.Marshal(v)
		return string(j)
	}
}
