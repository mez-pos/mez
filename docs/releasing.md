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
git tag v0.1.0
git push origin v0.1.0
```

Versions are `v` and three numbers. A tag with a hyphen (`v0.1.0-rc1`) is
marked a pre-release.

## What the workflow does

| Job | Makes |
|---|---|
| make check | Nothing. It stops the release if the commit does not pass. |
| binaries | `make dist`: Windows, Linux amd64 and arm64, macOS arm64 |
| Windows installer | `mez_<version>_windows_amd64_setup.exe`, on a Windows runner with Inno Setup |
| Docker image | `ghcr.io/byjoellobo/mez:<version>` for amd64 and arm64. Pushed at once. |
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
