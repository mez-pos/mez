# Phase 00 — Repo skeleton, docs bootstrap, design tokens

## Goal
A repository that builds, tests and lints from a single command, with the design
reference archived and the tokens extracted. No application logic yet.

## Depends on
Nothing.

## Tasks

- [x] **Task 1 — Initialise the repo.**
  Confirm the product name and GitHub owner with the human if `STATE.md` still
  lists it as open. Then: `go mod init <module path>`, Go 1.23+. Create
  `.gitignore` (Go, Node, `*.db`, `*.db-wal`, `*.db-shm`, `dist/`, `/data/`),
  `LICENSE` (per `docs/00-product.md`), and a placeholder `README.md`.
  `git init`, first commit.

- [x] **Task 2 — Archive the design reference.**
  Copy everything from `claude-design/` into `docs/design/reference/`, preserving
  structure. Write `docs/design/reference/INDEX.md`: one line per file naming
  which surface and screen it shows, so later phases can find a screen without
  opening every file. Do not transcribe the designs into prose — the files are
  the record. Commit.

- [x] **Task 3 — Verify and complete the tokens.**
  Open the design reference. Check every value in `docs/design/tokens.md` against
  it. Fill in anything the doc marks as "to be defined" — derived text colours,
  hairline, danger. Add any token the designs use that the doc is missing.
  Commit the updated `tokens.md`. Still no code.

- [ ] **Task 4 — Makefile and CI.**
  `Makefile` with: `build`, `test`, `check` (gofmt -l, go vet, go test ./...,
  go build), `run`, `generate`, `migrate`, `clean`, `dist` (cross-compile
  windows/amd64, linux/amd64, darwin/arm64 with CGO_ENABLED=0).
  A GitHub Actions workflow running `make check` on push. Both must pass on an
  empty project.

- [ ] **Task 5 — Folder skeleton.**
  Create the directory tree from `docs/01-architecture.md` with a `doc.go` in
  each Go package stating its one-line responsibility and its import rules.
  Add `internal/domain/imports_test.go` — a test that walks `internal/domain`
  and fails if it imports `internal/db` or `internal/api`. It must pass.

## Done when
`make check` passes on a clean clone. The design reference is in-repo and
indexed. `docs/design/tokens.md` is complete and verified. The human can safely
delete `claude-design/`.

## Notes
Tell the human explicitly, in the final message of Task 2, that the archive is
complete and `claude-design/` can now be deleted.
