// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"golang.org/x/crypto/ssh"
)

// SSHKeyCommentInsensitiveModifier returns a string modifier that normalizes SSH keys
// by stripping comments before comparison. This allows the same key with different
// comments to be treated as semantically equal, preventing unnecessary resource updates.
func SSHKeyCommentInsensitiveModifier() planmodifier.String {
	return sshKeyCommentInsensitiveModifier{}
}

// sshKeyCommentInsensitiveModifier implements the plan modifier.
type sshKeyCommentInsensitiveModifier struct{}

// Description implements [planmodifier.String] and returns a plain text description of the modifier.
func (m sshKeyCommentInsensitiveModifier) Description(_ context.Context) string {
	return "Compares SSH keys without their comments to determine equality"
}

// MarkdownDescription implements [planmodifier.String] and returns a markdown formatted description of the modifier.
func (m sshKeyCommentInsensitiveModifier) MarkdownDescription(_ context.Context) string {
	return "Compares SSH keys without their comments to determine equality"
}

// PlanModifyString implements [planmodifier.String] and modifies the plan value if the clean SSH keys (without comments) match.
func (m sshKeyCommentInsensitiveModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {

	// If state null, they're making a new one.
	if req.StateValue.IsNull() {
		return
	}

	// If plan is unknown, no modification needed.
	if req.PlanValue.IsUnknown() {
		return
	}

	// Extract clean keys (without comments) from both state and plan
	// so that we can compare them purely on authorised key content, ignoring any differences in comments.
	stateKey := strings.TrimSpace(req.StateValue.ValueString())
	planKey := strings.TrimSpace(req.PlanValue.ValueString())

	stateKeyClean, err := extractCleanSSHKey(stateKey)
	if err != nil {
		return
	}

	planKeyClean, err := extractCleanSSHKey(planKey)
	if err != nil {
		return
	}

	// If the clean keys match, use the state value to prevent unnecessary updates.
	// This is fine because it is effectively the same key.
	if stateKeyClean == planKeyClean {
		resp.PlanValue = req.StateValue
	}
}

// extractCleanSSHKey parses an SSH key and returns it without the comment segment.
func extractCleanSSHKey(fullKey string) (string, error) {
	fullKey = strings.TrimSpace(fullKey)

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(fullKey))
	if err != nil {
		return "", err
	}

	cleanKeyBytes := ssh.MarshalAuthorizedKey(pubKey)

	return strings.TrimSpace(string(cleanKeyBytes)), nil
}
