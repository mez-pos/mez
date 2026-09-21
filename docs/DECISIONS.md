# Decisions

Append-only. Never edit or delete an entry — supersede it with a new one that
references the old number. Keep each entry to roughly ten lines.

Format:

```
## ADR-NNN — Title
Date · Status: accepted | superseded by ADR-NNN
Decision: what was chosen.
Because: the reason, in one or two sentences.
Cost: what this gives up.
```

---

## ADR-001 — The counter machine is the source of truth
2026-09 · Status: accepted
Decision: The venue's own computer runs the server and owns the database. Every
other device (second terminal, kitchen display, waiter phone) is a LAN client of
it. The cloud is optional.
Because: A POS that stops selling when the internet drops is not a POS. Making
the local machine authoritative deletes the entire offline-sync and conflict-
resolution problem instead of solving it badly.
Cost: No multi-venue reporting without the optional cloud layer. Chain-scale
deployments need a different topology later.

## ADR-002 — Go for the server
2026-09 · Status: accepted
Decision: Go 1.23+, single static binary.
Because: Distribution is the whole game for an open-source POS. `pos.exe`,
double-click, done — no Node runtime, no Python, no Docker. Cross-compiles to
Windows/Linux/macOS/ARM from one machine. Low, stable memory over 14-hour
shifts. Readable enough that outside contributors can actually contribute.
Cost: Two languages in the repo and no automatic type sharing with the
frontend. Mitigated by generating TypeScript types from Go structs (ADR-006).

## ADR-003 — SQLite, not Postgres
2026-09 · Status: accepted
Decision: SQLite in WAL mode via `modernc.org/sqlite` (pure Go, no cgo).
Because: At cafe scale — low thousands of orders a day, a handful of concurrent
devices — SQLite has orders of magnitude of headroom. Requiring a second
install and a running service would lose most self-hosters.
Cost: Single-writer. Schema stays deliberately portable so a future cloud
deployment can move to Postgres, but that migration is not free.

## ADR-004 — sqlc, no ORM
2026-09 · Status: accepted
Decision: Hand-written SQL compiled to type-safe Go by sqlc.
Because: Money and invoice code must be auditable at the SQL level. ORMs hide
exactly the queries that need scrutiny and generate surprises under load.
Cost: More typing. Schema changes require regenerating.

## ADR-005 — Money as int64 minor units
2026-09 · Status: accepted
Decision: All monetary values are `int64` paise/cents in a `money.Amount` type
carrying a currency code. Floats are banned repo-wide.
Because: A POS off by ₹0.03 on a three-way split is a broken POS and loses
user trust permanently.
Cost: Explicit conversion at every display and input boundary.

## ADR-006 — Generated TypeScript types from Go
2026-09 · Status: accepted
Decision: API request/response structs are the single source of truth; TS types
are generated into the frontend at build time.
Because: 35+ screens with hand-maintained duplicate types will drift and break
silently.
Cost: A codegen step in the build that must be kept working.

## ADR-007 — Browser in kiosk mode, not Electron or Tauri, for v1
2026-09 · Status: accepted
Decision: The terminal is Edge/Chrome launched with `--app=http://localhost:7777`
in kiosk mode by a desktop shortcut. The server runs as a Windows Service.
Because: Skips an entire desktop build pipeline. Staff see an app with no URL
bar and no way to wander off. Hardware access (printer, drawer) happens
server-side over TCP, so the browser needs no native privileges.
Cost: Dependent on a browser being present. Revisit with Tauri only if a real
hardware requirement demands it.

## ADR-008 — Guest QR ordering via outbound reverse tunnel
2026-09 · Status: accepted
Decision: The local server opens a persistent outbound WebSocket to a small
stateless relay. Guest requests to `order.<domain>/t/<table>` are routed down
that socket.
Because: A guest's phone is on cellular and cannot reach a LAN IP. Outbound
tunnelling needs no port forwarding, no static IP, and works behind CGNAT and
any cafe router. It is also the natural open-core boundary: the POS is free,
the hosted relay is the paid convenience.
Cost: QR ordering pauses when the internet drops. The counter keeps selling.

## ADR-009 — Orders are append-only event streams
2026-09 · Status: accepted
Decision: An order is a sequence of immutable events; the displayed state is a
projection. Voids and discounts are corrective events, not destructive edits.
Because: Gives a real audit trail for tax authorities, makes refunds honest,
and makes the order timeline screen fall out of the model for free.
Cost: More code than a mutable row. Projections must be kept fast.

## ADR-010 — Name, module path and licence split
2026-09 · Status: accepted
Decision: Product is **Nofiltr POS**. Binary and command package are `nofiltr`
(`cmd/nofiltr`, `nofiltr.exe`). Go module is
`github.com/byjoellobo/nofiltr-pos`. Server is AGPL-3.0 at the repo root; the
guest ordering app will carry its own MIT `LICENSE` when `web/guest/` is
created in Phase 11.
Because: The module path is effectively one-way (it appears in every internal
import), so it was settled before the first line of Go. The licence split is
the one `docs/00-product.md` already specifies: copyleft protects the hosted
relay, MIT keeps the embeddable guest bundle frictionless.
Cost: A repo named `nofiltr-pos` publishing a binary called `nofiltr` is a
small inconsistency. Two licences means contributors must know which tree they
are in.

## ADR-011 — LF line endings enforced by .gitattributes
2026-09 · Status: accepted
Decision: `.gitattributes` sets `* text=auto eol=lf`, with CRLF kept only for
`.bat`/`.cmd`/`.ps1` and binary assets marked explicitly.
Because: Development happens on Windows but the build cross-compiles to Linux
and macOS. A `Makefile` (Phase 00 Task 4) or shell script checked out with CRLF
fails on Linux CI in a way that reads as a mysterious syntax error. Setting
this before the first commit keeps the entire history clean and avoids a later
`git add --renormalize` churn commit.
Cost: Windows editors that cannot handle LF will show single-line files. All
current tooling handles LF.

## ADR-012 — INDEX.md indexes screens, not just files
2026-09 · Status: accepted
Decision: `docs/design/reference/INDEX.md` carries one row per screen (35 of
them), keyed by the `sc-if` state token and its line number, rather than the
one line per file that Phase 00 Task 2 literally asked for.
Because: the stated purpose was "find a screen without opening every file", and
three of the five files hold 9, 12 and 14 screens each. Five rows would have
sent every later phase back into a 131 KB file to hunt. The design files turned
out to be single-page prototypes where each screen is a `<sc-if value="{{ s_x
}}">` block, which gives a stable identifier to index on.
Cost: Line numbers drift if the archive is ever re-exported. INDEX.md says so
and gives the grep that recovers them.

## ADR-013 - Tokens corrected from the design, rules left alone
2026-09 · Status: accepted
Decision: Phase 00 Task 3 corrected three factual token values against the
archived design (`--paper` `#FBF7EF` to `#F4F1E9`; radius 0/4/8/12 to 3px and
2px; hairline "12% of primary" to 7% on ink) and added the tokens the design
uses that the doc lacked: ink and paper surface ladders, five-step text ladders
per surface, a second vermilion for paper, per-surface sage/gold/danger,
`--slate` `#7E86A8`, and the four paper status chip triads. Three *rules* the
design contradicts were left unchanged and recorded under "Unresolved
conflicts" instead: the 4px spacing scale, the 13-15px body minimum, and the
guest 16px minimum.
Because: a wrong hex is a fact and the design settles it. A spacing scale or a
touch-size floor is a deliberate constraint, and a prototype rendered for a
desktop browser is weak evidence against an ergonomics rule for a gloved hand
on a 12-inch tablet under glare. CLAUDE.md says to write the case and stop
rather than switch unilaterally.
Cost: `tokens.md` now carries three open conflicts that Phase 05 must settle
before it can generate `tokens.css`.

## ADR-014 - The three token rule conflicts, settled
2026-09 · Status: accepted (settles the open items left by ADR-013)
Decision: (1) Spacing becomes a 2px base scale, 2/4/6/8/10/12/14/16/20/24/32/
48/64, replacing the old 4px grid. (2) Type gets a role-named scale with an
11px floor reserved for uppercase letter-spaced metadata, 13px minimum for
sentence-case, and 15px for every interactive label. (3) The guest 16px rule is
rescoped from body text to form controls, on all surfaces.
Because: a measurement is evidence about a fact, not about a constraint. The
4px grid could not express the 10px gap the design uses 67 times, and every
value in the new scale is a native Tailwind class, so the doc was wrong and it
moved. The type floor is the opposite case: the design sets "Void" at 11px
sentence-case beside "Modify", and a misread there destroys a line on a live
ticket, so the design moves. The 16px conflict was never real, because Safari's
zoom-on-focus applies to input/select/textarea and the mock has none.
Cost: Phase 06 must re-check the terminal's 392px right rail, whose line
lengths assume the smaller type. Widen the rail rather than dropping under the
floor. Four spacing values (7, 9, 11, 18) need snapping when screens are built.

## ADR-015 - `make check` skips vet and test on an empty module
2026-09 · Status: accepted
Decision: The Makefile computes `PKGS := $(shell go list ./...)` and skips
`go vet` and `go test` when it is empty, rather than running them
unconditionally. `make` recipes are POSIX and require `sh` on PATH, so on
Windows it runs from Git Bash or WSL, not cmd.exe or PowerShell.
Because: Phase 00 Task 4 requires `make check` to pass on an empty project, but
`go vet ./...` and `go test ./...` both exit 1 with "matched no packages" when
there are none. `go list` exits 0 and prints nothing, which makes it the honest
probe. The guard disappears by itself the moment Task 5 adds the first package,
so it is not a permanent exception, and the skip is printed rather than silent.
Cost: a typo that removes every package from the module would make `check` pass
loudly-but-vacuously. Acceptable while the tree is empty; the printed "skipped"
lines are the tell.

## ADR-016 - The domain boundary is a test, not a note
2026-09 · Status: accepted
Decision: `internal/domain/imports_test.go` walks the whole `internal/domain`
subtree, parses imports only, and fails on `internal/db`, `internal/api`,
`net/http` and `database/sql`. It reads the module path out of `go.mod` rather
than hardcoding it, includes `_test.go` files, and fails outright if it parsed
zero files. The `cmd/*` packages ship a placeholder `main` that logs its
version and exits.
Because: architecture rules written only in prose decay. This one is the reason
the money logic can be reviewed on its own, so it fails loudly in CI instead.
`net/http` and `database/sql` are banned alongside the two internal packages
because the architecture doc says the domain is "PURE GO, no http, no sql", and
the stdlib is the likelier way that leaks. The zero-files guard exists because a
broken walk would otherwise report success forever, which is worse than having
no test. `cmd/*` needs a real `main` because Go will not build a main package
without one, and `make build` compiles `./cmd/nofiltr` as soon as it exists.
Cost: the test sees direct imports only. A domain file importing some future
`internal/x` that itself imports `internal/db` would pass. Revisit if a third
internal package ever becomes importable from the domain; today none is.

## ADR-017 - The timezone database ships inside the binary
2026-09 · Status: accepted
Decision: `internal/config` blank-imports `time/tzdata`, so every named IANA
zone resolves from the binary itself. `Config.Timezone` is a parsed
`*time.Location`, validated at startup, not a string carried around.
Because: Go finds zone data in the system zoneinfo directories on Unix and, on
every platform, in `$GOROOT/lib/time/zoneinfo.zip`. A venue's Windows machine
has neither. `time.LoadLocation("Asia/Kolkata")` therefore succeeds on the
machine this was written on and fails on the machine it is sold to, which is
the worst place to discover it. Verified, not assumed: the test binary run with
`GOROOT` pointed at a path that does not exist passes with the import and fails
with `unknown time zone Asia/Kolkata` without it.
Cost: about 450 KB in every binary, and zone data frozen at the Go version the
release was built with. A country changing its offset needs a rebuild rather
than an OS update. For a POS that must work with no internet, a rebuild is the
honest channel anyway.

## ADR-018 - A hand-written TOML reader, not a dependency
2026-09 · Status: accepted
Decision: `internal/config/toml.go` parses the flat subset the config actually
uses: top-level `key = value`, `#` comments, double-quoted strings and bare
words. Tables, arrays, inline tables, single-quoted strings and duplicate keys
are refused with a line number. No TOML library is added.
Because: the configuration is six scalar settings. `03-conventions.md` puts the
bar for a dependency at an ADR plus a maintenance commitment, and a parser for
a format we use 5% of is the clearest case of paying that price for nothing.
The subset is about 120 lines and the part that matters is ordering: the
closing quote is found before the `#` scan, so `venue_id = "cafe #2"` keeps its
hash instead of silently becoming `cafe`.
Cost: a file that is valid TOML but outside the subset is rejected rather than
read. That is deliberate, but it means the reader has to be revisited, not just
extended, if configuration ever grows nested sections. Switch to a library at
that point rather than growing this one.

## ADR-019 - Configuration refuses what it cannot honour
2026-09 · Status: accepted
Decision: `config.Load` is strict. An unknown key in `config.toml` is an error
naming the known keys. A `-config` path that does not exist is an error, while a
missing file at the default location is not. `listen_addr` requires a numeric
port, so `:http` is rejected. `data_dir` is resolved to an absolute path.
Precedence is defaults, then file, then environment, then flags, and an
environment variable set to empty counts as a setting, which is how a relay
switched on by a file is switched off again.
Because: every one of these is a silent failure otherwise, and the operator is a
cafe owner who changed a setting and believes it took. A typo'd `listen_add` is
a port change that never happened. A `-config` path with a typo starts a server
configured by a file nobody read. A named port resolves differently per machine,
and a register nobody can reach is a register that cannot sell. `data_dir` is
absolute because a Windows Service starts in `system32`, so a relative default
would put a venue's sales somewhere nobody would think to back up.
Cost: a config file carrying a key for a future version fails the whole load
rather than being ignored, so adding a setting means shipping the code before
the file. Correct for a single self-hosted binary; revisit only if configs ever
have to be forward-compatible across versions.

## ADR-020 - internal/logging is a ninth package, and log_format a seventh key
2026-09 · Status: accepted
Decision: logging setup lives in `internal/logging`, not in `cmd/nofiltr`. The
repository layout in `01-architecture.md` named eight internal packages; this
adds one, and the tree in that document has been updated to match. A seventh
config key, `log_format` (`auto`, `json`, `text`), decides the handler, with
`auto` meaning text when stdout is a character device and JSON otherwise. Log
timestamps are written in UTC regardless of the venue's timezone.
Because: Phase 12 wraps this binary as a Windows Service in `internal/service`,
and a Go main package cannot be imported, so leaving the setup in `cmd/nofiltr`
guarantees a move later. Doing it now costs one directory; doing it in Phase 12
costs a move plus whatever has accumulated on top. `log_format` exists because
Phase 01 Task 2 asks for "JSON in production, text in dev" and nothing in the
configuration could tell those apart. Terminal detection is a guess, so it is
only ever the default and an operator can override it. UTC because log lines are
read next to database rows, which `03-conventions.md` stores in UTC, and a venue
trading through a daylight-saving change would otherwise produce an hour of
ambiguous timestamps.
Cost: one more package in the graph, and `internal/logging` imports
`internal/config`, which makes the logger specific to this application rather
than reusable. Acceptable: it is `internal/`, and the alternative was two enums
for one setting. The rotation policy is constants, not configuration, so a venue
that wants different retention needs a code change until the Phase 10 scheduler
takes it over.

## ADR-021 - The rotating log is written, not depended on
2026-09 · Status: accepted
Decision: `internal/logging/rotate.go` is a size-based rotating `io.WriteCloser`:
at 8 MB the live log becomes `nofiltr.log.1`, the previous `.1` becomes `.2`, and
anything past five is deleted. It closes the file before renaming, appends on
restart rather than truncating, and reopens after a failed rotation instead of
giving up. No `lumberjack` or equivalent.
Because: it is about eighty lines of standard library, and `03-conventions.md`
sets the dependency bar at an ADR plus a maintenance commitment. Numbered
suffixes rather than timestamps because the question a support call asks is
"what happened just before this", and `.1` answers it without anyone parsing a
filename. Closing before renaming is the Windows detail that matters: a rotation
that works on Linux and fails on the machine under a counter is exactly the
failure this project keeps trying to avoid. Appending on restart keeps the lines
written just before a crash, which are the ones worth having. A failed rotation
reopens and keeps writing because the venue is mid-service and an oversized log
is better than no log.
Cost: no compression and no age-based expiry, so a quiet venue keeps six files
that may be months old, and a busy one loses history faster than a time-based
policy would. `01-architecture.md` gives log rotation to the scheduler; when
that lands it should drive this writer rather than replace it.

## ADR-022 - Shutdown is a stack, and it does not stop at the first failure
2026-09 · Status: accepted
Decision: `cmd/nofiltr` keeps a stack of started subsystems and closes them in
reverse order under one 10-second deadline shared by all of them. A close that
fails is logged and joined into the returned error, and the ones below it still
run. The HTTP drain gets a fresh context, never the cancelled one that triggered
the shutdown. The listener is bound in `run`, before anything is announced.
Because: the logger is opened first and has to close last, since everything above
it needs somewhere to report what went wrong on the way down. One failure must
not strand the subsystems below, because the next thing that happens on that
machine is the operator starting the binary again and finding the port or the
database still held. The deadline is shared rather than per-subsystem so that one
hung close cannot turn shutdown into subsystems times the timeout, which on a
venue PC is the difference between a slow restart and one that looks hung. The
fresh drain context is the subtle one: passing the cancelled context makes
`Shutdown` return immediately and kills in-flight requests, which is the opposite
of graceful, and a cashier who pressed Charge mid-restart must see that payment
finish or fail rather than vanish. Binding early turns "port already in use",
the likeliest startup failure on a machine still running the old version, into
an exit-1 with a message instead of an error racing out of a goroutine.
Cost: a subsystem that ignores its context can still consume the whole budget and
leave the ones below it unclosed. That is a bug in the subsystem, and the shared
deadline is what makes it visible rather than merely slow.

## ADR-023 - CORS is opt-in through dev_origin, not a hardcoded localhost
2026-09 · Status: accepted
Decision: an eighth config key, `dev_origin`, names the one browser origin
allowed to call the API cross-site. Empty is the default and means no CORS
headers at all. The value is validated as an origin: http or https, a host, and
no path, query or fragment. A developer running Vite sets
`NOFILTR_DEV_ORIGIN=http://localhost:5173`.
Because: Phase 01 Task 3 asks for "CORS for the dev Vite origin only", and
nothing at runtime can tell a development machine from a venue's counter. The
alternatives were both worse. Hardcoding `http://localhost:5173` ships a
permanent hole: any page served from that port on the cashier's machine could
read the till, and the terminal has no auth until Phase 05. Inferring dev from
`log_level = debug` conflates two unrelated settings, so turning on debug
logging during a support call would quietly widen the API's exposure. The path
check exists because browsers compare origins by scheme, host and port exactly,
so `http://localhost:5173/` with its trailing slash is a rule that silently
never matches, and a rule that never matches is worse than no rule.
Cost: a developer who forgets the variable gets a CORS error in the browser
rather than a working dev server, which is a one-line fix but an unfamiliar one.
Documented in `docs/running.md` when Task 5 writes it.

## ADR-024 - Every response is the envelope, and the chain has one order
2026-09 · Status: accepted
Decision: `/api/v1/health` is registered twice, as `GET /api/v1/health` and as
`/api/v1/health`, and there is a `/` catch-all. The second and third exist so a
405 and a 404 answer in the `{"error": {...}}` shape rather than net/http's
plain text. The middleware chain is fixed, outermost first: requestID,
accessLog, recoverPanic, cors. An inbound `X-Request-Id` is honoured only if it
is printable ASCII no longer than 64 bytes. `recorder` implements `Unwrap`,
`Flush` and `Hijack`.
Because: `03-conventions.md` fixes one error shape for the whole API, and a
client that parses JSON on every path except the two most common failures is a
client with a bug waiting. Go 1.22 pattern precedence makes the double
registration the cheapest way to control 405 without a router dependency.
The chain order is the part worth recording: accessLog must sit outside
recoverPanic, because a panic unwinds past its deferred line before recovery
writes the 500, and the log then claims 200 for a request that failed. That was
verified by swapping the two and watching the assertion turn into
"status = 200, want 500". Request ids are validated because they land in a log
file a human reads, and an unchecked header containing a newline lets a caller
forge a log line. `recorder` implements the three optional interfaces because a
ResponseWriter wrapper that hides the underlying writer is exactly how the
Phase 04 WebSocket upgrade breaks, quietly, three phases from now.
Cost: two registrations per path that needs a controlled 405, which is
repetitive once there are twenty endpoints. Revisit with a small helper then,
not with a router. The 64-byte id limit rejects a few legitimate formats, such
as a W3C traceparent with baggage appended; nothing we run emits one today.

## ADR-025 - The frontends embed behind a build tag, with committed placeholders
2026-09 · Status: accepted
Decision: `package web` lives at `web/` and owns the embedding. By default it
embeds `web/placeholder/{terminal,backoffice,guest}`, three committed
`index.html` files. Built with `-tags frontend` it embeds `web/*/dist` instead.
`web.Placeholder()` reports which, and the server logs a warning at startup when
it is serving the stand-ins. `web.Bundles()` fails if a bundle has no
`index.html`.
Because: `web/*/dist` is build output and is gitignored, so an unconditional
`//go:embed web/terminal/dist` breaks `go build` on a fresh clone and in CI,
which is every build until Phase 05. The phase file allows a build tag or an
empty-dir fallback, and the tag is the honest one: the alternative, committing
placeholders inside `dist/`, means rewriting the ignore rules to
`web/*/dist/*` plus a negation, after which every real Vite build shows up as a
modified tracked file. The package sits at `web/` because a `go:embed` pattern
cannot reach outside its own package directory; the split is a good one anyway,
since `web` knows where the bundles are and `internal/api` knows how to serve
them, and they share a `map[string]fs.FS` rather than a type.
Cost: `go build -tags frontend` fails today with "pattern all:backoffice/dist:
no matching files found", which is correct but only obvious if you know why.
Phase 05 must add the tag to the release build and to `make dist`; until then
every binary, including the one Task 5 cross-compiles, ships placeholders.

## ADR-026 - The SPA fallback stops at the first dot
2026-09 · Status: accepted
Decision: within a mount, a request for a path that does not exist falls back to
`index.html` only when the path has no file extension. A missing `.js`, `.css`
or `.png` gets a 404 in the JSON error envelope. `/api/` is claimed before the
mounts so an unknown endpoint keeps the envelope instead of reaching the
terminal bundle. `index.html` is served `Cache-Control: no-cache`; anything
under `assets/` gets a year and `immutable`.
Because: "serve index.html for anything missing" is the usual shorthand and it
is wrong. A missing script then arrives as HTML with a 200, the browser reports
a MIME type error, and it reads like a bundler problem rather than a missing
file. The extension test is a heuristic, but it is the one that matches how
these apps are actually built: client-side routes are words, assets are names
with a dot in them. The `/api/` claim matters more than it looks: with a bundle
mounted at `/`, a typo in an endpoint would answer a terminal parsing JSON with
a page of HTML. Caching is split because `index.html` is what names the hashed
assets, so a cached copy pins a tablet to the build it was opened on and the
kitchen display keeps loading yesterday's bundle until someone hard-refreshes
it; the hashed files cannot go stale, by construction.
Cost: an extensionless asset, which some fonts and source maps manage to be,
would be answered with the app instead of a 404. Nothing Vite emits today is
extensionless. If that changes, test for the mount's asset directory as well as
the extension.

## ADR-027 - build_date is the commit date, not the wall clock
2026-09 · Status: accepted
Decision: the Makefile stamps `main.version` from `git describe --tags --always
--dirty`, `main.commit` from `git rev-parse HEAD`, and `main.buildDate` from
`git show -s --format=%cI HEAD`, the committer date. All three appear at
`/api/v1/health`. When the ldflags are absent, `cmd/nofiltr/version.go` falls
back to the VCS stamps the Go toolchain embeds by itself.
Because: a wall-clock build date makes every build of the same commit a
different binary, which throws away what `-trimpath` was bought for. With the
commit date, two builds of one commit are byte-identical, so a release can be
checked against its tag by anyone who can run the build. Verified rather than
assumed: two consecutive `make dist` runs produced the same SHA-256, and
swapping the commit date for `date -u` produced different ones.
The `debug.ReadBuildInfo` fallback exists because the Makefile is not the only
way this gets built. `go run ./cmd/nofiltr` would otherwise report version
"dev" and no commit at all, which is the state a developer is in when they most
need health to say what is running. It also marks a build from a dirty tree as
"dev-dirty", the same honesty `git describe --dirty` gives the Makefile path.
Cost: `build_date` answers "what was this built from", not "when was this built".
For a binary a venue will run for a year, the first question is the useful one,
but anyone debugging a stale deploy should read it as a commit date. Named as
such in `docs/running.md`.

## ADR-028 - Capturing flag output means printing the usage ourselves
2026-09 · Status: accepted
Decision: `config.Usage()` returns the flag documentation, and `cmd/nofiltr`
prints it to stdout on `flag.ErrHelp` and exits 0. One `newFlagSet` builds the
flags for both parsing and usage.
Because: `parseFlags` sends the flag package's output to a buffer so that an
unknown flag comes back as a wrapped error instead of being printed from inside
a library. That capture also swallowed the usage text, so `./nofiltr -h`
printed nothing at all and exited 0, and a code comment claimed the opposite.
It was found by checking a line of `docs/running.md` against the binary rather
than by a test, which is the argument for writing the operator documentation in
the same task as the feature. `Usage()` shares `newFlagSet` with `parseFlags`
so the two cannot end up describing different programs.
Cost: this is the one place the server writes to stdout without going through
slog. That is correct, since an answer to an operator who typed -h is not a log
line, but it is an exception to the rule in `03-conventions.md` and is commented
as one where it happens.

## ADR-029 - Every money operation returns an error, and the zero Amount is not an identity
2026-09 · Status: accepted
Decision: `Add`, `Sub`, `Mul`, `Neg` and `Compare` on `money.Amount` all return
an error. Currencies must match exactly, `Amount{}` with its empty currency
matches nothing, and no operation may produce or accept `math.MinInt64`.
Because: the alternative reads better and hides the two bugs that actually
happen. A zero-value identity turns a field nobody filled in into a silent
"zero of whatever you like", which is how a dollar line joins a rupee total; a
wrapping int64 turns a refused total into a plausible one. `MinInt64` is banned
because it has no positive counterpart, so an amount holding it could not be
negated for a refund: barring it as a *result*, not just as an input, is what
makes that guarantee true. `Sum(currency, ...)` exists so that a running total
still starts from something usable and an empty order still totals zero.
Cost: `a.Neg()` is two lines instead of one, everywhere, forever.

## ADR-030 - Currencies are checked for shape, not against a list
2026-09 · Status: accepted
Decision: a currency is three uppercase ASCII letters. Lowercase is rejected,
not folded. No list of real currency codes is kept. A short table in
`currency.go` records only the currencies whose minor unit is not one
hundredth, and an unknown code is assumed to have two decimal places.
Because: a full ISO 4217 list is a standing maintenance chore that buys almost
nothing, and it rejects a currency on the day it is introduced. Exponents are
different: they change how an amount is *printed*, and a yen rendered as 12.34
when it means 1234 is a hundredfold error on a real bill. Folding "inr" to
"INR" would be friendlier, but a silent transform in a money path can hide a
broken import file for a month.
Cost: `Amount{Minor: 1, Currency: "ZZZ"}` is accepted, and an unknown
three-decimal currency would print wrong until its row is added.

## ADR-031 - One canonical text form, and excess precision is refused
2026-09 · Status: accepted
Decision: `String` writes "1234.00 INR" and `Parse` reads that back, plus the
code-first order and grouped digits. Fewer decimals than the currency has are
padded; more are an error, never rounded. The domain knows no currency symbols.
Because: `String` is for logs, tests and files, where round-tripping matters
more than looking like a receipt; symbols, grouping and placement are locale
questions the UI answers. Rounding at the parsing boundary would be rounding at
the wrong moment, against the once-at-the-end rule in `docs/02-domain.md`, and
quietly dropping a digit from a file a venue imported is how money disappears.
Cost: an import that legitimately carries sub-paisa precision must decide what
to do about it at the call site rather than being handled here.

## ADR-032 - SplitEvenly breaks ties by position
2026-09 · Status: accepted
Decision: `SplitEvenly` gives the leftover minor units to the earliest parts,
so a thousand rupees three ways is 333.34 / 333.33 / 333.33 in that order. A
negative total splits symmetrically.
Because: largest remainder with equal weights leaves every remainder equal, so
something has to break the tie, and it has to be the same something every time:
a split payment is replayed from the event stream and must land on the same
numbers it did the first time. Rotating or randomising who carries the paisa
would be fairer across many bills and unreproducible on any one of them.
Cost: over a long run the first payer carries every extra paisa. At one paisa a
split that is noise, and it is the price of a deterministic projection.

## ADR-033 - The pickup code filter matches readings, not characters
2026-09 · Status: accepted
Decision: `Blocklist.Blocks` expands a code into every way it can be read
aloud, mapping each digit to the letters it resembles (0 to O, 1 to I or L, 5
to S, and so on), and blocks the code if any reading contains a blocked word
anywhere. The built-in list is English; `NewBlocklist(extra ...string)` takes
the venue's own language. Words longer than a pickup code, and words that are
not plain letters, are dropped when the list is built.
Because: the Crockford alphabet has no I, L, O or U, so a literal match would
find nothing, while `5H1T` and `C0CK` are both four-character codes a plain
generator hands out perfectly happily. Matching anywhere in the code rather
than only the whole of it is for the same reason: a code is heard as one
sound, and nobody hears where the word stops. A rejected draw costs one
redraw, so false positives are free and false negatives get called across a
counter.
Cost: the list is a judgement call that lives in the repository, and a word
written in leetspeak by whoever extends it will be silently dropped rather
than blocked. Tested and documented, not obvious.

## ADR-034 - Public IDs are forty bits and the unique index is what enforces
2026-09 · Status: accepted
Decision: `NewPublic` draws eight Crockford symbols, forty bits, after a
prefix: `ord_7K3M9QX2`, the format the phase file specifies. The package does
not claim they are unique. Every column holding one needs a unique index, and
the insert redraws on conflict.
Because: forty bits is roughly one expected repeat per one and a half million
draws, by the birthday bound. That is far beyond one venue's lifetime and well
inside the reach of a fleet, so a guarantee the generator cannot make belongs
in the database, where it is one index and a retry. The collision test asserts
a bound rather than zero repeats for the same reason: demanding zero would
fail about a third of the time for a perfectly good generator, and what it is
really looking for is a broken one, which produces thousands.
Cost: every table with a public ID column carries a constraint the schema must
not forget, and Phase 02 Task 4 is where that starts. If a longer code is ever
wanted, the format is public by then and changing it is a migration.

## ADR-035 - Randomness is injected the way the clock is
2026-09 · Status: accepted
Decision: `Generator` holds an `io.Reader`. `NewGenerator(nil)` means
`crypto/rand`. The package-level `NewPublic` and `NewPickupCode` are thin
wrappers over it, so the common call stays one argument.
Because: `docs/03-conventions.md` injects the clock into domain code for a
reason that applies unchanged to randomness. A package that reaches for its
own entropy can be tested for the shape of what it returns and nothing else:
not that the high three bits of each byte are discarded, not that a blocked
pickup code is redrawn rather than returned, not that a stuck source fails
instead of looping. All three are tested here because the source is a
parameter. `crypto/rand` rather than `math/rand` because a guessable public ID
is a way to read somebody else's order.
Cost: a second way to call everything, and a nil argument that means
"production" rather than "nothing".

## ADR-036 - modernc.org/sqlite pinned to v1.39.0 to keep the Go 1.23 floor
2026-09 · Status: accepted
Decision: the first dependency in the tree is `modernc.org/sqlite`, pinned to
v1.39.0. It brings `modernc.org/libc`, `mathutil`, `memory`, `go-strftime`,
`go-isatty`, `go-humanize`, `bigfft`, `google/uuid`, `golang.org/x/exp` and
`golang.org/x/sys` with it.
Because: `go get modernc.org/sqlite@latest` takes v1.59.0, which declares
`go 1.25.0` and silently rewrote the module's own directive from 1.23 to 1.25.
The stack in CLAUDE.md says Go 1.23, and CI reads its toolchain from
`go.mod`, so that one line would have raised the floor for every contributor
as a side effect of adding a driver. v1.39.0 is the newest release that still
declares 1.23. The driver is pure Go, which is the point: no cgo, so the
binary cross-compiles with `CGO_ENABLED=0` and a venue machine needs no
toolchain.
Cost: the SQLite engine is a year behind upstream, on a project that handles
money. Moving to v1.59 is `go get` plus a one-line floor change, and should be
a deliberate decision rather than a side effect of the next `go get`.

## ADR-037 - One writer connection, a read-only read pool
2026-09 · Status: accepted
Decision: `db.Open` returns two `*sql.DB` over the same file. The writer is
capped at `SetMaxOpenConns(1)` and opened with `_txlock=immediate`; the read
pool is sized from the machine and opened `mode=ro`. `Close` runs
`PRAGMA wal_checkpoint(TRUNCATE)` before closing either.
Because: SQLite has one writer. Capping the writer at one connection moves the
queue into Go, where waiting is orderly, instead of into SQLite, where it
surfaces as a busy error in the middle of a sale; a test with twenty
concurrent read-modify-write transactions lands on twenty. `_txlock=immediate`
takes the write lock as a transaction begins rather than discovering half way
through that somebody else holds it. `mode=ro` turns "do not write through the
reader" from a convention everyone has to remember into an error SQLite
raises, which a test confirms by trying. The checkpoint on close matters
because a venue machine is switched off at the wall most nights, and a
database with a full write-ahead log is not one a backup can copy on its own.
Cost: two handles to pass around, and a caller who takes `Writer()` for a read
gets no parallelism.

## ADR-038 - Connection settings live in the DSN, not in a PRAGMA after Open
2026-09 · Status: accepted
Decision: `journal_mode`, `foreign_keys`, `busy_timeout` and `synchronous` are
set as `_pragma=` parameters in the connection string.
Because: only `journal_mode` is stored in the file. The other three are per
connection, and `database/sql` opens connections whenever the pool decides to,
so the obvious `db.Exec("PRAGMA foreign_keys = ON")` after `sql.Open` protects
exactly the first connection and leaves every later one unguarded. Nothing
about that failure is visible until a foreign key silently does not fire on a
busy till. Putting the settings in the DSN makes the driver reapply them on
every new connection, and the test holds four connections open at once to see
the difference, because a test that takes one connection passes either way.
Cost: the settings are inside a string the compiler does not check, so the
tests assert the values back out of live connections rather than trusting it.

## ADR-039 - The health check is a cached background probe
2026-09 · Status: accepted
Decision: `DB` owns a goroutine that probes every fifteen seconds and caches
the answer; `Health()` returns it with no IO. `Open` probes once before
returning. `internal/api` takes `WithDBCheck(func() string)` rather than a
database handle, and `cmd/nofiltr` maps between the two vocabularies
explicitly.
Because: the health handler's own comment already said it: whatever is calling
`/api/v1/health` is usually calling it because something has hung, so an
endpoint that queries the database is one more thing that can hang exactly
when it is needed. A cached answer is also a better signal, since a
`CheckedAt` that stops moving is itself a symptom. The probe reads
`sqlite_schema` rather than `SELECT 1`, which is answered from memory and
would keep reporting ok with the file deleted underneath it. The function
argument keeps `internal/api` free of a storage import, and the explicit
mapping in `cmd/nofiltr` means the two packages do not agree by coincidence
because their constants happen to spell the same three words.
Cost: a status up to fifteen seconds stale, and one more goroutine to stop on
shutdown.

## ADR-040 - goose as a library, migrations embedded, applied on every startup
2026-09 · Status: accepted
Decision: `github.com/pressly/goose/v3` v3.26.0, pinned like the driver to a
release that still declares go 1.23. Migrations live in
`internal/db/migrations/*.sql`, are embedded with `go:embed`, and `Migrate`
runs on every startup before the listener serves anything. A failure to
migrate is a failure to start. goose's own logging is routed into slog, and
its `Fatalf` is implemented as a log line, not an exit.
Because: a venue machine restarts several times a day and nobody is going to
run a migration tool by hand at six in the morning, so applying on startup is
the only thing that works; that makes idempotence a requirement, not a nicety,
and it is tested. Embedding means the binary carries its own schema and a
restore is one file to copy. Failing loudly is the point of the exercise: a
server running against a half-migrated database writes rows the next version
cannot read. `Fatalf` is overridden because a library calling `os.Exit` inside
a server is how a machine stops mid service with nothing in the log.
Cost: four more dependencies for something a few hundred lines could do, and
the migration runs on the single writer connection, so a very long migration
blocks startup rather than running in the background. Both are the right way
round for a till.

## ADR-041 - The schema enforces the money and time invariants itself
2026-09 · Status: accepted
Decision: every monetary column carries `CHECK (typeof(x) = 'integer')`.
Timestamps are `TEXT`, ISO 8601, UTC, with a `GLOB` check, and no SQL default.
Booleans are `INTEGER` 0 or 1 with a check. Every public ID column has a
unique index.
Because: SQLite's INTEGER affinity is not a type. Given 12.5 it neither
refuses nor truncates: it stores a REAL, and the column holds half a paisa
forever. `typeof` is the only reliable guard, and it is worth having because
the first invariant in CLAUDE.md should not depend on every future caller
remembering it. The same reasoning covers the rest: SQLite will store the text
`'false'` in a boolean column, which every truthiness test then reads as set,
and it will store a local timestamp that reads as UTC three hours later. No
default on the timestamps because the clock is injected in Go, and a column
defaulting to `now()` silently goes around that. The unique indexes are
ADR-034 landing: a forty bit public ID needs a constraint to collide against.
Cost: a wordy schema, and a check per column that every later migration has to
remember. `docs/03-conventions.md` now lists them so it is a rule to follow
rather than a pattern to notice.

## ADR-042 - Migrations are numbered sequentially, not by timestamp
2026-09 · Status: accepted
Decision: `00001_venues_staff_devices.sql`, and `make migrate-new NAME=x`
computes the next number in plain shell rather than shelling out to goose.
Because: goose defaults to timestamps, which exist to stop two branches
picking the same number. This project has one line of history, so the number
is more useful as the order things ran in, and a gap or a repeat is then
visible on sight; a test asserts the sequence is 1..N with no gaps. Plain
shell means adding a migration needs no tool installed, which matters because
the server applies migrations itself and nothing else requires the goose CLI.
Cost: two people writing migrations at once would collide on a number, and
would have to renumber before merging. If that ever becomes real, timestamps
are the answer and this decision should be superseded.

## ADR-043 - The declared column type is the contract with Go
2026-09 · Status: accepted, amends ADR-041
Decision: timestamp columns are declared `TIMESTAMP` and boolean columns
`BOOLEAN`, not `TEXT` and `INTEGER`. The writer DSN sets
`_time_format=sqlite`, so a `time.Time` is stored as
`2026-09-18 04:05:06+00:00`, and the `GLOB` checks from ADR-041 match that
form and require the `+00:00`. Migration 00001 was edited rather than amended,
since nothing is deployed and SQLite cannot change a column type in place.
Because: SQLite ignores the declared type beyond affinity, but sqlc and the
driver both read it, which makes it the contract. Declared `TEXT`, a timestamp
generates a `string` and will not scan into a `time.Time` at all; declared
`TIMESTAMP` it generates `time.Time` with no sqlc override at all, and
`BOOLEAN` generates `bool` instead of an `int64` every caller compares against
1. One character of schema replaces a growing list of per-column overrides
that somebody would eventually forget to add.
The `_time_format` is not cosmetic. Without it the driver writes Go's
`time.Time.String`, `2026-09-18 04:05:06 +0000 UTC`, which SQLite's date
functions cannot read and which the `GLOB` check rejects, so the first write
through the generated code failed. RFC 3339 with a `Z` is not on offer: the
driver supports exactly one write format.
Cost: stored times are SQLite's form rather than RFC 3339, and a value read
back carries a zero offset with no zone name rather than `time.UTC`, so
anything rendering one calls `.UTC()` first or it serialises as `+00:00` where
the rest of the API says `Z`. Both are in `docs/03-conventions.md`.

## ADR-044 - sqlc generates from the migrations, and the output is committed
2026-09 · Status: accepted
Decision: sqlc v1.31.1, `sqlc.yaml` at the root, schema read from
`internal/db/migrations`, queries from `internal/db/queries`, output committed
to `internal/db/gen`. `emit_interface` and `emit_empty_slices` are on.
`database.Queries()` returns the query set on the read-only pool and
`database.WriteTx` runs one inside a transaction.
Because: pointing sqlc at the migrations rather than at a separate schema file
means the generated code and the database cannot disagree, and a query naming
a column that does not exist fails `make generate` rather than a request
during service. Committing the output means a clone builds without sqlc
installed, which matters for a project whose whole promise is that it runs on
a bare machine. `emit_empty_slices` because a `:many` that matches nothing
should reach a terminal as `[]`, not `null`, and a client handling both is a
client with a bug in it. The two accessors exist so that the read-only pool is
the default path and a write has to say so.
Cost: generated code in review diffs, and a second thing to run after editing
SQL. `make generate` finds sqlc in GOPATH/bin as well as on PATH, because
`go install` puts it somewhere Windows does not search by default.

## ADR-045 - Event IDs come from seq, and decoding is strict on types, lenient on fields
2026-09 · Status: accepted
Decision: an event that creates something identifies it by its own `seq`. The
line a `line.added` adds is line `seq`, and the payment a `payment.tendered`
records is payment `seq`. `line.added` and `payment.tendered` carry no ID
field of their own. On read, an unknown event type is an
`*UnknownEventTypeError` (wraps `ErrUnknownEventType`) and fails the whole
stream. An unknown field inside a known payload is ignored. Three payload
additions to the table in `docs/02-domain.md`: `line.added` carries `name`
(the snapshot rule asks for item names), and `discount.applied` and
`note.added` carry `line_id` when their scope is `line`. A percentage discount
is `basis_points`, an amount discount is `amount`, and `kind` says which.
Because: `seq` is already dense and unique per order, so it is a stable ID
that needs no generator and cannot collide, and a `line.voided{line_id: 3}`
reads as "the line event 3 added", so the timeline explains itself. Skipping
an unknown type would drop a payment or a void and produce a balance that
looks right and is not, so an old build must refuse the order rather than
guess. Unknown fields go the other way. The event log is permanent, and a
reader that rejected a newer optional field would make old orders
unreadable to it after a partial upgrade.
Cost: the leniency only holds under one rule, which code cannot enforce:
**anything that changes what an order is worth must be a new event type, not
a new field on an existing one.** A reviewer has to hold that line. The tax
snapshot on `invoice.issued` is raw JSON until task 5 defines its shape.

## ADR-046 - The projection's money rules
2026-09 · Status: accepted
Decision: `order.Project` is strict. It checks each event against the state
before it and recomputes every total after it, so the first impossible event
fails with a `*ProjectionError` naming its seq. The rules:
- Currency is fixed by `order.opened`, which gained a `currency` field. Every
  later amount must be in it.
- A line is worth `qty × (unit_price + Σ modifier prices)`, via the new
  `money.Amount.Times`. Voided and comped lines leave the total; their gross
  is kept as `Voided` and `Comped` for reporting.
- Line discounts come off each line's gross, then order discounts come off
  what is left. Each percentage is rounded once, half to even. Percentages
  track their base, so a quantity change after 10% off is still 10% off.
  Discounts that exceed what they apply to are an error, never clamped.
- `BalanceDue = Total - (Paid - Refunded)`. A tender may not exceed the
  balance due; change for cash is not part of the order. Tips are summed
  apart and never touch the balance.
- A refund needs a negative balance (the venue owes the guest), no more than
  that, and no more than the payment has left. A closed order accepts only
  `order.reopened` and `note.added`, so refunding a settled bill means
  reopening it and recording why (a void, a comp, a quantity cut).
- Close and invoice both need a balance of exactly zero. One invoice per
  settlement; a reopen allows the next.
- Voids, comps, payment voids and reopens need a reason and a non-zero
  authoriser. A comped line can still go to a station; a voided one cannot.
- Tax is zero until phase 03, task 5 puts a tax snapshot on `line.added`.
Because: a service appends an event by projecting the stream with it on the
end, so the projection is the one place these rules can live. Recomputing
after every event keeps percentages honest when quantities change, and costs
nothing at the size of a real order. The refund rule replaced an earlier plan
in which refunds sat outside the balance. That plan let a closed order be
refunded with nothing in the stream saying why, and it could not express
"voided after payment, refund the difference" without a balance that
disagreed with the money. Tying refunds to a negative balance makes every
refund explain itself in the events before it.
Cost: refunding a closed order takes two extra events (reopen, then the
correction). Requiring a zero balance before `invoice.issued` means a bill
presented before payment is a pro-forma with no number; phase 07 must honour
that or revisit this rule. Recomputing is O(n²) in events, which is fine to
hundreds of events and would need a snapshot well before it hurt.

## ADR-047 - Lifecycles are fsm transition tables, and the projection is checked against them
2026-09 · Status: accepted
Decision: `internal/domain/fsm` holds a generic `Machine[S ~string]`: a fixed
list of states and the moves out of each, with no current state of its own.
An illegal move is a `*fsm.TransitionError` unwrapping to
`fsm.ErrInvalidTransition`; staying in the same state is not a move. The four
lifecycles are tables built on it: order and payment in
`order/transitions.go`, table in `internal/domain/table`, station ticket in
`internal/domain/kitchen`. `order.Project` checks every status change it
derives against the order machine, and voids and full refunds of payments
against the payment machine, so a void of a voided payment matches both
`ErrPaymentNotCaptured` and `fsm.ErrInvalidTransition`. The tests restate
each table by hand from `docs/02-domain.md` and walk every pair of states
through `fsmtest.Exhaustive`, not a sample. Moves added beyond the domain
lines:
- order `settling -> open`: the only live tender was voided or refunded.
- order `open -> closed`: settled at zero with no tender (comped in full).
- table `seated -> free`: guests left before ordering.
- table `settling -> ordered`: another round after asking for the bill.
`held` (order) and `pending` (payment) exist in the tables but no event
produces them yet.
Because: one table per lifecycle is one place to read the rules, and a
machine with no state is a pure function the projection can call. The
projection derives status from its events, so without the check the derived
status and the documented lifecycle could drift apart unnoticed. Restating
the tables in the tests means a slip has to be made twice, the same way, to
pass. Table and ticket live in their own packages because they are not
orders: phase 08 builds on them without importing the order package.
Cost: two copies of every table, one in code and one in its test, by design.
The ticket table has no recall or cancel move; phase 08 adds what the kitchen
display turns out to need. A machine built with `fsm.Must` panics at startup
on a malformed table, which each package's tests catch first.

## ADR-048 - The invoice series: local-time years, a transactional allocator contract, and Audit
2026-09 · Status: accepted
Decision: `internal/domain/invoice` owns the format and the contract; the
database owns the counter.
- A financial year is judged in the venue's time zone, passed in
  (`FYRule.YearOf(at, loc)`), never UTC and never a clock. `IndiaFY` starts 1
  April, `CalendarFY` 1 January. Start days are 1 to 28 so the boundary
  exists every year.
- A number format is a parsed `Template` with `{prefix}`, `{fy}` (2026-27),
  `{fy:short}` (26-27), `{seq}` and `{seq:0Nd}`. Exactly one `{seq}` is
  required. A number wider than its padding is written in full, never
  truncated.
- `Series.MaxLength` caps a number (16 for GST). `NewSeries` formats a first
  number at setup, so a series that cannot issue anything fails there. The
  documented default `{prefix}/{fy}/{seq:06d}` is 17 characters with a
  two-letter prefix, one over GST's cap: a GST series should use
  `{prefix}/{fy:short}/{seq:06d}`.
- `Allocator.Next(ctx, key)` is the contract the DB layer implements: first
  number 1, then last committed plus one, keys independent, bound to the
  sale's transaction so a rollback returns the number, serialised per key.
  `invoicetest.Conformance` is that contract as a runnable suite; the DB
  allocator must pass it. `invoicetest.Memory` is a reference allocator that
  models SQLite's single writer.
- `invoice.Issue` is the one path from a sale to a number. Any error from it
  means roll back.
- `invoice.Audit` states the gapless guarantee: per key, the numbers are
  exactly 1..n. The property test runs 300 random scripts of committed and
  rolled-back sales across two venues, two series and a year end. The
  reference allocator passes all of them; an autoincrement-style allocator
  that ignores rollback fails most of them with `ErrGap`, which proves the
  property is testing what it claims.
Because: the year boundary is a legal fact about the venue's clock on the
wall, and a sale at 00:30 on 1 April in India is 31 March in UTC. Putting the
contract in the domain with a conformance suite means the rule is written
once and the DB layer is held to it by a test, not by a comment. Audit is
reusable beyond tests: a Z-report or tax export can refuse to proceed on a
gap.
Cost: `{fy}` is optional in a template. Without it numbers repeat across
years, which GST permits (uniqueness is per financial year) but which the DB
layer must allow for: a unique index on the invoice number alone would be
wrong, it must include the year. The conformance suite needs an adapter from
the DB write transaction to `invoicetest.Tx`.

## ADR-049 - Tax: rates in thousandths of a percent, one engine, line rounding reconciled per rate
2026-09 · Status: accepted
Decision: `internal/domain/tax` implements the `TaxProfile` interface from
`docs/02-domain.md` with three profiles (GST, VAT, US sales tax) on one
shared engine.
- **Rates are `tax.Rate`, thousandths of a percent** (100000 = 100%), not
  basis points. This departs from "percentages are basis points" in
  `02-domain.md` for tax rates only. `money.Amount.Scale(num, den)` was added
  for the arithmetic; `tax.RateFromBP` converts.
- The engine: each line's base is its amount (exclusive) or its amount with
  tax backed out and rounded half to even (inclusive). Each component on each
  line is base x rate, rounded half to even. Then, per group of lines sharing
  the same components, each component total must equal the summed base x
  rate rounded once; the difference is settled one minor unit at a time on
  the lines rounded closest to the other way, earliest first on a tie.
  Inclusive lines then let the taxable value absorb the remainder, so
  taxable + tax is the menu price exactly.
- GST: CGST + SGST (UTGST in the five UTs without a legislature) within the
  state, IGST across. An odd rate cannot split and is refused intra-state.
  HSN/SAC of 4, 6 or 8 digits required per line. GSTIN shape and state
  prefix checked. Rates are not checked against a slab list.
- VAT: one rate per line, either pricing. US sales tax: exclusive only, the
  jurisdiction's named parts each computed and reported, line rates ignored,
  exempt lines skipped.
- `TaxResult` carries JSON tags: it is the `tax_snapshot` an issued invoice
  stores.
Because: US rates need three decimals (New York City is 8.875%, including a
0.375% transit district), and 0.25% GST must halve into CGST and SGST
exactly; basis points can do neither. Reconciling per rate means an
invoice's lines always add up to its rate-wise summary and to the tax
register, which is what an audit compares, while no line moves more than a
paisa from its own rounding. CGST and SGST see identical inputs and so come
out equal on every line, as the law requires. A slab list would refuse a
legal sale the day the slabs change, as they did in 2025.
Cost: the GSTIN check digit is not verified; a typo that keeps the shape
passes. US tax is rounded per jurisdiction part, which can differ by a cent
from rounding the combined rate once; states differ on which they want, so
this may need a per-venue switch. Wiring tax into the order projection is
split out as Task 5b.

## ADR-050 - Tax in the projection: explicit Taxes, discounts spread before tax
2026-09 · Status: accepted. Supersedes the "tax is zero" line of ADR-046.
Decision: `order.Project(events, taxes)` takes the venue's `Taxes` (a
`tax.TaxProfile` and its `tax.VenueContext`) as an argument; `Taxes{}` is an
untaxed venue. After every event the projection:
1. spreads the order discounts over the active lines in proportion to each
   line's Net, by largest remainder, with the new `money.Allocate`;
2. passes each active line's Net less its share, with the tax facts
   snapshotted on its `line.added` (`tax_rate`, `hsn`, `tax_exempt`), to the
   profile;
3. sets `Totals.Taxable`, `Totals.Tax` and `Totals.Total = Taxable + Tax`,
   each line's `OrderDiscount` and `Tax`, and `State.TaxDetail`, the full
   `tax.TaxResult`, which is what `invoice.issued` stores as its snapshot.
Under inclusive pricing Total equals Subtotal; under exclusive pricing it is
Subtotal + Tax; untaxed, it is Subtotal. Voided and comped lines carry no
tax. A tax error (a GST line with no HSN) fails the event that caused it. A
venue taxed in a currency other than the order's is `ErrTaxConfig` at open.
Because: an argument keeps `Project` pure and makes forgetting tax a compile
error rather than a silent zero. Tax is charged on what the guest actually
pays for a line, so an order discount has to reach the lines first, and
largest remainder means the shares always add back to the discount. The tax
facts are snapshotted like the price, so a rate changed at 7pm does not
touch a line added at 6pm.
Cost: every projection runs the tax engine, O(lines) per event, fine at the
size of a bill. `money.Allocate` works in 128-bit arithmetic to rule out
overflow. Comps carry no tax here; if a regime wants tax on a free supply,
that is a profile rule to add later.

## ADR-051 - Split payment: four strategies, one checked invariant, Line.Due
2026-09 · Status: accepted
Decision: `internal/domain/payment` splits a bill with `Evenly`, `ByAmount`,
`ByItem` and `ByGuest`. Each returns a `Split` whose `Check` (shares in
payer order, nothing negative, one currency, amounts summing to the total
exactly) runs before it is returned. The rules:
- Evenly: `money.SplitEvenly`, the earliest payers carrying the odd paisa.
- ByAmount: each fixed amount must be positive; if they fall short, one more
  payer is added for the rest; if they exceed the bill, `ErrOverAllocated`.
- ByItem: every item needs one or more distinct payers, and the items must
  sum to the bill exactly, so a line left out or counted twice is
  `ErrMismatch`. A shared item splits evenly, the earlier-listed payers
  carrying the odd paisa.
- ByGuest: seat numbers are 1-based as staff say them; `SharedByTable` (0)
  splits a line across every guest. It is ByItem underneath.
- Tips are `Share.Tip`, set with `WithTips`, never part of `Amount`.
The package imports only `money`. To split by item, `order.Line` gained `Due`:
Net less the line's share of order discounts, plus tax under exclusive
pricing. The projection checks that the active lines' Due sums to
`Totals.Total` after every event, and an end-to-end test splits a taxed,
discounted bill by item and closes it at zero.
Because: the invariant is the whole point, so it lives in one function every
strategy must pass through, and the property test (2000 random bills, two to
eight payers, every strategy) checks it from outside. Line.Due belongs to the
projection because only it knows the discount spread and the pricing mode;
payment stays a small, pure arithmetic package.
Cost: odd paise on shared items always go to the earliest-listed payers, a
bias of at most a paisa per shared item per payer. Deterministic and small,
but not rotated; revisit if anyone notices.

## ADR-052 - Order persistence: read inside the write, snapshots as a checked cache, counters in SQL
2026-09 · Status: accepted
Decision: `internal/db` persists orders as `orders` (an index row),
`order_events` (the stream) and `order_snapshots` (one per order), and
invoice numbers as `invoice_series` (counters) and `invoices` (the register).
- `AppendOrder(orderID, taxes, decide)` reads the stream, calls the
  command's `Decide`, numbers its events, projects them with `order.Resume`
  and appends them, all in one write transaction. The `(order_id, seq)`
  unique index turns a second writer into `ErrSeqConflict`, retried from the
  read up to three times. `Decide` may run more than once, so it does nothing
  outside the transaction it is given.
- Snapshots are retaken whenever an append crosses a multiple of 50 events.
  Each records a basis: `order.ProjectionVersion` plus the tax profile and
  venue tax context. A snapshot is used only under the same basis. If the
  tail does not fold onto it, or a command is refused on top of it and a
  replay disagrees with it, the events win: the load replays, the append
  retries without it and rewrites it.
- `order.Resume(from, events, taxes)` carries a projection on from a State.
  `State.Invoiced` was added because it was the one flag of the fold that
  State did not already imply. `order.DecodePayload` decodes a stored body.
- The next invoice number is one upsert on the counter row inside the
  issuing transaction. Triggers let a counter move only forward by one,
  refuse an `invoices` row numbered past its counter, and make
  `order_events`, `invoice_series` and `invoices` append-only.
- Orders, events and invoices restrict deletes from `venues`: they are
  financial records.
Because: reading inside the write transaction means that under one writer a
conflict cannot happen at all, so the retry and the unique index are
backstops, not the mechanism. A snapshot keyed on its basis cannot serve a
state computed under yesterday's tax registration. The triggers hold the
series guarantee against a code path that forgets the allocator. The
equivalence tests (every split point in the domain; every append across
three snapshot points in the database) are the proof.
Cost: a change to the projection's rules or to State's shape must bump
`order.ProjectionVersion`, or old snapshots are served until they are
retaken. Nothing enforces that. Deleting a venue with any orders now fails;
closing a venue will be a status, not a delete.

## ADR-053 - Order service: commands as deciders, keys saved with their events, Close invoices
2026-09 · Status: accepted
Decision: `internal/service/order` has one method per command. Each builds a
decider that `db.AppendOrder` runs inside the write transaction; the service
stamps the events with its clock, the caller's actor and device, and
publishes a `Change` only after commit.
- Commands whose body is exactly their event's payload are defined as that
  payload type (`type AddLine order.LineAdded`). Where the server owns a
  field, the command leaves it out: `OpenOrder` takes no currency or opener,
  `TenderPayment` no tenderer, `Close` and `IssueInvoice` no body.
- An idempotency key is scoped to the venue and saved in the command's own
  transaction as `(fingerprint, order_id, first_seq, last_seq)`. The
  fingerprint hashes the command name, the order and the body; the same key
  on another request is `ErrIdempotencyKeyReused`. A replay reads the events
  back and projects to `last_seq`, which is the result the first request
  got. A command that fails saves no key.
- `Close` issues an invoice first, in the same transaction, when this
  settlement has none and the bill is not zero. `IssueInvoice` exists for
  issuing before close.
- The tax regime and invoice series come from the venue row plus settings
  (`tax.pricing`, `tax.state_code`, `tax.jurisdiction`, `invoice.series`,
  `invoice.prefix`, `invoice.template`), read on every command. Pricing
  defaults to inclusive, and exclusive under US sales tax. The series
  defaults to `INV`, `{prefix}/{fy:short}/{seq:06d}`, capped at 16
  characters under GST.
- `SendTicket` with no lines sends every unsent, unvoided line for the
  station whose course is 0 or fired.
- An order at another venue is `ErrOrderNotFound`, not forbidden.
- The package is `internal/service/order` because the phase says so, beside
  the Windows service package it does not otherwise relate to.
Because: the key and the events commit together or not at all, so a retry
after a crash can neither double-apply nor find a key with nothing behind
it. Storing a seq range rather than a serialised response keeps the key
row small and needs nothing the transaction does not already know. Closing
with the invoice in the same transaction means a sale can never be closed
unnumbered by a till that forgot a step.
Cost: a replay projects under today's tax settings, so if the owner changed
them since, the replayed totals follow; the events and invoice snapshot do
not. `AddLine` trusts the client's price and tax snapshot until the menu
tables exist. Keys never expire yet. GST place of supply is always the
venue's own state.

## ADR-054 - Device pairing: hashed codes and tokens, one transaction, 401 by kind
2026-09 · Status: accepted
Decision: `internal/auth.Devices` pairs and authenticates devices.
- A pairing code is 8 Crockford symbols (40 bits), shown as `XXXX-XXXX`,
  valid for 10 minutes, for one device of a name and kind fixed when it is
  made (terminal, kitchen or back office; never guest). Only its SHA-256 is
  stored. It may be typed as read: `id.Normalize` takes lowercase, the
  hyphen and O/I/L for 0/1.
- Redeeming it reads the code, creates the device with its token hash and
  marks the code used (`WHERE redeemed_at IS NULL`, one row or fail) in one
  write transaction. Unknown, expired, used and malformed codes all return
  the same `ErrInvalidPairingCode`.
- A device token is `nfd_` plus 256 random bits in unpadded base64url. Only
  its SHA-256 is stored, under a partial unique index. Tokens of the wrong
  shape are refused before the database is asked.
- Revoking sets `revoked_at` and keeps the hash, so a revoked token is
  answered `device_revoked` rather than `unauthenticated`: the device knows
  to show the pairing screen instead of retrying.
- `last_seen_at` is written only when it is a minute or more old, and a
  failure to write it does not fail the request.
- `internal/api`: `WithDevices` mounts `POST /api/v1/devices/pair` and the
  authenticated `GET /api/v1/devices/me`, behind `requireDevice`, which reads
  `Authorization: Bearer`. A 401 always carries `WWW-Authenticate`. Without
  `WithDevices` the routes do not exist, so a misconfigured server fails
  closed. `decodeJSON` bounds bodies at 64 KiB and refuses unknown fields
  and trailing values.
Because: SHA-256 is right for secrets with the entropy to make guessing
hopeless; PINs, which lack it, get Argon2id in 3b. Forty bits that live ten
minutes and work once cannot be guessed online at any rate a LAN allows.
Failing unknown, spent and expired codes the same way tells a guesser
nothing. The single transaction is what makes "exactly once" true under a
race, and a test of eight concurrent redeemers proves it.
Cost: there is no way yet to make the first code on a real install: no
venue is created at startup and no endpoint makes codes until 3c. The pair
endpoint is not rate limited. The access log does not carry venue_id or
device_id, because the device is known only inside the chain.

## ADR-055 - Dependency: golang.org/x/crypto, for Argon2id
2026-09 · Status: accepted
Decision: `golang.org/x/crypto` joins the allowed set of
`docs/03-conventions.md`, for `argon2` alone. It was already in the build as
an indirect dependency of goose, at v0.40.0; it is now direct, at the same
version. Pinning the older v0.28.0 that the phase file guessed at would have
dragged goose back three minor versions, so the version already selected is
the one taken.
Because: a four-digit PIN has about thirteen bits in it. Nothing but a
memory-hard hash makes a stolen `staff.pin_hash` cost anything to invert, and
`argon2.IDKey` is the reference implementation, maintained by the Go team,
already compiled into this binary.
Cost: one more direct dependency to watch. It is pure Go, so `CGO_ENABLED=0`
and cross-compilation are untouched.

## ADR-056 - Staff sign-in: PIN on a paired device, sliding-window lockout, every attempt written
2026-09 · Status: accepted
Decision: `internal/auth.StaffAuth` signs staff in.
- A PIN is 4 to 8 digits, stored as an Argon2id PHC string carrying its own
  parameters (19 MiB, t=2, p=1, OWASP's minimum), so they can be raised
  later without invalidating a single PIN. A stored hash is parsed
  defensively: the memory it asks for is capped, because it comes from a row.
- Sign-in takes the staff member's public ID and their PIN, on a device that
  `requireDevice` has already admitted. A wrong PIN, an unknown staff
  member, one from another venue and a deactivated one all return
  `ErrInvalidPIN`, and the last three are compared against a dummy hash so
  they take the same time.
- Every attempt is written to `sign_in_attempts` with its real outcome
  (`success`, `invalid_pin`, `unknown_staff`, `inactive_staff`,
  `locked_out`), which is both the audit trail and what the lockout reads.
- Lockout is a sliding window: 5 failures in 15 minutes for a staff member,
  20 for a device across everyone. A success resets the staff count; a
  refusal during a lockout is recorded but does not extend it, and the lock
  lifts when the oldest counted failure leaves the window. Refusals during a
  lockout are excluded from the count, or a client hammering the endpoint
  would push its own failures out of it. Attempts for one staff member are
  serialised by a keyed mutex, so parallel guesses cannot all pass the check
  before any is recorded.
- A session is `nfs_` plus 256 bits, hashed, bound to the device it was
  opened on and refused from any other, lasting 12 hours. It dies with a
  sign-out, an expiry, a revoked device, or the staff member being
  deactivated.
- `internal/api`: `X-Staff-Token` alongside the device's bearer token;
  `requireStaff` inside `requireDevice`; `GET /api/v1/staff`,
  `POST /api/v1/sessions`, `GET` and `DELETE /api/v1/sessions/current`.
  Typed errors `invalid_pin` (401), `locked_out` (429 with Retry-After),
  `sign_in_required` (401).
- From 3a's gaps: `POST /api/v1/devices/pair` is limited to 10 requests a
  minute per address, in memory, with no forwarded header trusted. The
  access log carries `venue_id`, `device_id` and `actor_id` once
  authentication knows them, through a holder the middleware fills in.
Because: PIN entry is the one place a POS is attacked from the floor, so the
answer to every wrong guess reads the same and every guess is on record.
Binding a session to its device means a token read off one screen is useless
at the next terminal. The device limit is what stops a tablet working
through the whole staff list four tries at a time.
Cost: a locked-out staff member is told so, and an unknown one is not, so a
paired device can tell which IDs exist. It already can: it lists them to
draw the sign-in tiles. Lockout state is derived from the attempts table on
every sign-in rather than cached. The rate limiter is per process and resets
with it. Sessions are not swept: expired rows stay until something deletes
them.

## ADR-057 - Permissions: a closed list, read leniently, and an override that checks the authoriser
2026-09 · Status: accepted
Decision: `internal/auth` owns the permission vocabulary and the override.
- `Permission` is a closed list of fifteen keys, ten for the register and
  five for the back office. A role is written through `NewPermissionSet`,
  which refuses a key this build does not know, so a typo fails in the back
  office rather than granting nothing quietly.
- A stored role is read leniently: an unknown key is dropped and logged
  rather than failing the read. A database written by a newer build must
  still let today's binary serve what it understands, and a permission this
  build never asks for cannot be granted by accident. A role that is missing
  entirely, or whose column cannot be parsed, grants nothing.
- `DefaultRoles` are owner, manager, cashier and kitchen. The owner holds
  everything; a manager cannot change prices or tax settings; a cashier
  cannot void, comp or discount without one. They are a starting point the
  owner edits, not a hierarchy the code enforces.
- `StaffAuth.Authorise(dev, staffRef, pin, permission)` is the manager
  override. It runs the same gauntlet as a sign-in, sharing its lockouts and
  its attempt log, but opens no session. The PIN is checked first and the
  permission second, so a correct PIN refused on permission is recorded as a
  success and does not count toward a lockout: a manager who lacks a
  permission has mistyped nothing.
- `internal/api`: `requirePermission(p)` inside `requireStaff`, answering a
  typed `forbidden` (403) naming the permission. `Override` is the struct a
  request body embeds, and `s.authorise` checks it and answers the client,
  mapping to `forbidden`, `invalid_pin` or `locked_out`. `Meta(r)` builds
  `service/order.Meta` from what authentication proved, plus the
  `Idempotency-Key` header, so a handler cannot pass a venue or an actor the
  request did not establish.
- `POST /api/v1/devices/codes` is 3a's code creation, behind
  `device.manage`.
Because: the override exists so that the person at the till cannot void
their own mistakes, which means the permission checked must be the
authoriser's and the audit trail must name them. Sharing the sign-in path
means an override cannot be used to brute-force a manager's PIN past the
lockout. A closed list keeps "what can a role do" answerable by reading one
file.
Cost: the permission set is checked per request, one indexed read, not
cached. A role changed mid-shift takes effect on the next request, which is
the behaviour wanted, but it also means a busy terminal reads the role row
constantly. The override PIN travels in the request body over the LAN;
phase 12's TLS story is what covers that.

## ADR-058 - First-run setup is a local subcommand, not an endpoint
2026-09 · Status: accepted
Decision: `nofiltr setup` creates the venue, `auth.DefaultRoles`, the owner
with a PIN, and the first pairing code, then prints them. It is the only
subcommand, read from `os.Args` before the server's flags, since that parser
refuses a positional argument.
- It shares nothing with the API. Being sat at the machine is the trust:
  local access to the data directory is already total, so an endpoint would
  add a way in without adding any protection.
- Everything but the pairing code is one transaction, with the check that no
  venue exists inside it, so two runs racing cannot both create a venue. A
  second run refuses with ErrAlreadySetUp.
- The owner's PIN is drawn (six digits from crypto/rand) and printed unless
  `-owner-pin` was given, in which case nothing is printed: the operator
  already knows it. It is the only time a PIN is shown.
- Only flags the server also has (`-data-dir`) are passed to
  `config.LoadArgs`, so the database resolves exactly where serving would
  find it, through config.toml and the environment.
- A GST venue's state code is taken from the first two digits of its GSTIN
  rather than asked for twice. US jurisdiction rates are a list, so they
  stay a back office job.
Because: the API cannot bootstrap itself. Making a pairing code needs a
permission, a permission needs a signed-in staff member, and signing in
needs a paired device, so something outside that circle has to start it.
The command's whole output is carried to a till by a person, which is what
its test asserts: the printed code pairs a device, and the printed PIN signs
the printed owner in, with every permission.
Cost: a venue's tax jurisdiction and cash rounding still need the back
office, which does not exist until phase 10, so a US venue cannot complete
setup from here. A drawn PIN is printed to a terminal, where it may reach a
scrollback buffer; it is meant to be changed on the first day, and nothing
enforces that yet.

## ADR-059 - HTTP handlers: a code for every refusal, and the override decides authority
2026-09 · Status: accepted
Decision: `internal/api` serves `/api/v1/orders` over
`internal/service/order`. Each route is the permission it needs wrapped
around a handler that decodes, makes one service call and encodes.
- Orders are addressed by public ID. `Service.Resolve` turns it into the
  internal one, and an order at another venue is not found rather than
  forbidden.
- Request bodies that are exactly a service command are that command
  (`addLineRequest = svcorder.AddLine`), so there is one definition of a
  line to keep right, and task 6 generates one TypeScript type for it. The
  rest add the manager's `Override` or take an ID from the path.
- A mutating endpoint answers with the whole order plus the events it
  appended, so a terminal redraws from the response and never has to
  re-read. A replayed idempotent command answers 200 rather than 201: it
  created nothing this time.
- **The override decides authority, not the route.** Void, comp, discount,
  payment void, refund and reopen ask only that the caller holds
  `order.open`, and check the authoriser's permission against their PIN
  inside the handler. Requiring the permission of the caller as well would
  mean a cashier could never void with a manager stood beside them, which
  is the opposite of what an override is for. The caller's own permission
  is still checked on every route, so a kitchen screen cannot work the
  register.
- `order_errors.go` maps every refusal to a status and a code, most-specific
  first. Anything not in the table is a bug, not a refusal, and becomes a
  500 with the detail in the log. The message returned is the domain's own,
  because it says what is wrong with the bill in words a till can show.
Because: the error table is the contract a terminal is written against, and
the rule that an unmapped error is a 500 is what keeps it honest: a new
domain error shows up as a 500 in testing rather than as a silent 409 with
the wrong code.
Cost: the response carries the whole order on every command, which is a few
kilobytes on a bill with fifty lines; the alternative is a client that has
to merge events itself, and it can still do that from the same response.
A currency mismatch inside an event arrives as `invalid_payload` with the
currencies named, because the projection formats it rather than wrapping
the sentinel; `currency_mismatch` is reserved for arithmetic outside an
event. Tables, menu read and shifts were in this task and moved to their own
phases: no schema, no service, nothing to serve.

## ADR-060 - The hub: a publish is a nudge, and every connection reads SQLite itself
2026-09 · Status: accepted
Decision: `internal/hub` streams a venue's events to its devices over
WebSocket, using `coder/websocket` (the allowed set; it was already in the
module graph, now direct).
- **A publish carries no events.** `Hub.Notify(venueID)` says only that the
  venue has moved. Each connection then reads `db.VenueEventsAfter` from its
  own position and sends what it finds. Replay and live streaming are
  therefore the same code path, and a device that was off for an hour
  catches up exactly as one that missed a second does.
- That settles backpressure without a policy: a slow device falls behind in
  the database, not in memory, and all the hub holds for it is a one-slot
  channel saying there is more to read. A write that blocks past the write
  timeout drops the connection.
- A position is `order_events.id`, which only grows, so one integer places a
  client in the venue's whole stream. `last_event_id: 0` means "only what
  happens next", which is what a screen opening fresh wants; `from_start`
  asks for the lot; a client ahead of the stream (a restored backup) is
  replayed from the beginning rather than left silent.
- Topics are `venue`, `order:<public id>`, `table:<n>` and `station:<name>`,
  derived from the event and its order. Subscribing to nothing means the
  venue topic: a client that asks for nothing gets everything rather than
  silence.
- The socket is authenticated as a paired device like every other route.
  A browser cannot set headers on a WebSocket, so the token rides in the
  subprotocol (`token.<device token>`) rather than a query parameter, which
  would put it in the access log. A native client may still use the header.
- The origin check is `coder/websocket`'s default, which refuses a browser
  page from another origin and allows a client that sends no Origin at all,
  plus the configured dev origin. Turning it off would have been the easy
  path and is the one thing a WebSocket must not do.
Because: the events are on disk before anyone is told, so the database is
the only thing worth streaming from. Sending the payload with the nudge
would mean two paths to keep identical, and a client could be sent an event
that a rolled-back transaction never committed.
Cost: each connection does one indexed read per nudge, so ten devices mean
ten reads per command. On a local file at a cafe's rate that is nothing, and
it is the price of the guarantee. A nudge that arrives while a connection is
mid-drain is coalesced, so the drain runs again; it never sends twice
because the cursor only moves forward. The hub holds no per-venue goroutine,
so a venue with no devices costs nothing at all.

## ADR-061 - TypeScript types: generated by reflection, guarded by a test, no new tool
2026-09 · Status: accepted
Decision: `web/shared/api-types.ts` is generated from the Go request and
response structs by `internal/codegen/ts`, driven by
`internal/api/typescript_test.go`, and committed.
- No generator tool is added. The emitter is about three hundred lines of
  reflection over the shapes this API actually uses: structs, embedded
  structs (flattened, as encoding/json flattens them), slices, maps,
  pointers, named string types, and the few whose JSON is not their struct.
  A third-party generator would be a dependency, an install step and a
  configuration file to buy a superset of that.
- **The drift guard is the test itself.** It regenerates in memory and fails
  if the committed file differs, so `make check` already catches a struct
  changed without regenerating, in CI and locally, with no extra job.
  `make generate` passes `-update` to rewrite the file.
- A vocabulary is generated as a union only when its Go type is named, and
  `Union` refuses a sample of an unnamed type: registering one against plain
  `string` would turn every string in the API into that union, which is how
  the first draft of this was wrong. That pushed four types into existence
  that should have existed anyway: `api.ErrorCode`, `auth.DeviceKind`,
  `hub.MessageType`, and the response structs now carrying `order.Status`,
  `order.PaymentMethod` and the rest rather than strings.
- `order.Event` is written by hand as a discriminated union, because its
  JSON is its custom marshaller's, not its struct's. The payload interfaces
  are generated, and a test compares the hand-written list with
  `order.Types()`, so an event type added without a line there fails.
Because: the ban on hand-written mirrors in `docs/03-conventions.md` is only
real if something enforces it, and the cheapest enforcement is the file
being generated and checked by the same test run that already gates every
commit.
Cost: the emitter handles what this API uses and will refuse what it does
not, which is the intended failure. A nil slice marshals as `null` while the
generated type says `T[]`; every response here builds its slices with make,
and a field that may be absent is `omitempty` and therefore optional. The
frontends do not consume the file until phase 05, so the first real proof
that these types compile against a Vite build is still to come.

## ADR-062 - Tokens: a palette on :root, roles on [data-surface], Tailwind sees only roles
2026-09 · Status: accepted
Decision: `web/shared/tokens.css` has two layers. Every named token in
`docs/design/tokens.md` is a custom property on `:root`, by its own name.
`[data-surface="ink"]` and `[data-surface="paper"]` then set role properties
(`--canvas`, `--raised`, `--fg-2`, `--line`, `--accent`, `--accent-text`,
`--status-success` and so on) to the palette value for that ground.
`web/shared/tailwind.preset.ts` exposes only the roles, and *replaces*
Tailwind's colours, font sizes, radii and shadows instead of extending them.
- A component is written once and is correct on both grounds, which the
  phase's done-when (every primitive on ink and paper) needs. "Pick by
  surface, never by taste" is enforced by the cascade, not by review.
- Replacing the defaults means `bg-red-500`, `text-xs` and `rounded-lg` do not
  exist. The one-accent rule and the 11px floor cannot be bypassed from a
  class name. Spacing stays native, as tokens.md already argues.
- Paper has fewer tokens than ink. Where tokens.md names no paper value, a
  role borrows the nearest one, marked `stand-in` in the CSS: paper
  `chrome` and `raised-high` use `--paper-raised`; paper `line-strong` uses
  `--paper-hairline`; paper `line-focus` uses `--paper-text-faint`; paper
  `accent-hover` does not change. On ink, `sunken` is `--ink` and
  `line-soft` is `--ink-hairline`. No status-chip triad exists for ink, and
  no paper shadow exists at all, so none is exposed.
- Fonts are subset from google/fonts sources with fonttools, not taken from
  a CDN or npm. `fonts/README.md` has the exact commands. JetBrains Mono and
  Instrument Serif have no rupee glyph, so Archivo's rupee is registered
  under both families with `unicode-range: U+20B9`. Otherwise every price
  would start with a character in a fallback face. About 88 KB across five
  files, and a browser only fetches a face a page uses.
Because: one set of classes that means the right thing on either ground is
cheaper than two sets and a rule about which to use. Replacing the defaults
turns two review rules into build facts.
Cost: the stand-ins are guesses until the back office is designed in
detail, and each one must go into tokens.md before it changes. `--vermilion-on`
on the paper fill measures 3.99:1, under the 4.5:1 AA line for body text;
on ink it is 5.07:1. `--paper-raised` on the paper fill is 4.72:1. The
charge button uses the doc's `--vermilion-on` today, and someone should
decide before phase 07 builds a paper charge surface.

## ADR-063 - Frontend build: one workspace, `make build` picks its embed, the guest budget counts JS, CSS and HTML
2026-09 · Status: accepted
Decision: `web/` is one npm workspace (terminal, backoffice, guest) with one
lockfile. The toolchain is pinned exactly: Vite 8.3, TypeScript 7.0,
Tailwind 4.3 through `@tailwindcss/vite`, React 18.3.1. All three apps share
`web/shared/vite-app.ts`, so they differ only in base path and dev port.
- **Tailwind v4 loads the preset with `@config`.** Probed, not assumed:
  `bg-red-500`, `text-xs`, `rounded-lg` and `shadow-md` generate nothing,
  the role classes generate `var(--...)`, and native spacing steps such as
  `gap-2.5` still work. The preset carries `satisfies Config`, and each app's
  tsconfig includes `web/shared`, so `api-types.ts` is now type-checked under
  `strict`, `noUncheckedIndexedAccess` and `exactOptionalPropertyTypes`. It
  passes.
- **`make build` embeds the real bundles when all three `dist/index.html`
  exist, and the placeholders otherwise, and says which.** That makes the
  phase's `make build-web && make build` true without a flag, and keeps a
  Node-free clone building. `make dist` depends on `build-web` and always
  passes `-tags frontend`, so a release rebuilds and cannot ship a stale or
  missing bundle.
- **The guest budget counts every `.js`, `.css` and `.html` file in
  `guest/dist`, gzipped at level 9.** That is what must arrive before the
  first screen renders. Fonts are listed but not counted: they load in
  parallel, only when used, and `font-display: swap` renders text without
  them. `web/scripts/check-size.mjs` needs no dependency, and a probe at a
  40 KB budget confirmed it fails with exit 1.
- CI gains a `web` job running `make build-web`, beside `make check`. A
  budget that only fails on the machines that happen to run it is not a
  budget, and keeping it out of `make check` means Go work still needs no
  Node.
Because: the done-when asks for one binary serving three apps, and the
guest budget is a hard rule in CLAUDE.md. Both should be enforced by
commands, not by memory.
Cost: **the guest app is 48.4 KB of its 60 KB with an empty screen.** React
18 and react-dom are 44 KB of that. Ordering, a menu and a cart must fit in
the remaining 11.6 KB, alongside React Query and the realtime hook (Task 3).
That is tight and may not fit. The obvious answer is Preact through
`preact/compat` for the guest bundle only, about 4 KB, with the same
components. The stack is settled as React, so this ADR does not switch. It
records the case, and the choice belongs to the human before phase 11.

## ADR-064 - Guest renders React source through preact/compat
2026-09 · Status: accepted (the human asked for a solution to ADR-063's budget problem)
Decision: the guest bundle aliases `react`, `react-dom`, `react-dom/client`
and the JSX runtimes to `preact/compat` in `shared/vite-app.ts`, for guest
only. Source stays React and type-checks against React's types; only the
bundler knows. Terminal and back office are unchanged.
- Measured: the empty guest app went from 48.4 KB to 11.7 KB gzipped. With
  React Query and the API client it is 19.4 KB, leaving 40.6 KB for ordering.
- Rejected: **no framework for guest**, because every shared primitive
  (`Money`, `StatusChip`) would then be written twice, and the phase 05
  gallery would prove nothing about the guest's copies. **Staying on React
  and dropping React Query from guest** saves about 8 KB, breaks the "React
  Query for all server state" rule, and still leaves ordering 20 KB.
  **Preact everywhere** buys nothing: terminal and back office have no budget.
- Guest gets Vite's own JSX transform, not `@vitejs/plugin-react`, whose
  React Refresh cannot run on Preact. Editing guest in development is a full
  reload, not a hot swap.
- `web/scripts/check-size.mjs` fails the build if React's runtime appears in
  the guest bundle, by a string only React contains. A stray import that
  slipped past the alias would otherwise put 40 KB back silently.
Because: the budget is a hard rule and React alone nearly spent it. This
keeps one component codebase and one mental model, at the cost of one alias
block.
Cost: a library that reaches into React internals will not work in guest.
The check above finds it at build time, not on a guest's phone. Preact is a
second runtime to keep current. CLAUDE.md, `docs/01-architecture.md` and
`web/README.md` now say this, so a later session does not "fix" it back.

## ADR-065 - Realtime client: a heartbeat on `subscribe`, a probe to explain failures, resync as a contract
2026-09 · Status: accepted
Decision: `web/shared/realtime.ts` is a framework-free `RealtimeConnection`;
`use-realtime.tsx` binds it to React with `useSyncExternalStore`. No server
change was needed for any of the following.
- **Heartbeat.** The hub pings at the WebSocket protocol level, which a
  browser answers but page code cannot see, so a link that died silently
  looks open until TCP gives up, minutes later. Every 15 s the client sends
  `subscribe` with its unchanged topics; the hub answers with a `welcome`
  and changes nothing. Any message resets the clock. Two intervals of
  silence closes the socket and reconnects.
- **Why it is down.** A refused handshake (401) and a server that is not
  there look the same to a browser. After a failure the client makes one
  request to `GET /api/v1/devices/me`: a 401 means unpaired, and retrying
  stops; a network error means the server is unreachable; a 200 means the
  stream alone dropped. `navigator.onLine` false skips the probe and says
  "No network".
- **Position.** The client keeps the highest id seen from `event` *and*
  `caught_up`, because the hub advances its cursor past events filtered by
  topic. A reconnect sends it as `last_event_id` and the hub replays the gap.
- **Resync is a contract with screens.** The first welcome, and any welcome
  whose `from_event_id` is not the position asked for (a restored backup),
  emits `resync`. Screens refetch on resync and apply events as deltas after
  it. That closes the race between a REST snapshot and the stream.
- **Backoff** is equal jitter, 0.5 s doubling to a 10 s cap: at least half the
  ceiling, so the countdown never shows zero, and spread, so a room of
  tablets does not return on one tick after a server restart.
- React Query runs with `networkMode: "always"`. Its default pauses whenever
  `navigator.onLine` is false, which reflects the internet, while the server
  is on the LAN or the same machine. Mutations are never retried
  automatically; a charge is retried on purpose, with its idempotency key.
Because: staff must be able to trust the indicator at a glance, and every
one of its states was checked against the real server, not only in tests:
Live, then "Server unreachable, retrying in 4s" with the binary killed,
then Live again on its own.
Cost: the heartbeat is 1 small message each way per device every 15 s. It
leans on `subscribe` answering with `welcome`; if the hub ever stops
answering it, every client will reconnect every 30 s, visibly. A `ping`
message type in the hub would be the tidier contract. Until Task 6, a
device token only reaches the client in development, through
`VITE_DEVICE_TOKEN` in a gitignored `.env.local`; production builds drop that
branch, and the built bundles were checked for it.

## ADR-066 - Primitives: money through BigInt, chips from typed vocabularies, rem left alone
2026-09 · Status: accepted
Decision: the phase 05 part-one primitives live in `web/shared/ui/`, shared by
all three apps, with their logic in plain modules tested by Vitest.
- **Money is formatted without a float.** `formatMoney` gives Intl the major
  part as a BigInt and replaces the fraction with digits cut from the minor
  integer's decimal string. That is exact to `Number.MAX_SAFE_INTEGER`
  paise, and BigInt works on the older phones guest must serve. The
  exponent comes from `currencyExponents`, now generated from
  `money.Exponents()`, so ¥1234 cannot be shown as 12.34 by a table that
  drifted. Grouping follows the currency (lakhs for INR) until the venue's
  locale exists. Negative is U+2212, the width of a digit. An amount that is
  not a safe integer renders "Invalid amount" in danger rather than a guess.
- **StatusChip takes `{ kind, status }`**, where each kind is a `Record` over
  a generated union (`OrderStatus`, `PaymentStatus`, `LineStatus`) or the
  kitchen list from tokens.md. A status added in Go fails the TypeScript
  build until it has a tone and a word. Tests hold vermilion to "Late" alone
  and strike through exactly the voided states.
- **Chip tokens for ink, and two for paper, are now in tokens.md.** Ink
  tints the status colour at 12% and 30% (accent at its measured 14% and
  40%). The design's paper "warning" chip is vermilion-toned, so it became
  the accent chip. Gold and slate chips on paper are derived with the
  measured recipe (8% and 25%), with darker text values
  (`--gold-paper-text`, `--slate-paper-text`) because the status colours
  are 2.4:1 and 3.2:1 on them. The derived text is 4.6:1, like vermilion's
  existing text value.
- **The surface rule no longer sets font-size on the root.** The apps put
  `data-surface` on `<html>`, so the 15px body size had redefined `rem` and
  shrunk every Tailwind spacing class by 6%: `h-8` was 30px. The tap
  targets were right only because they are px tokens. Measured in the
  gallery after the fix: 32, 36, 44 and 58px, card padding 16px.
- The gallery is a `pathname` check behind `import.meta.env.DEV` with a lazy
  import. There is no router yet; Task 6's guards will choose one.
Because: money and status are where a wrong glyph costs something, so both
are driven by data the server generates and pinned by tests. The rem bug
would have put every later screen 6% off its spec, silently.
Cost: the chip words for server states ("Paid" for captured, "Held",
"Comped") are this task's choice and are in one file to change. The paper
primary button still uses `--vermilion-on`, 3.99:1 (ADR-062).

## ADR-067 - Primitives, part two: native dialog, rules as types, logic beside each component
2026-09 · Status: accepted
Decision: `Sheet`, `Modal`, `NumPad`, `SearchField`, `Toast`, `EmptyState`,
`DataTable` and `Timeline` are in `web/shared/ui/`. Each has its logic in a
plain module with Vitest tests, and each was driven in a real browser on
both grounds before commit.
- **Modal and Sheet are the native `<dialog>`.** `showModal()` gives the top
  layer, an inert page behind, focus in and back out, and Escape as a
  `cancel` event, for no bytes. `use-dialog.ts` keeps it controlled by
  `open`. The browser fires `close` a task later even for a close the hook
  made, so the hook checks the owner's last `open` rather than a flag.
  `::backdrop` inherits no custom properties in pre-2024 browsers, so its
  rule carries the scrim's value as the `var()` fallback.
- **Sheet picks its shape by `pointer`, not width.** The terminal is a wide
  touch tablet; a width breakpoint would give it a mouse's side panel. It
  was checked both ways: touch emulated, a 1280 by 680 sheet on the bottom
  edge; with a mouse, a 384px panel down the right. The first touch run
  found it on the top edge, because `h-auto` overrode the dialog's
  `fit-content` height; that class is gone and the reason is in a comment.
- **Two rules became types.** `ToastTone` has no danger, so a toast cannot
  carry an error that needs a decision. `Timeline`'s switch is exhaustive over
  the generated `OrderEventType`, so an event added in Go breaks the build
  until it has words, as StatusChip does for statuses (ADR-066).
- **NumPad types amounts as minor units shifting in from the right**, as a
  till does: 1, 2, 5, 0 is 12.50. There is no decimal point to mistype and
  nothing to parse. A key that would pass the maximum is ignored, not
  clamped. Keys are 56px, over the 44px floor, for one thumb.
- **DataTable leaves sorting and fetching to its owner**, because with
  cursors the server sorts. Paging back keeps a stack of visited cursors. It
  is not exported from `ui/index.ts`, so guest cannot reach it by a barrel
  import. Every header is 15px, since sortable ones are controls and a mixed
  row reads as a mistake.
- `--scrim` is the design's measured `rgba(8,7,5,.72)`, now in tokens.md. Its
  `backdrop-filter: blur(3px)` is left out as too costly for a cheap tablet
  to paint mid-service.
Because: each of these is where a real screen would otherwise make its own
choice, and the ones that can hurt someone (a lost form, a mistyped amount,
an unreadable void) are held by the platform or by a type, not by care.
Cost: `<dialog>` needs Safari 15.4 or later. A toast shown while a modal is
open sits behind its scrim. `Timeline` shows a line's unit price, not the
line total, because the frontend renders money and never computes it.
