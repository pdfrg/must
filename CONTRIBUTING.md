# Contributing to must

Thanks for your interest in must. To keep the project maintainable and to avoid
wasted effort, please read the following before writing code.

## Open an issue first (required)

**Please open an issue for discussion and get a maintainer go-ahead before
starting work or opening a pull request.** This applies to features, behavior
changes, redesigns, new configuration options, new dependencies, and anything
touching visual design or defaults.

Pull requests without a linked, pre-approved issue may be closed without
review — not because the work isn't appreciated, but because unsolicited
direction-setting changes often don't match where the maintainer wants to take
the project. An issue first saves everyone time.

Trivial fixes (typos, broken links, obvious one-line bugs with no behavior
change) don't need a full proposal — but when in doubt, open an issue anyway.

## Areas the maintainer steers closely

These are most likely to be declined as unsolicited PRs:

- Player layout, visual identity, and default appearance
- Branding and artwork (including the logo and fallback art)
- Default configuration values and new config surface
- New dependencies (supply-chain and maintenance cost must be justified first)

Proposals in these areas need a convincing issue discussion *before* any code.

## Pull request guidelines

- Keep PRs small and focused: one change per PR, no bundled redesigns.
- Don't add dependencies without prior agreement in the issue.
- Preserve existing behavior and defaults unless the issue explicitly approved
  changing them.
- Add or update tests covering the change.
- Verify before submitting:
  - `go fmt ./...`
  - `golangci-lint run ./...`
  - `go build ./...`
  - `go vet ./...`
  - `go test ./...`
- Describe what you changed, why (link the issue), and how you verified it.
  Screenshots help for visual changes.
