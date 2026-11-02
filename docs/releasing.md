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

## The tablet app's key

Made once, by you, and never put in the repository (ADR-421). Every
tablet that installs the app trusts this key from then on: lose it, and
each one must uninstall the app and pair again; let it out, and someone
else can make an "update" a tablet would take.

```sh
keytool -genkeypair -keystore mez-release.jks -storetype PKCS12 \
  -alias mez -keyalg RSA -keysize 4096 -validity 10000 \
  -dname "CN=mez, O=mez-pos"
```

`keytool` (in any JDK) asks for a password, which a PKCS12 keystore uses
for the key as well. Back up `mez-release.jks` and its password, apart,
where the venue's backups go. Then, in the repository's **Settings**,
**Secrets and variables**, **Actions**, add four repository secrets:

| Secret | What |
|---|---|
| `MEZ_ANDROID_KEYSTORE_BASE64` | The keystore as base64: `base64 -w0 mez-release.jks`, or on Windows `[Convert]::ToBase64String([IO.File]::ReadAllBytes("mez-release.jks"))` |
| `MEZ_ANDROID_KEYSTORE_PASSWORD` | Its password |
| `MEZ_ANDROID_KEY_ALIAS` | `mez` |
| `MEZ_ANDROID_KEY_PASSWORD` | The key's password: the keystore's, for PKCS12 |

And one repository **variable** (the same page, **Variables**; it is not a
secret), `MEZ_ANDROID_KEY_SHA256`: the key's fingerprint, the `SHA256:`
line of `keytool -list -v -keystore mez-release.jks -alias mez`. The
release checks the package's signer against it and stops if they differ:
a wrong keystore in the secrets would otherwise make an app that no tablet
already holding the real one can update to. A key with no fingerprint
named stops the release too.

Without `MEZ_ANDROID_KEYSTORE_BASE64` a release is made without the app,
the run shows a warning, and the notes say "This release carries no
tablet app". To build it signed on your own PC: the same four names as
environment variables, `MEZ_ANDROID_KEYSTORE` the keystore's full path
in place of the base64, `MEZ_ANDROID_KEY_SHA256` if you want the same
check, then `make android-release`, and `make build` or `make dist` carry
what it made. Gradle keeps the passwords, encrypted, in its configuration
cache under `android-app/.gradle`. The app's version code is the build
number, the commit count of `main` (ADR-410), so a release is tagged on
`main` and from a full clone: `make android-release` refuses a shallow
one. Never a package signed with any other
key: a tablet that installed one could not take the real one without
uninstalling it. Before Google's developer verification reaches India
(2027 at the earliest), register as a developer ($25), naming the app's
id, `com.mezpos.meztablet`, and this key.

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
| tablet app | `make android-release`: the Android app, signed with the key above, for the binaries and the image to carry (ADR-421). Nothing without the key. |
| binaries | `make dist`: Windows, Linux amd64 and arm64, macOS arm64, each carrying the tablet app when there is one |
| Windows installer | `mez_<version>_windows_amd64_setup.exe`, on a Windows runner with Inno Setup |
| Docker image | `ghcr.io/mez-pos/mez:<version>` for amd64 and arm64, carrying the tablet app when there is one. Pushed at once. |
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

On 2026-11-02 (Phase 18 Task 7b): `make android-release` with a throwaway
key on the developer's PC, the package checked with `apksigner` and
`aapt2`, the fingerprint check stopping another key's package, and `make
dist` carrying it. The workflow's tablet app job, the
secrets and the image carrying the app are as written, not as seen, and
`actionlint` was not run on them.
