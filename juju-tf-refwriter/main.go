// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: juju-tf-refwriter <terraform-file-or-directory>")
		fmt.Println()
		fmt.Println("Rewrites literal UUID/name references in a `terraform query` plan")
		fmt.Println("into references to the corresponding juju_model, juju_application,")
		fmt.Println("juju_machine, and juju_secret resources defined in the same file.")
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

	totalRewritten := 0
	totalWarnings := 0
	failed := false

	for _, filename := range filesToProcess {
		rewritten, warnings, err := processFile(filename)
		if err != nil {
			fmt.Printf("  Error processing %s: %v\n", filename, err)
			failed = true
			continue
		}
		if rewritten {
			totalRewritten++
		}
		totalWarnings += warnings
	}

	fmt.Printf("\nSummary: %d out of %d files were rewritten\n", totalRewritten, len(filesToProcess))
	if totalWarnings > 0 {
		fmt.Printf("⚠️  Total warnings: %d reference(s) left as literals for manual review\n", totalWarnings)
	}
	if failed {
		os.Exit(1)
	}
}

// transformTerraformFile processes Terraform file content and returns the
// rewritten content. This function is the core transformation logic that can
// be tested independently.
func transformTerraformFile(src []byte, filename string) (rewritten []byte, warnings []string, err error) {
	f, diags := hclwrite.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("error parsing HCL: %v", diags)
	}

	// First pass: build an index of resources that can serve as rewrite
	// targets (models, applications, machines, secrets). Import blocks are
	// indexed first because `terraform query` output puts the composite
	// identity in the import block's `identity`, not on the resource block.
	idx := newRefIndex()
	for _, block := range f.Body().Blocks() {
		if block.Type() == "import" {
			idx.indexImport(block)
		}
	}
	for _, block := range f.Body().Blocks() {
		if block.Type() == "resource" {
			idx.indexResource(block)
		}
	}

	changed := false

	// Second pass: prune unconfigurable and redundant attributes, then
	// rewrite literal references inside resource blocks.
	for _, block := range f.Body().Blocks() {
		if block.Type() != "resource" || len(block.Labels()) < 2 {
			continue
		}
		kind := kindOf(block.Labels()[0])

		if pruneNullAttributes(block) {
			changed = true
		}
		if pruneComputedAttributes(block, kind) {
			changed = true
		}

		var w []string
		var ch bool
		switch kind {
		case kindApplication:
			ch, w = rewriteModelUUID(block, idx)
			warnings = append(warnings, w...)
			ch2, w2 := rewriteApplicationMachines(block, idx)
			if ch2 {
				ch = true
			}
			warnings = append(warnings, w2...)
		case kindIntegration:
			ch, w = rewriteIntegrationApplications(block, idx)
			warnings = append(warnings, w...)
			ch2, w2 := rewriteModelUUID(block, idx)
			if ch2 {
				ch = true
			}
			warnings = append(warnings, w2...)
		case kindMachine, kindSecret, kindSpace, kindSSHKey, kindStoragePool,
			kindOffer, kindAccessModel, kindAccessSecret:
			ch, w = rewriteModelUUID(block, idx)
			warnings = append(warnings, w...)
		}
		if ch {
			changed = true
		}
	}
	if !changed {
		return src, warnings, nil
	}
	return f.Bytes(), warnings, nil
}

func processFile(filename string) (changed bool, warnings int, err error) {
	fmt.Printf("Processing: %s\n", filename)

	fileInfo, err := os.Stat(filename)
	if err != nil {
		return false, 0, fmt.Errorf("getting file info: %w", err)
	}

	src, err := os.ReadFile(filename)
	if err != nil {
		return false, 0, fmt.Errorf("reading file: %w", err)
	}

	out, w, err := transformTerraformFile(src, filename)
	if err != nil {
		return false, 0, fmt.Errorf("transforming file: %w", err)
	}
	for _, warn := range w {
		fmt.Printf("  ⚠️  %s\n", warn)
	}
	if len(w) > 0 {
		fmt.Printf("  ⚠️  %d reference(s) left as literals for manual review\n", len(w))
	}

	if string(out) != string(src) {
		if err := os.WriteFile(filename, out, fileInfo.Mode()); err != nil {
			return false, len(w), fmt.Errorf("writing file: %w", err)
		}
		fmt.Printf("  ✓ File updated successfully\n")
		return true, len(w), nil
	}
	if len(w) == 0 {
		fmt.Printf("  - No references to rewrite\n")
	}
	return false, len(w), nil
}

// pruneNullAttributes removes any attribute whose value is the literal `null`
// from a resource block. `terraform query` emits every Optional attribute as
// `null` when the value is unset, which is redundant since the provider
// default for those attributes is already `null`. Removing them keeps the
// rewritten config closer to hand-written Terraform. Returns true if any
// attribute was removed.
func pruneNullAttributes(block *hclwrite.Block) bool {
	removed := false
	for name, attr := range block.Body().Attributes() {
		if v, ok := attrValue(attr); ok && v.IsNull() {
			block.Body().RemoveAttribute(name)
			removed = true
		}
	}
	return removed
}

// rewriteModelUUID rewrites a literal `model_uuid = "<uuid>"` attribute into
// `model_uuid = juju_model.<label>.uuid` when a matching juju_model resource
// exists in the index. Returns true and any warning emitted.
func rewriteModelUUID(block *hclwrite.Block, idx *refIndex) (bool, []string) {
	addr := resourceAddress(block)
	uuid := readAttrString(block, "model_uuid")
	if uuid == "" {
		return false, nil
	}
	ref := idx.modelRef(uuid)
	if ref == "" {
		return false, []string{fmt.Sprintf("%s: model_uuid %q has no matching juju_model resource; left as literal", addr, uuid)}
	}
	block.Body().SetAttributeRaw("model_uuid", refTokens(ref, "uuid"))
	return true, nil
}

// rewriteApplicationMachines rewrites literal machine IDs in an application's
// `machines = ["1", "2"]` attribute into references like
// `juju_machine.<label>.machine_id`. The application's model UUID is used
// to scope the lookup.
func rewriteApplicationMachines(block *hclwrite.Block, idx *refIndex) (bool, []string) {
	addr := resourceAddress(block)
	modelUUID := resolveModelUUID(block, idx, kindApplication)
	if modelUUID == "" {
		return false, nil
	}

	machinesAttr := block.Body().GetAttribute("machines")
	if machinesAttr == nil {
		return false, nil
	}
	machines, ok := attrStringList(machinesAttr)
	if !ok || len(machines) == 0 {
		return false, nil
	}

	var warnings []string
	changed := false
	elems := make([]hclwrite.Tokens, len(machines))
	for i, id := range machines {
		ref := idx.machineRef(modelUUID, id)
		if ref == "" {
			warnings = append(warnings, fmt.Sprintf("%s: machine %q has no matching juju_machine resource; left as literal", addr, id))
			elems[i] = hclwrite.TokensForValue(cty.StringVal(id))
			continue
		}
		elems[i] = refTokens(ref, "machine_id")
		changed = true
	}
	if !changed {
		return false, warnings
	}
	block.Body().SetAttributeRaw("machines", multilineListTokens(elems))
	return true, warnings
}

// rewriteIntegrationApplications rewrites literal application `name` attributes
// inside a juju_integration resource's `application` blocks into references to
// the corresponding juju_application resource.
func rewriteIntegrationApplications(block *hclwrite.Block, idx *refIndex) (bool, []string) {
	addr := resourceAddress(block)
	modelUUID := resolveModelUUID(block, idx, kindIntegration)
	if modelUUID == "" {
		return false, nil
	}

	changed := false
	var warnings []string
	for _, sub := range block.Body().Blocks() {
		if sub.Type() != "application" {
			continue
		}
		name := readAttrString(sub, "name")
		if name == "" {
			continue
		}
		ref := idx.appRef(modelUUID, name)
		if ref == "" {
			warnings = append(warnings, fmt.Sprintf("%s: application %q has no matching juju_application resource; left as literal", addr, name))
			continue
		}
		sub.Body().SetAttributeRaw("name", refTokens(ref, "name"))
		changed = true
	}
	return changed, warnings
}

// resolveModelUUID determines the model UUID that a resource block belongs to,
// read from the matching import block's identity.
func resolveModelUUID(block *hclwrite.Block, idx *refIndex, kind resourceKind) string {
	id := idx.importIdentityID(resourceAddress(block))
	if id == "" {
		return ""
	}
	e, err := parseIdentity(kind, id)
	if err != nil {
		return ""
	}
	return e.modelUUID
}

// attrStringList evaluates a top-level attribute as a list/tuple of strings.
// Returns false if the attribute is not a wholly-known list/tuple of strings;
// callers leave such attributes untouched.
func attrStringList(attr *hclwrite.Attribute) ([]string, bool) {
	v, ok := attrValue(attr)
	if !ok || (!v.Type().IsListType() && !v.Type().IsTupleType()) {
		return nil, false
	}
	var items []string
	for it := v.ElementIterator(); it.Next(); {
		_, elem := it.Element()
		if !elem.Type().Equals(cty.String) {
			// Non-string element: leave the whole attribute untouched.
			return nil, false
		}
		items = append(items, elem.AsString())
	}
	return items, true
}

// multilineListTokens builds a bracketed list expression with one element
// per line, matching the layout `terraform query` itself uses for lists.
func multilineListTokens(elems []hclwrite.Tokens) hclwrite.Tokens {
	tokens := hclwrite.Tokens{{Type: hclsyntax.TokenOBrack, Bytes: []byte("[")}}
	for _, elem := range elems {
		tokens = append(tokens, &hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")})
		tokens = append(tokens, elem...)
	}
	if len(elems) > 0 {
		tokens = append(tokens, &hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")})
	}
	return append(tokens, &hclwrite.Token{Type: hclsyntax.TokenCBrack, Bytes: []byte("]")})
}

// refTokens builds the token slice for a bare reference expression like
// `juju_model.model_0.uuid` (no quotes).
func refTokens(addr, attr string) hclwrite.Tokens {
	return hclwrite.TokensForTraversal(hcl.Traversal{
		hcl.TraverseRoot{Name: addr},
		hcl.TraverseAttr{Name: attr},
	})
}

// discoverTerraformFiles finds all .tf files to process from a given target
// path. Returns a slice of file paths and any error encountered.
func discoverTerraformFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("error accessing target: %v", err)
	}

	var filesToProcess []string

	if info.IsDir() {
		err := filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == ".terraform" {
				return filepath.SkipDir
			}
			if !d.IsDir() && strings.HasSuffix(path, ".tf") {
				filesToProcess = append(filesToProcess, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error walking directory: %v", err)
		}
	} else {
		filesToProcess = append(filesToProcess, target)
	}

	return filesToProcess, nil
}
