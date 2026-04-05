# Iteration 1 — Initial implementation

## What was done

- Go module `github.com/midaswr/elsabo` with a Bubble Tea TUI, FastPanel `mogwai sites list` parser, cloak vault (`manifest.json` + `files/`), SFTP-based replace with HTTP verify and rollback, YAML config with domain tags.
- Makefile + GoReleaser + GitHub Actions workflow for tag-driven releases to `github.com/midaswr/elsabo`.
- Unit tests for mogwai parsing (fixed `strings.TrimRight` argument order bug), HTTP stub detection, and replace rollback paths.

## Why these choices

- **SSH from admin machine**: matches user workflow; no agent on the panel host required beyond normal SSH/SFTP.
- **One SSH session per site**: simpler than pooling; sequential jobs match the spec and avoid connection limits.
- **Strict host key opt-in**: defaults to permissive verify for first-time setup; `ssh_strict_host` enables known-hosts.

## Difficulties

- Bubble Tea requires **pointer model** for `Update`; initial value receivers dropped state.
- `mogwai` output varies; parser supports header rows plus heuristic fallback for path-at-end-of-line rows.

## Next iteration ideas

- Optional **single persistent SSH/SFTP session** for a whole batch.
- `mogwai` JSON driver if the installed version supports it.
- Safer tag editing (validation, presets).
