# Personal fork maintenance

`ooojustin/monarchmoney-cli` has two long-lived branches:

- `main` is an exact fast-forward mirror of `thedavidweng/monarchmoney-cli/main`.
- `jstn` contains the personal fork and is the default and installed branch.

Upstream changes merge from `upstream/main` into `jstn`. The branch is never
rebased or force-pushed. The private `monarch-fork-sync` agent skill performs
the merge, runs the repository gate, pushes both branches, and updates the Nix
source lock in the dotfiles repository.

GitHub CI runs on `main` and `jstn`. Dependabot, Release Please, artifact
publishing, and Codeberg mirroring are disabled in the fork; upstream sync is
the only update channel.
