# Sapphire CLI

A tool for managing Sapphire agents.

## Install

### Homebrew

The shipped binary name is `sapphire`.

Recommended one-command install:

```bash
brew install duggal1/Sapphire-cli/sapphire
```

Then launch it with:

```bash
sapphire
```

If you want the tap added explicitly first:

```bash
brew tap duggal1/Sapphire-cli
brew install sapphire
```

Note: plain `brew install sapphire` on a completely clean machine only works after `duggal1/Sapphire-cli` has already been tapped, or if Sapphire is accepted into `homebrew/core`. The release flow here publishes the formula back into this repository.

## Maintainer Release Flow

Production shipping is wired through GoReleaser and GitHub Actions:

- `.github/workflows/snapshot.yml` validates the release pipeline on PRs and `main`
- `.github/workflows/release.yml` publishes tagged releases and updates the Homebrew formula in `duggal1/Sapphire-cli`
- `.goreleaser.yml` builds the `sapphire` binary, archives, checksums, completions, manpages, and the Homebrew formula

Maintainer checklist:

```bash
task build
task test
task release
```

Required GitHub secrets for the release workflow:

- optional: `PERSONAL_ACCESS_TOKEN` or `HOMEBREW_TAP_GITHUB_TOKEN` if you do not want to rely on `GITHUB_TOKEN`
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
