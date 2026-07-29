// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// refIndex is a lookup table built from the resource and import blocks in a
// generated `terraform query` plan. It maps entity identifiers (model UUIDs,
// app names within a model, machine IDs within a model, ...) to the Terraform
// addresses of the resources that represent them, so that literal values can
// be rewritten into references.
type refIndex struct {
	// modelByUUID maps a model UUID to "juju_model.<label>".
	modelByUUID map[string]string

	// appByModelAndName maps "<model-uuid>:<app-name>" to
	// "juju_application.<label>".
	appByModelAndName map[string]string

	// machineByModelAndID maps "<model-uuid>:<machine-id>" to
	// "juju_machine.<label>". The machine name is intentionally not part of
	// the key: applications reference machines by ID, not by name.
	machineByModelAndID map[string]string

	// secretByModelAndName maps "<model-uuid>:<secret-name>" to
	// "juju_secret.<label>".
	secretByModelAndName map[string]string

	// importIDByAddr maps a resource address (e.g. "juju_application.all_apps_0")
	// to the identity `id` found in its matching `import` block. This is the
	// common source of composite identities in `terraform query` output.
	importIDByAddr map[string]string
}

// newRefIndex returns an empty refIndex.
func newRefIndex() *refIndex {
	return &refIndex{
		modelByUUID:          make(map[string]string),
		appByModelAndName:    make(map[string]string),
		machineByModelAndID:  make(map[string]string),
		secretByModelAndName: make(map[string]string),
		importIDByAddr:       make(map[string]string),
	}
}

// resourceAddress returns the Terraform address for a block, e.g.
// "juju_model.model_0".
func resourceAddress(block *hclwrite.Block) string {
	labels := block.Labels()
	if len(labels) < 2 {
		return ""
	}
	return strings.Join(labels[:2], ".")
}

// indexResource registers a resource block in the index. Only the resource
// kinds that can serve as rewrite targets are indexed: models, applications,
// machines, and secrets.
//
// The identity is read from the matching `import` block's `identity.id`, which
// `terraform query` always emits. indexImport must be called before
// indexResource.
func (idx *refIndex) indexResource(block *hclwrite.Block) {
	if len(block.Labels()) < 2 {
		return
	}
	resourceType := block.Labels()[0]
	kind := kindOf(resourceType)
	addr := resourceAddress(block)

	id := idx.importIdentityID(addr)
	if id == "" {
		return
	}

	switch kind {
	case kindModel:
		idx.modelByUUID[id] = addr
	case kindApplication:
		e, err := parseIdentity(kind, id)
		if err != nil {
			return
		}
		idx.appByModelAndName[e.modelUUID+":"+e.appName] = addr
	case kindMachine:
		e, err := parseIdentity(kind, id)
		if err != nil {
			return
		}
		idx.machineByModelAndID[e.modelUUID+":"+e.machineID] = addr
	case kindSecret:
		e, err := parseIdentity(kind, id)
		if err != nil {
			return
		}
		idx.secretByModelAndName[e.modelUUID+":"+e.secretName] = addr
	}
}

// indexImport registers an `import` block in the index. It records the
// mapping from the resource address (the `to` attribute) to the identity `id`
// found in the `identity` object attribute.
func (idx *refIndex) indexImport(block *hclwrite.Block) {
	if block.Type() != "import" {
		return
	}
	to := readAttrRef(block, "to")
	if to == "" {
		return
	}
	// The identity is an object attribute: identity = { id = "..." }.
	id := readObjectAttrString(block, "identity", "id")
	if id != "" {
		idx.importIDByAddr[to] = id
	}
}

// importIdentityID returns the identity `id` recorded for the given resource
// address from an import block, or "" if none.
func (idx *refIndex) importIdentityID(addr string) string {
	return idx.importIDByAddr[addr]
}

// modelRef returns a Terraform reference to the model resource for the given
// UUID, or "" if no model resource matches.
func (idx *refIndex) modelRef(modelUUID string) string {
	return idx.modelByUUID[modelUUID]
}

// appRef returns a Terraform reference to the application resource for the
// given model UUID and application name, or "" if none matches.
func (idx *refIndex) appRef(modelUUID, appName string) string {
	return idx.appByModelAndName[modelUUID+":"+appName]
}

// machineRef returns a Terraform reference to the machine resource for the
// given model UUID and machine ID, or "" if none matches.
func (idx *refIndex) machineRef(modelUUID, machineID string) string {
	return idx.machineByModelAndID[modelUUID+":"+machineID]
}

// secretRef returns a Terraform reference to the secret resource for the given
// model UUID and secret name, or "" if none matches.
func (idx *refIndex) secretRef(modelUUID, secretName string) string {
	return idx.secretByModelAndName[modelUUID+":"+secretName]
}

// readAttrString reads a top-level string attribute from a block, returning
// the unquoted value, or "" if the attribute is absent or not a simple
// string literal.
func readAttrString(block *hclwrite.Block, name string) string {
	attrs := block.Body().Attributes()
	attr, ok := attrs[name]
	if !ok {
		return ""
	}
	tokens := attr.Expr().BuildTokens(nil)
	return tokensToQuotedString(tokens)
}

// readAttrRef reads a top-level attribute whose value is a dotted reference
// expression (e.g. `to = juju_model.model_0`), returning the canonical
// "type.label" form, or "" if the attribute is absent or not a simple
// reference.
func readAttrRef(block *hclwrite.Block, name string) string {
	attrs := block.Body().Attributes()
	attr, ok := attrs[name]
	if !ok {
		return ""
	}
	tokens := attr.Expr().BuildTokens(nil)
	return tokensToRef(tokens)
}

// readObjectAttrString reads a string field from a top-level attribute whose
// value is an object literal (e.g. `identity = { id = "..." }`). Returns ""
// if the attribute or field is absent or not a simple string literal.
func readObjectAttrString(block *hclwrite.Block, objName, field string) string {
	attrs := block.Body().Attributes()
	attr, ok := attrs[objName]
	if !ok {
		return ""
	}
	tokens := attr.Expr().BuildTokens(nil)
	return tokensToObjectFieldString(tokens, field)
}

// tokensToQuotedString extracts a string literal from a token slice, stripping
// the surrounding quotes. Returns "" if the tokens don't form a single quoted
// string literal.
func tokensToQuotedString(tokens hclwrite.Tokens) string {
	var s strings.Builder
	inString := false
	started := false
	for _, tok := range tokens {
		switch {
		case tok.Type == hclsyntax.TokenOQuote:
			inString = true
			started = true
		case tok.Type == hclsyntax.TokenCQuote:
			inString = false
		case tok.Type == hclsyntax.TokenQuotedLit && inString:
			s.WriteString(string(tok.Bytes))
		case tok.Type == hclsyntax.TokenStringLit && inString:
			s.WriteString(string(tok.Bytes))
		case (tok.Type == hclsyntax.TokenOHeredoc || tok.Type == hclsyntax.TokenCHeredoc):
			// Heredocs aren't simple literals we care about here.
			return ""
		}
	}
	if !started {
		return ""
	}
	return s.String()
}

// tokensToRef extracts a dotted reference expression (e.g.
// `juju_model.model_0`) from a token slice, returning it as "type.label".
// Returns "" if the tokens aren't a sequence of identifiers separated by dots.
func tokensToRef(tokens hclwrite.Tokens) string {
	var parts []string
	expectIdent := true
	for _, tok := range tokens {
		switch tok.Type {
		case hclsyntax.TokenIdent:
			if !expectIdent {
				return ""
			}
			parts = append(parts, string(tok.Bytes))
			expectIdent = false
		case hclsyntax.TokenDot:
			if expectIdent {
				return ""
			}
			expectIdent = true
		default:
			// Any other token (whitespace is folded in by hclwrite, but
			// be defensive) means this isn't a simple reference.
			return ""
		}
	}
	if len(parts) < 2 || expectIdent {
		return ""
	}
	return strings.Join(parts, ".")
}

// tokensToObjectFieldString extracts a string field from an object literal
// token slice of the form `{ id = "..." , ... }`. Returns "" if the field is
// absent or not a simple string literal.
func tokensToObjectFieldString(tokens hclwrite.Tokens, field string) string {
	// Scan for the pattern: <OBrace> ... <Ident field> <Equals> <string> ...
	// hclwrite folds whitespace into adjacent tokens, so we walk the raw
	// token stream.
	i := 0
	// Find the opening brace.
	for ; i < len(tokens); i++ {
		if tokens[i].Type == hclsyntax.TokenOBrace {
			i++
			break
		}
	}
	if i >= len(tokens) {
		return ""
	}
	// Walk the object body looking for `field = "value"`.
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Type == hclsyntax.TokenCBrace {
			return ""
		}
		if tok.Type == hclsyntax.TokenIdent && string(tok.Bytes) == field {
			// Expect `=` then a string literal.
			j := i + 1
			if j >= len(tokens) || tokens[j].Type != hclsyntax.TokenEqual {
				return ""
			}
			j++
			// Skip to the first string token.
			for j < len(tokens) {
				switch tokens[j].Type {
				case hclsyntax.TokenOQuote:
					// Collect the string literal starting here.
					rest := tokens[j:]
					return tokensToQuotedString(rest)
				case hclsyntax.TokenComment, hclsyntax.TokenNewline:
					j++
					continue
				default:
					return ""
				}
			}
			return ""
		}
		i++
	}
	return ""
}

// formatRef formats a Terraform reference like "juju_model.model_0.uuid".
func formatRef(addr, attr string) string {
	return fmt.Sprintf("%s.%s", addr, attr)
}
