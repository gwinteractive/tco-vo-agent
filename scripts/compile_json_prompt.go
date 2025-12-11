package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <json-file> [output-file] [package-name]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s promts/f.json\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s promts/f.json output.go fraudagent\n", os.Args[0])
		os.Exit(1)
	}

	jsonFile := os.Args[1]

	// Read JSON file
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", jsonFile, err)
		os.Exit(1)
	}

	// Validate JSON by parsing it
	var jsonObj interface{}
	if err := json.Unmarshal(data, &jsonObj); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid JSON in %s: %v\n", jsonFile, err)
		os.Exit(1)
	}

	// Minify JSON (remove whitespace)
	minified, err := json.Marshal(jsonObj)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	// Escape for Go string literal
	// Use backticks for raw strings, but if backticks exist in JSON, use regular string with escaping
	jsonStr := string(minified)

	var output *os.File
	if len(os.Args) >= 3 {
		outputFile := os.Args[2]
		var err error
		output, err = os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file %s: %v\n", outputFile, err)
			os.Exit(1)
		}
		defer output.Close()
	} else {
		output = os.Stdout
	}

	// Determine package name
	packageName := "tco_vo_agent" // default
	if len(os.Args) >= 4 {
		// Package name explicitly provided
		packageName = os.Args[3]
	} else if len(os.Args) >= 3 {
		// Try to infer package from output file path
		outputPath := os.Args[2]
		if strings.Contains(outputPath, "cloudfunction") {
			packageName = "tco_vo_agent"
		} else if strings.Contains(outputPath, "src/") && !strings.Contains(outputPath, "cloudfunction") {
			packageName = "tco_vo_agent"
		}
	}

	// Write package declaration
	fmt.Fprintf(output, "package %s\n\n", packageName)

	// Check if JSON contains backticks
	if strings.Contains(jsonStr, "`") {
		// Use regular string literal with escaping
		escaped := strings.ReplaceAll(jsonStr, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\r", "\\r")
		escaped = strings.ReplaceAll(escaped, "\t", "\\t")
		fmt.Fprintf(output, "const defaultSystemPrompt = \"%s\"\n", escaped)
	} else {
		// Use raw string literal (backticks) for cleaner output
		fmt.Fprintf(output, "const defaultSystemPrompt = `%s`\n", jsonStr)
	}

	if output != os.Stdout {
		fmt.Printf("Successfully compiled JSON to %s\n", os.Args[2])
	}
}
