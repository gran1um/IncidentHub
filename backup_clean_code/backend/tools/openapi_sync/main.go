package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	specPath := filepath.Join(root, "internal", "api", "openapi.json")
	base := map[string]any{}
	if raw, readErr := os.ReadFile(specPath); readErr == nil {
		_ = json.Unmarshal(raw, &base)
	}

	generator, err := newOpenAPIGenerator(newModuleLoader(root, "incidenthub/backend"))
	if err != nil {
		panic(err)
	}
	spec, err := generator.BuildSpec(base)
	if err != nil {
		panic(err)
	}

	output, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		panic(err)
	}
	output = append(output, '\n')
	if err := os.WriteFile(specPath, output, 0o600); err != nil {
		panic(err)
	}
}
