# Sapphire CLI

A tool for managing Sapphire agents.

## Install

### Homebrew

The shipped binary name is `sapphire`.

If your tap is not added yet:

```bash
brew install duggal1/tap/sapphire
```

After the tap is added once, you can use:

```bash
brew install sapphire
```

Then launch it with:

```bash
sapphire
```

Note: `brew install sapphire` on a completely clean machine only works after `duggal1/tap` has already been tapped, or if the formula is accepted into `homebrew/core`. The release flow in this repo publishes to the custom tap, not `homebrew/core`.

## Maintainer Release Flow

Production shipping is wired through GoReleaser and GitHub Actions:

- `.github/workflows/snapshot.yml` validates the release pipeline on PRs and `main`
- `.github/workflows/release.yml` publishes tagged releases and updates `duggal1/homebrew-tap`
- `.goreleaser.yml` builds the `sapphire` binary, archives, checksums, completions, manpages, and the Homebrew formula

Maintainer checklist:

```bash
task build
task test
task release
```

Required GitHub secrets for the release workflow:

- `PERSONAL_ACCESS_TOKEN` or `HOMEBREW_TAP_GITHUB_TOKEN` for pushing the Homebrew tap formula
- optional: `FURY_TOKEN`
- optional: `AUR_KEY`
- optional: `GPG_KEY_PATH`

## Documentation

- [AGENTS.md](AGENTS.md): Comprehensive guide for AI agents operating in this repository.

## Utilities

- `mock-up/`: A directory containing doubler utilities.

## Extended Skills

- `sapphire skills` opens the Extended Skills browser in the terminal.
- Set `SAPPHIRE_API_KEY` or save a key in the Extended Skills dialog to search and install skills.
- Installs write to `<data-dir>/skills/<name>/SKILL.md` and `<data-dir>/plugins/<name>/plugin.json` plus `SKILL.md`.
