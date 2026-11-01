# Releasing mez

For whoever makes a release. A venue owner wants [`deploy.md`](deploy.md).

A release is a tag. Pushing a tag like `v0.1.0` runs
[`.github/workflows/release.yml`](../.github/workflows/release.yml), which
makes everything and leaves it as a **draft**. Nothing is announced and
nothing is "latest" until you have read the draft and published it.

## Before the tag

1. `main` is green: the CI run for the commit you mean to tag has passed.
2. [`STATE.md`](STATE.md) and [`KNOWN.md`](KNOWN.md) say what is not done.
   A release does not need them empty. It needs them true.
3. Run the Windows installer once on a spare machine, from `make dist &&
   make installer`: install, go through the setup wizard, make a sale,
   uninstall. The workflow compiles the installer; it does not run it.
4. See what the notes will say:

   ```sh
   git tag v0.1.0            # locally, not pushed yet
   sh build/release.sh notes v0.1.0
   ```

   The notes are the commit subjects since the last tag. If one of them
   would mean nothing to an owner, that is fixed in the draft afterwards,
   by hand, not by rewriting history.

## The tag

```sh
git tag -a v0.2.0-beta.2 -m "mez v0.2.0-beta.2: what it is, in one line"
git push origin v0.2.0-beta.2
```

The rules (ADR-410):

- **Versions are SemVer**: `v` and three numbers, with `-alpha.N`,
  `-beta.N` or `-rc.N` before a release. A tag with a hyphen is marked a
  pre-release. Alpha is what only you run, beta what a venue you can stand
  beside runs, `v1.0.0` when ADR-380's conditions are met.
- **A tag is a build that leaves your PC**: an installer given to a venue,
  or a release. Annotated (`-a`), on a commit whose CI passed. Once pushed
  or installed it is never moved or deleted; a wrong one is replaced by the
  next. Phases are not tagged.
- **The build's name** is `git describe`, which the Makefile stamps into
  `mez version` and the file names: `v0.2.0-beta.1-34-g161c1363` is 34
  commits after `v0.2.0-beta.1`, at commit `161c1363`, and `-dirty` if the
  tree had changes.
- **The build number** is the commit count of `main`,
  `git rev-list --count HEAD`. It only grows while `main` is never
  force-pushed or rewritten. The Android app uses it as its `versionCode`
  and `git describe` as its `versionName`, so a tablet's app and its PC
  are named by the same tag.
- **A commit subject is under 72 characters.** The notes below are made
  from the subjects.

| Tag | Where | What |
|---|---|---|
| `v0.1.0-alpha.1` | gone | Lost in the history rewrite of 2026-10-20; not made again |
| `v0.1.0-alpha.2` to `.4` | GitHub | 2026-10-07 and 08, plain tags |
| `v0.2.0-beta.1` | GitHub | 2026-10-26, the first beta (ADR-380) |
| `v0.2.0-beta.2` | GitHub | 2026-11-02, Phase 17; the workflow's first run. A draft until the venue's walk passes on its installer |

## What the workflow does

| Job | Makes |
|---|---|
| make check | Nothing. It runs `make build-web` and `make check`, and stops the release if either fails. |
| binaries | `make dist`: Windows, Linux amd64 and arm64, macOS arm64 |
| Windows installer | `mez_<version>_windows_amd64_setup.exe`, on a Windows runner with Inno Setup |
| Docker image | `ghcr.io/mez-pos/mez:<version>` for amd64 and arm64. Pushed at once. |
| draft release | The files above, `mez.service`, `docker-compose.yml`, `mez_<version>_checksums.txt`, and the notes |

The image is the one thing that is public before you publish: it is pushed
under its version and never as `latest`, so nobody gets it without asking
for it by number.

Run again for the same tag (re-run the workflow), it replaces the files on
the draft that is there.

## After it has run

1. Open the draft under **Releases**. Read the notes. Edit them.
2. Download the installer and the checksums file and check one against the
   other, as the notes tell an owner to.
3. The first time only: the image's package on GitHub starts private. Make
   it public under the repository's **Packages**, or `docker pull` asks
   for a login.
4. **Publish**.

## Taking one back

A release that is wrong is replaced by the next one, not deleted: venues
may already be running it. Say so at the top of its notes, and make the
fix `v0.1.1`. Delete a tag only if its draft was never published.

## What is not here

- **Signing.** The installer is not signed (ADR-265), so Windows warns.
  The notes say so and give the checksum to check instead. With a
  certificate: `MEZ_SIGNTOOL` for `make installer`, and a signing step in
  the workflow, neither of which exists yet.
- **macOS notarisation.** The macOS binary is not signed either, and
  Gatekeeper will say so.
- **The relay.** It is deployed from source by
  [`build/relay/`](../build/relay/README.md), not released as a binary.
- **A changelog file.** The notes on each release are the changelog.

## What has and has not been run

Written on 2026-10-07. On the developer's PC: `make dist` for all four
platforms, `make installer`, the checksums over them and their checking
with `sha256sum -c`, the notes against two test tags in a scratch clone,
the image built for amd64 and arm64 with the arm64 one run, and the
workflow through `actionlint`. **The workflow itself has never run**: no
tag has been pushed, so the Windows runner's Inno Setup, the push to
`ghcr.io` and the making of the draft are as written and not as seen. Expect
the first release to need a second try.
