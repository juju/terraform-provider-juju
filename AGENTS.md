# AGENTS.md

## Start here

- [README.md](./README.md) — project overview, requirements, and build instructions.
- [CONTRIBUTING.md](./CONTRIBUTING.md) — contribution workflow for code and docs.
- [Developing wiki](https://github.com/juju/terraform-provider-juju/wiki/Developing) — provider development guide.
- [Juju](https://github.com/juju/juju) and [JAAS (JIMM)](https://github.com/canonical/jimm) — upstream source repositories.
- [Run an acceptance test](./.github/skills/run-acceptance-test/SKILL.md) — run one named test against a Juju controller.

## Things to keep in mind

- The Terraform provider can target JAAS or a Juju Controller. The API interface is the same for both, but JAAS adds some extra resources and data sources. 
- The client to call into JAAS/Juju is compatible with 3.x and 4.x version of Juju controllers.
- Use the Juju/JAAS CLI implementation as a reference when adding new functionality.
- Not all acceptance tests need to run against all Juju and JAAS version. Evaluate/ask if it's needed before.
- Keep fixes and new features backward compatible. Add a new acceptance test in most cases, unless it would be prohibitively complex or not possible. For risky changes, validate the acceptance test manually.
- Unit tests are not required if the resource/data source is simple, but they are needed if the code has multiple conditions and possible paths.
- Prefer erroring out at plan phase rather than apply phase.
- Many pieces of infrastructure are served via Terraform modules, so additional testing and care should be added to handle "Unknown" values.
