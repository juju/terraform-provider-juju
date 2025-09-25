// ESM commitlint configuration used by wagoid/commitlint-github-action
// Extends the standard Conventional Commits rules to enforce a set of
// project-specific rules.
// See: https://github.com/conventional-changelog/commitlint

export default {
  extends: ['@commitlint/config-conventional'],

  // Project-specific rules
  rules: {
    // allow long body lines (no limit for git commit messages)
    'body-max-line-length': [2, 'always', Infinity]
  },
};
