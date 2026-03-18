# Terraform Provider for Juju Documentation Standards

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
- **Generic procedures first**: Focus on the logic and flow of the procedure, not specific implementations
  - Example: "1. Set up provider in controller mode, 2. Obtain cloud credentials, 3. Define controller resource"
  - Keep the main workflow abstract enough to apply to any cloud/environment
- **Specific examples in dropdowns**: Cloud-specific or detailed workflows go in expandable sections
  - Label as "Example workflow:" followed by the specific scenario (e.g., "Example workflow: Bootstrap to LXD")
  - Show complete end-to-end implementations with actual values, commands, and code
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
- **Steps = workflow actions only**: Non-workflow info goes in notes/tips, not numbered steps
- **Examples in dropdowns**: Labeled "Example workflow:" or "Preview an example workflow:" not "Complete example:"
- **Example placement by content type**:
  - **Workflow sections** (Bootstrap, Import, Deploy, etc.): Place dropdown **after opening sentence(s)** that explain what/why, but **before generic procedure**
    - Rationale: Shows "proof it works" early, clear scope binding, doesn't bury example
    - Pattern: `[Title] → [1-2 sentences: what and why] → [Dropdown example] → [Generic procedure steps]`
  - **Configuration sections** (parameters, options): Place dropdown **after generic procedure**
    - Rationale: Example needs context from procedure to make sense
  - **Short sections**: Use judgment - if example dominates, consider inline code blocks instead of dropdown
- **No unnecessary subsections**: Examples shouldn't create TOC clutter

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
