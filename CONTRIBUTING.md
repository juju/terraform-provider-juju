Thanks for your interest in the Terraform Provider for Juju -- contributions like yours make good projects
great!

Whether it is code or docs, there are two basic ways to contribute: by opening
an issue or by creating a PR. This document gives detailed information about
both.

> Note: If at any point you get stuck, come chat with us on
[Matrix](https://matrix.to/#/#terraform-provider-juju:ubuntu.com).

## Open an issue

You will need a GitHub account ([sign up](https://github.com/signup)).

### Open an issue for docs

To open an issue for a specific doc, find [the published doc](https://canonical-terraform-provider-juju.readthedocs-hosted.com/en/latest/tutorial/), then use the **Give feedback** button.

To open an issue for docs in general, do the same for the homepage of the docs
or go to https://github.com/juju/terraform-provider-juju/issues, click on **New issue** (top right corner
of the page), select “Blank issue”, then fill out the issue template and
submit the issue.

### Open an issue for code

Go to https://github.com/juju/terraform-provider-juju/issues  click on **New issue** (top right
corner of the page), select whatever is appropriate, then fill out the issue
template and submit the issue.

> Note: For feature requests please use
https://matrix.to/#/#terraform-provider-juju:ubuntu.com

## Make your first contribution

You will need a GitHub account ([sign up](https://github.com/signup) and [add
your public SSH key](https://github.com/settings/ssh)) and `git` ([get
started](https://git-scm.com/book/en/v2/Getting-Started-What-is-Git%3F)).

Then:

1. [Sign the Canonical Contributor Licence Agreement
   (CLA)](https://ubuntu.com/legal/contributors).

2. Configure your `git` so your commits are signed:

```
git config --global user.name "A. Hacker"
git config --global user.email "a.hacker@example.com"
git config --global commit.gpgsign true
```

> See more: [GitHub | Authentication > Signing commits](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits)

3. Fork juju/terraform-provider-juju. This will create `https://github.com/<user>/terraform-provider-juju`.

4. Clone your fork locally.

```
git clone git@github.com:<user>/terraform-provider-juju.git
```

5. Add a new remote with the name `upstream` and set it to point to the upstream
`juju` repo.

```
git remote add upstream git@github.com:juju/terraform-provider-juju.git
```

6. Set your local branches to track the `upstream` remote (not your fork). E.g.,

```
git checkout main
git branch --set-upstream-to=upstream/main
```

7. Sync your local branches with the upstream, then check out the branch you
want to contribute to and create a feature branch based on it.

```
git fetch upstream
git checkout main
git pull
git checkout -b main-new-stuff # your feature branch
```

8. Make the desired changes. Test changes locally.


----------------
<details>

<summary>Further info: Docs</summary>

The documentation is in `terraform-provider-juju/docs-rtd`.

### Standards

All changes should follow the existing patterns, including
  [Diátaxis](https://diataxis.fr), the [Canonical Documentation Style
  Guide](https://docs.ubuntu.com/styleguide/en), the modular structure, the
  cross-referencing pattern, [MyST
  Markdown](https://canonical-documentation-with-sphinx-and-readthedocscom.readthedocs-hosted.com/style-guide-myst/),
  etc.

### Testing

Changes should be inspected by building the docs and fixing any issues
discovered that way. To preview the docs as they will be rendered on RTD, in
`terraform-provider-juju/docs-rtd` run `make run` and open the provided link in a browser. If you get
errors, try `make clean`, then `make run` again. For other checks, see `make
[Tab]` and select the command for the desired check.

</details>

----------------

9. As you make your changes, ensure that you always remain in sync with the upstream:

```
git pull upstream main --rebase
```

10. Stage, commit and push regularly to your fork. Make sure your commit messages
comply with conventional commits ([see upstream
standard](https://www.conventionalcommits.org/en/v1.0.0/)). E.g.,

```
git add .
git commit -m "docs: add setup and teardown anchors"
git push origin main-new-stuff
```

> Note: For most code PRs, it's best to type just `git commit`, then return; the
terminal will open a text editor, enabling you to write a lengthier, more
explicit message.

> Tip: If you've set things up correctly, typing just `git push` and returning
may be enough for `git` to prompt you with the correct arguments.

> Tip: If you don't want to create a new commit message every time, do
`git commit --amend --no-edit`, then `git push --force`.

11. Create the PR.

12. Ensure GitHub tests pass.

13. In [the Matrix Terraform Provider for Juju
channel](https://matrix.to/#/#terraform-provider-juju:ubuntu.com), drop a link to your
PR with the mention that it needs reviews. Someone will review your PR. Make all
the requested changes.

14. When you've received two approvals, your PR can be merged. If you are part
of the `juju` organization, at this point in the Conversation view of your PR
you can type `/merge` to merge. If not, ping one of your reviewers and ask them
to help merge.

> Tip: After your first contribution, you will only have to repeat steps 7-14.

Congratulations and thank you!


