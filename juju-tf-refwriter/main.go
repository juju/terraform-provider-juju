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

	for _, filename := range filesToProcess {
		rewritten, warnings := processFile(filename)
		if rewritten {
			totalRewritten++
		}
		totalWarnings += warnings
	}

	fmt.Printf("\nSummary: %d out of %d files were rewritten\n", totalRewritten, len(filesToProcess))
	if totalWarnings > 0 {
		fmt.Printf("⚠️  Total warnings: %d reference(s) left as literals for manual review\n", totalWarnings)
	}
}

// transformationResult holds the result of transforming a Terraform file.
type transformationResult struct {
	ModifiedContent []byte
	WasRewritten    bool
	Warnings        int
}

// transformTerraformFile processes Terraform file content and returns the
// rewritten content. This function is the core transformation logic that can
// be tested independently.
func transformTerraformFile(src []byte, filename string) (*transformationResult, error) {
	f, diags := hclwrite.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("error parsing HCL: %v", diags)
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

	rewritten := false
	warnings := 0

	// Second pass: prune unconfigurable and redundant attributes, then
	// rewrite literal references inside resource blocks.
	for _, block := range f.Body().Blocks() {
		if block.Type() != "resource" || len(block.Labels()) < 2 {
			continue
		}
		resourceType := block.Labels()[0]
		kind := kindOf(resourceType)

		// Prune attributes that `terraform query` emits but that
		// can't or shouldn't be set in config.
		if pruneNullAttributes(block) {
			rewritten = true
		}
		if pruneComputedAttributes(block, kind) {
			rewritten = true
		}

		// Specific rewrites
		switch kind {
		case kindApplication:
			rewrittenModel := rewriteModelUUID(block, idx, &warnings)
			rewrittenMachines := rewriteApplicationMachines(block, idx, &warnings)
			if rewrittenModel || rewrittenMachines {
				rewritten = true
			}
		case kindIntegration:
			if rewriteIntegrationApplications(block, idx, &warnings) {
				rewritten = true
			}
			// Integrations also carry a model_uuid.
			if rewriteModelUUID(block, idx, &warnings) {
				rewritten = true
			}
		case kindMachine, kindSecret, kindSpace, kindSSHKey, kindStoragePool,
			kindOffer, kindAccessModel, kindAccessSecret:
			if rewriteModelUUID(block, idx, &warnings) {
				rewritten = true
			}
		}
	}

	return &transformationResult{
		ModifiedContent: f.Bytes(),
		WasRewritten:    rewritten,
		Warnings:        warnings,
	}, nil
}

func processFile(filename string) (bool, int) {
	fmt.Printf("Processing: %s\n", filename)

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

	if result.WasRewritten {
		err = os.WriteFile(filename, result.ModifiedContent, fileInfo.Mode())
		if err != nil {
			fmt.Printf("  Error writing file: %v\n", err)
			return false, result.Warnings
		}
		fmt.Printf("  ✓ File updated successfully\n")
	}

	if result.Warnings > 0 {
		fmt.Printf("  ⚠️  %d reference(s) left as literals for manual review\n", result.Warnings)
	}

	if !result.WasRewritten && result.Warnings == 0 {
		fmt.Printf("  - No references to rewrite\n")
	}

	return result.WasRewritten, result.Warnings
}

// pruneNullAttributes removes any attribute whose value is the literal `null`
// from a resource block. `terraform query` emits every Optional attribute as
// `null` when the value is unset, which is redundant since the provider
// default for those attributes is already `null`. Removing them keeps the
// rewritten config closer to hand-written Terraform. Returns true if any
// attribute was removed.
func pruneNullAttributes(block *hclwrite.Block) bool {
	removed := false
	for name := range block.Body().Attributes() {
		if isNullAttr(block, name) {
			block.Body().RemoveAttribute(name)
			removed = true
		}
	}
	return removed
}

// isNullAttr reports whether the named attribute on the block is a simple
// `name = null` literal.
func isNullAttr(block *hclwrite.Block, name string) bool {
	attr, ok := block.Body().Attributes()[name]
	if !ok {
		return false
	}
	tokens := attr.Expr().BuildTokens(nil)
	// hclwrite folds leading spaces into the first token, so the null
	// literal appears as a single TokenIdent with bytes "null".
	for _, tok := range tokens {
		switch tok.Type {
		case hclsyntax.TokenIdent:
			if string(tok.Bytes) != "null" {
				return false
			}
		case hclsyntax.TokenNewline, hclsyntax.TokenComment:
			// Ignore trailing whitespace/comments.
		default:
			return false
		}
	}
	return true
}

// computedAttributes lists the Computed-only attributes (Computed, not
// Optional, not Required) for each resource kind. These attributes cannot be
// set in config, so the literal values that `terraform query` emits for them
// fail validation on apply and are pruned. Attributes that are both
// Computed and Optional (or Computed and Required) are intentionally omitted:
// the user may set those, so they are kept.
//
// Keep this in sync with the provider schemas in internal/provider.
var computedAttributes = map[resourceKind]map[string]struct{}{
	kindModel: {
		"uuid": {},
		"id":   {},
	},
	kindApplication: {
		"id":           {},
		"model_type":   {},
		"unit_numbers": {},
		"storage":      {},
	},
	kindMachine: {
		"id":          {},
		"machine_id":  {},
		"instance_id": {},
		"hostname":    {},
	},
	kindSecret: {
		"id":         {},
		"secret_id":  {},
		"secret_uri": {},
	},
	kindIntegration: {
		"id": {},
	},
	kindAccessModel: {
		"id": {},
	},
	kindAccessSecret: {
		"id": {},
	},
}

// pruneComputedAttributes removes the Computed-only attributes for the given
// resource kind from a block. Returns true if any attribute was removed.
// Unknown kinds are left untouched, since the set of Computed attributes for
// them is not known here.
func pruneComputedAttributes(block *hclwrite.Block, kind resourceKind) bool {
	names, ok := computedAttributes[kind]
	if !ok {
		return false
	}
	removed := false
	for name := range names {
		if block.Body().GetAttribute(name) != nil {
			block.Body().RemoveAttribute(name)
			removed = true
		}
	}
	return removed
}

// rewriteModelUUID rewrites a literal `model_uuid = "<uuid>"` attribute into
// `model_uuid = juju_model.<label>.uuid` when a matching juju_model resource
// exists in the index. Returns true if the attribute was rewritten.
func rewriteModelUUID(block *hclwrite.Block, idx *refIndex, warnings *int) bool {
	attrs := block.Body().Attributes()
	attr, ok := attrs["model_uuid"]
	if !ok {
		return false
	}
	uuid := readAttrString(block, "model_uuid")
	if uuid == "" {
		// Not a simple literal (could already be a reference); leave it.
		return false
	}

	ref := idx.modelRef(uuid)
	if ref == "" {
		*warnings++
		fmt.Printf("  ⚠️  %s.%s: model_uuid %q has no matching juju_model resource; left as literal\n",
			block.Labels()[0], block.Labels()[1], uuid)
		return false
	}

	block.Body().SetAttributeRaw("model_uuid", refTokens(formatRef(ref, "uuid")))
	_ = attr
	return true
}

// resolveModelUUID determines the model UUID that a resource block belongs to,
// read from the matching import block's identity. Returns "" if no import
// identity is available for the block.
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

// rewriteApplicationMachines rewrites literal machine IDs in an application's
// `machines = ["1", "2"]` attribute into references like
// `juju_machine.<label>.machine_id`. The application's model UUID is used to
// scope the lookup; if the model_uuid is itself a reference (already
// rewritten), the machine references are emitted as references too but scoped
// by the literal model UUID captured before rewriting.
func rewriteApplicationMachines(block *hclwrite.Block, idx *refIndex, warnings *int) bool {
	attrs := block.Body().Attributes()
	attr, ok := attrs["machines"]
	if !ok {
		return false
	}

	// Determine the model UUID for this application from its import identity.
	modelUUID := resolveModelUUID(block, idx, kindApplication)
	if modelUUID == "" {
		return false
	}

	tokens := attr.Expr().BuildTokens(nil)
	items, ok := parseStringListTokens(tokens)
	if !ok || len(items) == 0 {
		return false
	}

	rewritten := false
	newItems := make([]listItem, 0, len(items))
	for _, it := range items {
		if !it.isString {
			// Not a literal string; keep as-is.
			newItems = append(newItems, it)
			continue
		}
		ref := idx.machineRef(modelUUID, it.value)
		if ref == "" {
			*warnings++
			fmt.Printf("  ⚠️  %s.%s: machine %q has no matching juju_machine resource; left as literal\n",
				block.Labels()[0], block.Labels()[1], it.value)
			newItems = append(newItems, it)
			continue
		}
		newItems = append(newItems, listItem{
			raw:   formatRef(ref, "machine_id"),
			isRef: true,
		})
		rewritten = true
	}

	if !rewritten {
		return false
	}

	block.Body().SetAttributeRaw("machines", listTokens(newItems))
	return true
}

// rewriteIntegrationApplications rewrites literal application `name` attributes
// inside a juju_integration resource's `application` blocks into references to
// the corresponding juju_application resource. Returns true if any rewrite
// happened.
func rewriteIntegrationApplications(block *hclwrite.Block, idx *refIndex, warnings *int) bool {
	modelUUID := resolveModelUUID(block, idx, kindIntegration)
	if modelUUID == "" {
		return false
	}

	rewritten := false
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
			*warnings++
			fmt.Printf("  ⚠️  %s.%s: application %q has no matching juju_application resource; left as literal\n",
				block.Labels()[0], block.Labels()[1], name)
			continue
		}
		sub.Body().SetAttributeRaw("name", refTokens(formatRef(ref, "name")))
		rewritten = true
	}
	return rewritten
}

// refTokens builds the token slice for a bare reference expression like
// `juju_model.model_0.uuid` (no quotes).
func refTokens(ref string) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	parts := strings.Split(ref, ".")
	for i, p := range parts {
		if i > 0 {
			tokens = append(tokens, &hclwrite.Token{
				Type:  hclsyntax.TokenDot,
				Bytes: []byte("."),
			})
		}
		tokens = append(tokens, &hclwrite.Token{
			Type:  hclsyntax.TokenIdent,
			Bytes: []byte(p),
		})
	}
	return tokens
}

// listItem represents one element of a list expression. Either a literal
// string (isString=true, value set), a reference (isRef=true, raw set), or
// some other expression kept verbatim (raw set, isString/isRef false).
type listItem struct {
	value    string
	raw      string
	isString bool
	isRef    bool
}

// parseStringListTokens parses a list expression token slice into items. It
// recognises string literals and passes everything else through verbatim. The
// returned bool is false if the tokens don't form a list expression.
func parseStringListTokens(tokens hclwrite.Tokens) ([]listItem, bool) {
	// Flatten to a simple scan. We expect: [ "a" , "b" ] possibly with
	// whitespace/newlines. Anything more complex (nested lists, functions)
	// causes us to bail out and leave the attribute untouched.
	trimmed := trimSpaceTokens(tokens)
	if len(trimmed) == 0 {
		return nil, false
	}
	if trimmed[0].Type != hclsyntax.TokenOBrack {
		return nil, false
	}

	items := []listItem{}
	i := 1 // skip [
	currentRaw := hclwrite.Tokens{}
	for i < len(trimmed) {
		tok := trimmed[i]
		switch tok.Type {
		case hclsyntax.TokenCBrack:
			// End of list. Flush any trailing raw tokens as a single item.
			if len(currentRaw) > 0 {
				items = append(items, listItem{raw: string(currentRaw.Bytes())})
			}
			return items, true
		case hclsyntax.TokenComma:
			if len(currentRaw) > 0 {
				items = append(items, rawItemFromTokens(currentRaw))
				currentRaw = hclwrite.Tokens{}
			}
			i++
		case hclsyntax.TokenOQuote:
			// Collect a full string literal (OQuote ... CQuote).
			str, consumed, ok := consumeStringLiteral(trimmed, i)
			if !ok {
				return nil, false
			}
			items = append(items, listItem{value: str, isString: true})
			i = consumed
		default:
			currentRaw = append(currentRaw, tok)
			i++
		}
	}
	return nil, false
}

// rawItemFromTokens turns a verbatim token slice into a listItem.
func rawItemFromTokens(toks hclwrite.Tokens) listItem {
	return listItem{raw: string(toks.Bytes())}
}

// consumeStringLiteral reads a string literal starting at index i (an OQuote
// token) and returns the unquoted value, the index of the next token to
// process, and ok=false if the literal is malformed.
func consumeStringLiteral(tokens hclwrite.Tokens, i int) (string, int, bool) {
	var s strings.Builder
	inString := false
	for ; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok.Type {
		case hclsyntax.TokenOQuote:
			inString = true
		case hclsyntax.TokenCQuote:
			inString = false
			return s.String(), i + 1, true
		case hclsyntax.TokenQuotedLit, hclsyntax.TokenStringLit:
			if inString {
				s.WriteString(string(tok.Bytes))
			}
		default:
			if inString {
				return "", 0, false
			}
		}
	}
	return "", 0, false
}

// listTokens builds the token slice for a list expression from items.
func listTokens(items []listItem) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	tokens = append(tokens, &hclwrite.Token{
		Type:  hclsyntax.TokenOBrack,
		Bytes: []byte("["),
	})
	for i, it := range items {
		if i > 0 {
			tokens = append(tokens, &hclwrite.Token{
				Type:  hclsyntax.TokenComma,
				Bytes: []byte(","),
			})
		}
		tokens = append(tokens, &hclwrite.Token{
			Type:  hclsyntax.TokenNewline,
			Bytes: []byte("\n"),
		})
		tokens = append(tokens, itemTokens(it)...)
	}
	if len(items) > 0 {
		tokens = append(tokens, &hclwrite.Token{
			Type:  hclsyntax.TokenNewline,
			Bytes: []byte("\n"),
		})
	}
	tokens = append(tokens, &hclwrite.Token{
		Type:  hclsyntax.TokenCBrack,
		Bytes: []byte("]"),
	})
	return tokens
}

// itemTokens builds the token slice for a single list item.
func itemTokens(it listItem) hclwrite.Tokens {
	if it.isString {
		return hclwrite.Tokens{
			{Type: hclsyntax.TokenOQuote, Bytes: []byte("\"")},
			{Type: hclsyntax.TokenQuotedLit, Bytes: []byte(it.value)},
			{Type: hclsyntax.TokenCQuote, Bytes: []byte("\"")},
		}
	}
	if it.isRef {
		return refTokens(it.raw)
	}
	// Verbatim raw text.
	return hclwrite.Tokens{
		{Type: hclsyntax.TokenOHeredoc, Bytes: []byte("<<-EOT\n")},
		{Type: hclsyntax.TokenStringLit, Bytes: []byte(it.raw)},
		{Type: hclsyntax.TokenCHeredoc, Bytes: []byte("\nEOT")},
	}
}

// trimSpaceTokens returns a copy of the token slice with whitespace/comment
// tokens removed.
func trimSpaceTokens(tokens hclwrite.Tokens) hclwrite.Tokens {
	out := make(hclwrite.Tokens, 0, len(tokens))
	for _, tok := range tokens {
		switch tok.Type {
		case hclsyntax.TokenNewline, hclsyntax.TokenComment:
			continue
		}
		// hclwrite folds leading spaces into the following token's bytes
		// for some token types; strip a leading run of spaces from the
		// bytes so list parsing isn't confused. We only do this for
		// structural tokens we inspect.
		out = append(out, tok)
	}
	return out
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
