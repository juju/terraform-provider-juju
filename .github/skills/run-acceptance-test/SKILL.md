---
name: run-acceptance-test
description: Run one named Terraform provider acceptance test against a user-provided Juju controller.
---

# Run one acceptance test

1. Get the controller name and exact Go test name from the user if they haven't provided them. If no test controller exists, ask the user to bootstrap one with `juju bootstrap <cloud> <controller>` and stop until it's ready. Otherwise, if the requested controller isn't active, run `juju switch <controller>`. Stop if the switch fails.
2. From the repository root, run the following in Bash, replacing `TestAcc_ResourceModel` with the requested test name. Keep the anchors so only that test runs.

   ```bash
   (
     umask 077
     make generate-env-file || exit
     # Read the generated KEY="value" lines without evaluating credentials as shell code.
     while IFS='=' read -r key value; do
       case "$key" in ''|\#*) continue ;; esac
       value=${value:1:-1}
       if [[ "$key" == JUJU_CA_CERT ]]; then
         value=${value//\\n/$'\n'}
       fi
       export "$key=$value" || exit
     done < ./test.env || exit
     go test ./internal/provider -v -count=1 -timeout 120m -run '^TestAcc_ResourceModel$'
   )
   ```

`test.env` contains controller credentials. Don't print or commit it. Report whether the named test passed, failed, was skipped, or wasn't found.
