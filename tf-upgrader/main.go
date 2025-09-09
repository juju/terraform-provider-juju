// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: tf-upgrader <terraform-file-or-directory>")
		os.Exit(1)
	}

	target := os.Args[1]

	filesToProcess, err := discoverTerraformFiles(target)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	if len(filesToProcess) == 0 {
		fmt.Println("No .tf files found to process")
		return
	}

	fmt.Printf("Found %d Terraform files to process:\n", len(filesToProcess))
	for _, file := range filesToProcess {
		fmt.Printf("  - %s\n", file)
	}
	fmt.Println()

	totalUpgraded := 0
	totalWarnings := 0

	for _, filename := range filesToProcess {
		upgraded, warnings := processFile(filename)
		if upgraded {
			totalUpgraded++
		}
		totalWarnings += warnings
	}

	fmt.Printf("\nSummary: %d out of %d files were upgraded\n", totalUpgraded, len(filesToProcess))
	if totalWarnings > 0 {
		fmt.Printf("⚠️  Total warnings: %d variable(s) flagged for manual review across all files\n", totalWarnings)
		fmt.Println("Please review variables named 'model', 'model_name', or containing 'model_name' to ensure they use UUIDs instead of names where appropriate.")
	}
}

// transformationResult holds the result of transforming a Terraform file
type transformationResult struct {
	ModifiedContent []byte
	WasUpgraded     bool
	Warnings        int
}

// transformTerraformFile processes Terraform file content and returns the upgraded content
// This function is the core transformation logic that can be tested independently
func transformTerraformFile(src []byte, filename string) (*transformationResult, error) {
	// Parse with hclsyntax for source location info
	srcFile, srcDiags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if srcDiags.HasErrors() {
		return nil, fmt.Errorf("error parsing HCL for source info: %v", srcDiags)
	}

	// Parse with hclwrite for modifications
	f, diags := hclwrite.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("error parsing HCL: %v", diags)
	}

	upgraded := false
	warnings := 0

	// Create a map of block labels to source blocks for line number lookup
	srcBlockMap := make(map[string]*hclsyntax.Block)
	for _, block := range srcFile.Body.(*hclsyntax.Body).Blocks {
		key := block.Type
		for _, label := range block.Labels {
			key += "." + label
		}
		srcBlockMap[key] = block
	}

	// Walk through all blocks
	for _, block := range f.Body().Blocks() {
		blockKey := block.Type()
		for _, label := range block.Labels() {
			blockKey += "." + label
		}

		switch block.Type() {
		case "resource":
			processResourceBlock(block, filename, &upgraded)
		case "output":
			processOutputBlock(block, filename, &upgraded)
		case "variable":
			processVariableBlock(block, filename, srcBlockMap, blockKey, &warnings)
		case "data":
			processDataBlock(block, filename, &upgraded, srcBlockMap, blockKey, &warnings)
		case "terraform":
			processTerraformBlock(block, filename, &upgraded)
		}
	}

	return &transformationResult{
		ModifiedContent: f.Bytes(),
		WasUpgraded:     upgraded,
		Warnings:        warnings,
	}, nil
}

func processFile(filename string) (bool, int) {
	fmt.Printf("Processing: %s\n", filename)

	// Get original file info to preserve permissions
	fileInfo, err := os.Stat(filename)
	if err != nil {
		fmt.Printf("  Error getting file info: %v\n", err)
		return false, 0
	}

	src, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("  Error reading file: %v\n", err)
		return false, 0
	}

	result, err := transformTerraformFile(src, filename)
	if err != nil {
		fmt.Printf("  Error transforming file: %v\n", err)
		return false, 0
	}

	if result.WasUpgraded {
		// Write the upgraded content back to the original file with original permissions
		err = os.WriteFile(filename, result.ModifiedContent, fileInfo.Mode())
		if err != nil {
			fmt.Printf("  Error writing file: %v\n", err)
			return false, result.Warnings
		}
		fmt.Printf("  ✓ File updated successfully\n")
	}

	if result.Warnings > 0 {
		fmt.Printf("  ⚠️  %d variable(s) flagged for manual review\n", result.Warnings)
	}

	if !result.WasUpgraded && result.Warnings == 0 {
		fmt.Printf("  - No upgrades needed\n")
	}

	return result.WasUpgraded, result.Warnings
}

// processResourceBlock handles resource blocks that need model -> model_uuid transformation
func processResourceBlock(block *hclwrite.Block, _ string, upgraded *bool) {
	if len(block.Labels()) < 2 {
		return
	}

	resourceType := block.Labels()[0]

	// Define resource type transformations: source_field -> target_field
	supportedResources := map[string]map[string]string{
		"juju_application":   {"model": "model_uuid"},
		"juju_offer":         {"model": "model_uuid"},
		"juju_ssh_key":       {"model": "model_uuid"},
		"juju_access_model":  {"model": "model_uuid"},
		"juju_access_secret": {"model": "model_uuid"},
		"juju_integration":   {"model": "model_uuid"},
		"juju_secret":        {"model": "model_uuid"},
		"juju_machine":       {"model": "model_uuid"},
	}

	transformation, isSupported := supportedResources[resourceType]
	if !isSupported {
		return
	}

	// Get the source field name (should be "model" for all current resources)
	var sourceField, targetField string
	for src, tgt := range transformation {
		sourceField = src
		targetField = tgt
		break
	}

	// Look for the source attribute
	attr := block.Body().GetAttribute(sourceField)
	if attr == nil {
		return
	}

	// Check if it's a juju_model.*.name reference
	attrStr := getAttributeString(attr)

	if !isJujuModelNameReference(attrStr) {
		return
	}

	// Replace .name with .uuid
	traversal, err := upgradeModelReference(attrStr)
	if err != nil {
		return
	}

	// Set the target field and remove source field if different
	block.Body().SetAttributeTraversal(targetField, traversal.Traversal)
	if sourceField != targetField {
		block.Body().RemoveAttribute(sourceField)
	}
	*upgraded = true

	referenceType := getReferenceType(attrStr)
	if sourceField == targetField {
		fmt.Printf("  ✓ Upgraded %s.%s: %s reference .name -> .uuid (%s reference)\n", resourceType, block.Labels()[1], sourceField, referenceType)
	} else {
		fmt.Printf("  ✓ Upgraded %s.%s: %s -> %s (%s reference)\n", resourceType, block.Labels()[1], sourceField, targetField, referenceType)
	}
}

// processOutputBlock handles output blocks that reference juju_model.*.name
func processOutputBlock(block *hclwrite.Block, _ string, upgraded *bool) {
	if len(block.Labels()) < 1 {
		return
	}

	// Look for output blocks that export juju_model.*.name
	attr := block.Body().GetAttribute("value")
	if attr == nil {
		return
	}

	attrStr := getAttributeString(attr)
	if !isJujuModelNameReference(attrStr) {
		return
	}

	// Replace .name with .uuid
	traversal, err := upgradeModelReference(attrStr)
	if err != nil {
		return
	}

	// Update the output value
	block.Body().SetAttributeTraversal("value", traversal.Traversal)
	*upgraded = true

	referenceType := getReferenceType(attrStr)
	fmt.Printf("  ✓ Upgraded output.%s: .name -> .uuid (%s reference)\n", block.Labels()[0], referenceType)
}

// processVariableBlock handles variable blocks that might need manual review
func processVariableBlock(block *hclwrite.Block, filename string, srcBlockMap map[string]*hclsyntax.Block, blockKey string, warnings *int) {
	if len(block.Labels()) < 1 {
		return
	}

	// Check for variables that might need manual review
	varName := block.Labels()[0]
	if !strings.Contains(varName, "model") {
		return
	}

	// Get line number from source block
	lineNum := 0
	if srcBlock, exists := srcBlockMap[blockKey]; exists {
		lineNum = srcBlock.DefRange().Start.Line
	}

	*warnings++
	fmt.Printf("  ⚠️  WARNING: %s:%d:1 - Variable '%s' may need review - check if it should use model UUID instead of name\n", filename, lineNum, varName)

	// Check if there's a description that mentions "model"
	if descAttr := block.Body().GetAttribute("description"); descAttr != nil {
		expr := descAttr.Expr()
		tokens := expr.BuildTokens(nil)
		descStr := strings.Trim(strings.Trim(string(tokens.Bytes()), "\""), " ")
		if strings.Contains(strings.ToLower(descStr), "model") {
			fmt.Printf("      Description: %s\n", descStr)
		}
	}
}

// processDataBlock handles data source blocks that reference juju_model.*.name
func processDataBlock(block *hclwrite.Block, filename string, upgraded *bool, srcBlockMap map[string]*hclsyntax.Block, blockKey string, warnings *int) {
	if len(block.Labels()) < 2 {
		return
	}

	dataSourceType := block.Labels()[0]

	// Define data source type transformations: source_field -> target_field
	supportedDataSources := map[string]map[string]string{
		"juju_model":       {"name": "uuid"},
		"juju_application": {"model": "model_uuid"},
		"juju_secret":      {"model": "model_uuid"},
		"juju_machine":     {"model": "model_uuid"},
	}

	transformation, isSupported := supportedDataSources[dataSourceType]
	if !isSupported {
		return
	}

	// Get the source field name (should be "model" for all current resources)
	var sourceField, targetField string
	for src, tgt := range transformation {
		sourceField = src
		targetField = tgt
		break
	}

	// Look for the source attribute
	attr := block.Body().GetAttribute(sourceField)
	if attr == nil {
		return
	}

	// Check if it's a juju_model.*.name reference
	attrStr := getAttributeString(attr)

	if !isJujuModelNameReference(attrStr) {
		// Check for data sources that might need manual review
		dataSourceName := block.Labels()[0]
		if !strings.Contains(dataSourceName, "model") {
			return
		}

		// Get line number from source block
		lineNum := 0
		if srcBlock, exists := srcBlockMap[blockKey]; exists {
			lineNum = srcBlock.DefRange().Start.Line
		}

		*warnings++
		fmt.Printf("  ⚠️  WARNING: %s:%d:1 - Data source '%s' may need review - check if it should use model UUID instead of name\n", filename, lineNum, dataSourceName)
		return
	}

	// Replace .name with .uuid
	traversal, err := upgradeModelReference(attrStr)
	if err != nil {
		return
	}

	// Set the target field and remove source field if different
	block.Body().SetAttributeTraversal(targetField, traversal.Traversal)
	if sourceField != targetField {
		block.Body().RemoveAttribute(sourceField)
	}
	*upgraded = true

	referenceType := getReferenceType(attrStr)
	if sourceField == targetField {
		fmt.Printf("  ✓ Upgraded %s.%s: %s reference .name -> .uuid (%s reference)\n", dataSourceType, block.Labels()[1], sourceField, referenceType)
	} else {
		fmt.Printf("  ✓ Upgraded %s.%s: %s -> %s (%s reference)\n", dataSourceType, block.Labels()[1], sourceField, targetField, referenceType)
	}
}

// processTerraformBlock handles terraform blocks that need provider version upgrades
func processTerraformBlock(block *hclwrite.Block, _ string, upgraded *bool) {
	// Look for required_providers block
	requiredProvidersBlock := block.Body().FirstMatchingBlock("required_providers", nil)
	if requiredProvidersBlock == nil {
		return
	}

	// Look for juju provider configuration
	jujuAttr := requiredProvidersBlock.Body().GetAttribute("juju")
	if jujuAttr == nil {
		return
	}

	// Parse the juju provider configuration to check if it needs upgrading
	attrStr := getAttributeString(jujuAttr)

	// Use regex to replace version values containing 0.x with >=1.0.0
	versionRegex := regexp.MustCompile(`version\s*=\s*"[^"]*0\.[^"]*"`)
	if versionRegex.MatchString(attrStr) {
		updatedContent := versionRegex.ReplaceAllString(attrStr, `version = ">=1.0.0"`)

		// Use raw tokens for the replacement
		rawTokens := []byte(updatedContent)
		tokens := hclwrite.Tokens{}
		tokens = append(tokens, &hclwrite.Token{
			Bytes: rawTokens,
		})
		requiredProvidersBlock.Body().SetAttributeRaw("juju", tokens)

		*upgraded = true
		fmt.Printf("  ✓ Upgraded terraform.required_providers.juju: version 0.x -> >=1.0.0\n")
	}
}

// isJujuModelNameReference checks if an attribute string references juju_model.*.name
func isJujuModelNameReference(attrStr string) bool {
	return (strings.Contains(attrStr, "juju_model.") || strings.Contains(attrStr, "data.juju_model.")) && strings.HasSuffix(attrStr, ".name")
}

// getReferenceType determines if the reference is to a resource or data source
func getReferenceType(attrStr string) string {
	if strings.Contains(attrStr, "data.juju_model.") {
		return "data source"
	}
	return "resource"
}

// upgradeModelReference replaces .name with .uuid and returns the new traversal
func upgradeModelReference(attrStr string) (*hclsyntax.ScopeTraversalExpr, error) {
	newAttrStr := strings.Replace(attrStr, ".name", ".uuid", 1)
	newExpr, diags := hclsyntax.ParseExpression([]byte(newAttrStr), "", hcl.Pos{})
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse expression: %v", diags)
	}

	if traversal, ok := newExpr.(*hclsyntax.ScopeTraversalExpr); ok {
		return traversal, nil
	}

	return nil, fmt.Errorf("expression is not a traversal")
}

// getAttributeString extracts the string representation of an attribute
func getAttributeString(attr *hclwrite.Attribute) string {
	expr := attr.Expr()
	tokens := expr.BuildTokens(nil)
	return string(tokens.Bytes())
}

// discoverTerraformFiles finds all .tf files to process from a given target path
// Returns a slice of file paths and any error encountered
func discoverTerraformFiles(target string) ([]string, error) {
	// Check if target is a file or directory
	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("error accessing target: %v", err)
	}

	var filesToProcess []string

	if info.IsDir() {
		// Find all .tf files in the directory and subdirectories
		err := filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".tf") && !strings.Contains(path, "_upgraded") {
				filesToProcess = append(filesToProcess, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error walking directory: %v", err)
		}
	} else {
		// Single file
		filesToProcess = append(filesToProcess, target)
	}

	return filesToProcess, nil
}
