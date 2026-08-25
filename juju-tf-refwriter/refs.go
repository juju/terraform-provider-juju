// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// refIndex maps entity identifiers (model UUIDs, app names, machine IDs, ...)
// to the Terraform addresses of the resources that represent them, so that
// literal values can be rewritten into references.
type refIndex struct {
	modelByUUID          map[string]string
	appByModelAndName    map[string]string
	machineByModelAndID  map[string]string
	secretByModelAndName map[string]string
	importIDByAddr       map[string]string
}

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

// indexResource registers a resource block in the index. Only models,
// applications, machines, and secrets are indexed as rewrite targets.
// indexImport must be called first.
func (idx *refIndex) indexResource(block *hclwrite.Block) {
	if len(block.Labels()) < 2 {
		return
	}
	kind := kindOf(block.Labels()[0])
	addr := resourceAddress(block)
	id := idx.importIdentityID(addr)
	if id == "" {
		return
	}
	e, err := parseIdentity(kind, id)
	if err != nil {
		return
	}
	switch kind {
	case kindModel:
		idx.modelByUUID[id] = addr
	case kindApplication:
		idx.appByModelAndName[e.modelUUID+":"+e.part(0)] = addr
	case kindMachine:
		idx.machineByModelAndID[e.modelUUID+":"+e.part(0)] = addr
	case kindSecret:
		idx.secretByModelAndName[e.modelUUID+":"+e.part(0)] = addr
	}
}

// indexImport records the mapping from a resource address (the `to`
// attribute) to the identity `id` found in the `identity` object.
func (idx *refIndex) indexImport(block *hclwrite.Block) {
	if block.Type() != "import" {
		return
	}
	to := readAttrRef(block, "to")
	if to == "" {
		return
	}
	id := readObjectAttrString(block, "identity", "id")
	if id != "" {
		idx.importIDByAddr[to] = id
	}
}

func (idx *refIndex) importIdentityID(addr string) string {
	return idx.importIDByAddr[addr]
}

func (idx *refIndex) modelRef(modelUUID string) string { return idx.modelByUUID[modelUUID] }
func (idx *refIndex) appRef(mu, n string) string       { return idx.appByModelAndName[mu+":"+n] }
func (idx *refIndex) machineRef(mu, id string) string  { return idx.machineByModelAndID[mu+":"+id] }
func (idx *refIndex) secretRef(mu, n string) string    { return idx.secretByModelAndName[mu+":"+n] }

// --- attribute evaluation via hclsyntax.ParseExpression ---

// attrValue parses an attribute's expression from its raw token bytes and
// evaluates it in an empty context. Only literal expressions produce known
// values; references and unknowns return (NilVal, false).
func attrValue(attr *hclwrite.Attribute) (cty.Value, bool) {
	src := attr.Expr().BuildTokens(nil).Bytes()
	expr, diags := hclsyntax.ParseExpression(src, "<attr>", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return cty.NilVal, false
	}
	v, diags := expr.Value(nil)
	if diags.HasErrors() || !v.IsWhollyKnown() {
		return cty.NilVal, false
	}
	return v, true
}

// readAttrString evaluates a top-level attribute as a plain string literal.
// Returns "" if absent or not a literal string (e.g. already a reference).
func readAttrString(block *hclwrite.Block, name string) string {
	attr := block.Body().GetAttribute(name)
	if attr == nil {
		return ""
	}
	v, ok := attrValue(attr)
	if !ok || !v.Type().Equals(cty.String) {
		return ""
	}
	return v.AsString()
}

// readAttrRef reads a top-level attribute whose value is a dotted reference
// (e.g. `to = juju_model.model_0`), returning "type.label".
func readAttrRef(block *hclwrite.Block, name string) string {
	attr := block.Body().GetAttribute(name)
	if attr == nil {
		return ""
	}
	// References don't evaluate to known values; parse the raw tokens as a
	// traversal and join the steps.
	src := attr.Expr().BuildTokens(nil).Bytes()
	t, diags := hclsyntax.ParseTraversalAbs(src, "<attr>", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return ""
	}
	return traversalString(t)
}

// readObjectAttrString evaluates a top-level attribute as an object literal
// and returns the value of the given string field.
func readObjectAttrString(block *hclwrite.Block, objName, field string) string {
	attr := block.Body().GetAttribute(objName)
	if attr == nil {
		return ""
	}
	v, ok := attrValue(attr)
	if !ok || !v.Type().IsObjectType() || !v.Type().HasAttribute(field) {
		return ""
	}
	fv := v.GetAttr(field)
	if !fv.IsWhollyKnown() || fv.IsNull() || !fv.Type().Equals(cty.String) {
		return ""
	}
	return fv.AsString()
}

// traversalString joins a traversal's steps into "root.attr.attr" form.
func traversalString(t hcl.Traversal) string {
	var parts []string
	for _, step := range t {
		switch s := step.(type) {
		case hcl.TraverseRoot:
			parts = append(parts, s.Name)
		case hcl.TraverseAttr:
			parts = append(parts, s.Name)
		default:
			return ""
		}
	}
	return strings.Join(parts, ".")
}
