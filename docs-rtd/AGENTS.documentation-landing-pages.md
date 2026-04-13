---
customInstructions:
  role: >-
    Documentation standards for Terraform Provider for Juju homepage and landing pages. When asked to
    update landing pages, FIRST assess the current state against these standards, identify gaps, and
    create a todo list grouped by format and content issues. ALWAYS present assessment to user before
    making changes.
applyTo:
  - pattern: "**/index.md"
    reason: Homepage and landing pages follow specific structure and content patterns
  - pattern: "**/tutorial/index.md"
    reason: Tutorial landing page should group and explain content flow
  - pattern: "**/howto/index.md"
    reason: How-to landing page should organize by lifecycle/workflow
  - pattern: "**/reference/index.md"
    reason: Reference landing page should organize by dependency flow and provider patterns
  - pattern: "**/explanation/index.md"
    reason: Explanation landing page should group related conceptual topics
---

# Landing Pages Documentation Standards for Terraform Provider for Juju

## Homepage Intro Paragraphs

**Standard structure**: Four brief paragraphs covering:

1. **What the product is** - Succinct, memorable definition
2. **What the product does** - Core capabilities in plain language  
3. **What problem the product solves** - The need it meets
4. **Who the product is for** - Target audience

**Template example**:
```markdown
Something that says what the product is, succinctly and memorably consectetur adipiscing elit, sed do eiusmod tempor.

A description of what the product does. Urna cursus eget nunc scelerisque viverra mauris in. Nibh mauris cursus mattis molestie a iaculis at vestibulum rhoncus est pellentesque elit.

An account of what need the product meets. Dui ut ornare lectus sit amet lam.

Something that describes whom the product is useful for. Nunc non blandit massa enim nec dui nunc mattis enim.
```

**Key principles**:
- Keep each paragraph brief (1-2 sentences maximum)
- Avoid technical jargon in favor of clear benefits
- Make the first sentence memorable and quotable
- End with a clear call to value for the target audience

## Organizing Principle: Provider Workflow and Resource Categories

**Core principle**: Organize content following the Terraform provider workflow and the logical grouping of Juju resources being managed through Terraform.

### Terraform Provider Dependency Flow

The Terraform Provider for Juju lets you manage Juju resources using Terraform. The dependency flow is:

**Provider Configuration** → **Authentication** → **Juju Resources via Terraform**

1. **Provider Configuration** (setup layer)
   - Installing the provider
   - Configuring provider connection to Juju controller
   - Provider versioning and compatibility

2. **Authentication** (access layer)
   - Static credentials
   - Client credentials (JAAS)
   - Environment variables
   - CLI-based authentication

3. **Juju Resources** (resource layer)
   - Terraform resources (create/manage Juju entities)
   - Terraform data sources (read Juju entities)
   - Organized by Juju resource type (controllers, models, applications, etc.)

### Applying This to Landing Pages

**Reference landing page** (`docs-rtd/reference/index.md`):
Organize sections to reflect Terraform provider patterns:

```markdown
## Provider Configuration
(How to set up and authenticate the provider)
- Provider setup, authentication methods, version compatibility

## Terraform Resources
(Juju entities you can create and manage via Terraform)
- Grouped by entity type: Controllers, Credentials, Models, Applications, etc.
- Each resource links to full reference documentation

## Terraform Data Sources
(Juju entities you can read/reference via Terraform)
- Grouped by entity type
- Explains relationship to corresponding resources

## Provider Actions
(Special operations beyond standard CRUD)
- Enable HA, custom operations
```

**Explanation landing page** (`docs-rtd/explanation/index.md`):
Organize by conceptual foundation:

```markdown
## Provider fundamentals
(How the provider works)
- Provider architecture, authentication flow, state management

## Resource lifecycle
(Understanding how Terraform manages Juju resources)
- Import patterns, update behavior, deletion handling

## Integration patterns
(Best practices for using the provider)
- Multi-controller setups, JAAS integration, version management
```

**How-to landing page** (`docs-rtd/howto/index.md`):
Organize by operational tasks:

```markdown
## Set up the provider
(Initial configuration)
- Install provider, configure authentication, verify connectivity

## Manage controllers and clouds
(Control plane operations)
- Bootstrap controllers, add clouds, manage credentials

## Deploy and manage applications
(Application lifecycle)
- Deploy applications, configure, relate, scale

## Advanced operations
(Complex workflows)
- Import existing infrastructure, migrate between controllers, upgrade patterns
```

**Tutorial** (`docs-rtd/tutorial.md`):
Single comprehensive tutorial following workflow:
- Set up provider → Deploy first application → Scale → Clean up

### Why This Organization Works for Terraform Provider

- **Matches Terraform users' mental model**: "Configure provider → Authenticate → Manage resources"
- **Reflects Terraform patterns**: Resources, data sources, and provider configuration are familiar Terraform concepts
- **Shows Juju through Terraform lens**: Resources organized by Juju entity type, but accessed via Terraform workflow
- **Scales with provider growth**: New resources fit into existing resource categories

## Quality Checklist for Terraform Provider Landing Pages

**Homepage format**:
- [ ] "In this documentation" section with bullet points organized by workflow
- [ ] "How this documentation is organised" section with Diátaxis explanation (text format only - no quadrant grid)
- [ ] "Project and community" section with subsections (Get involved, Releases, Governance)
- [ ] NO horizontal lines (`---`) used for section separators

**Reference**:
- [ ] Provider configuration section covers all authentication methods
- [ ] Resources and data sources clearly distinguished
- [ ] Resources grouped by logical Juju entity categories
- [ ] Links to auto-generated Terraform registry documentation where appropriate

**Explanation**:
- [ ] Provider architecture explained for Terraform users
- [ ] Import/lifecycle patterns documented
- [ ] Differences from Juju CLI usage clarified

**How-to**:
- [ ] Setup guides assume Terraform knowledge, explain Juju specifics
- [ ] Entity-focused organization (manage-controllers, manage-models, etc.)
- [ ] Examples show Terraform HCL, not CLI commands
- [ ] Bridge sections link between related entity types

**Tutorial**:
- [ ] Assumes Terraform familiarity, teaches Juju concepts
- [ ] Uses realistic example application deployment
- [ ] Shows complete workflow from setup to cleanup
- [ ] Points to how-to guides for deeper coverage of each step
