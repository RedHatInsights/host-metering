# Contributing

Open a pull request to contribute a change.

The project is using
- [Semantic Versioning](https://semver.org/)
- [Semantic Release](https://semantic-release.gitbook.io/semantic-release/)
- [Conventional Commits](https://www.conventionalcommits.org)
## Commit message format
Is defined by [Conventional Commits](https://www.conventionalcommits.org).
Commit lint enforces the formatting rules on commit messages.

The format is (source [Angular](https://github.com/angular/angular/blob/main/CONTRIBUTING.md#-commit-message-format)
project):

Each commit message consists of a **header**, a **body**, and a **footer**.

```
<header>
<BLANK LINE>
<body>
<BLANK LINE>
<footer>
```

The `header` is mandatory and must conform to the [Commit Message Header](#commit-header) format.

The `body` is mandatory for all commits except for those of type "docs".
When the body is present it must be at least 20 characters long and must conform to the [Commit Message Body](#commit-body) format.

The `footer` is optional. The [Commit Message Footer](#commit-footer) format describes what the footer is used for and the structure it must have.


### <a name="commit-header"></a>Commit Message Header

```
<type>(<scope>): <short summary>
  │       │             │
  │       │             └─⫸ Summary in present tense. Not capitalized. No period at the end.
  │       │
  │       └─⫸ Commit Scope: config|selinux|rpm|daemon|hostinfo|notify|packaging|changelog|
  |                          makefile
  │
  └─⫸ Commit Type: build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test
```

The `<type>` and `<summary>` fields are mandatory, the `(<scope>)` field is optional.


#### Type

Must be one of the following:

* **build**: Changes that affect the build system or external dependencies (example scopes: makefile)
* **chore**: Other changes that don't modify src or test files
* **ci**: Changes to our CI configuration files and scripts (examples: GitHub Actions)
* **docs**: Documentation only changes
* **feat**: A new feature
* **fix**: A bug fix
* **perf**: A code change that improves performance
* **refactor**: A code change that neither fixes a bug nor adds a feature
* **revert**: Reverts a previous commit
* **style**: Changes that do not affect the meaning of the code (white-space, formatting, missing semi-colons, etc)
* **test**: Adding missing tests or correcting existing tests

### <a name="commit-body"></a>Commit Message Body

Just as in the summary, use the imperative, present tense: "fix" not "fixed" nor "fixes".

Explain the motivation for the change in the commit message body. This commit message should explain _why_ you are making the change.
You can include a comparison of the previous behavior with the new behavior in order to illustrate the impact of the change.

### <a name="commit-footer"></a>Commit Message Footer

The footer can contain information about breaking changes and deprecations and is also the place to reference GitHub issues, Jira tickets, and other PRs that this commit closes or is related to.
For example:

```
BREAKING CHANGE: <breaking change summary>
<BLANK LINE>
<breaking change description + migration instructions>
<BLANK LINE>
<BLANK LINE>
Fixes #<issue number>
```

or

```
DEPRECATED: <what is deprecated>
<BLANK LINE>
<deprecation description + recommended update path>
<BLANK LINE>
<BLANK LINE>
Closes #<pr number>
```

Breaking Change section should start with the phrase "BREAKING CHANGE: " followed by a summary of the breaking change, a blank line, and a detailed description of the breaking change that also includes migration instructions.

Similarly, a Deprecation section should start with "DEPRECATED: " followed by a short description of what is deprecated, a blank line, and a detailed description of the deprecation that also mentions the recommended update path.


### Check commit message locally:

```
$ npm install commitlint@latest @commitlint/config-angular@latest
$ npx commitlint --from HEAD~1
```

## Release

Is using semantic release with semantic versioning.

Semantic-release is configured in [.releaserc](.releaserc).

It:
- determines the current version based on git tags
- determines the next version based on commit messages
- bumps version in version/version.go
- generates CHANGELOG.md update
- commits the changes
- creates a git tag
- pushes the changes to the repo
- makes a tarball with vendor dependencies included
- creates a GitHub release with the tarball attached

### Start a release

1. Go to Actions tab on GitHub
2. Run "Release" workflow.

### Testing release

It is a bit tricky to test the release process locally. It might be easier
to do the changes, pushed them to your fork's main branch on GitHub and execute
the release there.

In local machine testing, make sure that the git repo has only your fork as
the git remote to avoid accidental release in primary repo.
