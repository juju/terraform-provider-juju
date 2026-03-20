# Documentation Standards for AI Agents

This guide provides documentation standards for AI agents working on the Terraform Provider for Juju documentation.

## Documentation Framework (Diataxis)

This documentation follows the [Diataxis framework](https://diataxis.fr/):

- **Tutorial**: Learning-oriented lessons for newcomers
- **How-to guides**: Task-oriented recipes for specific goals (what we focus on most)
- **Reference**: Information-oriented complete technical descriptions
- **Explanation**: Understanding-oriented discussions of concepts

### Reference Documentation
- Must be a **complete description** of the software for the current version
- All resources, data sources, and provider configuration options fully documented
- No gaps - every feature covered

### How-to Guides Structure
- **Entity lifecycle organization**: Docs grouped by the entity being managed (controllers, models, applications, etc.)
- **Full lifecycle coverage**: Each entity doc covers all basic operations from creation through destruction
- **Continuous prose over numbered steps**: Use flowing text with "First," "Then," "Next" rather than boldface numbered steps
  - Reduces repetition and visual clutter
  - Makes the workflow feel more natural and less mechanical
- **Examples integrated throughout**: Show relevant examples at each step rather than complete workflows upfront
  - Use "For example:" or "For example, for a [specific cloud/scenario]:" to introduce snippets
  - Keep examples contextual - show just what's relevant to the current step
  - When complete workflow examples are valuable (e.g., for import operations), place them in dropdowns at the end of the relevant step where cloud-specific details matter most
- **Entity-focused naming**: `manage-{entity}.md` pattern (manage-controllers, manage-models, manage-applications)
- **Bridge to related entities**: Include sections that link to other entity lifecycles when they intersect
  - Example: In `manage-controllers.md`, include "Add a cloud to a controller" → links to `manage-clouds.md`
  - Example: In `manage-controllers.md`, include "Add a credential to a controller" → links to `manage-credentials.md`
  - Keep these bridge sections minimal - just enough context and a link to the full entity doc

## Writing Style
- **Eliminate verbosity**: Give information only where needed, only in the amount needed
- **Be direct**: No introductory phrases like "Here's how..." or "I will now..."
- **Use brief bullets**: Not lengthy prose
- **Action-oriented**: Start with what the user needs to do
- **Location before action**: Specify where something should be done before stating what to do
  - Good: "In your `juju_controller` resource, specify the `controller_config` attribute"
  - Bad: "Specify the `controller_config` attribute in your `juju_controller` resource"
  - Rationale: Reader needs context (where) before instruction (what)
- **CLI references with precision**: Use backticks for command names to avoid ambiguity
  - Use "the `juju` CLI" not "the Juju CLI" (backticks clarify it's the juju command, not any CLI that works with Juju)
  - Use "the `terraform` CLI" not "the Terraform CLI"
  - Rationale: Prevents confusion with other tools (e.g., Terraform Provider for Juju also qualifies as "a Juju CLI")
- **Em-dashes with spaces**: Use ` -- ` (space-dash-dash-space) for em-dashes
  - Good: "does not unset it on the controller -- it remains at its previous value"
  - Bad: "does not unset it on the controller - it remains at its previous value"
  - Rationale: Consistent punctuation style across documentation

## Structure & Organization
- **Headings follow "Do X" pattern**: E.g., "Bootstrap a controller", "Import an existing controller"
- **Avoid metalanguage in titles**: Don't describe the documentation itself, describe the task
  - Bad: "Using the provider", "Provider configuration", "Resource creation"
  - Good: "Set up the Terraform Provider for Juju", "Bootstrap a controller", "Import an existing controller"
  - Exception: Reference documentation may use technical/descriptive titles
- **Prefer verbal over nominal readings**: In how-tos, titles should be actions, not topics
  - "Using X" is ambiguous between nominal (topic: "using") and verbal (action: "to use")
  - Reserve "using" for tool modifiers: "Bootstrap a controller using LXD" (tool being employed)
  - Don't use "using" as the main verb: ~~"Using the provider"~~ → "Set up the provider"
- **Titles tell the story**: Reading just the titles in sequence should preview the workflow
  - The TOC becomes a quick outline of what's possible and in what order
  - Example sequence: "Install the provider → Set up the provider → Bootstrap a controller → Enable HA → Import a controller"
- **Steps = workflow actions only**: Non-workflow info goes in notes/tips, not prose steps
- **Example dropdowns for complete workflows**: When showing full end-to-end examples (especially for operations where cloud-specific details vary significantly, like import), place them in dropdowns at the step where they're most relevant
  - Label as "Example: [specific scenario]" (e.g., "Example: LXD controller resource definition for import")
  - Place at the end of the step that defines the resource or where cloud-specific variations matter most
  - Rationale: Shows complete working configuration exactly where users need to see how all the pieces fit together for their specific cloud
- **No unnecessary subsections**: Examples shouldn't create TOC clutter
- **Code blocks with filenames**: Use `{code-block}` directive with `:caption: `filename`` for semantic file labeling

## Anchor Standards
- **Hyphenated lowercase of title**: `(bootstrap-a-controller)=`, `(set-up-the-terraform-provider-for-juju)=`
- **Use correct verb forms**: "set up" (verb) not "setup" (noun) when title uses "Set up"
- **Match title structure**: `(enable-controller-high-availability)=` for "Enable controller high availability"

## Consistency Rules
- **Numbered lists**: Use periods (e.g., `1.` not `1:`)
- **Mode references**: "in controller mode" not "for controller mode"
- **Technical details in context**: OAuth 2.0 clarifications in code comments, not parameter lists
- **Resource vs data source**: Clarify when something is import-as-resource vs read-only data source

## Information Hierarchy
- **Conceptual flow first**: Show the logic/steps before detailed examples
- **Details in expandable sections**: Keep main flow scannable
- **Self-help emphasized**: Point users to `--help` commands and official docs for discovery

## Quality Checks
- Use `make linkcheck` for broken links (not manual grep)
- Fix typos consistently throughout
- Verify all internal references after anchor changes

## For AI Assistants
When working on these docs, apply all the above standards consistently. Ask clarifying questions if the standards conflict with user requests, but default to following these patterns for consistency across the documentation.
