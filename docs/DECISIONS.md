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

## ADR-068 - Pairing and the staff lock: one auth store, guards as a function, wouter
2026-09 · Status: accepted
Decision: `web/shared/auth/` holds an `AuthStore` whose state is `checking`,
`unpaired`, `locked` or `signed_in`, and a pure `routeFor(state, path)` that
every guard in both apps applies through `Gate`. `PairScreen` and
`LockScreen` in `web/shared/screens/` serve the terminal on ink and the back
office sign-in on paper. Spec: `docs/design/screens/terminal-lock.md`.
- **Router: wouter 3.11**, terminal and back office only. About 2 KB, hooks,
  a `base` for `/backoffice`, and it runs on Preact for guest's routing in
  phase 11. react-router is ten times the size for three routes. The phase
  asked for a choice; this is it.
- **Where credentials live.** The device token is in `localStorage`, so
  pairing is once. The staff session is in `sessionStorage`, so a crashed
  till asks for a PIN again. Keys are `nofiltr.<app>.*`, because the terminal
  and the back office share an origin. Stored values are validated on read,
  and storage that is refused degrades to memory.
- **The server's word moves the state.** The API client reports
  `device_revoked`, `unauthenticated` and `sign_in_required` from any
  request, and the realtime stream reports a refused socket, both to the
  store: a revoked till goes to Pair with the reason, and an expired session
  goes to Lock with the reason. A 429 carries `Retry-After` into
  `ApiError.retryAfterMs`, which the lockout countdown uses.
- **Booting trusts storage when the server cannot answer.** A till that
  starts before its server must not have to pair again; the indicator says
  the server is missing. `boot()` runs once per store. React's development
  double effect had run it twice, and the second run erased why the device
  was unpaired; a browser run found it and a test holds it.
- **A code for another app is refused client-side.** A terminal code redeemed
  in the back office creates a terminal device on the server, and the app
  will not store it. That device is orphaned until the back office can
  revoke devices (phase 10). The message says which kind of code to ask for.
- Checked in a browser against a real server: bad code, pair, auto-chosen
  name, wrong PIN, right PIN, reload while signed in, Lock, the sixth
  failure locking the pad with a live countdown that survives a reload
  because the server enforces it, the back office end to end under
  `/backoffice`, an unrecognised token clearing with its notice, and the
  production binary from a clean browser to a signed-in terminal.
Because: sign-in is where a POS meets a stranger and a hurry at once, so its
states are one machine, its guards one function, and each refusal a
sentence for the person at the till.
Cost: `devCredentials` and `VITE_DEVICE_TOKEN` are gone; a dev build now
pairs like a real one. Keys typed before the staff list has loaded are
dropped. After a reload the pad does not know about a lockout until the
first attempt is refused. Staff tiles show no role, and the heading shows
the device's name, until the API returns both.

## ADR-069 - The menu: one versioned document, tax classes, invalidated by the stream
2026-09 · Status: accepted
Decision: migration 00006 adds `stations`, `courses`, `tax_classes`,
`categories`, `items`, `modifier_groups`, `modifiers`,
`item_modifier_groups` and `menu_versions`. `GET /api/v1/menu` returns the
whole active menu as one document with a version; the hub announces a new
version as `menu_changed`, and `useMenu` (web/shared) caches it until then.
- **Tax classes, a table the phase did not list.** An item points at a
  class ("Restaurant service 5%") holding a `tax.Rate` and an exempt flag,
  and carries its own HSN. A GST rate change is one row, not every item.
- **Items carry what a line snapshots, resolved.** The response gives each
  item its station's code, its course's number and its class's rate, exactly
  the fields of `order.LineAdded`, so ringing an item up needs no second
  lookup. Nothing an order records joins back to these tables.
- **Nothing is deleted.** Rows retire with `active = 0`. A retired category
  or station takes its items off the till; a retired group leaves the items.
  Every table carries `venue_id`, with composite foreign keys, so a row can
  never hang off another venue's parent.
- **One document, one version.** A cafe menu is small, so the terminal
  fetches all of it, inside one read transaction so the version matches the
  rows. The ETag is the version: a revalidation is a 304 from one indexed
  read. `menu_versions` is its own table rather than a column on `venues`,
  because a new column changed the generated `gen.Venue` and every query
  returning it.
- **Invalidation rides the stream's recovery.** `welcome` carries
  `menu_version`, and after every drain (a nudge or a ping) a connection
  compares the venue's version with the one it last sent and sends
  `menu_changed` to every connection, whatever its topics. A missed nudge is
  caught on the next ping, and a device that was offline catches up on its
  welcome, the same way events do. Nothing writes the menu over HTTP until
  phase 10, so today only `CreateMenu` and `WriteMenu` bump the version.
- **The menu types live in `internal/domain/menu`**, plain data, so
  `internal/api` still imports nothing from `internal/db`.
- **The demo menu is Go, behind `nofiltr setup -demo-menu`**, written in
  setup's own transaction. It is not a migration, so a real venue never
  gets fake items. It is priced in paise and refused for any other currency.
Because: the till must ring up from what it has cached when the network
blinks, and must never keep selling from a menu the owner has changed.
Cost: no menu writes over the API yet (phase 10). `AddLine` still trusts the
client's price and tax snapshot; it can now be checked against the menu,
and is not. A modifier group's min/max is enforced by the terminal only.

## ADR-070 - The register: one tap on defaults, an ordered outbox, keys that work over plain HTTP
2026-09 · Status: accepted
Decision: the register (`web/terminal/src/register/`, spec
`docs/design/screens/terminal-register.md`) rings an item up in one tap
with its default modifiers, sends every tap through a `TicketQueue` that
runs one command at a time, and draws the ticket only from the server's
projection.
- **One tap when the defaults suffice.** An item whose every group's
  defaults meet its min and max rings up at once; one that needs a choice
  goes to the ModifierSheet (Task 3). Filter coffee, the first item of the
  first category, is one tap from the screen opening. The design agrees:
  its tile goes straight to the ticket, and Modify is on the line.
- **An ordered outbox.** A tap becomes a pending line ("Adding") before any
  request, and the queue sends taps in order, opening the order first, so
  eight fast taps make one order and eight lines. A refused tap stays on
  the ticket, in danger, with Retry and Remove, never lost silently.
- **Idempotency keys per tap, kept for Retry,** and one for the open,
  kept until an order exists. A retry after a lost answer adds nothing
  twice; a failed open retried makes one order. Keys come from
  `crypto.getRandomValues`, not `crypto.randomUUID`, which a till on
  `http://192.168.x.x` does not have: it is not a secure context.
- **The current ticket's order id is in `sessionStorage`**
  (`nofiltr.terminal.ticket`), validated on read, so a reload returns to
  it. An id the server answers 404 for starts a new ticket. Task 5's
  ticket list replaces the single slot.
- **`SignedInShell` gained `fill`**: the screen owns the height below the
  header and scrolls inside itself, so the total can never scroll away.
- Checked against the real binary with the demo menu and GST: one tap
  rings Filter coffee up with its defaults; search by word prefix; eight
  burst taps land in order on one order; the ticket survives a reload; a
  line added over HTTP from outside appears through the stream; with the
  server down a tap becomes "Not added" with Retry, and Retry after the
  server returns adds exactly one line.
Because: the till's worst failures are a lost tap and a doubled one, and
both happen on the wifi a cafe actually has.
Cost: only the counter channel, one ticket at a time, and no cover count
until Task 5. The "Adding" state is covered by the queue's tests; in a
browser a refused connection fails too fast to see it.

## ADR-071 - `line.modified`: changing a line's modifiers before the kitchen has it
2026-09 · Status: accepted (chosen by the owner over void and re-add)
Decision: a new event, `line.modified { line_id, from, to }`, replaces an
active line's modifiers while the line is not sent and its course is not
fired. `POST /api/v1/orders/{id}/lines/{line}/modifiers` (`order.edit`)
appends it; the terminal's ModifierSheet sends it from a line's Modify.
- **Why an event, not void and re-add.** A void needs a manager's PIN and
  a reason, and stays on the bill forever. Swapping dairy for oat on a
  coffee nobody has started is not a void; asking a manager for it would
  teach staff to skip the change. After the kitchen has the line, the item
  may be made, and a change is a void and a new line again.
- **`from` is required and checked,** like `line.qty_changed`'s `From`: a
  till looking at a stale ticket is refused with `modifiers_mismatch`. The
  terminal goes further and watches the stream: when the open line changes
  underneath it, is sent or is voided, Save is disabled with the reason.
- **Order is part of the snapshot.** `from` and `to` compare as ordered
  lists: the order is how the ticket and invoice print them.
- The item's unit price, name, station, course and tax facts cannot change
  this way; only the modifiers. The line's discounts follow the new price,
  and an amount discount the new price cannot carry is refused.
- New sentinels `ErrModifiersMismatch` and `ErrLineInKitchen`, codes
  `modifiers_mismatch` and `line_in_kitchen` (409). `docs/02-domain.md`
  lists the event; the Timeline says "Changed Flat white: Dairy → Oat".
  `order.State` does not change shape, so `ProjectionVersion` stays.
- **Negative modifier prices are refused on the menu too.** The domain has
  always refused them (`checkPrice`); ADR-069's demo menu offered "No milk,
  less 20", which no till could have rung up. The menu writer now refuses
  one as `ErrMenuSpec`, the menu read leaves out any already stored, and
  the demo's "No milk" is free.
- **Refusals are worded from the code.** The API's `message` is for
  developers ("db: in transaction: projecting ..."); the register's words
  come from `order-messages.ts`, keyed by `ErrorCode`, with a plain
  fallback, so no server internals reach the till.
- The sheet's rules (`modifier-selection.ts`): defaults chosen for a new
  line, the line's own choices when editing; a pick-one group swaps and
  clears on a second tap only when optional; a full pick-several group
  refuses another pick rather than dropping one. Confirm is disabled until
  every group's minimum is met, and the message line names the first that
  is not.
Because: the cheapest honest record of "the guest changed their mind before
anything was made" is one event that says exactly that.
Cost: a sixteenth event type every reader of the stream must know. An edit
of a line with choices the menu has since retired drops them from the
sheet; saving then removes them from the line.

## ADR-072 - The cart: one sheet per line, approvals by PIN, change stops at the kitchen
2026-09 · Status: accepted
Decision: every cart action goes through the register's ordered queue
(`TicketQueue.run`, one idempotency key per command) and emits the event
`docs/02-domain.md` names: `line.qty_changed`, `note.added`,
`line.voided`, `line.comped`, `discount.applied`. Nothing is computed on
the till; the rail redraws from the order each command returns.
- **A line opens a sheet.** Tapping a line opens its LineSheet: quantity
  and a note saved together, and Modify, Discount, Comp, Void. The design's
  row of 34px buttons on every line fails the 44px and 15px rules, and
  puts Void next to `+`.
- **Quantity changes stop at the kitchen, as modifiers do (ADR-071).** The
  stepper is only on a line not sent and not in a fired course. After that,
  lowering a quantity would be a void with no manager and no reason; the
  till offers Void instead. The server does not enforce this yet: a client
  that sends `line.qty_changed` for a sent line is accepted (see STATE).
- **Approvals are a person and their PIN.** Void, comp and discount need
  the authoriser's permission, checked by the server against the PIN they
  type, so the ApprovalSheet asks who is approving (the signed-in person
  first) and takes their PIN. A wrong PIN, someone without the permission
  and a lockout each have their own words; the PIN clears, the reason and
  amount stay.
- **Reasons are fixed lists in the terminal** (`cart-logic.ts`): stable
  snake_case codes, because the event stores the code and reports count
  it. The back office will own them in phase 10.
- **A discount is typed on the amount pad from the right** in both modes:
  1250 is 12.50% (basis points, exactly what the event takes) or ₹12.50.
  The sheet refuses nothing over 100% or over what it comes off before
  asking the server, which checks again.
- **Found and fixed on the way:** the approval pad dropped digits typed
  faster than a render (a stale closure over the PIN); it now uses a
  functional update, as the lock screen already did. `PinDots` moved to
  `web/shared/screens/PinDots.tsx` so both pads share it.
- Checked in a browser against the real binary: quantity and a note in
  one save; a void refused for a wrong PIN, then accepted, struck through
  with its reason and off the total; a comp; 10% off a line; ₹20 off the
  order; an order note. Items, line discounts, the order discount,
  subtotal and total all agreed with the arithmetic.
Because: the cart is where a till loses money quietly, so every change is
an event with a person and a reason on it, and the till offers nothing a
manager has not approved once the kitchen has started.
Cost: removing a mistaken line that nobody has made still needs a void
and a manager. Notes cannot be edited or removed; a second note corrects
the first.

## ADR-073 - Quantity stops at the kitchen; a mistaken line comes off without a manager until money is taken
2026-09 · Status: accepted (decided on the owner's behalf, at their request)
Decision: two rules the cart (ADR-072) left open.
- **`line.qty_changed` is refused for a line sent, or in a fired course**
  (`ErrLineInKitchen`), in the domain. Lowering it would be a void with no
  approval and no reason; raising it would charge for an item the kitchen
  is never told about, because a sent line is not sent again. A void, or a
  new line, records what happened. The terminal already hid the stepper;
  now a client that sends one anyway is refused.
- **`POST .../lines/{line}/remove` takes off a line rung up by mistake
  with no manager,** as `line.voided` with reason `entered_in_error` and
  the caller as the authoriser. Only while nothing has happened to it: not
  sent, its course not fired, and **no payment on the order** that is not
  voided. Otherwise `needs_approval` (409), and the till offers Void.
  Checked in the service, inside the append transaction, against the
  state the event is appended to.
- Why the payment rule: the fraud this invites is "take the cash, remove
  the line, keep the cash". With no payment recorded, removing a line
  loses nothing the till knew about; once money is on the order, taking
  anything off needs a second person. The void stays on the order and in
  reports either way, with who removed it.
Because: a mis-tap at the counter is the commonest correction in a shift,
and calling a manager for it teaches staff to leave mistakes on the bill.
Cost: a remove is counted with voids in reports; phase 09's exception
report should show `entered_in_error` separately, per staff member, so a
pattern is visible.

## ADR-074 - Open tickets: hold as events, one outbox per ticket, a cache that only moves forward
2026-09 · Status: accepted
Decision: a till works several tickets. `order.held` and `order.resumed`
park and unpark an order; `GET /api/v1/orders` lists the unclosed ones;
the terminal keeps one TicketQueue per ticket in a TicketBook.
- **Hold is two events, not a flag on the side.** The order machine
  always had `open -> held -> open`; the projection now produces it. Only
  an open order can be held (one being settled has money on it). A held
  order takes only a note and a resume, so nothing is added to a parked
  ticket by a till that does not know it is parked (`order_held`, 409).
- **The open-ticket list is capped, not paginated.** It is a working set,
  tens of orders, oldest first, each projected from its snapshot; the cap
  is 200. Order history, which is paginated, is the Orders screen's.
- **The late threshold is a venue setting,** `terminal.ticket_late_minutes`
  (default 20, whole minutes 1 to 600), sent with the list. A bad value
  falls back to 20 rather than stopping sales: it only colours a clock.
  Past it the time is vermilion with a "Late" chip, colour and word.
- **One outbox per ticket.** Switching tickets changes which queue is in
  front; a queue still sending keeps sending to its own order. A tap on
  ticket A, a switch to ticket B while A is still opening, and a tap on B
  land on A and B respectively. Each queue carries its own opening (table,
  covers), sent when its first item opens the order. Only the ticket in
  front is remembered across a reload.
- **The order cache only moves forward.** A read that leaves before a
  command and lands after it would put the older order back: found in a
  browser, where switching to a held ticket fetched it while the resume
  was in flight, and the rail showed "held" after the server had resumed
  it. Every write to the cache keeps the copy with the higher `last_seq`.
- Checked in a browser against the real binary: a counter ticket and a
  table ticket side by side; hold, which moves the till to a fresh counter
  ticket; the list with both, the held one marked; switching back; a
  reload keeping the ticket in front; 25 minutes on, "Late 25m" in
  vermilion; resume from the list, three times over the race; and a second
  tab signed in on the same ticket seeing a line added in the first.
Because: a counter runs a queue and a table at once, and the ticket a
cashier is looking at must be the one their taps land on.
Cost: covers and table are fixed when the order opens; there is no event
to move a ticket to another table until phase 08.

## ADR-075 - The order timeline: events with names, from one read-only endpoint
2026-09 · Status: accepted
Decision: `GET /api/v1/orders/{id}/events` returns an order's whole stream
with `staff` and `devices` (id and name) and the venue's `time_zone`; the
register's History sheet renders it with the `Timeline` primitive.
- **Names travel with the events.** Events carry internal ids (actor,
  device, authoriser); the staff list the till has uses public ids and
  holds only active staff. History must name the person who left last
  month and the till that was unpaired, so the endpoint sends every staff
  member and device the venue has ever had. It is a small list, and
  cheaper than resolving the ids each payload type can mention.
- **The timeline says which till.** `TimelineEntry.device`, from an
  optional `deviceName`; an id with no name is left out, not guessed.
- **Times are in the venue's zone,** now that an endpoint carries it. One
  the browser does not know falls back to the device's own.
- **Reason codes read as words everywhere.** Discount reasons became codes
  in ADR-072; the timeline now humanises them as it did voids and comps.
  A counter sale's "0 covers" is left out.
- The history refreshes as the order moves: its query key sits under the
  order's, which the stream already invalidates.
- Checked in a browser: an order with lines, a sent line voided with a
  reason and approval, a discount, a hold and resume and a note, read back
  in order with names, till and times; a modifier change made by another
  client appeared in the open sheet.
Because: the event stream is the audit trail, and this is the screen an
owner opens when a number looks wrong.
Cost: every history read sends the venue's staff and device lists.

## ADR-076 - The tender screen: the server keeps the money, the till keeps only the change
2026-09 · Status: accepted
Decision: Charge on the register swaps it for a tender screen
(`terminal-tender.md`) that posts `TenderPayment` through the ticket's
ordered queue. No backend change: the endpoint and domain came in phase 04.
- **Cash sends the lesser of the balance and what was handed over less
  the tip.** The domain refuses an over-tender, so the change is never
  recorded; it is shown, worked out in `tender-logic.ts`, the only money
  sums on the screen. Amount due is always `totals.balance_due`.
- **The tip is keyed as an amount, not picked as a percentage.** A percent
  button is money arithmetic in the UI with its own rounding rule; the
  domain has none yet. Card and UPI buttons show amount plus tip, because
  that is what the card machine is keyed with; the two go to the server
  apart.
- **One idempotency key per tender attempt, kept across retries.**
  `TicketQueue.run` takes an optional key. A retry of the same tender
  reuses it, so a lost answer cannot charge twice; any change to method,
  amount, tip or reference makes a new key, since the server refuses a
  reused key with a different body.
- **Quick-tender is exact, then the next multiple of ₹10, ₹50, ₹100 and
  ₹500 strictly above the balance**, in the currency's major unit. The
  steps are hardcoded for rupees until the venue has settings (phase 10).
- **Charge takes the register's one accent;** the total went to the
  default text colour.
- The screen stops at "Paid in full". Issuing the invoice and closing the
  ticket are Task 3, inside the settling transaction.
- Checked in a browser against the built binary: ₹180 paid as ₹100 card
  with a ₹10 tip and a slip reference, then ₹100 cash for the ₹80 left,
  showing ₹20 change and a zero balance.
Because: the bill is the server's projection, and a till that did its own
sums would one day disagree with the invoice.
Cost: the tender screen is not a route; a reload returns to the register,
which shows the payments and the balance, and Charge again.

## ADR-077 - Splits: the server divides, each share is one whole tender
2026-09 · Status: accepted
Decision: `POST /api/v1/orders/{id}/split` runs `internal/domain/payment`
(Evenly, ByAmount, ByItem, ByGuest) over the order's own lines and returns
the shares. It is read-only: no event, no idempotency key. The till shows
the shares and takes one tender per share.
- **No arithmetic in the UI.** Every share is the server's. The one sum on
  the till is the staleness check (the unpaid shares against the balance),
  which decides whether a split may still be used, never an amount.
- **A split is a plan, not a record.** Nothing new is stored: the tenders
  are the record, as before. A split lives on the tender screen and is
  gone on Back or a reload; the balance is still right, and the rest can
  be split again.
- **A share is paid whole.** Card and UPI take exactly the share, cash at
  least it. A part-paid share would need the UI to track what is left of
  it, which is money arithmetic the task forbids.
- **Evenly and by amount divide the balance; by item and by guest divide
  the lines,** which sum to the whole bill. Once money is taken the lines
  no longer say what is owed, so those two are refused with
  `split_after_payment` rather than guessed at.
- **By guest takes its seats from the panel.** Lines have no seat until
  tables arrive (phase 08); then the panel can default each line to its
  seat.
- New codes: `split_invalid`, `split_over_allocated`, `split_unassigned`,
  `split_mismatch`, `split_after_payment`. At most 20 payers.
- Checked in a browser against the built binary: ₹410.00 split evenly
  three ways showed ₹136.67, ₹136.67, ₹136.66 and "In all ₹410.00", and
  was paid by cash (₹200, ₹63.33 change), card with a slip reference, and
  UPI, to a zero balance. By item with a shared latte also checked.
Because: a split that loses or invents a paisa is money gone, and the
domain package already refuses one; the UI only has to not do it again.
Cost: a split does not survive leaving the tender screen.

## ADR-078 - The invoice comes with the settling payment, and freezes the bill
2026-09 · Status: accepted
Decision: the tender that brings the balance to exactly zero also issues
the invoice, in the same transaction (`TenderPayment`), with a price
snapshot beside the tax snapshot. Reprints are an event, never a number.
- **Numbered at settlement, not at close.** Before, the number waited for
  Close; a ticket paid and never closed had none, and the preview had
  nothing to show. Now the payment and its number commit or roll back
  together. A tender the order refuses allocates nothing. Close still
  issues one for a bill that somehow has none, and `POST /invoice` remains
  for bills paid before this change.
- **`invoice.issued` gains `prices`:** the seller as the venue stood
  (trading and legal name, profile, registration, GST state code), each
  active line as billed, and the totals. The projection refuses a snapshot
  whose total is not the bill's. Old invoices have none and still read.
- **An invoiced bill is frozen until a reopen** (`ErrOrderInvoiced`,
  `order_invoiced`). Numbering at settlement opened a gap Close never had:
  a line added to a paid, invoiced ticket would have closed a sale bigger
  than its invoice. Refunds and payment voids are Task 6's to rule on.
- **`invoice.reprinted {invoice_no, reason, authorised_by}`,** allowed on a
  closed or held order, counted on `State.Invoices[].Reprints`
  (`ProjectionVersion` 2). It needs `invoice.reprint` from the approver,
  checked with their PIN like a void; migration 00007 grants it to owner
  and manager roles that still have their default names. Printing the copy
  is the print queue's (Task 5); the event is the log.
- **Routes address an invoice by the seq of its `invoice.issued`,**
  because numbers hold slashes: `GET /orders/{id}/invoices/{seq}`,
  `POST /orders/{id}/invoices/{seq}/reprint`.
- The till: "Paid in full" shows the invoice as a paper slip from its
  snapshots, Close ticket, and Reprint behind a manager's approval. Charge
  on a paid, open ticket reads Close ticket (or Invoice and close).
- Checked in a browser against the built binary: a ticket paid before this
  change invoiced as INV/26-27/000001, reprinted once with the owner's PIN
  (same number, "Reprinted 1 time"), closed; a fresh espresso paid in cash
  was INV/26-27/000002 at once; a line added after was refused in words.
  Migration 00007 applied to that older database.
Because: an invoice is a legal record of what was sold. It must be numbered
with the money that settled it and never disagree with the bill.
Cost: a guest who wants one more coffee after paying needs a new ticket or
a manager's reopen.

## ADR-079 - ESC/POS: ASCII on the wire, layouts from snapshots, a builder that refuses whole
2026-09 · Status: accepted
Decision: `internal/device/escpos` builds byte streams from the Epson
TM-series commands; `Send` writes them to TCP 9100; `nofiltr --test-print`
prints a test page with no venue, database or server.
- **ASCII only, code page PC437.** Printers disagree on code pages and none
  prints the rupee sign reliably, so `Printable` spells accents and marks
  in ASCII ("Creme brulee", "Rs.") and turns anything else, including a
  control byte in an item name, into "?". A name can never become a
  printer command. Devanagari menus print as "?" until a raster font path
  exists; that is a later phase's.
- **The builder keeps its first error and refuses the whole stream.** A
  size of 9 or a barcode with a brace makes `Bytes` fail rather than send
  a half-built receipt.
- **The drawer kick is `DLE DC4 1 m t`**, as the task asks: real time, so
  it opens mid-receipt or with the paper out. `ESC p` would wait behind
  the print buffer. 200ms pulse, pin 2.
- **Layouts take what to print and never sum it.** `Receipt` takes the
  invoice's snapshots (ADR-078), `KitchenTicket` the lines, `ZReport` the
  figures phase 09 will compute. Totals are double height, not double
  width, so a large total never wraps. A receipt with no invoice number is
  a bill and says "BILL - NOT A TAX INVOICE"; a reprint says DUPLICATE.
  The invoice number is also a Code 128 barcode for a refund to scan.
- **Paper is columns:** 48 on 80mm, 32 on 58mm. Every layout is tested at
  both through a decoder that replays the bytes as paper and fails on a
  line over the width, a byte outside ASCII, or a command it does not
  know.
- **`--test-print ADDRESS`** (`-paper 58`, `-drawer`, `-save FILE`) is
  checked before the server's own flags, like `setup`. The page shows a
  full-width ruler, so a wrong paper width is read off the slip.
- Checked against a TCP listener standing in for a printer: the built
  binary sent 954 bytes that replay as the page, with the drawer kick and
  the cut; an address with nothing listening fails in words. Not yet
  checked on a physical printer.
Because: a receipt is the one thing the guest takes away, and a printer
that prints garbage mid-service is worse than one that prints nothing.
Cost: no non-Latin text on paper yet.

## ADR-080 - The print queue: bytes in SQLite, one job at a time per printer, never dropped
2026-09 · Status: accepted
Decision: `print_jobs` holds each job's finished ESC/POS bytes and its
state; `internal/device/printq` sends them; `internal/domain/printjob`
decides what follows each send.
- **The bytes are the job.** A job is rendered once, when it is queued,
  and a retry sends exactly those bytes. A reprint of the receipt later
  is a new job from the invoice's snapshots (ADR-078), not a re-render of
  an old one.
- **States:** queued, printing, done, failed (waiting out its backoff),
  dead (no more automatic tries). Done is final. Dead goes back to queued
  only by a person (5b). A job with no printer is created dead with
  "no printer is set up for this", so it is on the screen, not lost.
- **Retries:** 2s doubling to a five-minute cap, ten tries, 13.5 minutes
  in all. A failure marked `printjob.ErrPermanent` (a retired printer, a
  transport this build lacks, a Windows printer name that does not exist)
  goes dead at once.
- **One job at a time per printer, in order,** decided in SQL: the due
  query returns a printer's oldest unfinished job only, so a job printing
  or backing off holds back that printer's later jobs and nobody else's.
  Printers are sent to in parallel, so a jammed bar printer never delays
  the kitchen. A claim moves a job to printing only from the state the
  worker read, so it cannot be sent twice at once.
- **A crash mid-send prints twice rather than never.** On start, jobs left
  printing go back to queued with the reason noted. A send that completes
  as shutdown begins is still recorded, so it does not print again.
- **Transports:** TCP 9100 through `escpos.Send`; the Windows RAW spooler
  through `winspool.drv` via `syscall` (no cgo), in a Windows-only file.
  Elsewhere a spooler job fails permanently with the reason. The real
  spooler answered a missing printer name as permanent in the tests; the
  write path has not met a real printer.
- The worker is owned by `printq.Queue` (Start, Close, Wake), polls every
  second for due backoffs, and stops before the database closes.
- Checked: migration 00008 applied to the scratch database from earlier
  tasks and the server started with the queue running. Nothing queues a
  job yet; that is 5b.
Because: printers jam mid-service, and a receipt that never printed is an
invoice the guest never got. Every job ends printed or in front of a person.
Cost: a job's bytes are fixed when queued: a wrong paper width set on the
printer later needs a new job, not a retry.

## ADR-081 - Print jobs are written in the order's transaction, and routed by station
2026-09 · Status: accepted
Decision: an order command's appended events decide its print jobs inside
the same transaction (`db.OrderTx.OnAppended`, `internal/service/printing`),
so a job exists exactly when its event does.
- **What prints:** `invoice.issued` queues the receipt on the receipt
  printer, with the drawer kick first when a captured cash tender is on
  the order; `invoice.reprinted` queues a copy marked DUPLICATE, from the
  original invoice's snapshots and its original time, without the kick;
  `ticket.sent` queues a kitchen ticket on that station's printer. Each
  job names the order and the event that caused it. A replayed command
  appends nothing, so it queues nothing twice.
- **Routing (migration 00009):** `stations.printer_id`, and one printer
  per venue flagged `is_receipt`. No printer is not an error: the job is
  created dead with "no printer is set up for this", kept on screen, and
  a retry takes the printer set up since (a kitchen job remembers its
  station for this). The back office sets these in 5c; until then they
  are set in the database.
- **A layout that fails is still a job:** dead, with the reason, never a
  failed payment. A database error does fail the command: the job and
  its event commit together or not at all.
- **The till hears about jobs like it hears about the menu:** a per-venue
  `print_versions` counter moves in every transaction that changes a job
  (queued, claimed, finished, requeued, retried, cancelled); the stream
  compares it after each drain and sends `print_changed`. The queue's
  `OnChange` nudges the hub after its own writes.
- **API:** `GET /api/v1/print-jobs` (every job not printed, and what
  printed in the last two hours), `POST .../{id}/retry` (failed or dead,
  ten fresh tries) and `.../{id}/cancel` (`order.edit`; queued or failed
  to dead, "cancelled by" who). Codes `print_job_not_found`,
  `print_job_state`, `no_printer`.
- **Terminal:** a Print queue button in the header that becomes "Not
  printed 2" in the danger tone while anything is failing or dead, and a
  sheet listing jobs with a chip (Waiting, Printing, Printed, Retrying,
  Not printed), what is happening in words with a countdown to the next
  try, and Retry and Cancel (`terminal-print-queue.md`).
- The drawer kicks only with a receipt: a cash part payment before the
  bill is settled does not open it yet.
- Checked in a browser against the built binary: a cash sale with no
  receipt printer showed "Not printed 1" in the header as it settled; a
  printer row pointed at a TCP listener, then Retry in the sheet, turned
  the job to Printed, and the listener got the 48-column receipt with the
  drawer kick first and a partial cut. Kitchen tickets are covered by the
  service tests only: the terminal has no Send button until phase 08.
Because: a receipt queued after its payment committed can be lost to a
crash between the two, and one queued before can print for a payment
that rolled back. Only the same transaction is neither.
Cost: rendering happens inside the order transaction, which holds the
write lock a few milliseconds longer.

## ADR-082 - Printer settings: the back office's first screen, owner only
2026-09 · Status: accepted
Decision: `internal/service/printers` holds the printer settings, served
at `/api/v1/printers` and `/api/v1/stations/{code}/printer`, and edited on
the back office's first real screen (`backoffice-printers.md`).
- **Owner only for now:** every call needs `settings.edit`, which only the
  owner role has by default. A manager can retry and cancel jobs on the
  till (ADR-081) but not rewire printers mid-service.
- **Checked on the server, shown under the field:** a name (1 to 40,
  unique per venue), network or USB, an address (a host, or host:port with
  a port 1 to 65535, never a URL) or a Windows printer name, 80mm or 58mm.
  A refusal is `invalid_printer` or `printer_name_taken` with the field,
  and the form puts it there.
- **Every change answers with the settings as they now stand,** which the
  screen puts straight in its cache; the stream's `print_changed` moves each
  printer's last result (its newest job) live.
- **Retire, not delete:** a printer's jobs name it forever. Retired, it
  prints nothing (its waiting jobs go dead as the worker reaches them), it
  stops printing receipts, and its stations print nowhere until given
  another. Brought back, it gets its stations back but not the receipts,
  which someone may have moved on purpose.
- **A test page is a queued job** (`kind` test, `escpos.TestPage` at the
  printer's paper width): it retries, and shows on the terminals' print
  queue, so the owner tests the same path a receipt takes.
- `PUT /api/v1/printers/receipt` sits beside `PUT /api/v1/printers/{id}`;
  the literal path wins, and "receipt" is never an id. The API client
  gained `put`.
- No idempotency keys on these: a retried create is refused as a name
  taken, and the rest set a value, so a repeat is harmless.
- Checked in a browser on a fresh venue with the demo menu: a URL as the
  address was refused under the field; a printer at a TCP listener was
  added, made the receipt printer, and its test page queued, printed
  (the listener got the 48-column page) and showed Printed live; the
  kitchen station assigned; retiring cleared receipts and the station
  selects and disabled the test.
Because: before this, a venue could not route a receipt anywhere without
editing its database.
Cost: a manager cannot set up a replacement printer during service without
the owner, until roles can be edited (phase 10).

## ADR-083 - Refunds on an invoiced bill are credit notes, never a second invoice
2026-09 · Status: accepted (the owner's choice, 2026-09-25)
Decision: when money goes back to a guest on a bill that already has an
invoice, the till issues a credit note: its own gapless series, naming the
invoice it corrects, with a snapshot of what it credits and the tax on it.
The original invoice is never changed and never issued again.
This supersedes the part of ADR-046 that said "one invoice per settlement;
a reopen allows the next", and the path it described for refunding a
settled bill (reopen, correct, refund, close, which then issued a second
invoice).
- **Why:** under GST a reduction to an issued tax invoice is recorded as a
  credit note against it. Two tax invoices for one sale, the first never
  cancelled, overstates the register and has no document the guest or an
  auditor can match to the refund. VAT receipts work the same way.
- **What is decided now:** credit notes exist, are gapless per venue,
  series and financial year like invoices, and are appended with the
  refund in one transaction, allowed on a closed order, with a reason and
  a manager's `refund.give`. Reopening no longer clears the invoice.
- **As built in Task 6a:**
  - `credit_note.issued {credit_no, series, against_invoice_no, reason,
    authorised_by, lines[], taxable, tax, total, components[], seller}`.
    Each line: line_id, name, qty (units credited whole; 0 for a share
    of an amount), taxable, tax, amount, and its tax by component.
  - **Both kinds of credit.** By units: a line's Due and tax times the
    units over its quantity, rounded once half to even, and the units
    that finish a line take exactly what is left of it, so parts always
    add up to the line to the paisa. By amount: spread over the lines by
    what each has left (`money.Allocate`), each share carrying tax in
    proportion to the tax the line has left. A line's tax is split over
    its components (CGST, SGST) by what is left of each.
    `order.PlanCredit` does this, pure; the projection checks the limits
    (the current invoice, active lines, nothing past what is left, figures
    that add up), not the rounding.
  - **The balance nets the credit:** `Totals.Credited`, and `BalanceDue =
    Total - Credited - NetPaid`. `Total` stays the invoiced total. A
    closed order takes `credit_note.issued` and `refund.issued`; a refund
    still needs a negative balance, so a closed bill is refunded only
    after a credit note. A reopen no longer clears `Invoiced`, and an
    invoiced bill stays frozen through it (ADR-078).
  - **Service:** `PlanCredit` (a preview, nothing written) and `Credit`,
    which plans, checks the refunds pay out the credit exactly (one per
    payment named), allocates from the credit note series, writes the
    `credit_notes` register (migration 00010, numbers checked against the
    counters like invoices, never edited or deleted) and appends the
    credit note with its refunds, in one transaction. A refused credit
    takes no number.
  - **Series:** `credit_note.series` (default `CN`) and
    `credit_note.prefix` (default the code), with the invoice template and
    GST's 16-character limit. The same code as the invoice series is
    refused: the two share the counters table.
  - **API:** `POST /orders/{id}/credit-notes/preview` and
    `POST /orders/{id}/credit-notes` (`refund.give` from the approver, with
    their PIN). Codes `not_invoiced`, `over_credit`, `refunds_mismatch`.
    The order carries `credits[]` and `totals.credited`; the timeline
    shows "Credit note CN/...", against which invoice, and why.
  - `ProjectionVersion` 3. A stream that reopened an invoiced bill and
    then changed it no longer projects; nothing in the field has one,
    since nothing has shipped.
- **Payment voids are not credit notes.** An invoice records the sale, not
  its payment. A payment voided after the invoice (a card charged twice,
  a UPI reversal) returns the balance to due; the next tender settles it
  without allocating a number (ADR-078's open question).
Because: the invoice register is what the venue files its tax from. It
must say what was sold and, separately, what was credited, each under its
own unbroken run of numbers.
Cost: Task 6 is split in two (6a domain and API, 6b the till), and a
refund is a document of its own that must print.

## ADR-084 - Past bills at the till: found by number, voided in one command, credit notes printed
2026-09 · Status: accepted
Decision: a settled bill is reached from Tickets, **Past bills**, and every
change to it goes through the commands ADR-083 already has.
  - **Finding a bill:** `GET /api/v1/orders/closed?q=` (`order.open`). No
    query gives the latest 30 closed orders. A query matches an invoice or
    credit note number exactly, or, when it is all digits, the invoice
    sequence (typing "4" finds INV/26-27/000004). A scanner types the
    receipt's barcode into the same field. Matching is on the registers,
    not on order payloads, so it stays an index lookup.
  - **Voiding a payment on a closed bill** appends `order.reopened` and
    `payment.voided` in one command with one approval, rather than asking
    for a reopen first. The bill then shows **Take the balance**, which
    brings it to the register's tender screen; the invoice keeps its
    number (ADR-083).
  - **The credit note prints** on the receipt printer through the queue,
    in the same transaction as the credit note (ADR-081): the `CreditNote`
    layout in `escpos`, headed CREDIT NOTE with the invoice it corrects,
    the lines or the part credited, tax by component, and the refunds with
    their methods. A refund to a cash payment kicks the drawer.
  - **Lines carry `credited_qty` and `credited`**, so the refund sheet's
    steppers stop at what is left without the till doing sums.
  - The till refunds to one payment per credit note; the API takes several.
    A refund larger than one payment has left is refused (`over_refund`)
    and the manager splits it into two credit notes.
  - Reason codes are worded on screen, on the timeline and on the printed
    note ("cold_or_late" is "Cold or late").
Because: a manager at the counter has the receipt in hand. The number on it
is the fastest way back to the bill, and a void that needs a reopen first is
two approvals for one mistake.
Cost: one credit note per payment from the till; a search that knows only
exact numbers and invoice sequences, not items, amounts or dates.

## ADR-085 - The floor plan: a fixed grid, one table per cell, retired never deleted
2026-09 · Status: accepted
Decision: the back office lays out the floor (phase 08, Task 1) as areas,
each a grid of 12 columns by 8 rows, and tables placed one per cell.
  - **Layout only.** `areas` and `tables` (migration 00011) hold names,
    seats, shape (square, round, long) and a cell. Whether a table is free,
    seated or dirty is a projection of its orders, from Task 2, never a
    column here.
  - **A cell, not coordinates.** A table's `x` and `y` are whole cells from
    the top left; every shape takes one cell. The grid size lives in the
    service (`floor.Columns`, `floor.Rows`) and is sent with the plan, so
    the terminal draws the same grid.
  - **One table in service per cell**, by a partial unique index. The
    service checks the cell first, inside the write transaction, so a
    refusal names the table already there under the column field.
  - **Retired, never deleted**, like printers: an order's `table_id` keeps
    pointing at a real row. A retired table gives its cell up, can still be
    edited and moved, and comes back only to a free cell in an area in
    service. An area retires only once its tables have.
  - **Names are unique across the venue**, not the area: staff say "table
    4" without saying where.
  - **Three ways to move a table**, all the same `PUT`: drag (pointer
    events, so fingers and pens work), Move then tap a cell, and the column
    and row typed in the edit form. Drag is never the only path.
  - Every call needs `settings.edit`, including the read; Task 2's floor
    screen will need its own read for terminals.
  - `orders.table_id` gets no foreign key yet: SQLite cannot add one
    without rebuilding `orders`, and nothing writes a table id until Task 2.
Because: a grid is enough to make the screen look like the room, and it
makes "is this spot free" a lookup rather than geometry. Free positioning
and table sizes can come later without changing what an order refers to.
Cost: a long table cannot span two cells; a room larger than 12 by 8 must
be split into areas.

## ADR-086 - A table's turn is derived from its orders; only "cleared" is stored
2026-09 · Status: accepted
Decision: where a table is in its turn (free, seated, ordered, settling,
dirty) is worked out on every read, by `table.Derive` in
`internal/domain/table`, from the orders at it. The one stored fact is
`tables.cleared_at` (migration 00012).
  - **With orders open at the table:** ordered if any has an active line
    and is not settling (a second round after the bill counts as ordered);
    else settling if any is settling; else seated. A held or reopened
    order still sits at its table. Covers add up; the balance is the sum
    of what is owed; `since` is the first order's opening.
  - **With none open:** dirty if an order closed there after `cleared_at`,
    else free. Clearing (`POST /api/v1/floor/tables/{id}/clear`) sets
    `cleared_at`; clearing a free table again succeeds, since two staff
    clearing one table is not an error, and an occupied table is refused
    (`table_occupied`).
  - **The live floor** is `GET /api/v1/floor/live`, for anyone with
    `order.open`: the areas and tables in service, each with status,
    covers, since, balance and the public ids of its open orders.
  - **Terminals learn of changes two ways.** Anything an order does
    (seating, a line, a payment, closing) is already an order event on the
    stream, carrying its `table_id`. What orders do not carry, a table
    cleared or the plan edited, moves a per-venue `floor_versions` counter
    in the same transaction, announced as `floor_changed` like
    `print_changed` (ADR-081).
Because: a stored table status would be a second record of what the orders
already say, and the two would drift the first time a payment and a status
update landed in different transactions. Dirty is the exception because
nothing about an order says the table was wiped down.
Cost: the live read projects every open order at the venue (capped at 200)
on each call. "Guests left" (Task 2b) closes an empty order, which on its
own would leave the table dirty: 2b must clear it in the same step.

## ADR-087 - The floor is a view of the register; seating opens the order at once
2026-09 · Status: accepted
Decision: the terminal's floor (phase 08, Task 2b, `terminal-floor.md`)
is a view the register swaps in, as it does the bill and tender screens,
not a route of its own.
  - **Inside the register**, so the till's ticket book (ADR-074) lives on
    while the floor shows: a ticket being rung up on another table is not
    lost by looking at the room.
  - **Seating opens the order now**, through the ticket queue
    (`TicketQueue.openNow`, `TicketBook.seat`), not with the first item as
    a counter ticket does. A seated party with nothing ordered is exactly
    what the floor must show (ADR-086).
  - **A tap does the likely thing:** a free table seats a party; an
    ordered or paying table with one ticket opens it; a seated table, a
    table to clear, or two tickets at one table open a sheet, because each
    has a choice in it.
  - **Words are the floor's:** Free, Seated, Ordered, Paying, To clear, in
    success, info, neutral, warning and danger. Vermilion stays for Late.
  - **Guests left** is `POST /api/v1/floor/tables/{id}/release`: every
    order at the table must be empty (`table.Leavable`: no active line, no
    money taken, nothing owed); each is closed by the order service's own
    close command, with no invoice, and the table is cleared. It is not one
    transaction, but a retry finds the orders closed and clears again.
  - **The server checks the table** when an order opens: one the venue
    does not have in service is refused (`table_not_found`). The Tickets
    sheet's typed table number goes, replaced by Seat a table, since a
    typed number is an id and the table's name is what staff know.
  - **`make check` now typechecks the three frontends.** It did not,
    though the conventions said so, and Task 2a shipped a terminal type
    error (the new `floor` notice in `useTicket`), fixed here.
Because: the floor and the register are two looks at the same shift's
tickets. Keeping one book under both means a seated party's ticket is the
ticket in front, with nothing to hand over between screens.
Cost: the floor has no URL of its own, so a till cannot open straight to
it; the register is the landing screen.

## ADR-088 - Moving and joining tables are order events; order_tables indexes where a party sits
2026-09 · Status: accepted
Decision: where a party sits is part of its order's story, told by three
events, and the floor reads it through an index kept with the order.
  - **Events:** `order.table_changed` `{from, to}` moves the order; `from`
    is where the till saw it, so a move on a stale screen is refused
    (`table_moved`) rather than applied to wherever the order went since.
    `order.tables_joined` and `order.tables_parted` `{tables[]}` spread one
    order, and one bill, over more tables and let them go. `State.Joined`
    holds them; `ProjectionVersion` is 4. Allowed after an invoice, since
    they change nothing on the bill; refused while held or closed.
  - **Free is checked in the command's transaction.** Move and Join refuse
    a table that is not in service at the venue (`table_not_found`) or not
    free (`table_taken`): an order open at it, joined or its own, or a bill
    closed there since it was last cleared. The single writer means two
    tills cannot take the same free table.
  - **`order_tables`** (migration 00013) has a row per table an order sits
    at, with the order's `closed_at`, rewritten by the append that changes
    either. "Is an order open here" and "when did the last one close" are
    index lookups however many orders a table has seen, and a joined table
    goes dirty with the party's bill like its own table does. The live
    floor and `TableTurn` read it; 00012's index on `orders` is dropped.
  - **A joined table** shows the party's state with `joined_to`; covers
    and balance are counted at the party's own table only. Guests left on
    any of the party's tables closes the empty ticket and clears all of
    them. A parted table goes back to its own turn, usually free.
  - **On the floor**, Move, Join and Part are modes: header buttons, a
    banner, taps, Done. `stepMode` in `floor-logic.ts` decides each tap,
    so the rules are tested apart from the screen. A table with two tickets
    cannot be moved from the floor, since a tap cannot say which.
Because: an order that knows its tables keeps the floor a projection of
orders (ADR-086) rather than a second record of who sits where, and the
timeline shows every move like every other change.
Cost: a table with two tickets can only be moved by moving the other
ticket away first; the timeline names tables by number, not by name.

## ADR-089 - Firing a course sends it; Send fires the first course
2026-09 · Status: accepted
Decision: routing is in the domain (`internal/domain/order/route.go`), and
a fire is never recorded without its tickets.
  - **`order.Tickets(state, course)`** is one `ticket.sent` per station for
    the course's lines not yet sent, stations in the order their first line
    was rung up. A voided line never goes; a comped one does, since it is
    still made; a line with no station is left out. The bar and the
    kitchen each get only their own lines.
  - **`FireCourse`** appends `course.fired` and those tickets in one
    command. A course fired with nothing in it is still fired, so lines
    added to it later go on Send.
  - **`Send`** (`POST /api/v1/orders/{id}/send`) appends `order.ToSend`:
    the unsent lines of every fired course and of course 0, and, while no
    course has been fired, the lowest course with lines waiting, fired now.
    A cafe whose menu is all course 1 sends everything with one tap; a
    restaurant's mains wait for **Fire course 2**. Nothing waiting is
    `nothing_to_send`.
  - `SendTicket` (one station, or chosen lines) stays for the API.
  - Kitchen tickets print as before: the print planner turns each
    `ticket.sent` into a job at the station's printer (ADR-081).
  - On the till, **Send N** and **Fire course N** sit over Discount, Note
    and Hold; `send-logic.ts` reads the same rule from the lines to label
    them, and the server decides.
Because: a course fired with no ticket is a table whose mains never reach
the pass, and a till that has to send each station by hand will miss one.
Cost: the first Send fires only the first course; a table that wants
everything at once fires each course, one tap each.

## ADR-090 - A ticket's life at its station is order events; the display needs no PIN
2026-09 · Status: accepted
Decision: bumping and recalling are order events, and a device paired as
a kitchen display uses the kitchen's API with no staff sign-in.
  - **Events:** `ticket.bumped` and `ticket.recalled` `{ticket_seq,
    station}`. A `ticket.sent` makes its ticket `fired`; a bump makes it
    `ready` with a ready time; a recall puts it back to `fired`. Both are
    checked against `internal/domain/kitchen`, which gains the two moves a
    display makes: fired to ready (no separate "picked up" tap) and ready
    to fired (recall). `collected` is left for later: on a display, bumped
    off is done. `ProjectionVersion` is 5.
  - **The kitchen works after the bill.** Bump and recall are taken by a
    closed or held order: a cafe paid at the counter still makes the
    coffee.
  - **The hub files them under their station**, so a display subscribed to
    `station:kitchen` gets its bumps and recalls and nothing else's.
  - **`station_tickets`** (migration 00014) indexes every ticket sent, with
    its status and times, kept in step by the append. A station's board is
    one index read: tickets fired, oldest first, and those bumped within
    `RecallWindow` (30 minutes), from open and closed orders alike. No
    backfill: a ticket sent before the display existed would disagree with
    its order, and a display must not open on a day of old orders.
  - **API:** `GET /api/v1/kitchen/{station}/tickets`, and `POST
    /api/v1/kitchen/orders/{id}/tickets/{seq}/bump` and `.../recall`,
    each answering with the station's board: lines with modifiers and
    notes, the order's notes, the table's name (joined ones too), covers,
    and times. Only a device of kind `kitchen` gets in, with no staff
    session: the events carry the device and actor 0. A terminal is
    refused. The order response's tickets carry their status too.
Because: a display that drops off must catch up from the stream with
nothing lost (Task 5), which only events give; and a screen on the wall of
a kitchen has nobody to type a PIN into it.
Cost: anyone at the pass can bump, and the timeline names the device, not
a person.

## ADR-091 - The kitchen display is a fourth app in the terminal's bundle
2026-09 · Status: accepted
Decision: the kitchen display (`kitchen-display.md`) is served at
`/kitchen` from the terminal's bundle, as its own app.
  - **Its own app kind, `kitchen`:** its device token under
    `nofiltr.kitchen.*`, pairing as a kitchen device (a terminal code is
    refused), and no PIN: paired is enough (ADR-090). `main.tsx` picks the
    app from the path before anything else starts, as it already does for
    the gallery. No fourth bundle, vite config or mount: a display on the
    LAN loads the terminal's 100 KB once and caches it.
  - **One station per device,** kept in `localStorage`. The stream is
    subscribed to `station:{code}` alone, so the display hears only its
    own tickets; changing station reloads the page to subscribe afresh.
    A void or note on a sent line carries no station, so the board is also
    read every 20 seconds.
  - **Timers** escalate from the venue's own late time: gold with the word
    "Waiting" from half of it, vermilion with "Late" from it. The board
    carries `late_after_minutes`.
  - **Type for three metres:** two tokens, `--text-kitchen` (26px) for
    lines and `--text-display` (44px) for where and when; Bump is 72px.
  - **The till shows Ready** on a line whose ticket is bumped, from the
    order's own tickets.
Because: the display is a small, single-purpose screen that shares the
terminal's primitives, stream and API client; a separate bundle would be
plumbing for no user.
Cost: the display carries the register's code it never runs, and the
station is per device, not per venue setting.

## ADR-092 - The dine-in loop is one test on the real stack, in process
2026-09 · Status: accepted
Decision: phase 08's realtime loop and its done-when are proved by
`TestTheDineInLoopAcrossDevices` (`internal/api/loop_test.go`), not by a
manual check.
  - **The real stack:** a temp-file database, the order service
    publishing to a real hub, the floor service, and the API with the
    stream, behind an `httptest` server. Two real WebSocket clients
    (`coder/websocket`) stand in for the till (the venue topic) and a
    kitchen display paired as a kitchen device (`station:kitchen`); the
    till's commands go over HTTP as the browser's do.
  - **What it proves:** a sent ticket is on the display, and a bump on the
    till, each within a second (measured, logged; a few milliseconds on a
    laptop). The display is unplugged with no goodbye, service goes on (a
    second round, a recall, the bar), and it reconnects with the last id
    it saw: its replay is exactly the kitchen's events after that id, in
    order, then `caught_up`; no event reaches either device twice; the
    live stream carries on. Then course 2 is fired to the kitchen and the
    bar, the bill is settled and closed, table 4 goes dirty, is cleared
    (the till is told `floor_changed`) and is free.
  - The browser's half of the reconnect, sending its last id in the hello,
    is `realtime.test.ts`'s.
  - It is in process rather than against the built binary: the binary adds
    only `main.go`'s wiring, which the test repeats, and a test that
    builds and runs a binary is slow and fragile on Windows. The carried
    "done-when script against the real binary" stays open for phase 12's
    installer smoke test.
Because: the phase says to write this as an automated test, and a test of
the loop that fakes the hub or the database proves the fakes.
Cost: one long test, where a failure names the step but reruns the whole
service.

## ADR-093 - A shift is a row, a float count and an append-only ledger
2026-09 · Status: accepted
Decision: phase 09's shift (Task 1a) is stored as three tables, and the
trading day is decided once, when the shift opens.
  - **One open shift per venue**, enforced by a partial unique index on
    `shifts (venue_id) WHERE status = 'open'`: one venue, one drawer in
    v1. Two tills opening at once give the second `ErrShiftOpen`.
  - **Lifecycle open -> closed, and closed is final.** A trigger refuses
    any change to a closed shift, and an open one may change only by
    closing. A mistake found after close is a later document (the
    Z-report's correction, Task 3), never a reopened shift.
  - **The trading day** is the local date in the venue's zone at opening,
    or the day before when it opens before `shift.DefaultDayStart`
    (04:00, by the wall clock, so a daylight-saving night does not move
    it). It is stored on the shift, so a timezone edit later cannot move
    a closed day. There is no venue setting for the day start yet.
  - **The float is a drawer count,** the `opening` row of
    `drawer_counts`, not a column on the shift. Task 2's blind count is
    the `closing` row of the same shape. A count is denominations
    (`[{face, count}]`, face in minor units) that must add up to its
    total, or a lump sum with none. Counts are never updated or deleted.
  - **Cash movements are `shift_events`** (`opened`, `cash_in`,
    `cash_out`, `closed`), append-only by trigger like order events. A
    movement's amount is positive and the type is the direction; the
    database itself refuses one with no reason or no authorising staff
    member. Checking that the authoriser may is the API's (Task 1b),
    through the same override flow as a void.
  - **Idempotency on the event,** not the order-keyed
    `idempotency_keys`: `shift_events.idempotency_key`, unique per shift.
    A retry with the same key and the same request gets the shift back;
    the same key for a different request is `ErrIdempotencyKeyReused`.
  - `Shift.Drawer` is the float plus cash in less cash out. It becomes
    the expected cash once 1b attaches cash payments and refunds, and is
    shown to staff only after the blind count (Task 2).
Because: the shift is what the Z-report and the blind count stand on, so
its record has to be as tamper-proof as the order events, and a day
decided at report time would let a zone change or a late close move
money between days.
Cost: a payout larger than the drawer is not refused (the count at close
shows it); the day start is a constant until the back office has
settings for it.

## ADR-094 - Every order event carries its shift; the store refuses a sale with none open
2026-09 · Status: accepted
Decision: orders and payments are attached to shifts (phase 09, Task 1b)
where events are written, in `internal/db`, not in each command.
  - **Stamped per event, not per order.** `order_events.shift_id` is the
    shift open when the event was recorded, read inside the appending
    transaction (SQLite's one writer means a close cannot slip in
    between). A tab opened in one shift and paid in the next reports its
    payment, discounts and voids in the shift they happened in; the
    Z-report (Task 3) reads events by shift, not orders by day.
    `orders.shift_id` is the shift the order opened in, for lists.
  - **Refused with no shift open:** every order event except
    `ticket.bumped`, `ticket.recalled` and `invoice.reprinted`
    (`shift.NeedsShift`), none of which touch money. A type not listed
    needs a shift, so a new event is let out on purpose, and a test names
    every type. One refused event refuses the whole append.
    `shift.ErrNoOpenShift` is `409 no_open_shift`.
  - **0 is no shift,** as `table_id` is: orders from before migration
    00016, and the kitchen's bumps after close. No foreign keys, since
    `ALTER TABLE` cannot add one with a non-null default.
  - **Closing does not wait for open tickets.** They carry into the next
    shift, and so does anything taken for them. Whether close should warn
    about them is Task 2's screen.
  - **The API:** `GET /api/v1/shift` (order.open) answers the open shift
    or `{"shift": null}`, never 404; `POST /shift/open` and
    `/shift/{id}/close` need the new `shift.manage`; `/shift/{id}/cash`
    needs payment.take from the cashier and an override for the new
    `cash.move`, as a void does. Migration 00016 grants both to owner and
    manager, as 00007 did for reprints. The shift response has **no
    expected drawer figure**: Task 2's blind count must come first.
Because: a check in the store cannot be forgotten by the next command
added, and per-event stamping is the only rule under which a shift's
figures still add up when a bill spans two shifts.
Cost: every test that sells has to open a shift first; the terminal
cannot sell until a shift screen exists (Task 2), only over the API.

## ADR-095 - The blind count is taken once and reveals the drawer in its answer
2026-09 · Status: accepted
Decision: phase 09, Task 2a's close is count, reveal, explain, close, and
the server holds the expected figure back until the count is in.
  - **Expected cash** is the float, plus cash in, less cash out, plus the
    shift's cash from sales: cash tenders taken in the shift, bill and tip
    (the tip was handed over at the till and sits in the drawer until paid
    out, which is a cash movement), less cash tenders voided in it (bill
    and tip) and refunds against cash tenders given in it, whichever shift
    took the money. It is worked out from the whole streams of the orders
    touched in the shift (`shift.CashFromOrders`), so a refund today of
    yesterday's cash bill leaves today's drawer.
  - **Blind.** `GET /api/v1/shift` serves no expected figure, no cash
    sales and no drawer before the count; the answer to
    `POST /shift/{id}/count` is the first to carry `reconciliation`. A
    test fails if any of those words appear in an uncounted shift.
  - **Once.** One closing count per shift (the unique index from 00015).
    The same count retried is answered; any other count is
    `already_counted`, so nobody recounts until the drawer matches. A
    miscount is explained, not replaced.
  - **Recorded with what it was measured against:** the expected figure,
    the variance (the database checks it is total less expected) and the
    threshold then in force, on the count's row. Loosening the threshold
    afterwards cannot excuse a counted shortfall.
  - **Threshold:** venue setting `shift.variance_threshold_minor`, in
    minor units, default 0: any variance at all needs a reason until the
    owner says otherwise. Exactly at the threshold needs none.
  - **Reason:** `variance_reasons`, one per shift, write-once, up to 500
    characters, given after the reveal; allowed within the threshold too.
  - **A counted shift is shut:** the store's open-shift lookup skips it,
    so orders get `no_open_shift`, and cash movements `drawer_counted`.
    It is still the current shift until closed, and still blocks opening
    the next one.
  - **Close** needs the count and, past the threshold, the reason
    (`not_counted`, `variance_unexplained`).
  - Counting and explaining need payment.take (whoever handles the drawer);
    closing still needs shift.manage.
Because: a count that can be retaken until it matches, or a threshold
applied after the fact, is not a control; and an expected figure anywhere
before the count is one a cashier can read.
Cost: a genuine miscount stays on the record with its explanation; there
is no recount.

## ADR-096 - The till's shift screens take the register's place
2026-09 · Status: accepted
Decision: phase 09, Task 2b's screens (`terminal-close.md`) are states of
the register, not routes.
  - **No shift, no register:** with `{"shift": null}` the register is
    "The till is closed", so there is no path to a sale the server would
    refuse. A counted shift shows its reveal on every till until someone
    closes it, since that till can sell nothing either.
  - **The counter counts by value:** one row per face value, largest
    first; a note and a coin of the same value share a row. The tables
    live in the terminal (`shift-logic.ts`) for INR, USD, EUR and GBP; a
    currency with none is counted as one total on the pad. The pad types
    a row's count rather than steppers, since forty-seven Rs. 10 notes
    are typed, not tapped.
  - **The count is confirmed first,** because it cannot be retaken
    (ADR-095). Back to the register is offered only before it.
  - **The shift is polled** every 30 seconds and read from every
    command's answer; the stream carries no shifts yet.
Because: a till that can show the register with no shift open invites a
sale the server refuses, and a counted drawer that looks like a working
register invites one more.
Cost: every till polls; a shift opened elsewhere reaches a closed till
within 30 seconds, not at once.

## ADR-097 - A Z-report adds up events by the shift they were recorded in
2026-09 · Status: accepted
Decision: phase 09, Task 3a's figures come from `zreport.Build`, a pure
fold over the whole streams of the orders touched in a shift, projected
event by event so each of the shift's events is measured against the bill
as it stood just before and after it.
  - **A sale is an invoice issued in the shift,** at the bill's state when
    it was issued: its active lines' gross, discounts (line and the
    order's share), tax and due. Every heading (category, item, channel,
    staff) is built from those same lines, and a bill whose lines do not
    add up to its total fails the report (`ErrInconsistent`).
  - **Everything else belongs to the shift it happened in:** credit notes
    (sales reversed, and their tax off the tax lines), refunds, comps and
    voids at the line's gross, payments by method taken, voided and
    refunded. A refund today of yesterday's bill is today's.
  - **Removes apart from voids:** a line taken off as entered in error
    (`order.ReasonEnteredInError`, moved into the domain) is counted per
    staff member who removed it, apart from a manager's voids (ADR-073).
  - **Staff** is who opened the bill. **Categories** are the menu's when
    the report is built, since a line does not carry its category; the
    report is frozen when generated (3b), so a later menu edit cannot move
    a closed day's figures.
  - **Tips** are counted apart and are in neither sales nor payments.
  - **Balance:** payments net of voids and refunds, against sales net of
    credit notes. On a day whose bills were paid and invoiced within it
    the difference is zero; otherwise it is money taken for bills invoiced
    in another shift or not yet (a deposit on an open tab).
Because: the only rule under which two shifts' reports add up to the two
days' money is that each event counts once, where it was recorded; and a
report that recomputes prices can disagree with the invoices it reports.
Cost: a report projects every stream it touches from the start, which is
fine for a shift's few hundred orders.

## ADR-098 - The Z-report is issued in the closing transaction and frozen
2026-09 · Status: accepted
Decision: phase 09, Task 3b stores each Z-report as a numbered, immutable
document written by the same transaction that closes its shift.
  - **With the close, or not at all.** `Close` builds the report and
    stores it before committing; a report that cannot be built (a stream
    the projection refuses) refuses the close. No closed shift is ever
    without its report, and no report describes a shift still open.
  - **Numbered per venue** from `z_report_series`, one upsert in the
    transaction, never reset and never `MAX(number) + 1`, as invoices are.
  - **Frozen:** the body is the whole document as JSON: the figures
    (`zreport.Report`), the drawer's reconciliation, the shift, and the
    names of the staff it mentions as they were then. It is never worked
    out again from the orders once written; a trigger refuses any update or
    delete.
  - **Corrections** are new reports: the next number, `corrects_id` and a
    reason (the schema requires a reason exactly when it corrects), the
    figures worked out afresh. The original stays. A shift's report "as it
    stands" is its highest number. A retry of the same correction by the
    same person with unchanged figures returns it rather than issuing
    another. Only a closed shift can be corrected.
  - **API:** reading needs reports.view; correcting needs shift.manage.
    The list is cursor-paginated on the number, 50 a page.
Because: a Z-report is the owner's and the tax office's record of a day;
it must exist for every closed day, read the same forever, and change
only by a later document that says why.
Cost: a corrupted stream stops the till from closing until it is fixed;
that is loud on purpose.

## ADR-099 - The Z-report prints from the stored document, with the close
2026-09 · Status: accepted
Decision: phase 09, Task 3c prints Z-reports through the print queue, laid
out from the frozen document (ADR-098), never from the orders.
  - **Queued in the same transaction** as the close or the correction, so
    a report is on the queue exactly when it exists; the queue is woken
    once that commits. It goes to the receipt printer; with none it waits
    dead with the reason, as every job does, and prints once one is set up
    and the job retried.
  - **A copy on request** (`POST /api/v1/z-reports/{n}/print`,
    reports.view) is marked COPY; a correction prints "CORRECTS Z-REPORT n"
    and its reason.
  - **What the paper carries:** the Phase 07 layout, extended: trading
    day, discounts with their count, tax per component and rate ("CGST
    2.5%"), net of credit notes when there are any, by category, channel
    and staff, payments net by method, tips apart, "Paid for other shifts"
    when payments and sales differ, the exceptions (removed in error apart
    from voids, credit notes, reprints) and the drawer's make-up (float,
    cash in, cash out, cash sales, expected, counted, difference, why). By
    item is left to the screen: a hundred lines of paper is not read.
  - The report now counts invoice reprints in the shift.
Because: the paper and the screen must say what the stored report says,
and a report printed from anything else could differ from it.
Cost: none worth naming; the escpos layout grew optional sections, and
an X-report (Final false) still prints as before.

## ADR-100 - One report view, shared by the till and the back office
2026-09 · Status: accepted
Decision: phase 09, Task 3d shows a Z-report through one component,
`web/shared/zreport/ZReportView`, over the stored document.
  - **No arithmetic:** every figure, count and total on screen is the
    document's (ADR-098); the view only chooses words (a channel's name,
    a method's, "2.5%" from thousandths by string, "Short Rs. 10.00").
  - **Shared, not duplicated:** the till and the back office import the
    same view, screen wrapper (loading, not allowed, not found, Print a
    copy) and hooks. A report is read once and kept (`staleTime:
    Infinity`), since it never changes.
  - **The till** shows the report its own close issued, from the close's
    answer, with View and Print a copy; it keeps no list. The back office
    lists every report, cursor-paged 50 at a time, and opens one by
    number at `/reports/{n}`.
  - **By item** is on screen, folded to ten until asked, and not on paper.
  - The print route answers 204, which the API client reads as done; the
    shift service now tells the stream after a close, a correction or a
    copy queues a print, so a till's print badge follows.
Because: a report on two screens that could each add up differently is
two reports.
Cost: the terminal bundle carries the view (about 2 KB gzipped).

## ADR-101 - Tips are pooled once per closed shift, by a venue rule
2026-09 · Status: accepted
Decision: phase 09, Task 4a pools a shift's tips after it closes, split by
the venue's rule, and records the split once.
  - **The pool** is every tip taken on a tender in the shift, less the tips
    of tenders voided in it (`tips.Taken`, by who tendered). It equals the
    Z-report's tips; tips are never in sales or payments.
  - **Rules:** `even`, `hours` or `role_weight`, venue setting
    `tips.rule` (default even). Role weights are `tips.role_weights`, a
    JSON object by role name, 1 to 100, a role not named weighing 1.
    Hours are typed in at pooling, in minutes, since there are no
    timesheets until phase 10.
  - **Exact:** `tips.Split` gives every share through `money.Allocate`, so
    the shares always add up to the pool; the paisa left over goes by the
    largest remainder, earliest member first.
  - **Who:** the preview suggests everyone who took a tip or recorded an
    order event in the shift; the manager chooses. Anyone who took a tip
    stays on the report with what they took, in the pool or not.
  - **Once:** pooling needs a closed shift and shift.manage, and is
    recorded immutably (`tip_pools`, one per shift). A retry of the same
    pool by the same person returns it; any other is `already_pooled`.
    Reading needs reports.view.
  - **Staff are named by public ID** in the API (`stf_...`), as everywhere
    a client picks a person.
Because: a split that anyone can redo after the money is handed out is
not a record of what was handed out.
Cost: a pool entered wrongly cannot be redone; the fix is outside the
system until a correction exists.

## ADR-102 - The tips screen shows the server's shares before pooling
2026-09 · Status: accepted
Decision: phase 09, Task 4b's screen (`tips.md`) asks the server for the
shares of every choice of people, and never splits money itself.
  - **A preview route,** `POST /api/v1/shift/{id}/tips/preview`
    (reports.view), gives what pooling would, recorded nowhere. The screen
    calls it on every change of who is in or their hours, keeping the last
    answer on screen while the next comes.
  - **The only arithmetic** on the screen is hours and minutes into
    minutes (whole hours, 0 to 59 minutes, at most 36 hours).
  - **Where it lives:** from a Z-report in the back office (Tips), at
    `/shifts/{id}/tips`; the Z-report screen takes extra actions for it.
    The till has no tips screen.
  - **Starting people:** the server's suggestions in, then every active
    staff member out; a pooled shift is the read-only report, with anyone
    who took a tip and was left out shown with no share.
Because: a share the screen works out could differ from the one recorded,
and a manager should see exactly what they are committing to.
Cost: a request per change while choosing; a shift's pool is small.

## ADR-103 - The tax register is filled by column key, never by regime
2026-09 · Status: accepted
Decision: phase 09, Task 5a writes the exports in `internal/domain/export`
from the documents issued in a shift, with no code that asks which tax
regime the venue is under.
  - **The register** is the profile's `RegisterColumns()`, in order, with
    its labels. Each key is either a dimension (invoice number, date,
    place of supply, HSN, rate, jurisdiction, invoice value) or an amount
    (taxable value, net, taxable, exempt, CGST, SGST, IGST, VAT, tax,
    gross). A line's values come from the invoice's own tax snapshot;
    components go to the column named for them (UTGST with SGST). Rows are
    summed over everything that shares every dimension column, so the
    profile's choice of columns is its choice of granularity: GST gives a
    row per invoice, HSN and rate, VAT per receipt and rate, US sales tax
    (whose columns have no invoice number) per day and authority. A
    register with a `jurisdiction` column is kept by authority, one row per
    component; with an `exempt` column, lines with no tax go there. A key
    this build cannot fill fails the export, and a test walks every
    profile's columns.
  - **Credit notes** are negative rows, their lines' HSN and rate taken
    from the bill.
  - **Tally** is a CSV of ledger entries (date, voucher type, number,
    reference, ledger, debit, credit) for TallyPrime's spreadsheet import:
    a Sales voucher per invoice debits the payment methods it was paid by
    and credits "Sales @ 5%" and each "CGST @ 2.5%"; a Credit Note is the
    reverse, crediting the methods its refunds went to. An invoice whose
    voucher does not balance fails the export; a credit note not wholly
    refunded in the shift balances against "Customer credit".
  - **The day's sales:** a row per invoice or credit note, with its
    channel, who opened it, its figures and what it was paid by.
  - Amounts are plain decimals ("-285.30"); dates are in the venue's zone.
Because: the phase asks for a VAT venue to export VAT columns with no code
change, and grouping by the profile's own columns gives each regime its
register's shape from one path.
Cost: Tally's import needs its ledgers mapped once, by name, in Tally.

## ADR-104 - Exports are written with the close and read back as JSON
2026-09 · Status: accepted
Decision: phase 09, Task 5b stores a closed shift's three files (tax
register, Tally vouchers, day's sales) in the closing transaction, and the
back office downloads them through the ordinary signed-in API.
  - **Written with the close,** after the Z-report and before its print,
    in the same transaction: every closed shift has its files, and a shift
    whose files cannot be written (an invoice voucher that does not
    balance) does not close, as with its report (ADR-098). One file of
    each kind per shift, write-once by trigger (`exports`).
  - **Named for filing:** `2026-09-28-shf_7K3M9QX2-tax-register.csv`,
    the trading day first so a folder sorts by day.
  - **Read as JSON:** `GET /api/v1/exports/{id}` answers the file's name,
    content type and CSV text; the back office turns it into a file with a
    Blob and a download link. A plain link could not carry the device and
    staff tokens, and a second, cookie-based way to sign in is not worth a
    CSV of a few kilobytes.
  - Reading needs reports.view. "Queued at close" in the phase is met by
    writing at close; there is no background job, since the files take
    milliseconds and must exist exactly when the close does.
Because: a bookkeeper's files should be as certain as the report they
come from, and as easy to take again.
Cost: a file travels as a JSON string, a few percent larger; shifts closed
before this have none.

## ADR-105 - The session carries its permissions; the back office hides by them
2026-09 · Status: accepted
Decision: phase 10, Task 1 gives the back office a shell whose sections
are shown only to staff who may open them
(`docs/design/screens/backoffice-shell.md`).
  - **Permissions on the session,** not on the staff list:
    `POST /api/v1/sessions` and `GET /api/v1/sessions/current` answer
    `permissions`, the role's set in canonical order. `GET /api/v1/staff`
    is open to any paired device, and what each person may do is not its
    business.
  - **Hiding is a courtesy.** Every endpoint still runs
    `requirePermission`; the client's list only keeps a manager from
    landing on screens that would answer 403. A path typed by hand shows
    "That is not yours to open" and the screen never mounts.
  - **One list** (`web/backoffice/src/shell/nav-logic.ts`) drives the
    navigation, the dashboard, the route guard and the `g` chords, so
    they cannot disagree. A section not built yet stays in it as Soon.
  - Stock is gated on `menu.edit` until Task 3 decides whether stock
    needs a permission of its own.
  - A session stored before this has no permissions and shows nothing
    gated until the server answers the boot check.
Because: the phase asks for permission-gated sections, and the client had
no way to know a role's permissions short of trying every endpoint.
Cost: a role edited mid-session reaches the shell only at the next sign-in
or reload; the server refuses in the meantime.

## ADR-106 - Menu edits go through one door that bumps the version
2026-09 · Status: accepted
Decision: phase 10, Task 2a writes categories and items from the back
office through `internal/service/menus`, over `/api/v1/menu/...` with
`menu.edit`.
  - **One door:** `db.DB.EditMenu` runs an edit and `BumpMenuVersion` in
    one transaction; the service then calls the hub's `Notify`, whose
    ping makes every connection compare versions and send
    `menu_changed`. An edit that fails writes nothing, bumps nothing and
    nudges nobody.
  - **Nothing is deleted:** retire and bring back, as the schema intends.
    The editor's read (`GET /api/v1/menu/editor`) includes retired rows;
    the till's does not. Retiring a category takes its items off the till
    without touching their own flag, so bringing it back restores them.
  - **HSN by the profile, not by name:** `tax.NeedsHSN` reads the
    profile's own per-line invoice fields. Where it is true every item
    needs a code `tax.ValidHSN` accepts (4, 6 or 8 digits, the same rule
    GST invoicing applies), so an item that could never be invoiced is
    refused at save, not at the till. Elsewhere an HSN is refused.
  - **Ordering by whole list:** `PUT .../order` takes every id in scope,
    retired ones included; anything else is `menu_order_stale` (409), so
    an editor holding a stale list cannot drop rows from the order.
  - A price arrives as `money.Amount` and must be in the venue's
    currency. An edit answers only the id; the editor reads the menu
    again, as the printers screen does.
  - Like the printer settings, these routes do not honour
    `Idempotency-Key`: a retried create can add a second item.
Because: the snapshot rule already keeps open tickets safe from any edit;
what can still go wrong is a till selling from a menu it thinks is current,
and the one door closes that.
Cost: every edit re-reads the whole editor document; fine at a cafe's
few hundred items.

## ADR-107 - The menu editor says what a price change does before Save
2026-09 · Status: accepted
Decision: phase 10, Task 2b's editor (`docs/design/screens/backoffice-menu.md`)
shows the snapshot rule where the owner decides, not after.
  - **The notice is live:** editing an item and typing a different price
    shows "Tickets already open keep ₹200.00. Lines rung up after you save
    are ₹220.00." under the price, before Save. The toast after a repriced
    save repeats it, and a line under the page title says it always.
  - **Prices are typed in major units and read as text:**
    `parseAmount` (`web/shared/money.ts`) joins the whole and fraction
    digits into one integer, refuses more decimals than the currency has,
    and never divides or multiplies a float. `amountInputText` is its
    inverse for the form's starting value.
  - **Order by arrows, not drag:** each move saves the whole list at once
    (ADR-106); a stale list reloads and says so.
  - **Layout:** categories beside the items only from 1536px; below that
    they sit above, so a 1366px laptop keeps every column and action. The
    shell's content column gained `min-w-0`: without it a wide table
    stretched the page instead of scrolling inside its own box.
Because: "open tickets keep their price" is only understood if it is
read at the moment of choosing a new one.
Cost: the notice appears only in the form; a retire has no equivalent
("tickets already open keep this item") yet.

## ADR-108 - A group of choices is saved whole, and must be satisfiable
2026-09 · Status: accepted
Decision: phase 10, Task 2c edits modifier groups through the menu door
of ADR-106.
  - **Saved whole:** `POST /api/v1/menu/groups` and `PUT .../groups/{id}`
    take the name, the bounds and every choice in order. A choice with an
    id is updated in place; one without is new; one the group had but the
    request leaves out is retired (never deleted: a line that chose it
    still names it). Listing a retired choice's id brings it back.
  - **Satisfiable or refused:** `internal/domain/menu.CleanGroup` refuses
    a group its own choices cannot meet (asks for more than it offers,
    more picked to start than may be picked) and two choices with one
    name. More allowed than offered is fine. A refusal about one choice
    names it as `modifiers.N.name`, so the form shows it on that row.
  - **Items offer groups as one ordered list:** `PUT .../items/{id}/groups`.
    Retiring a group keeps the links; the till's menu already drops a
    retired group, so bringing it back restores it everywhere. The item
    form saves the item, then its groups: two requests. If the second
    fails after a new item was created, the form closes with a warning
    rather than stay open where Save again would add the item twice.
  - **Supersedes ADR-106 on HSN away from GST:** a well-formed HSN is now
    kept on a venue whose profile does not use one, and only a malformed
    one refused. The demo menu carries HSN codes on every venue, so the
    first rule made its items uneditable outside GST; a venue leaving GST
    would have met the same wall.
Because: a choice sheet the till can never complete is a stuck order at
the counter; refusing it when it is saved is the cheap place.
Cost: a negative choice price is not checked against the items that
offer it, so a line can be priced below zero by its choices.

## ADR-109 - Sold out is a time, and the till's clock ends it
2026-09 · Status: accepted
Decision: phase 10, Task 2d1 marks an item sold out for today by storing
when it is back on sale (`items.sold_out_until`, migration 00021), not a
flag.
  - **Until the next trading day:** `shift.NextDayStart` gives the moment
    the day after now's trading day begins, at `shift.DefaultDayStart`
    (04:00) on the venue's wall clock. Sold out at 23:00 or at 01:30 is
    back at 04:00, the same boundary the shift and Z-report use.
  - **The till decides when it ends:** the menu carries `sold_out_until`;
    `isSoldOut` compares it with the device's clock, re-read once a
    minute. Nothing has to bump the menu version at 04:00, and an
    unreadable time counts as on sale.
  - **Marking is a menu edit** through ADR-106's door: the version bumps
    and every till shows it at once (seen in the browser: the tile turned
    sold out with no reload).
  - **Sold out is not retired.** The tile stays in its place, struck, with
    "Sold out" in words, and cannot be tapped, so staff can tell a guest.
  - The server does not refuse ringing up a sold-out item: `AddLine` is
    not checked against the menu at all yet (STATE). The till's disabled
    tile is the only guard.
Because: a flag would need someone, or a job, to clear it every morning;
a time clears itself, and offline tills agree without asking the server.
Cost: a till whose clock is wrong ends it at the wrong time; only the back
office can mark it, not the till during service.

## ADR-110 - Photos are shrunk in the browser and stored by their hash
2026-09 · Status: accepted
Decision: phase 10, Task 2d2 gives items a photo, with no new dependency.
  - **Shrunk in the browser:** the editor draws the chosen file on a
    canvas at most 480px on its longer side, on white, and sends a JPEG
    (`web/backoffice/src/menu/photo.ts`). Canvas applies a phone photo's
    EXIF rotation, which Go's `image/jpeg` ignores, and resizing in Go
    would mean `golang.org/x/image` or a hand-rolled filter. A 2400px,
    393 KB photo arrived as 480 x 360 and 17 KB.
  - **Checked again on the server** (`internal/service/menus/photos.go`):
    at most 512 KB, JPEG only, 64 to 1024px a side read from the header
    before decoding, then a full decode. The bytes are stored as sent,
    not re-encoded.
  - **Stored by hash** in `<data dir>/photos/<sha256>.jpg`, written whole
    and renamed into place; `items.photo` holds the name (migration 00022).
    The same photo on two items is one file. Removing a photo leaves the
    file: nothing sweeps unreferenced ones yet.
  - **Served in public:** `GET /api/v1/menu/photos/{name}` needs no
    tokens, because an `<img>` cannot send them and nothing on a menu is
    private. The name must match `^[0-9a-f]{64}\.jpg$` before any path is
    built from it. Served `immutable` for a year: a new photo is a new
    name, and a till offline after its first load still shows it.
  - **Setting one is a menu edit** (ADR-106): the version bumps and tills
    show it at once (seen in the browser on the register).
  - The raw upload is `PUT` with `Content-Type: image/jpeg`; the shared
    API client now sends a `Blob` body as it is.
Because: the till needs a small, correctly rotated picture, and the
browser that chose the file is the one place that can make it cheaply.
Cost: the photos folder is not in the database, so Litestream does not
back it up (phase 12 must); an upload from a client that skips the
editor is kept at up to 1024px and 512 KB.

## ADR-111 - Stock is a ledger, in thousandths, under its own permission
2026-09 · Status: accepted
Decision: phase 10, Task 3a keeps stock as an append-only ledger
(`internal/domain/stock`, `internal/service/stock`, migration 00023).
  - **On hand is a sum, never stored:** `stock_movements` rows are never
    updated or deleted (triggers refuse it), and on hand is the sum of
    their deltas. A count records counted and the delta from the ledger,
    so a count that finds less shows as a loss someone can see, not as a
    number quietly replaced.
  - **Kinds:** received and wasted take a positive amount, wasted with a
    reason code (expired, spoiled, spilled, dropped, other with a note);
    adjusted takes a signed change and a note; counted takes what was
    counted. `sold` is in the vocabulary for Task 3b and refused by hand.
  - **Quantities are thousandths of the unit** (g, ml or each), int64:
    18.5 g is 18500, half a lime is 500. No floats, as with money. The
    unit cannot change once an item has history.
  - **A movement is recorded in one write transaction** that reads on
    hand, computes the delta and appends, so two counts at once cannot
    both use a stale figure. It records the staff member signed in and
    the open shift, or 0.
  - **`stock.manage` is a new permission,** not `menu.edit`: a manager
    who counts the walk-in should not need to be able to change prices.
    Migration 00023 grants it to roles still named owner and manager, as
    00007 did for `invoice.reprint`; new venues' manager template has it.
    The back office's Stock section now needs it (it was `menu.edit`,
    ADR-105).
  - History is cursor-paginated by id, newest first (`?before=&limit=`).
  - Low is on hand at or under the item's level; a level of 0 is never
    low, and a retired item is never low.
Because: stock is only trusted if every number on it can be explained by
the rows under it.
Cost: on hand is a SUM over every movement of an item; fine for years of
a cafe's rows, and a snapshot can be added without changing the ledger.
The open shift is found as order events find it, so a movement after the
closing drawer count has no shift (Task 3d must decide).

## ADR-112 - Stock is taken when an order closes, and reconciled, not added
2026-09 · Status: accepted
Decision: phase 10, Task 3b takes a sale's stock in the transaction that
closes the order (`svcstock.DeductSale`, called from the order service's
append hook when the appended events include `order.closed`).
  - **At the close, not the invoice.** A bill of nothing but comps totals
    zero and closes with no invoice; its food was made. Every order that
    is done closes.
  - **Reconciled:** the close works out what the order's lines use now
    and writes the difference from what its earlier `sold` movements took.
    Closing again after a reopen with the same lines writes nothing; a
    line voided after a reopen gets its stock back; one added takes only
    its own.
  - **What counts:** every line not voided, times its quantity: the
    item's recipe plus each chosen modifier's recipe. A comped line counts
    (it was made); a voided one does not (if it was made and thrown away,
    that is a waste movement). A credit note gives nothing back: food that
    went out does not come back into the store.
  - **Recipes live on items and modifiers** (migration 00024). A
    substitution is modelled on the modifiers: Dairy takes 200 ml of milk,
    Oat 200 ml of oat milk, and the latte's own recipe only what every
    latte uses. Editing a recipe needs `stock.manage`.
  - **The recipes used are today's,** not a snapshot from when the line
    was rung up: unlike money, stock is not billed, and a recipe corrected
    at 7pm should cost the 6pm tickets it closes the corrected amount.
  - The sold movements are the closing staff member's, in the open shift,
    naming the order.
Because: an order's stock is only right if it follows the order to the
end, however many times it is reopened.
Cost: a close reads every recipe of the venue (a cafe's few hundred rows).
A sold movement needs a real staff id, so an order closed with no staff
(the guest channel, phase 11) must name who it is closed for.

## ADR-113 - Recipes are kept on the Stock screen, not in the menu forms
2026-10 · Status: accepted
Decision: phase 10, Task 3c puts recipe editing on the Stock screen's
Recipes view (`docs/design/screens/backoffice-stock.md`), not on the menu
editor's item and choice forms.
  - **One permission per screen.** Recipes save under `stock.manage`
    (ADR-112); the menu forms need `menu.edit`. On the menu forms, a
    manager with only one of the two would be refused half way through a
    save, after the item had saved and the recipe had not.
  - **The dishes come from the till's menu** (`GET /api/v1/menu`), which
    any paired device may read, so a stock keeper without `menu.edit`
    sees every dish and choice on sale. Retired dishes are not listed:
    they sell nothing, so their recipes take nothing.
  - **Amounts are typed in the unit,** up to three decimals, and sent in
    thousandths (`web/backoffice/src/stock/qty.ts`), read as digits like
    money and never through a float.
  - A count shows what it will record ("The ledger says 5,000 g. This
    records −50 g.") before it is saved, as a price change does (ADR-107).
Because: the person who counts the walk-in and sets the recipes is often
not the one who prices the menu.
Cost: an owner setting up a new dish edits it in two places: the menu for
what it costs, the Stock screen for what it takes.

## ADR-114 - Closing stock is a blind count the close waits for
2026-10 · Status: accepted
Decision: phase 10, Task 3d counts chosen stock items as part of closing
the shift on the till, and shows what is low on the back office dashboard.
  - **Chosen items:** a stock item marked **Count at close** (migration
    00025) is on the till's closing list. Most venues count a few costly
    items nightly (milk, beans, spirits), not everything.
  - **Blind, as the drawer is:** the till shows names and units only; what
    the ledger says, and what the count moved, arrive with the answer to
    the count. A count that can see the expected figure tends to find it.
  - **Once per shift, whole:** `POST /api/v1/shift/{id}/stock-count` takes
    every item on the list in one transaction, as `counted` movements of
    that shift marked `closing`. A count missing an item or naming one not
    on the list is refused (`stock_count_stale`); a second count is
    refused (`stock_already_counted`), and the till reads a retry's refusal
    as the count having gone in. A closed shift is refused.
  - **It names its shift.** This settles ADR-111's open question: the
    count runs after the closing drawer count, when the order events'
    "open shift" lookup no longer finds the shift, so the shift is in the
    path, checked to be this venue's and still open.
  - **Close the shift waits for it** while there is a list to count; with
    nothing marked, there is no step. It needs `shift.manage`, as the close
    does: it is part of closing, done by whoever closes.
  - **Low stock on the dashboard,** for whoever has `stock.manage`: each
    low item, its on hand and its level.
Because: "closing stock at shift end" is only worth having if it is
honest, and only done if it sits in the close everyone already does.
Cost: a manager who must close without time to count cannot skip it (they
can untick Count at close). There is no closing-stock report yet (Task 5).

## ADR-115 - Nobody hands out what they do not hold, and someone can always manage staff
2026-10 · Status: accepted
Decision: phase 10, Task 4a lets the back office add, change, deactivate
and re-PIN staff and edit roles (`internal/service/people`,
`/api/v1/people`, `staff.manage`), under two rules checked inside each
change's transaction.
  - **Only what you hold.** The person acting must hold every permission
    of a role they create, edit, delete or assign, and of the role of
    anyone they change, deactivate or re-PIN (`auth.PermissionSet.Covers`).
    `staff.manage` alone would otherwise be every permission: a manager
    could make themselves owner, or reset the owner's PIN and sign in as
    them.
  - **Someone can always manage staff.** After the change, the venue must
    have an active person whose role grants `staff.manage`; if not, the
    change is refused (`last_staff_manager`). No screen can repair a venue
    nobody may manage. Nobody deactivates themselves.
  - **A new PIN signs the person out everywhere,** ending their open
    sessions in the same transaction: whoever knew the old one should not
    stay signed in. Deactivating needs no such step: a session already
    stops when its person is inactive (`auth.StaffAuth.Session`).
  - **Nothing is deleted but an unused role.** People are deactivated, as
    before (their names are on orders and voids); a role can be deleted
    only when nobody, active or not, holds it.
  - Role changes take effect on the next request (permissions are read per
    request); the back office shell learns them at the next sign-in
    (ADR-105).
Because: an owner hands the back office to a manager to run staff; that
must not be a way to take the owner's place.
Cost: an owner who wants a manager to manage owners must make them an
owner. A permission removed from a role takes effect at once, mid-shift.

## ADR-116 - The sign-in screen reads who works here every time it shows
2026-10 · Status: accepted
Decision: the lock screen (`web/shared/screens/LockScreen.tsx`) reads
`GET /api/v1/staff` afresh whenever it mounts (`refetchOnMount: "always"`),
on every device.
  - Found in phase 10, Task 4b's browser run: a manager added in the back
    office did not appear on the sign-in tiles of the same browser after
    signing out, so their PIN went to the wrong person as a failed
    attempt. The list was cached with the app's 30-second default and
    never read again on that screen.
  - The lock screen is exactly when someone is about to sign in, and the
    list may have changed elsewhere: added, renamed or deactivated
    (ADR-115). One small read each time it shows is the price.
  - The People screen keeps its own list (`["people"]`); it is not shared
    with the tiles, which show only active staff and only names.
Because: a person the till cannot show cannot work, and a wrong tile
spends a lockout attempt on someone else.
Cost: a sign-in screen offline shows the list it last read, as before.

## ADR-117 - The audit trail is the order events, with the bill's own figures
2026-10 · Status: accepted
Decision: phase 10, Task 4c reads every void, comp, discount, refund,
payment void and reopen from `order_events` (`internal/service/audit`,
`GET /api/v1/audit`, `reports.view`).
  - **No second copy.** The trail is a query over the append-only events
    (indexed by type and time, migration 00026), so it cannot disagree
    with what happened.
  - **The bill's own figures.** What each was worth comes from its order's
    projection: a void or comp is the line's gross, a discount its
    computed value, a refund its amount, a payment void the tender's. Each
    order on a page is projected once.
  - **Who authorised it is now kept for discounts and refunds too.** The
    API checked the manager's override for both but threw the name away;
    `DiscountApplied` and `RefundIssued` now carry `authorised_by`, and the
    state's `Discount` and `Refund` keep it (`ProjectionVersion` 6). An
    older discount or refund has none, and the trail says nothing rather
    than guess. A void, comp, payment void or reopen always had it.
  - **Filter by trading day, person and kind.** Days run 04:00 to 04:00 in
    the venue's timezone, as the shift counts them; a person matches what
    they did or what they authorised; a year at most per query. Newest
    first, cursor paged; counts per kind cover the whole filter, every kind
    present.
  - It needs `reports.view`, as the Z-report does: it is a report on the
    day's money.
Because: an owner asking "who has been voiding coffees?" needs one answer,
straight from the record, with the amounts the guest was charged.
Cost: a page projects up to its 100 orders; a filtered count scans the
range's correction events, cheap at a cafe's volume.

## ADR-118 - Range reports count through the Z-report's builder, whole, not paged
2026-10 · Status: accepted
Decision: phase 10, Task 5a reads sales over a range of trading days
(`internal/domain/report`, `internal/service/reports`,
`GET /api/v1/reports/sales?from=&to=&every=`, `reports.view`).
  - **One builder.** `zreport.BuildEach` adds up the orders into one
    report per key in a single pass; `zreport.Build` is it with the shift
    as the only key. The range report keys each event by its trading day
    (04:00 to 04:00 in the venue's timezone): the range's totals, a period
    of sales over time (day, week from Monday, or month, cut to the
    range), and a weekday by hour cell. A range's figures are therefore
    the Z-report's own rules: a sale is a bill invoiced in the range, as
    invoiced; a credit note, void, comp, payment or refund falls where it
    was recorded. A test holds a range and a Z-report over the same events
    to the same figures.
  - **The heatmap is the trading day's weekday and the clock's hour**, so
    01:30 on a Saturday is Friday night's trade, at 01:00.
  - **By time, not by shift.** An order is read when it was opened before
    the range ends and last written after it starts (migration 00027
    indexes `orders (venue_id, updated_at)`); its whole stream is
    projected, since a refund may be against a payment from before.
  - **Whole, not paged.** The phase asks for cursor pagination. These are
    aggregates: their size is bounded by the menu, the staff and the days
    asked for, never by how much was traded, so they come in one answer.
    Paging stays where rows grow with trade: the audit trail and the
    Z-report list.
  - Staff groups are named as people are named now; items and categories
    as the menu stands now, as in the Z-report (ADR-097).
Because: an owner must never see one figure on the Z-report and another
in the reports for the same day.
Cost: a range projects every order it touches, each from its first event;
a year at a busy cafe is some seconds. Stored Z-reports could answer
whole shifts faster, but not a day that splits a shift or an hour.

## ADR-119 - A report's CSV is its own figures as a table, through the Tally writer
2026-10 · Status: accepted
Decision: phase 10, Task 5b serves every report of Task 5a and the audit
trail as CSV (`GET /api/v1/reports/sales/csv?from=&to=&every=&report=`,
`GET /api/v1/audit/csv?from=&to=&staff=&kind=`, `reports.view`).
  - **One CSV writer.** A report becomes an `export.Table`, the header and
    rows the shift's Tally files already use, and `Table.CSV` writes it
    (RFC 4180, CRLF). `export.Decimal` is now exported for the amounts.
  - **A sheet per report.** `report.Result.Table(sheet)`: summary,
    over_time, items, categories, channels, staff, payments, taxes, hours.
    It is built from the same `Result` the JSON answers with, so a file
    cannot disagree with its screen.
  - **Money in major units, the currency in the header.** A cell is
    "1285.50"; the column is "Net (INR)". "Named once" is read as once per
    column, never per cell: a file that is renamed, or a column pasted
    elsewhere, still says what its numbers are.
  - **Days are trading days.** Sales over time gives each period's first
    and last trading day; the heatmap names the trading day's weekday; an
    audit row has its trading day and its time on the venue's clock, so a
    void at 01:30 sits under the night it belongs to.
  - **The audit file is the whole filter, not a page.** `audit.Table`
    walks the trail's own cursor, a hundred at a time, projecting each
    order once across the pages. `before` and `limit` are not read.
  - **Typed text cannot run as a formula.** `export.Text` puts an
    apostrophe before a cell that starts with =, +, - or @ (an item's
    name, a reason, a person's name). Amounts are not passed through it.
  - **JSON, not the file.** As with a shift's exports (ADR-104) the answer
    is `{filename, content_type, text}`, read by the signed-in client and
    saved by the screen. The text opens with a UTF-8 byte order mark:
    without one Excel reads the file in the machine's code page and a
    Devanagari item name arrives as noise.
Because: an owner hands these to an accountant, who opens them in a
spreadsheet; the numbers must sum there and match the screen.
Cost: a sheet reads the whole range again, as the JSON does; nine sheets
are nine reads. The Tally files are not passed through `export.Text` and
carry no byte order mark: they are written at the close and Tally's
import has not been tried with either.

## ADR-120 - The Sales reports screen is one screen of sections, its bars plain elements
2026-10 · Status: accepted
Decision: phase 10, Task 5c builds `/sales` (`backoffice-reports.md`,
`web/backoffice/src/sales`).
  - **One screen, one range.** The headline, sales over time and the
    payment and channel mix are sections under one range picker; Tasks 5d
    to 5f add their sections to it, not screens of their own. One read of
    `GET /api/v1/reports/sales` feeds every section.
  - **The cut follows the range until chosen.** A day up to 31 days, a
    week up to 26 weeks, a month beyond; choosing Day, Week or Month by
    hand holds from then on.
  - **Bars without a library.** A bar is an element whose height is its
    share of the tallest period, the one inline style on the screen: a
    height that is data has no class. Slate, the neutral data series; no
    accent. The same periods are a table under the bars, so the bars are
    hidden from screen readers and no figure is only a picture.
  - **Three things are worked out in the browser:** a bar's height, a
    share as a whole percentage of the listed rows (so shares may sum to
    99 or 101), and the average bill, the total over the bills to the
    nearest minor unit. Every other figure is the server's. A share is
    not shown when a row is negative or the sum is not positive.
  - **A download is the section's own sheet** (ADR-119), saved through
    `save-file.ts`, which the shift exports now share.
Because: an owner compares a week at a glance and then wants the same
numbers as a file; a chart library would be the back office's largest
dependency for seven rectangles.
Cost: the average bill is not in the CSV, since the server does not
compute it. A year by day is 366 thin bars; the cut defaults away from
that but does not forbid it.

## ADR-121 - What sold and when: the server's rows, sorted in the browser, and a heatmap that is a table
2026-10 · Status: accepted
Decision: phase 10, Task 5d adds four sections to the Sales screen
(`web/backoffice/src/sales/SalesTables.tsx`): product mix, category
performance, staff performance and the hour by weekday heatmap.
  - **No new read.** They are parts of the `SalesReport` the screen
    already holds (ADR-118, ADR-120); only a download asks again.
  - **Sorted in the browser.** These tables are whole, not paged
    (ADR-118), so a header reorders the rows already here: by name,
    quantity or total, opening by total, largest first. `DataTable`'s
    sort props were built for a server that sorts a page; here the owner
    of the rows sorts them. No figure changes, and a share stays a share
    of the whole table.
  - **The heatmap is a table, and the count is the figure.** A row per
    weekday, all seven; a column per hour from the first with a bill to
    the last. Each cell prints its bill count over a slate tint in four
    steps by its total; a screen reader hears the cell in words. Colour
    alone would break the rule the terminal lives by, and a back office
    reader may be colour-blind too.
  - **Hours run as the trading day does,** 04:00 first and 03:00 last,
    so 01:00 sits to the right of the evening it belongs to, on the
    weekday the server already gave it.
  - Staff performance shows an average bill, worked out as the headline's
    is (ADR-120).
Because: "what sells, who sells it, and when are we busy" is the
question after "how did the week go", and it should not cost a second
screen or a second request.
Cost: a CSV is in the server's order, not the order on screen. The tint
compares totals while the cell prints bills, so a cell of few large
bills is dark with a small number; the hover and the Busiest line give
both.

## ADR-122 - The corrections analysis is the audit trail's own entries, grouped
2026-10 · Status: accepted
Decision: phase 10, Task 5e1 adds up corrections over a range
(`internal/service/audit/analysis.go`,
`GET /api/v1/audit/analysis?from=&to=&staff=&kind=`,
`GET /api/v1/audit/analysis/csv?...&by=reason|by|authorised_by`,
`reports.view`). Task 5e was split in two: this, then its screen (5e2).
  - **The trail's entries, not a second query.** The analysis reads every
    page of the same filter the trail lists (ADR-117) and groups the
    entries, so a count here is the number of rows the owner finds when
    they open the trail on that filter. It takes the trail's filter
    whole: range, person and kind.
  - **Four groupings, each a count and an amount.** By kind (every kind,
    none included); by kind and reason; by who did it; by who authorised
    it. A person's row also has every kind's tally, so a screen need not
    show only the blended amount. Nobody on record is its own row: a
    guest as who did it, "not recorded" as who authorised it (a discount
    or refund from before ADR-117).
  - **An amount is what each was worth to its bill,** as the trail has
    it: a void or comp its line's gross, a discount its value, a refund
    or payment void the money. A reopen counts and is worth nothing.
    Sums across kinds are therefore "the value corrected", not money out
    of the drawer.
  - **Largest first,** by amount, then count, then a fixed order, so the
    answer does not shuffle between two reads.
  - **In the audit package, not the sales report.** It is about who and
    why, lives beside the trail it links to, and shares its filter; the
    sales report's own discounts, comps and voids tallies stay as they
    are.
  - An amount in another currency than the venue's is refused, never
    summed.
Because: "who is voiding, who is letting it through, and what reason do
they give" is the trail read at a distance; it must agree with the trail
read up close.
Cost: it projects every order the range's corrections touch, as the
trail's CSV does; a year at a busy venue is some seconds. A sheet reads
the trail again.

## ADR-123 - The corrections section links down to a trail that opens on its address
2026-10 · Status: accepted
Decision: phase 10, Task 5e2 puts the corrections analysis (ADR-122) on
the Sales screen (`web/backoffice/src/sales/SalesCorrections.tsx`) and
lets the Audit screen open on a filter.
  - **A section with its own read.** It asks
    `GET /api/v1/audit/analysis` for the screen's range apart from the
    sales report, so the sales show while the corrections add up, and
    one failing does not blank the other. It follows the range only: the
    person and kind filters stay the audit screen's.
  - **Every row is a way down.** A kind links to
    `/audit?from=&to=&kind=`, a person to `&staff=`. A kind with none, a
    guest, "not recorded" and someone no longer on record link nowhere.
    Reasons are not links: the trail has no reason filter.
  - **The Audit screen reads its address once, on opening**
    (`filterFromSearch`). An address is typed by anyone, so a day that
    is not one, a range backwards, or a kind we do not have falls back
    to the default and is never sent. Changing a filter afterwards does
    not rewrite the address: the link is a way in, not a second copy of
    the filter to keep in step.
  - **A person's link may show more than their count.** The trail finds
    a person by what they did or authorised (ADR-117); the two tables
    split those. The screen says so under them and does not pretend the
    link is exact.
  - The Download CSV button became `useCsvDownload`, shared by the sales
    sections and this one; each caller downloads one file at a time.
Because: the analysis answers "who and why" at a distance; the next
question is always "show me those", and it should be one click.
Cost: back from the trail returns to the Sales screen's default range,
since its range is not in its address either. A deactivated person is
not in the Person list (active staff only), so the box says "Someone no
longer active" for a link to them, without their name.

## ADR-124 - Stock has a cost per kilogram, litre or each, and the ledger keeps it as it was
2026-10 · Status: accepted
Decision: phase 10, Task 5f1 gives a stock item a cost (migration 00028,
`internal/domain/stock/cost.go`, the Stock screen's item form). Task 5f
was split in three: this, the closing stock report (5f2), its section
(5f3). The phase asked for a variance "at cost" and stock had none.
  - **What an invoice says.** A cost is minor units of the venue's
    currency for a kilogram of an item counted in grams, a litre of one
    in millilitres, or one of an item counted by the piece. A supplier
    prices milk by the litre, not the millilitre, and the form asks for
    what the owner can read off the bill. Zero is "not known", not free.
  - **One rule for worth.** `stock.Value(qty, unit, cost)` is the
    quantity (thousandths of the unit) times the cost over the measure,
    in whole numbers, a half rounding away from zero; a shortfall is
    negative. Nothing else turns stock into money.
  - **The ledger keeps the cost it was written at.** Every
    `stock_movements` row has `unit_cost`, filled by the insert itself
    from the item's cost at that moment: no caller passes it, so none
    can forget it or pass another. A cost changed later is for stock
    moved from then on. Valuing last month's variance at today's price
    is the stock form of "a historical total from current menu prices".
  - **In the venue's currency or refused.** The item carries its cost as
    a `Money`; an input in another currency is refused, not converted.
    The stock list says which currency that is, for a form with no item
    to read it from.
  - Purchasing stays out of scope: there is no cost per delivery, no
    average or FIFO cost. The cost is one figure the owner keeps current.
Because: "we were short 2 litres" becomes a decision only when it reads
"and that is 128.00".
Cost: rows written before migration 00028 have no cost and value at
zero; a report must say "not known", not 0.00. An owner who never
updates a cost gets variances at an old price, shown as the price it was.

## ADR-125 - Closing stock is reported by shift, each variance at the cost it was counted at
2026-10 · Status: accepted
Decision: phase 10, Task 5f2 reports the closing counts over a range
(`internal/service/stock/closing_report.go`,
`GET /api/v1/reports/closing-stock?from=&to=`, `.../csv`, `reports.view`).
  - **By the shift's trading day.** A closing count belongs to its shift
    (ADR-114), and a shift to the trading day it opened on, which is
    stored. The range picks shifts by that day: no timezone arithmetic,
    and a count made at 01:00 sits with the night it closed.
  - **Expected is derived, here only.** A count stores what was found
    and the change it made; expected is counted less that change.
    Variance is the change: below zero is short.
  - **At the cost on the count** (ADR-124), through `stock.Value`. A
    count with no cost has no cost and no value in the answer and empty
    cells in the file, never 0.00. Each total says how many of its counts
    had none (`uncosted`), so "23.60 lost" is never read as the whole of
    it when half the items were not costed.
  - **Two views of the same rows.** By shift, oldest first, each with its
    lines; and by item over the range, the largest loss first, which is
    where "milk is short every night" shows.
  - **reports.view, not stock.manage.** It is mounted with the stock
    routes, which own its service, but read by whoever reads the
    Z-report. The CSV is a row per item per shift, quantities in whole
    units.
  - Whole, not paged, as the range reports are (ADR-118): bounded by the
    shifts in at most a year and the few items counted nightly.
Because: the close already makes staff count; the owner needs to see
what the counts found, in money, without opening each shift.
Cost: items are named as they are named now. A shift with no closing
count is not listed, so a night nobody counted looks like no night at
all; the screen should say what it lists.

## ADR-126 - The closing stock section says which way, and what its total leaves out
2026-10 · Status: accepted
Decision: phase 10, Task 5f3 puts the closing stock report (ADR-125) on
the Sales screen (`web/backoffice/src/sales/SalesClosingStock.tsx`). It
finishes Task 5f, and with it Task 5.
  - **A section with its own read,** as Corrections has (ADR-123): the
    last on the screen, following the screen's range.
  - **The headline is words.** "Short by 23.60", "Over by 6.40" or
    "Counted as expected", with the danger, warning or success colour
    behind the words and never in place of them. The amount is shown
    without a sign: the word says which way.
  - **"Not costed" is never 0.00.** A count with no cost reads "Not
    costed" where its cost and value would be. A total that skipped
    counts says how many under it; when no count had a cost the headline
    gives no figure at all and says why.
  - **Over is a warning, not good news.** More stock than the ledger
    expected means a delivery not recorded or a sale not rung up, so it
    is not painted as a success.
  - Two tables from the one answer: by item over the range, largest loss
    first, and a row per item per shift. One CSV, the server's.
Because: an owner reading "23.60" needs to know it is a loss and that
two items are missing from it before deciding anything.
Cost: the by-shift table repeats the day on every row and has no
subtotal per shift, though the answer carries one. Only shifts whose
close counted stock are listed, and the section says so.

## ADR-127 - The product is renamed mez, all the way down
2026-10 · Status: accepted
Decision: Nofiltr POS becomes **mez**, written lower case as its wordmark
is. The rename is whole, not a display name over an old core:
  - Binary `mez`, `cmd/mez`, module `github.com/byjoellobo/mez`.
  - Environment `MEZ_*`; database `mez.db`; log `mez.log`; the default
    data directory ends in `mez`.
  - Browser storage keys `mez.<app>.*` and the stream's subprotocol
    `mez.v1`, changed on both sides in the one commit.
  - Device and staff tokens keep `nfd_` and `nfs_`: they are opaque
    prefixes, not the name, and nothing reads them as words.
  - Entries above this one, finished phase files and the archived design
    reference still say Nofiltr. They are history and are left as written.
Because: the owner renamed the product with its new design language, and
nothing is released, so there is no installed venue to migrate.
Cost: a data directory from before this commit is not found: move it, and
rename `nofiltr.db` (with its `-wal` and `-shm`) to `mez.db`. A paired
browser must pair again. The module path names a GitHub repository that
is still called `nofiltr-pos` until the owner renames it.

## ADR-128 - The mez design language replaces ink and vermilion
2026-10 · Status: accepted. Supersedes the palette, type and radius of
ADR-014 and ADR-062; their rules on text floors and roles stand.
Decision: the look moves to the design in `docs/design/reference/mez/`,
written down in `docs/design/tokens.md`. Looks only: no screen gains or
loses a function.
  - **Roles held, values changed.** Components name roles, so the change
    is `tokens.css`, the preset, the fonts and the shared primitives.
  - **Paper everywhere, night in the kitchen.** The till leaves the dark
    ground; `data-surface="ink"` now means espresso night and is set for
    `/kitchen` alone.
  - **Two voices.** `accent` is green, which commits. A new `attention`
    role is terracotta: active navigation, links, a new `attention`
    button. Brick is danger.
  - **Archivo stays for numbers.** Fraunces and DM Sans as shipped have
    no tabular figures, so `font-mono` is Archivo, the numeral face.
  - **The floors stay.** 11px metadata, 13px sentence text, 15px
    interactive labels and 16px controls outrank the design's smaller
    buttons, for the reasons ADR-014 gave.
  - **The logo is recoloured** to the sheet's green and mustard, in
    `web/shared/brand/`, and drawn inline by `Logo`.
  - A table's five states keep five tones; "to clear" is now the
    cleaning grey and "ordered" the occupied rose.
Because: the owner chose the new language with the new name.
Cost: the screen specs in `docs/design/screens/` still describe the old
layouts and cite the old archive. Screens are restyled through the
primitives only so far; `docs/phases/redesign.md` lists what each still
needs. `tokens-demo.html` shows the old palette. A light till under
service lighting is untested by anyone on a shift.

## ADR-129 - Settings is split by write path, and backups wait
2026-10 · Status: accepted.
Decision: phase 10's Task 6 is twelve sub-tasks, 6a to 6l, in
`docs/phases/phase-10.md`.
  - **Server first, then screens**, as Tasks 2 to 5 went: 6a to 6f are
    write paths with Go tests, 6g to 6j the screen over them.
  - **Split where the rules differ, not where the form does.** Tax,
    invoice numbering and the trading day each carry a rule that costs
    money when wrong, so each is its own server task.
  - **One write path for the `settings` table** (6a) that knows every
    key and its type. Today a bad value is found when a shift is counted
    or tips are pooled (`ErrVenueConfig`); it should be refused on save.
  - **Printers, station mapping and the floor plan are not rebuilt.**
    They exist; 6e and 6j add only the stations themselves.
  - **Currency is shown, never edited** (`UpdateVenue` already leaves it
    out).
  - **Backup configuration (6l) is parked last.** Litestream arrives in
    phase 12, Task 4; a form before then would store settings nothing
    reads, and where the bucket's credentials live is that task's call.
Because: the task lists eleven areas; three have no server at all.
Cost: phase 10 cannot be ticked until 6l is built or moved, and its
done-when does not need backups. The owner decides which (STATE.md).

## ADR-130 - Settings: a key registry, and no generic write route
2026-10 · Status: accepted.
Decision: `internal/service/settings` holds every key the code reads from
the `settings` table, each with its default and a check of its value alone.
`Set` refuses an unknown key or a bad value and saves all or none; an empty
value clears a key back to its default.
  - **No HTTP route writes keys yet.** Only venue details are editable over
    HTTP (`PUT /api/v1/settings/venue`). A generic key write would let a
    client save a GST profile's state code and not its registration id.
    Tasks 6b to 6d each add a route that checks their keys together, then
    calls `Set`.
  - **`GET /api/v1/settings`** lists every key with its value, whether it
    is set, and its default, beside the venue. **`GET /api/v1/venue`**
    gives a paired device the name alone, for the lock screen.
  - **Defaults are stated twice**: in the registry, for display, and where
    each key is read (`order/venue.go`, `shift`). A test checks each default
    passes its own check; nothing checks the two agree. 6d, which moves the
    day start into a setting, should make the readers use the registry.
Because: a bad value should be refused when saved, not when a bill is paid.
Cost: a setting changed by hand in the database still fails late, as before.

## ADR-131 - Tax profile and tax classes
2026-10 · Status: accepted.
Decision: the tax profile is edited as one unit (`PUT /api/v1/settings/tax`),
and tax classes get create, edit, retire and restore
(`/api/v1/settings/tax-classes`), all `settings.edit`.
  - **One unit, checked by the profile itself.** The profile, registration
    id, pricing, GST state code and US jurisdiction are written in one
    transaction. Each profile's own `Compute`, run on no lines, refuses a
    context it could not sell under (a GSTIN in another state than the state
    code), and the sale path's own reading of the venue runs on what was
    just written; either refusing rolls it back. What a profile does not
    use is cleared, so switching leaves no stale state code or rates.
  - **A GST state code left empty is the GSTIN's first two digits.**
  - **US sales tax is always exclusive**, and says so when asked for
    inclusive rather than ignoring it.
  - **A change of profile is refused while any ticket is not closed**
    (`tickets_open`): its lines would be taxed under one regime and
    settled under another. Correcting a GSTIN or a rate under the same
    profile is allowed.
  - **A profile that prints HSN does not refuse items without one.** It
    lists the active items with no valid code (`items_without_hsn`); they
    keep selling and their invoices fall short until each is edited.
  - **Tax classes can be retired** (migration 00029, `active`). A class
    that active items charge cannot be retired, and the refusal names
    them. An item cannot be created on, edited onto or brought back onto
    a retired class. Names are unique ignoring case, retired included.
    An exempt class must be at 0%.
  - **A rate change is for new lines only**, as an item's price change
    is: a line copies its rate when rung up. Every class edit bumps the
    menu version and nudges the stream, through `EditMenu`.
Because: a wrong tax profile costs the owner a filing, and one that only
fails when a bill is paid costs a customer a wait.
Cost: no screen yet (6h). Invoice numbers already issued under a profile
are not touched by changing it; a venue that switches mid financial year
keeps one series. Whether it should start another is 6c's question.

## ADR-132 - Invoice numbering: what may change, and when
2026-10 · Status: accepted.
Decision: `GET` and `PUT /api/v1/settings/numbering` (`settings.edit`) edit
the invoice and credit note series codes and prefixes, the shared number
template, the financial year start month and the cash rounding step. Saved
in one transaction, then read back as a sale reads it and checked.
  - **The year must be in the number, and so must the prefix.** Counters
    restart per financial year, so a template without `{fy}` repeats last
    year's numbers; invoices and credit notes share a template (ADR-083),
    so one without `{prefix}`, or with the same prefix on both, gives an
    invoice and a credit note the same number. `invoice.CheckNumbering`
    refuses all three, as well as one code for both series.
  - **The financial year start is fixed once any number is issued**
    (`numbering_locked`). Numbering restarts on that date; moving it
    under issued numbers is an accountant's migration. Before the first
    sale it moves freely. Series code, prefix and template may change at
    any time: a prefix or template change continues the same counter, a
    new code starts a new run at 1 (and going back to an old code
    continues its old counter).
  - **A series must not write a number already issued.** After saving, the
    next number of each series is looked up in `invoices` and
    `credit_notes`, and a hit is refused (`number_taken`): renaming INV
    to NF and keeping the prefix INV would write INV/26-27/000001 again
    and the unique index would refuse the sale at the till. Only the next
    number is checked; a template change that makes a later number meet
    an old one is not.
  - **A prefix or code equal to its default is not stored**, so a prefix
    keeps following its code. The registry's defaults now come from
    `service/order`, the same constants the sale path uses; ADR-130's
    note on the two copies drifting is closed for series and template.
  - **The cash rounding step is saved and checked** (1 up to one whole
    unit of the currency) **but nothing applies it yet.** It is a venue
    column no sale or payment reads.
  - **Series codes are capitals and digits**, uppercased on the way in.
Because: a repeated invoice number is a compliance incident and a refused
sale; both are cheaper to refuse here.
Cost: a venue that mis-set its financial year after trading cannot fix it
from the back office.

## ADR-133 - Shift and tip rules: one route, and the day start as a setting
2026-10 · Status: accepted.
Decision: `GET` and `PUT /api/v1/settings/rules` (`settings.edit`) edit the
trading day's start, the variance threshold, the tip rule and its role
weights, and the minutes before a ticket is late. The values are checked
together, then saved through `settings.Service.Set`, all or none.
  - **The day start is a venue key, `shift.day_start`, written "04:00"**
    (24 hour, two digits each side; `shift.ParseDayStart`). Unset is
    `shift.DefaultDayStart`. `shift.DayStart(ctx, q, venueID)` reads it,
    and a value that is not a time is `ErrVenueConfig`, like the other
    shift keys. Every place that used the constant reads it now: a shift
    opening (the `Options.DayStart` option is gone), the range reports,
    the audit trail and its file, and "sold out until tomorrow".
  - **A change reaches the next shift opened.** A shift stores its trading
    day when it opens and never changes it, so the one running keeps its
    day. The reports, the audit trail and the CSV place events by their
    time, as they are asked: after a change they bucket the past by the
    new start too, so a range read before and after can differ for events
    between the two times. Sold out keeps the time it was marked with.
    The response carries `open_shift_day` so the screen can say so.
  - **Weights must name real roles**, by exact name (that is how a pool
    looks them up), each from 1 to 100. The rule `role_weight` with no
    weights is refused: every role would weigh 1 and the rule would be
    `even` under another name. Weights saved while the rule is another are
    kept. They are stored with sorted keys, so the same weights are the
    same text.
  - **A value equal to its default is not stored**, as in ADR-132, so the
    key reads as unset. The registry's defaults for these keys come from
    the readers' own constants (`order.DefaultLateMinutes`,
    `shift.DefaultVarianceThreshold`, `tips.Even`,
    `shift.DefaultDayStart`): the registry imports the services, so the
    readers cannot import it, and the constants are the one copy.
  - **One route, not four**, as the numbering route is one: the five values
    are saved together or not at all, and the screen (6i) is one section.
  - **Reading refuses a stored value nothing could use**, as numbering
    does; a save through the route still repairs it.
Because: the day start decided where a late night's sales landed, and the
only way to change it was a rebuild; the others were settings with no way
to set them but the database.
Cost: a renamed role orphans its weight (the pool reads the old name and
ignores it; no check runs on a role rename). Re-bucketing the past on a
day start change, as above.

## ADR-134 - Stations and courses: retiring says where things go
2026-10 · Status: accepted.
Decision: stations and courses are created, renamed, put in order, retired
and restored under `/api/v1/menu/stations` and `/courses` (`menu.edit`),
through `EditMenu`, so each edit bumps the menu version and tills hear of it.
Migration 00030 gives courses an `active` flag, as stations and tax classes
have; the till's menu reads active courses only.
  - **A station's code is chosen once.** It is what a line snapshots and what
    the kitchen stream routes on, so the form sends it only when the station
    is made (empty takes one from the name: "Juice Bar" is `juice-bar`) and a
    rename changes the name alone. A code a retired station holds is still
    taken.
  - **Retiring names a destination, `move_to`.** A station with any item
    (retired items too, so one brought back never names a station out of
    service) or a printer must say which station in service takes them; a
    course with any item, the same. A station or course that carries nothing
    retires with no body. The answer says what moved (`items_moved`,
    `printer_moved`). The printer follows to the new station when that has
    none; when it has its own, the retired station's printer is freed and the
    other's stays. Either way the retired station prints nowhere.
  - **A venue keeps one station and one course in service.** Items need both,
    and the till reads a menu with neither as one nobody can sell from.
  - **Restoring moves nothing back.** What was moved off stays where it went.
    Bringing an item back, or making one, refuses a retired station or course,
    as it already did a retired tax class: the menu read fails on an item
    naming a course it does not have.
  - **Courses are numbered by their place, and renumbering waits for the
    floor to be clear.** Lines copy their course's number and the kitchen
    fires by number, so putting courses in a new order is refused while any
    ticket is unsettled (`tickets_open`); an order that changes no number is
    always accepted. Adding, renaming and retiring need no such wait. Retiring
    leaves a gap in the numbers (1 and 3 in service) rather than renumbering
    the rest under open tickets; the next ordering closes it.
  - **Open tickets are untouched.** Tickets already sent to a retired station
    stay on its board until done, and a display set to it keeps showing it.
Because: a station with no way to leave service is clutter that stays on every
form for good, and one that could leave carelessly would drop a kitchen's
items off the till or its tickets onto no printer.
Cost: a retired station's display needs setting to another by hand; the
numbering gap after a retire; the item form (web) now filters retired
courses as it does stations, but nothing on a screen manages them yet (6j).

## ADR-135 - The receipt template: lines the venue adds, parts it may leave off
2026-10 · Status: accepted.
Decision: `GET` and `PUT /api/v1/settings/receipt` (`settings.edit`) edit what a
venue adds to its customer documents: up to four header lines (under its name
and registration), up to four footer lines, and three parts it may hide: the
invoice number's barcode, the order's id under the date, and the choices
under each line. Saved through `settings.Service.Set`, all or none. The
receipt, a bill printed before payment and a credit note read it when they are
queued (`printing.templateFor`, inside the transaction that queues them).
  - **Only what the tax profile does not require is optional.** The seller's
    registration, the number, the date, place of supply, HSN, the tax lines and
    the total are not in the template and cannot be turned off.
  - **The zero template is the receipt as it was.** The three switches are
    `hide_*` in the JSON, the layouts and the keys (`receipt.hide_barcode`,
    ...; "true" hides, anything else shows), so a request that leaves one out
    cannot take a part off a receipt. The table number stays when the order id
    is hidden.
  - **Printing is ASCII, so text the printer would turn into "?" is refused
    when saved** (`escpos.Unprintable`, the same table `Printable` prints
    with), naming the line and character. Accents and the rupee sign pass:
    they print spelled out ("Cafe", "Rs."), as they do on item names
    (ADR-079). Lines are at most 48 characters, one full 80mm line; on 58mm
    they wrap.
  - **Printing never refuses a sale over it.** The reader (`TemplateOf`)
    cannot fail: a switch that is not "true" is off, at most four lines are
    read, text that will print as "?" prints as it will. Only the settings
    write path is strict, so a hand-edited value costs a "?" and not a payment.
  - **A pasted block is its lines.** A newline inside a line splits it, and the
    four-line limit applies to the result; blank lines are dropped. Lines are
    stored joined by a newline, one key each for header and footer.
  - **A credit note takes the header and footer** (its seller block is a legal
    document too) **but not the switches:** it has no choices or order id of
    its own, and its barcode is what a later refund scans.
Because: a cafe's address, phone number and "thank you" are the first thing
asked of a receipt, and they must not be a way to print a receipt that
misses what the tax authority requires, or a reason a payment fails.
Cost: no logo (a raster path is later work, ADR-079); the till's on-screen
invoice preview does not show the template; changing the template changes
the next duplicate of an old receipt as well, since a duplicate is laid out
when it is asked for.

## ADR-136 - The venue settings screen: a frame, and a section that saves alone
2026-10 · Status: accepted.
Decision: `/settings` is a frame (`web/backoffice/src/settings/`): a rail of
sections beside the one open, at `/settings/<section>`. Each section keeps its
own draft and saves on its own, in the one footer every section shares
(`SectionCard`): Discard and Save, both disabled until the draft differs;
"Unsaved changes", then "Saving", then "Saved" in words in one live region; a
refusal about a field under that field, one about no field in the footer.
Spec: `docs/design/screens/backoffice-settings.md`. The Venue section edits
the name, legal name and timezone through `PUT /api/v1/settings/venue`, and
shows the currency, which is never edited.
  - **One Save per section, not one for the screen.** The archive's Settings
    has one Save at the very end. The sections here write through different
    routes with different checks (tax refuses while tickets are open, numbering
    locks the financial year), so one button over all of them could save some
    and refuse the rest, with a screen showing neither. A save sends one
    section and answers with that section as it now stands, which becomes the
    draft: what is on screen after Save is what the server holds.
  - **Leaving a dirty section asks, in the frame.** A rail entry asks "Leave
    without saving?" (Keep editing, Leave); the browser asks before a tab
    closes or reloads. The shell's own navigation does not ask: a guard on
    every back office link is a cross-cutting change to the shell, and a
    draft of a name or a timezone is cheap to retype. The frame knows only
    whether the open section is dirty; the section reports it.
  - **A section is a path** (`/settings/venue`), so reload and a pasted link
    keep the place, and the shell's `/settings` entry stays current across
    them. `/settings` and a path that names no built section go to Venue.
    Sections not built are shown muted with Soon, not as links, the shell's
    rule, so the rail already shows what 6h to 6l add.
  - **The client checks what is free to check** (a name given, lengths,
    a timezone given) so a refusal is instant; the server checks again and its
    words win for anything else, such as a timezone that is not a zone.
  - **The timezone is a text field with the browser's zone list as
    suggestions** (`Intl.supportedValuesOf`, a short list where it is
    missing), not a select: 400 names is not a menu, and the server decides
    what is a zone.
Because: the settings are the one place an owner can break the venue, and a
screen that says plainly what is saved and what is not is most of what stops
that.
Cost: the leave prompt cannot be seen until a second section exists (it is
only reachable from the rail); the shell's navigation drops a draft silently.

## ADR-137 - Tax and numbering on the screen: a preview the server works out
2026-10 · Status: accepted.
Decision: the Tax and numbering section (`/settings/tax`) is three cards: the
tax profile and the invoice numbering, each a draft saved on its own through
`SectionCard`, and the tax classes, each change saved at once. Spec:
`docs/design/screens/backoffice-settings.md`. Two additions outside the screen:
`POST /api/v1/settings/numbering/preview`, and `/menu?item=<id>`.
  - **The preview is the server's, not a second template engine in the
    browser.** `PreviewNumbering` takes a draft numbering and answers with the
    next invoice and credit note numbers it would write, running the same
    checks a save runs (`cleanNumbering`, the series build, `CheckNumbering`,
    the issued-number check) and writing nothing. A draft a save would refuse is
    refused with the same field and words, which the screen shows as "No preview
    yet" until it is fixed. The browser asks 350ms after typing pauses and drops
    an answer for a draft that is no longer the one on screen. It is the one
    place a financial year start that a save would refuse (numbers issued) is
    not refused: the screen disables that field beside its own reason.
  - **The items without an HSN are listed from the save's answer.** The server
    reports them for the profile it holds, so the list belongs to the saved
    profile and is hidden while the draft is another (`showWithoutHSN`). A
    change of profile therefore shows what it leaves owing as soon as it is
    saved, not before: working it out for a draft would be a second computation
    of "what does this profile require" that the tax package already owns.
    Each item links to `/menu?item=<id>`; the menu editor reads the query once,
    when its data arrives, selects the item's category and opens its form. An id
    the menu lacks, or anything else in the query, opens nothing.
  - **A prefix keeps following its code.** The server stores a prefix equal to
    its code as unset (ADR-132), so the form does the same while typing: a code
    typed carries the prefix with it while the two were the same, and stops once
    the owner writes a prefix of their own.
  - **Rates are typed as percent, sent as thousandths of a percent** (ADR-049),
    by string arithmetic, never a float. The cash rounding step is typed in
    major units ("0.50") and sent in minor units; its range comes from the
    currency's largest step the server reports, so no currency's decimals are
    written into the client.
  - **A refusal names the server's keys, the form shows its inputs.** A shared
    `problemWith` sends a refusal about a field the form does not have to the
    section's footer, so nothing is refused silently.
  - **A retire that is refused names at most eight items** and how many more
    (`namesLine`): the full list on a menu of hundreds is not a sentence.
Because: the numbering and tax settings are where a venue most easily breaks its
own invoices, and the point of the screen is that the owner sees the number
they are about to create before they create it.
Cost: the preview and the screen each ask the server per change of draft (cheap,
local); the screen's draft state for the profile is lost when the owner leaves by
the shell's navigation, as for the venue section (ADR-136).

## ADR-138 - Shift, tips and receipt on the screen: the preview is the printer's own layout
2026-10 · Status: accepted.
Decision: the Shift, tips and receipt section (`/settings/shifts`) is two cards,
each a draft saved on its own through `SectionCard`: the shift and tip rules
(ADR-133) and the receipt template (ADR-135). Spec:
`docs/design/screens/backoffice-settings.md`. The receipt card carries a preview
of a sample receipt at the printer's width, which the server lays out.
  - **The preview is rendered by the server, from the real layout.**
    `POST /api/v1/settings/receipt/preview` takes a draft template and a paper
    width, builds a made-up paid bill under the venue's real name, registration,
    tax profile and next invoice number (`escpos.SampleReceipt`), lays it out
    with `Receipt.Layout` at 48 or 32 columns, and replays the bytes as paper
    (`escpos.Replay`). The screen only draws the lines it is sent. The
    alternative, a second layout in the browser, would drift from the printer
    the first time a line wrapped differently or `Printable` spelled a letter
    another way; this way what the owner sees is what is printed, including the
    wrapping at 58mm and the refusal of text the printer cannot print.
  - **`Replay` is the layout tests' decoder, promoted.** The decoder that checked
    every layout in tests became `escpos.Replay`, extended to track alignment and
    character height and to report a barcode as a line of its own; the tests now
    call it, so the preview and the tests read a stream the same way, and a
    command the package does not know is an error in both. It returns an error
    where the test decoder called `t.Fatalf`.
  - **The preview runs a save's checks.** The template is checked as `Set` checks
    it (line count, line length, characters the printer cannot print), so a
    draft a save would refuse is refused with the same field and words, shown
    as "No preview yet:". The sample is for the receipt only; a credit note takes
    the same header and footer but is not drawn.
  - **The drawer threshold is typed in major units.** The rules response carries
    `minor_per_major` (100 for rupees, 1 for yen), the same count the numbering
    screen uses for its rounding step; `majorText` and `parseMajor` are shared by
    both, and no currency's decimals are written into the client.
  - **Weights are a list of the venue's roles.** The form has a row per role the
    server names in `roles`; a weight stored for a role that is gone is not shown
    and drops at the next save, since the server refuses one (the cost noted in
    ADR-133). Tips shared by role need a weight for at least one role, as the
    server requires.
  - **One preview hook for both screens.** `usePreview` generalises the
    numbering preview: asked once typing pauses, an answer for a draft that is no
    longer on screen is dropped, and the last good answer stays up while the next
    is worked out, so a receipt does not flicker to a message on every key.
Because: the receipt is the one document the guest takes away, and the owner
should see its wrapping, its barcode and its footer before the first one prints.
Cost: the preview asks the server on every pause in typing (local, cheap); the
sample's figures are made up and the preview does not show a credit note; the
screen has not been seen against a thermal printer, only against the layout.

## ADR-139 - Stations and courses live in the menu editor, not in settings
2026-10 · Status: accepted.
Decision: stations and courses are managed in a third view of the menu editor,
"Stations and courses" (`/menu?view=setup`), beside Items and Choices. The phase
plan put stations on the Printers screen and courses in the settings frame
(ADR-129, written before the routes existed); the routes are `menu.edit`
(ADR-134), so they are drawn where `menu.edit` is the gate. The settings rail
loses its "Stations and courses" entry, which will never be built.
  - **The screen's permission follows the route's.** Printers and the settings
    frame need `settings.edit`; the station and course routes check `menu.edit`.
    A role with one and not the other would have been shown controls that answer
    "only someone who can change the venue's settings", or been refused a screen
    it could use. Only the owner holds both today, but roles are the owner's to
    cut (phase 10, Task 4b). The Menu screen already carries `menu.edit`, the
    editor's data (each row's item count, from the items it reads) and the item
    form that picks from both lists.
  - **Printers keeps a pointer, not a second editor.** Under "Where things print"
    a person who may edit the menu sees "Stations are added, renamed and retired
    in the menu" with a link to `/menu?view=setup`; the printer assignment itself
    stays where the printers are. A station retired there leaves the select.
  - **A retire asks where things go, in words that fit what is known.** The
    client knows the items a row carries (retired items included, as the server
    counts them) and not whether a station has a printer, so the modal names the
    items and says the printer follows when the chosen station has none. A row
    with items must name a destination (refused before asking); a station with
    none may name none, and the server's own refusal covers a printer it could
    not place. A course with no items asks nothing. The only row in service shows
    "This is the only station in service. Add another before retiring it." and no
    way to confirm. What moved comes back from the server and is said in a toast:
    "Bar retired. 10 items moved to Kitchen."
  - **Ordering is arrows, as for categories and items,** over every row,
    retired ones included, since that is what the server's order takes. Courses
    renumber as they move; a reorder under open tickets is refused with the
    server's words (ADR-134) in the view's problem line, and the description
    says so before it happens.
  - **A station's code is chosen once**: the add form offers it (empty makes one
    from the name) and the rename form shows it read only.
  - **`?view=` joins `?item=`**: the menu editor reads the view the link asks for
    once, on first load, as it reads an item (ADR-137).
Because: a screen that can be opened should be able to do what it shows, and an
owner setting up a venue adds stations and courses in the same place as the
items that name them.
Cost: a venue setting that is not under Settings; an owner looking for stations
under Printers is pointed, not served; the view cannot show whether a station
has a printer.

## ADR-140 - Devices: list, rename, revoke, and a device never revokes itself
2026-10 · Status: accepted.
Decision: `GET /api/v1/devices`, `POST /api/v1/devices/{id}/rename` and `POST
/api/v1/devices/{id}/revoke` (all `device.manage`), beside the pairing-code route
that already existed, and a Devices screen on them (`/devices`, key `g e`; spec
`docs/design/screens/backoffice-devices.md`). The last entry of the back office
navigation stops being Soon: every section is built.
  - **No device may revoke itself, whatever its kind.** The task named the back
    office; any paired device with a staff member signed in on it could otherwise
    unpair the screen it is using, and a back office that does it cannot pair
    another. `Devices.RevokeByPublicID` takes the acting device and refuses its own
    id (`cannot_revoke_self`, 409), so the rule is in one place and not in a
    handler. Two back offices can revoke each other; the one in use cannot revoke
    itself. A venue's last back office therefore cannot be revoked from itself, and
    the screen says so beside the disabled button.
  - **The per-device routes are sub-paths, not `PUT /devices/{id}`.** The mux
    refuses two patterns when neither is more specific: the existing method-less
    catch-alls on `/api/v1/devices/pair`, `/me` and `/codes` (for a 405 in the
    API's own words) overlap `PUT /api/v1/devices/{id}`. `POST .../{id}/rename`
    and `.../{id}/revoke` do not, and follow `retire` and `restore` elsewhere.
    Each answers with the list as it now stands.
  - **A revoked device is listed, last, and keeps its name.** Revoking sets
    `revoked_at` and keeps the row, as before, so history can still name the till
    an event came from (`DeviceNamesForVenue`). A rename of a revoked device is
    refused (`device_unpaired`, 409): relabelling a retired till would rename it
    in history for no one's benefit. A rename of a live device relabels it
    everywhere, history included, and does not touch its token or pairing.
  - **Waiting codes are listed without their codes.** Only a code's hash is kept
    (ADR-053), so the list says "Counter 1 (Till) runs out in 7 min" and cannot
    show the code again: it was shown once, when it was made. A code used or run
    out is not listed.
  - **A refused name says it is the name.** A pairing code refused for its name
    used to name the field `kind`. `auth.InvalidDevice` carries the field, still
    an `ErrInvalidDevice`, and the name and kind refusals come back under their
    own fields.
  - **"Last seen" is as fresh as the server records it:** at most once a minute
    (`touch`), so the list is read every 30 seconds and says "Seen just now" for
    anything under a minute. A day without a request is shown in the warning ink
    and in words. Dates are written out ("3 Oct 2026"), not asked of the browser's
    locale, whose abbreviations differ.
  - **Rename and revoke carry no `Idempotency-Key`,** as the menu editor's routes
    do not: each is idempotent by nature (a revoke of a revoked device is not an
    error; a rename to the same name is the same rename).
Because: a till that walks out of the building, or a tablet that was sold, had to
be unpaired by editing the database; and the one mistake that cannot be undone
from the screen, revoking the screen itself, should not be possible from it.
Cost: an open live stream of a revoked device is not closed: it authenticates when
it connects, so it is refused at its next connect, while every request it makes
is refused at once and it can sell nothing; its name in the hub is the one it
connected with until it does.

## ADR-141 - Backup configuration moves to phase 12, Task 4
2026-10 · Status: accepted (decided by the human).
Decision: phase 10's Task 6l (backup configuration) is built inside phase 12's
Task 4 (Litestream), which already asks for backups "configured in back office".
Phase 10's Task 6 is ticked with 6l marked moved.
Because: there is nothing to configure until Litestream is in the binary, and a
settings screen over a feature that does not exist would be guessed twice.
Cost: phase 10 ends without a Backups section in settings; a venue has no
backups until phase 12.
---

## ADR-142 - The Windows Service wrapper lives in internal/winsvc
2026-10 · Status: accepted. Amends ADR-020's pointer to `internal/service`.
Decision: phase 12, Task 1 puts the `kardianos/service` wrapper in
`internal/winsvc`. `internal/service` stays the parent of the application
services (`internal/service/order`, `menus`, `shift` and the rest), and its
package comment says so.
Because: the phase 00 skeleton reserved `internal/service` for the Windows
Service, and from phase 04 on it became the application layer. Its comment still
described the wrapper, so a reader, or phase 12, would follow it to the wrong
place.
Cost: none in code; ADR-020's text names the old path and is read with this one.
---

## ADR-143 - The screens are rebuilt from the mez design, by role
2026-10 · Status: accepted (decided by the human). Amends ADR-128's "looks
only".
Decision: a new phase, `docs/phases/rebuild.md`, runs before phase 11. Every
staff screen is rebuilt layout for layout from `docs/design/reference/mez/`.
  - **By role, not by device.** One sign-in; a cashier lands on the order
    screen, a waiter on their tables, the kitchen on its board, a manager
    with the full navigation. Pairing stays as the device's trust.
  - **The mez HTML files are the only reference.** The PNG mockups are not.
  - **The server grows where the design needs it:** a waiter role, roles at
    sign-in, a service type (dine-in, takeaway, delivery), item description
    and dietary marks. Each is a domain task with tests.
  - **Logic stays, JSX goes.** Hooks and `*-logic.ts` files and their tests
    are kept; functions the design does not draw move under a More menu.
  - **Done is seen, not tested.** A screen task closes on a screenshot beside
    its design and the owner looking at it in the dev server.
  - Reservations, waitlist, customers, offers, notifications, the setup
    wizard and the guest phone are not in it.
Because: the owner ran the build and rejected it. ADR-128 moved tokens and
primitives onto the old Nofiltr layouts, so the app had the design's colours
and none of its flow, and no session had walked it as a user would.
Cost: phase 11 waits. The specs in `docs/design/screens/` are wrong until the
phase's last task rewrites them. Events gain fields, so the projection
version moves.
---

## ADR-144 - `mez pair`, and a demo venue with public PINs
2026-10 · Status: accepted
Decision: `mez pair -device-kind K [-device-name N] [-data-dir D]` makes a
pairing code at the machine, as `setup` makes its first one. It refuses a
missing database (it would otherwise create an empty one) and a database with
no venue, and prints the expiry on the venue's clock. The code records no
creator: no person made it. `-demo-menu` now makes a whole demo cafe: the menu,
twelve tables in two areas, and one person for each role but the owner, with
fixed PINs that setup prints and calls public. The flag keeps its name, since
the docs use it. `make dev` (`build/dev.sh`) builds, sets up `.dev-data` on its
first run with a code for each app, starts the server on 7791 and the two
apps under Vite, and stops all three on Ctrl+C.
  - A role added to `auth.DefaultRoles` must arrive with a demo person: a test
    fails until it has one. The waiter (Task 1a) is the first.
  - The demo floor and staff are written in setup's one transaction, straight
    to the tables, not through the floor and people services: a venue never
    exists half demonstrated. A test passes every table back through the floor
    service's own validation.
Because: the first device decides what can be reached. A venue set up with
a till had no screen to pair the back office from, and the rebuild is judged
by signing in as each role, which needs each role to exist with a PIN known
before the venue is made. Fixed PINs are safe only because a demo venue never
sells, and the flag, the summary and the docs all say so.
Cost: a person who runs `-demo-menu` on a real venue has four known PINs until
they change them. A dev venue made before this has no floor or staff: delete
`.dev-data` and run `make dev` again.
---

## ADR-145 - The waiter role, and a role's name on every staff member
2026-10 · Status: accepted
Decision: a `waiter` role joins `auth.DefaultRoles`: order.open, order.edit,
payment.take, kitchen.view. Migration 00031 adds it, once, to venues that exist,
skipping a venue that already has a role called "waiter" in any case. `GET
/api/v1/staff` and every session response carry `role`, the role's name, on the
staff member; `auth.Staff` gains `RoleName`, filled by the staff list, a
sign-in, a session found again and an override.
  - **A name is a label.** Nothing checks a role by name; what a person may do
    is the permissions on the session. The sign-in screen (Task 3) shows the
    name under the face and the landing (Task 2) will follow permissions, not
    the word "waiter".
  - **Waiter and cashier hold the same permissions today.** The design draws
    the waiter settling the bill (T05), and neither may refund, void, discount
    or manage the shift. What differs is where each lands. "Own tables" (T01) is
    a view of the orders a person opened, built in Task 11; it is not a
    permission.
  - A session stored in a browser before this has no role, so it is dropped and
    the person signs in again.
Because: the rebuild lands people by role, and a role that does not exist
cannot be signed into or demonstrated.
Cost: the shift's count route is guarded by payment.take, so a waiter can count
a drawer; Task 4 decides whether that stays. The migration's list and
`DefaultRoles` are two copies of one list, each held by a test.
---

## ADR-146 - Service type on an order: dine-in, takeaway, delivery
2026-10 · Status: accepted
Decision: `order.opened` gains `service` (`dine_in`, `takeaway`, `delivery`).
The service always writes it: a client that leaves it out gets dine-in with a
table and takeaway without. An event stored without it reads the same way, at
projection, so no stored event changes. `State.Service` carries it, and
`ProjectionVersion` moves 6 to 7. The order, the open-ticket list and the
kitchen ticket responses carry `service`.
  - Takeaway and delivery with a table are refused (`invalid_payload`). Dine-in
    may open with no table.
  - Seating a takeaway (`order.table_changed` from 0) makes it dine-in. A
    delivery cannot be seated: `ErrServiceNoTable`, 409 `service_no_table`. So
    the service and the table never disagree, with no event to change a
    service.
  - Guests were already `covers` on the opening event, so 1b adds nothing for
    them. Nothing changes covers after open; a screen that needs it adds an
    event then.
  - Not done: the Z-report does not split sales by service (the task allowed
    that unless it was free, and it was not looked into further), and a kitchen
    ticket prints the table or nothing, not "Takeaway" or "Delivery". The
    kitchen display response has the field.
Because: the order screen's rail (D03) and the kitchen board draw how an order
is served, and the old model could only say "has a table".
Cost: an order's service can change only by seating it. A dine-in order may be
priced, invoiced and closed without ever being seated, and a takeaway can be
seated after its bill is issued (table moves are allowed then): nothing here
forbids either. A guest who turns a
dine-in into a takeaway needs a new order, or an event this does not add.
---

## ADR-147 - What an item card shows: description, dietary mark, gluten free, popular
2026-10 · Status: accepted
Decision: an item gains four optional fields, migration 00032, all empty or
false for every existing item.
  - `description`: one line, at most 140 characters (counted as characters,
    checked in the table and in the editor), trimmed, no line breaks.
  - `diet`: `veg`, `non_veg`, `vegan`, or `''` for no mark.
  - `gluten_free` and `popular`: flags.
They flow through the editor (`ItemInput`, `Item`), the terminal's menu
(`menu.Item`, left out of the JSON when empty so an old reader sees the item as
before) and `db.ItemSpec`. The back office item form gets a description box, a
choice for the mark, and two ticks, plain until Task 16. `-demo-menu` gives
every item a description and a mark, with some popular and gluten free, so a
card looks finished with no photo.
  - **Gluten free is a flag, not a fifth mark.** The design's filter chips list
    Veg, Non-Veg, Vegan and Gluten Free as peers, but a dish can be vegetarian
    and gluten free, and one column could not say both. The chips can still be
    four: Veg should include vegan items, which is the screen's rule (Task 5).
  - A save is the whole item, as it was: a client that leaves a field out
    clears it. The back office sends all four every time.
  - Nothing reads `popular` yet: the order screen's Popular tab is Task 5.
    The form's hint says only that.
Because: the order screen's item cards (D03), the item modal (D04) and the
guest phone draw these, and a card with only a name and a price looks
unfinished.
Cost: no screen reads them yet; the back office form was tried in a browser
against a real server (open, edit, save, reopen). `db.ItemSpec` (setup, tests)
is not run through the editor's checks; only the table's constraints guard it.
The line-break check refuses controls and Unicode line or paragraph separators,
not format characters, which Indic scripts use. Not done:
the guest menu (phase 11) does not carry them, and the kitchen ticket does not
print the mark.
---

## ADR-148 - The terminal's shell: routes, landing by role, one frame, one pairing
2026-10 · Status: accepted. Amends ADR-145's "landing follows permissions".
Decision: the terminal is routes, not state flags in one component.
`/pos` (the order screen), `/tables`, `/orders` (history) and `/orders/:id` (a
settled bill), `/pay/:id`, `/shift`, `/shift/close`, `/shift/z/:n`, `/waiter`,
`/kitchen`; `/` lands a person by role. Each existing screen is carried behind a
path unchanged; the screens are rebuilt from the design in the tasks after.
  - **Not `/order`.** The server gives `/order/` to the guest app, so a reload on
    `/order` loaded the guest app (the plan's name for the route, and the one
    mistake a walk through the screens by link does not show). The order screen
    is `/pos`; `routes.test.ts` keeps every terminal path out of the other apps'
    mounts, and the static tests reload each of them.
  - **One provider above the routes.** The ticket in front, the floor, the shift
    and the Z-report just issued are hooks with state of their own; `TerminalProvider`
    holds them once, so going to Tables and back keeps the ticket (tried: a
    60.00 ticket survived the round trip).
  - **The shift gate.** No shift open: every screen waits at `/shift`. A counted
    drawer: the close is the only way on. A pure function, `shiftGate`.
  - **Landing by role.** Owner, manager, cashier: `/order`. Waiter: `/waiter`,
    which is `/tables` until Task 11. Kitchen: `/kitchen`, a page load into the
    kitchen display, which is another app of the bundle paired as a kitchen
    device, so a cook signing in on a terminal is asked for a kitchen pairing
    until Task 15 rebuilds it around a signed-in cook. The role's name decides
    for the default roles, because waiter and cashier hold the same permissions
    (ADR-145); any other role is landed by what it can do.
  - **The frame.** A 72px top bar (logo, Orders, Tables, Menu, Reports, then
    the name, role and avatar, which open Lock, Close the shift, Print queue) and
    a 40px status bar (version, Online or what is wrong, the venue, the date and
    time in the venue's zone). Pills are 48px, not the design's 44 (ADR-014). A
    screen's own header controls come after the navigation when it is rebuilt.
    Menu and Reports are shown to those whose role can use them (`menu.edit`,
    `reports.view`), as the back office does, so a manager sees Reports and not Menu.
  - **Left out, because nothing does them:** the bell (phase 11), Keyboard
    Shortcuts and Open Cash Drawer in the status bar. The date and time are in the
    status bar (D03), not the top bar (D05), so there is one of them.
  - **Icons: `lucide-react` 1.51.0, exact, in the terminal and the back office
    only, never the guest bundle** (its own `ui/icons.tsx` stays dependency
    free). The design is drawn with lucide's names and the screens ahead need
    dozens of them. `LucideProvider` sets the design's 1.75px stroke once. ISC
    licence. The terminal bundle grew by about 5 KB gzipped.
  - **The venue's name and zone** come from the existing `GET /api/v1/venue`,
    which gains `timezone`: the clock is the venue's, not a tablet's own.
  - **One pairing per browser.** The back office shares the terminal's device
    pairing when it has none of its own. While it does, the device's session slot
    is one slot (the server ties a session to its device): a sign-in or a lock at
    either app is one at both, and a revoked device is forgotten for both. Menu
    and Reports open the back office signed in as whoever is at the till. A back
    office paired of its own uses its own keys alone, as before. Staff
    permissions still gate everything: what a till's browser gains is the back
    office's sign-in, not access.
  - **The profile menu is a disclosure**, not an ARIA menu: plain buttons, focus
    moved in on opening, back to the button on Escape, closed by Tab leaving it.
Because: a screen cannot be rebuilt around a flow that lives in one component's
state, and a manager should not pair a browser twice to reach two apps.
Cost: kitchen staff on a terminal hit a pairing screen until Task 15; a back
office lock locks the terminal too (one session, one device); `/waiter` is the
floor until its screen exists.
---

## ADR-149 - Staff Login is D01, one screen for the terminal and the back office
2026-10 · Status: accepted
Decision: the lock screen is rebuilt from D01 and stays one shared screen: a
brand panel on the left (the logo, "Great Food Brighter Days", the script line,
the version) and on the right "Staff Login", the profiles, the PIN dots and the
keypad. It serves the terminal and the back office alike, as the flow map says
("one sign-in for everyone"); where a person lands after it is their role's.
  - **Profiles** are a round avatar of the person's initials, their name, and
    their role under it (the server sends the role, ADR-145). Four to a page and a
    More that goes to the next page and round again. The selected one is ringed in
    terracotta, as the design draws it. The design's photographs do not exist.
  - **The keypad** is 1 to 9, delete, 0, and a green Login as the twelfth key where
    a Clear was. A wrong PIN still clears the dots. The keyboard still works.
    The chosen person's name sits in the dots well (the design has none there),
    because paging away from their tile leaves no tile ringed and a sighted person
    must still see whose PIN this is. Enter on a focused tile, More or Try again is
    that control's own click; Enter elsewhere signs in. The chosen person is held
    by id, so someone removed or deactivated while the screen is open stops being
    chosen and what was typed goes.
  - **Left out:** the design's illustration of a chef at the pass and the dark
    script card beside it. Both are placeholders for artwork nobody has made, and
    a labelled grey box is not a screen. The panel can take them when they exist.
  - **Sizes:** a new token, `text-hero` (66px), for the headline from 1536px wide,
    and 44px below. Keys are `clamp(48px, 7vh, 76px)`: 76 on the design's 1080,
    54 on a 768 laptop, never under the 48px floor (ADR-014). The brand panel is
    hidden below 1024px, where a tablet upright has no room.
  - The venue's name and the date and time (in the venue's zone) are along the
    bottom, and the connection indicator keeps its place, with its retry. The
    clock, the version text and the venue hooks moved to `shared/` for it.
Because: one screen for every role is the point of the rebuild, and the lock
screen is the first thing anyone sees on a shift.
Cost: the avatar is initials, not a face; the device's name ("Counter 1") is no
longer on the sign-in screen, as the design has none; the older screen spec
`terminal-lock.md` is wrong until Task 17 rewrites it.
---

## ADR-150 - Open the shift is D02, and a closed till names who can open it
2026-10 · Status: accepted. Builds on ADR-094 (the shift's permissions).
Decision: `/shift` is rebuilt from D02. Someone who may open the shift sees
"Step 1 of 2, Open the shift": a card for each note and coin with a box for how
many and its value, the counted float in a green panel, Cancel and "Open shift &
start selling", and beside it "Tonight's shift" (outlet, terminal, the person and
their role). Anyone else sees "The shift isn't open yet" and who can open it.
  - **The permission is unchanged: `shift.manage`.** The flow map says a cashier
    opens the shift, but the default cashier role does not hold `shift.manage`
    (ADR-094: opening and closing are the manager's, and the same permission also
    pools tips, corrects Z-reports and closes stock). So with the default roles a
    cashier sees the "who can open it" page, and a manager or the owner sees D02.
    The owner can give the cashier role `shift.manage` in Roles to get the design's
    flow, at the price of the cashier closing as well. A narrower `shift.open` is
    the alternative and is the owner's call; nothing here prevents it.
  - **Who can open it** comes from `GET /api/v1/shift`, which gains `openers`
    (id, name, role of each active person whose role grants `shift.manage`) and
    only while no shift is open. It sits there and not on the staff list because
    that list is open to any paired device and carries no permissions (ADR-105).
    `auth.StaffAuth.WithPermission` builds it from the roles as the permission
    check does, so a role the code cannot read grants nothing. If building it
    fails, the till is still reported closed, the failure is logged, and the page
    says "Ask the owner or a manager." A close re-reads the shift so the list is
    there at once.
  - **The count is still blind.** The design shows "Expected from last close" and
    "Matches" beside the counted float. They are left out: an expected figure
    anywhere near the counter is what ADR-094 refuses, at the open as at the close.
  - **Left out, because the server has nothing behind them:** the note for the
    manager (an opening count carries no note), the shift window, "Staff on floor"
    and "Kitchen" (who is signed in is not recorded), the low-stock count and
    "Review inventory" (stock figures need `stock.manage` and the shift screen
    has no business with them), and the chef artwork (as in ADR-149). What is
    shown from the menu is what is sold out now, with a "Review the menu" link for
    those who may edit it. A currency with no table of notes gets one box for the
    whole amount.
  - **Cancel** goes back to the sign-in screen, which is the step before this one.
  - **Sizes:** the count boxes are 48px high and 17px, not the design's 44px and
    16px (ADR-014); the title is `text-total`, not the design's 40px. Below 1280px
    the right column drops under the card, and the float panel and the buttons are
    below the fold on a 768px-high screen.
  - The screen's words say "shift", not "till" (the till is "the order screen",
    ADR-143), in the gate's waiting line and in the server's two refusals.
Because: a cashier's day starts here, and a waiter who finds the till shut should
be told who to fetch, not shown a count they cannot submit.
Cost: with the default roles a cashier cannot open the shift; the sold-out card
was not seen by eye (the venue had nothing sold out); `GET /api/v1/shift` costs
one more read per poll while the till is closed.
---

## ADR-151 - The order screen is D03
2026-10 · Status: accepted
Decision: `/pos` is rebuilt from D03 as three columns. A 240px sidebar of All
Items, Popular (only when something is popular) and each category, each with an
icon and a count. In the middle the title of the view, the four dietary filters
(Veg, Non-Veg, Vegan, Gluten Free, which add up as `filterItems` says) and the
search, over the item cards. On the right a 400px order rail: where it is and how
many guests, how it is served, the lines, the totals, Save Order and Pay.
  - **Cards look finished with no photo.** A tinted block wears the category's icon
    (`categoryIconKey` guesses it from the words in the name, with a fork and knife
    when it cannot). A diet mark, the name, the description, the number of choices,
    the price, a round "+" and a crown on a popular item. Sold out stays in place.
  - **How it is served** (Dine-in, Takeaway, Delivery) is chosen before the order
    opens. After that it is written on the opening event and moves only by seating
    (ADR-146), so the others are dimmed. A ticket with no table says Takeaway, which
    is what the server makes of it. A dine-in order with no table shows "Pick a
    table", which goes to the floor, where it is seated; the floor is rebuilt in
    Task 7. Guests are shown, not set here: they are counted when a party is seated.
  - **Lines** take a stepper and a bin where the kitchen has not seen the line. A
    line it has seen shows its quantity plain, and its bin asks a manager to void it.
    Tapping the name still opens the line's sheet (note, modify, discount, comp).
  - **Words:** Pay for Charge, Save Order for Hold, Orders for Tickets, Send to
    Kitchen for Send. The choices sheet and the payment screen keep their words until
    Tasks 6 and 8.
  - **Send to Kitchen is added to the design,** as a button over Save Order and Pay,
    only while something waits to go. The design draws it on the tables screen, and a
    counter order has no other way to reach a station.
  - **Under the "...":** Orders (every open order, to bring one forward or start
    one), Order history, Discount the order, Fire course N when one is held, and Past
    bills. All were on the old rail; none is drawn in D03. The menu is
    `useDisclosure`, taken out of the profile menu so both behave alike.
  - **Left out, because nothing does them:** the sort and list/grid switches, the
    bell (phase 11), Ctrl+K for search, the table and service controls in the top bar
    (they are in the rail, once, because the top bar carries the navigation), a
    thumbnail on each line, the CGST and SGST split (the projection has one tax
    figure), a removable discount chip, and the sidebar's script and leaf.
Because: the old screen had Charge, Hold and Tickets for a cashier who thinks in
orders, and nothing to tell dine-in from takeaway.
Cost: a counter sale's order cannot be moved to a table from here; the rail is 400px
at 1366 wide and leaves two card columns; the diet filters are in memory only and
reset on a reload; Popular appears only once the owner marks items.
---

## ADR-152 - Opening the shift is `shift.open`, and the cashier holds it
2026-10 · Status: accepted. Settles the question ADR-150 left to the owner; narrows
ADR-094 (`shift.manage` was both open and close).
Decision: a new permission, `shift.open`, is what `POST /api/v1/shift/open` asks
for. `shift.manage` keeps the rest: closing, pooling tips, correcting a Z-report and
the closing stock count. The default owner, manager and cashier hold `shift.open`;
the waiter and the kitchen do not.
  - **Why not give the cashier `shift.manage`.** The flow map puts D02 in the
    cashier's path, and with the old defaults a cashier could never see it. But
    `shift.manage` also closes the shift and issues the Z-report, which ADR-094
    keeps with a manager. Opening is the safe half: the count is blind, it moves no
    money, and a wrong float shows as a variance at the close, under the opener's name.
  - **Existing venues** (migration 00033): every role holding `shift.manage` gains
    `shift.open`, whatever it is called, so nobody who could open loses it; and a
    role still named `cashier` gains it, once, as 00016 did for owner and manager. A
    renamed cashier role is left alone and the owner ticks the box in Roles.
  - **Who can open it** (`openers` on `GET /api/v1/shift`, ADR-150) is now everyone
    whose role grants `shift.open`. The terminal shows D02 to the same people.
  - **In Roles** the two read "Open the shift" and "Close the shift".
  - `shift.manage` is not renamed: a permission's string is what roles store
    (`permissions.go`), so it keeps its key and narrows in meaning only.
Because: a cashier's day starts at D02 in the design, and a cafe with one person on
the morning shift should not need the manager's PIN to start selling.
Cost: a cashier who opens with a wrong float is found out only at the close; a venue
that wants only managers to open must untick it for the cashier role; a venue that
renamed its cashier role gets nothing from the migration.
---

## ADR-153 - The item modal is D04, and a new line can carry its kitchen note
2026-10 · Status: accepted
Decision: `ItemModal.tsx` replaces `ModifierSheet.tsx`. It is a centred dialog 1020px
wide, not a side sheet: on the left a 400px panel with the item's photo, or its
category's icon where there is none, and a "Popular" pill; on the right the diet mark,
name, description and price, each group of choices, the course, a kitchen note, and
along the bottom the quantity, Cancel and "Add to Order" with what the line will cost
(the item and its choices, times how many).
  - **When it opens is unchanged.** One tap still rings an item up when its defaults
    satisfy every group; the modal opens when someone has to choose, and from a
    line's Modify. Opening it for every item with choices would be the design's flow
    to the letter and a second tap on every coffee; that is `ringUp` in
    `OrderScreen.tsx` and the owner's call.
  - **Groups.** A choose-one group is radio tiles, three to a row, with the extra
    price under the name; a choose-several group is checkbox tiles, two to a row, with
    the price on the right. Beside the name: "Required" (in brick, as drawn),
    "Optional", "Select multiple", or the rule in words (`groupHint`). The design's
    "Spice level" is a group like any other, so it wears a radio too.
  - **Quantity and course go on `AddLine`,** which always took both. The course is a
    plain select of the venue's courses, shown only for a new line and only when
    there is more than one: nothing moves a line between courses once it is added.
    The modal's quantity stops at 99.
  - **The kitchen note of a new line** is a second command. A line's id is the `seq`
    of its `line.added` event, which the add's answer carries, so the outbox
    (`TicketQueue`) writes the note on that line straight after the add, under an
    idempotency key made at the tap. If the note is refused the line stays and a toast
    says the note is missing: taking the line back is not the till's to decide.
  - **Changing a line** saves what differs, in turn: the choices, the quantity, a
    note. If one fails after another was saved the modal closes and a toast says only
    part was saved, because the modal then holds an old copy of the line. A quantity
    changed on another till now also shuts the modal's button.
  - **Left out, because nothing does them:** "Also ordered with" (nothing records
    what sells together), a size's "10 inch" (a choice has a name and a price), and
    the first choice's own price (the header has the item's).
  - **Sizes:** the title and price are `text-total` (32px), group names `text-xl`,
    tiles 48px or taller, the two buttons 56px, the note and course 16px (ADR-014).
    Below 1024px wide the left panel is dropped; on a 768px-high screen the middle
    scrolls and the footer stays.
Because: the old sheet could only choose; "two, no onions, with the mains" took three
trips to the line's sheet.
Cost: a note can be lost while its line is kept; a scrim tap does not close the modal
(a half-written note is not lost to a stray tap, as with `Modal`); not seen by eye are
a photo, the Popular pill, a refusal, and a narrow screen.
---

## ADR-154 - An item card is two targets: the card opens the modal, the "+" adds
2026-10 · Status: accepted. Changes ADR-153's "when it opens is unchanged" and
ADR-151's "the whole card is the target". The owner chose it on 2026-10-04.
Decision: a tap on an item card opens the item modal whenever the item has anything
to choose; a tap on its round "+" rings it up at once with its defaults
(`ringsUpWith` in `register-logic.ts`).
  - An item with no choices is rung up by either tap.
  - An item whose defaults leave a choice to make (a required group with no default)
    opens the modal from either tap, as before.
  - The "+" is a button of its own laid over the card's corner, since a button cannot
    hold a button; a spacer in the card keeps its place. It is 48px, not the 40px it
    was drawn at while it was only a picture (ADR-014), and is named "Add <item>".
  - A sold-out item shuts both.
Because: with one target, "an oat latte with an extra shot" was the tap, the line, its
sheet, Modify, the choices and Save. Opening the modal on every tap would fix that
and put a second tap on every plain coffee, at a venue where most items have choices.
Cost: a tap meant for the "+" that lands on the card opens a modal to cancel; a screen
reader meets two buttons per item, the card unnamed beyond its text.
---

## ADR-155 - The tables screen is D05: a tap picks a table, the panel acts on it
2026-10 · Status: accepted. Replaces the floor's sheets (ADR-087) and where Move, Join
and Part start (ADR-088); the modes themselves stand.
Decision: `/tables` is `TablesScreen.tsx`, three regions as drawn: a 240px sidebar
(Floor Plan, Table List, Order History), the floor in the middle with area pills and a
legend, and a 400px panel for the table picked. `FloorView.tsx` and its two sheets are
gone.
  - **A tap picks, it does not act.** The old floor jumped to the order screen or opened
    a sheet (`tapFor`). Now `panelFor` says what the panel shows: a free table seats a
    party, one to clear is cleared, one with an order shows it, one with several asks
    which. A joined table picks its party's own table (`partyTable`).
  - **The panel's order is read beside the order in front** (`use-table-order.ts`), by
    the same cache key, so looking at a table does not take the cashier's own order out
    of their hands. Send to Kitchen and Fire post to that order directly. Add Items is
    what brings it to the front (off hold if it was held) and goes to the order screen;
    Settle Bill goes to `/pay/:id`.
  - **Seating stays on the floor.** The old sheet seated a party and went to the menu.
    Now the table turns Seated and the panel offers Add Items: a host seats, a waiter
    orders later. It costs the "seat and order at once" case one tap; the owner's call.
  - **More Actions** holds what the design draws no button for: Move to another table,
    Join tables, Part tables (only for a joined party), Fire course N, Order history,
    and Guests left (only while seated with nothing ordered). Move and Join start from
    the table in the panel, its first tap already made (`beginMode`); after a move the
    panel follows the party.
  - **The plan packs out empty tracks** (`packed`). The back office lays tables on a
    12 by 8 grid; drawn to scale the demo floor was narrow tiles, wide gaps and two
    scrollbars. Each table keeps its place among the others, and the tiles share the
    room. A round table's inner fill is round.
  - **All Areas** stacks each area under its name: areas share one grid's coordinates,
    so they cannot be drawn as one picture.
  - **The table list** is ours: D06 is the reservations screen and only names a table
    list in its sidebar. It is the same tables as rows (table, area, guests, status,
    time, balance) under D06's "Table availability" bars, one per state.
  - **Words:** "covers" on a tile is now "guests"; "Charge" here is "Settle Bill". A
    paid order still open keeps the order screen's words on the button.
  - **Totals** are `OrderTotals.tsx`, taken out of `RailFooter.tsx`, so the rail and the
    panel cannot disagree. In the panel they scroll with the lines, as drawn; the
    amount is on Settle Bill, which does not scroll.
  - **Left out, because nothing does them:** Reservations, Waitlist, Customers and
    Settings in the sidebar; the Reserved state; zoom; the entrance and bar strips (a
    floor has no such data); Edit Layout and Add Table (the back office's floor
    editor); the bell; the panel's "..." (More Actions is the one menu); the waiter's
    name (an order keeps who opened it as a number, the staff list as a string id); the
    Guests tab (an order keeps a count, not guests); a line's photo thumbnail; CGST and
    SGST as two rows (the rail shows one Tax row, and so does this); the status bar's
    table counts (they sit beside the area pills).
  - **Sizes:** tiles hold four lines down to 92px wide, so six columns fit beside the
    panel at 1366px; a wider floor scrolls sideways. The table name is `text-total`.
Because: the owner rejected the floor as the old layout recoloured; D05 is a different
flow, not a different skin.
Cost: seating and ordering is two taps more than before (pick, seat, Add Items); a
floor of seven or more columns scrolls at 1366px; the tile shows no shape but round or
square.
---

## ADR-156 - The payment screen is D07: three steps, and the paid panel only at the last
2026-10 · Status: accepted. Replaces the tender screen's layout (terminal-tender.md);
its sums and its idempotency (ADR-062, `tender-logic.ts`) stand.
Decision: `/pay/:id` is `PayScreen.tsx`. The order is down the left in 400px (Back to
Order, where and how it is served, its lines, `OrderTotals`, payments taken, Add
Discount, Add order note). The middle is the three steps: Payment, Split Bill,
Complete. `TenderScreen.tsx` and `TenderSummary.tsx` are gone.
  - **The design draws four moments in one picture** (method, tip, change, success). A
    payment is in one step at a time, so the 400px "Payment Successful" panel is drawn
    only at Complete, beside the invoice. That is also what lets the amount and tip
    cards fit at 1366px.
  - **Payment** is method tiles (Cash, Card, UPI), an amount card (Amount to collect,
    Amount received, the quick amounts and Custom, Change to return; for card and UPI
    the amount and its reference), and a tip card (5, 10 and 15%, a custom amount, and
    Bill, Tip, To collect). "Complete payment" with the sum stays at the foot and does
    not scroll.
  - **A tip percentage is of the balance or the share being paid, to a whole unit of the
    currency** (`tipFor`): 5% of ₹1,053.00 is ₹53.00, as drawn. A second tap takes it
    off. It is still kept apart from the bill, and for cash it still comes out of what
    was handed over: a cash payment that then falls short of the bill is taken as a
    part payment, and the foot says how much stays owed before it is taken.
  - **The keypad stays** (rule 4). The design has typed fields and no keypad; a touch
    till has no keyboard. An amount field is drawn as the design's input and keyed as a
    till keys money: digits shift in from the right in minor units, from a keyboard
    while it has focus or from the keypad while it is the active field. It is a third
    column at 1920px and sits under the cards at 1366px, where the middle scrolls.
  - **Split Bill** opens the split panel as it was; Task 9 redraws it. The step buttons
    are how to move between paying and splitting. Complete is never a button.
  - **New Order** closes the order (`ticket.close`), which starts the next ticket and
    returns to the order screen. It waits for the invoice, as Close ticket did.
  - **Add Discount and Add order note** open the order screen's own sheets
    (`TicketPanels`), until the bill is paid or invoiced.
  - **`Money` has two more variants,** `heading` (serif, 22px) and `display` (serif,
    44px), so a serif figure never needs a second font-size class fighting the first.
  - **Left out, because nothing does them:** the Wallet tile; "Print receipt" as a
    choice (a receipt is queued with the settling payment, always) and "Send receipt
    to customer"; "Send via SMS / Email"; the mascot art and the leaf; CGST and SGST as
    two rows on the left (they are on the invoice); the line photo thumbnails; "100% of
    tips go to the floor and kitchen team" (who shares a tip is the venue's rule,
    ADR-101, not this screen's to promise).
  - **Changed from the drawing:** "Payment Successful!" is the serif, not script: a
    status is never script (tokens.md). The script line is the thank-you under it.
    "Charge" in the tip card is "To collect".
Because: the owner rejected the tender screen as the old layout recoloured, and "Charge"
as kitchen-till jargon.
Cost: at 1366x768 the payment step scrolls to reach the tip summary and the keypad; a
reprint is still the invoice panel's button, not the paid panel's.
---

## ADR-157 - The SQLite driver is v1.46.1: a cancelled read left a connection on an old snapshot
2026-10 · Status: accepted. Changes the version ADR-036 pinned; the driver is the same.
Decision: `modernc.org/sqlite` goes from v1.39.0 to v1.46.1, and the module's Go
version from 1.23 to 1.24, which that release needs.
  - **What was wrong.** A read whose context was cancelled while its query was in
    flight (a browser reloading cancels its requests) could leave a pooled read
    connection holding its old WAL snapshot. Every later read on that connection then
    answered from the past, with no error: an order just opened was "not found", an
    invoice just issued was "not on this ticket", the open-orders list failed on an
    order one query had seen and the next had not. Requests landed on a good or a bad
    connection by chance, so the same read gave 404 and then 200. Only a restart
    cleared it.
  - **It is the driver, not our SQL.** Plain queries and read transactions both did it,
    and only when the cancellation raced a query; a query cancelled before or after it
    ran did not. Fixed upstream in v1.41.0.
  - **Why v1.46.1.** It is the newest release that builds with Go 1.24; v1.47.0 and
    later need 1.25, and the newest (v1.60.1) needs 1.26. One step of Go is the least
    this fix costs. CI reads the Go version from `go.mod`; the Docker build already
    uses 1.26.
  - **The test** is `TestCancelledReadsLeaveNoReadConnectionOnAnOldSnapshot` in
    `internal/db/stale_read_test.go`: sixteen goroutines read with random deadlines,
    then every connection of the read pool is asked for a row written afterwards. On
    v1.39.0 it fails within five rounds of forty; on v1.46.1 it passes.
Because: a till that cannot find the order it just opened loses the sale, and the venue
cannot know to restart the server.
Cost: Go 1.24 is now the least a contributor needs; the test takes about a second; the
driver's own SQLite moved forward with it, checked only by the existing suite.
---

## ADR-158 - Seat and order, the tax split in the totals, Print receipt in the paid panel
2026-10 · Status: accepted. The owner agreed these on 2026-10-05. Changes ADR-155's
"seating stays on the floor" and ADR-156's "a reprint is still the invoice panel's
button"; ADR-151's single Tax row.
Decision:
  - **A free table has two buttons.** "Seat N guests" seats the party and stays on the
    floor, for a host. "Seat and order" seats it and goes to the menu with its order in
    front, for whoever takes the order at once. The green one is the plain seat: it is
    the one that cannot surprise.
  - **The totals name each tax** (`taxRows` in `rail-logic.ts`): "CGST 2.5%" and "SGST
    2.5%" as the server worked them out, where there was one "Tax" row. With no
    components (nothing on the order yet, a profile with one tax) it is still "Tax".
    `OrderTotals` is shared, so the order rail, the table's panel and the payment
    screen all show it. The amounts are the server's; nothing is summed here.
  - **Print receipt is on the paid panel,** where D07 draws it, above New Order. It is
    the reprint: a manager's approval and a reason, a copy marked DUPLICATE, the same
    number. The invoice panel's own Reprint button is gone.
Kept as they were, and why: the plan packs out empty rows and columns (it fits 1366px);
the keypad stays on the payment screen (a touch till has no keyboard); the paid panel
is drawn only at Complete (it does not fit beside the form, and it should not say
"successful" before the payment); "Payment successful" is the serif (a status is never
script, tokens.md).
Cost: two more rows in the rail's totals on a GST venue; a cashier who taps Print
receipt meets a manager's PIN, since the first copy was printed with the payment.
---

## ADR-159 - Split is D08a inside the Split Bill step; card and UPI wait to be marked paid (D08b)
2026-10 · Status: accepted. Rebuild Task 9. Changes ADR-077's four split tabs and
ADR-156's "Complete payment records a card at once".
Decision:
  - **Three modes, as drawn:** By item, Equally, Custom amounts. The till no longer
    offers the server's by guest: a line can have several guests, which covers a
    shared plate. The route and the domain keep it.
  - **By item starts with every item unassigned.** A guest's card is picked, an item
    tapped goes to them; "Share" gives it to every guest, and the cross on a card
    takes that guest off it. The server refuses a line with no payer, so loose lines
    ride on one payer past the guests (`requestFor`): each Due and "Assigned X of Y"
    are the server's figures while the split is still being made. Nothing here
    divides money. By item therefore takes 19 guests, not 20.
  - **"Complete split" fixes the shares; it does not end the paying.** The same cards
    then carry Paid or Pending and "Collect", which goes to the payment step for that
    share alone. The design shows Collect beside unassigned items; a share cannot be
    paid while the split can still change under it.
  - **A card or UPI payment waits.** "Collect by Card" commits the amount; the waiting
    card shows it, counts the wait, takes the slip or UPI reference, and "Mark as
    paid" records it. Cash is still one tap.
  - **A refusal is the server's.** "Try again" resends with the same idempotency key;
    "Try another method" goes back to the tiles.
  - **Money that leaves some owed is confirmed** (cash too, with its change), then
    "Next share" returns to the cards or "Next payment" to the tiles. A bill paid in
    full goes to Complete as before.
Left out, and why (rule 3): the QR (no UPI ID is kept, and no encoder is in the till),
the terminal's name and "Connected", "Retry terminal" (no card machine is driven from
here, and a sale must finish offline); "Share of tax" on a card (a line's figure
already has its tax in it); "Receipt sent to the guest's phone"; the time on the
confirmed card (a payment in the order's answer carries none).
Cost: a card or UPI payment is two taps where it was one. A line shared by some guests
and not others takes "Share" and then a cross for each guest who did not have it.
---

## ADR-160 - The orders list is the trading day's, from one route; Task 10 is three tasks
2026-10 · Status: accepted. Rebuild Task 10a.
Context: D09 lists the day's orders in one table (open, paid, refunded, voided), with
each order's type and items, and "86 orders today · collected". The server had the
open tickets (`GET /orders`) and the last thirty closed bills (`GET /orders/closed`),
neither with a service type or item names, and no total.
Decision:
  - **`GET /api/v1/orders/today`.** Every order opened or closed since the trading day
    began, and any still open from before it, newest first, capped at 500. The day is
    the venue's (its timezone and `shift.day_start`), so a bar at 01:30 lists the night
    it is in. Each order is projected, as the other two lists do: no figure is kept
    twice.
  - **`DayStatus` is four words,** read from the projection: open (anything not
    closed), refunded (closed with a credit note), voided (closed with every line
    voided), paid (the rest). It is the list's vocabulary, not a fifth order status.
  - **Collected is the server's sum** of what those orders kept (payments less
    refunds), tips apart. The tabs' counts are the screen's to count.
  - **The key `shift.day_start` is declared in the order service** and the shift
    service names it from there: that package imports this one, and a second copy of
    the string could drift.
  - **Task 10 is 10a (this), 10b (the screen) and 10c (the refund modal, the print
    queue).** The split was made before starting, in the same session as 10a.
Rejected: widening `/orders/closed` (it is a search by number over all time, and the
list is a day); a status column kept on `orders` (a projection would then have a
second home).
Cost: one more list that loads every order it names. Search by guest, as the design's
box offers, has nothing to search: an order carries no guest's name.
---

## ADR-161 - The Orders screen is D09: the day's table, and the order picked beside it
2026-10 · Status: accepted. Rebuild Task 10b. Replaces the past bills sheet over the
order screen (ADR-148) and the bill's own screen (ADR-084).
Decision:
  - **One route, `/orders/:id?`.** The table and the panel are one screen; the id in the
    path is the order in the panel, so a reload or a link lands on it, and the tab and
    the search stay as rows are picked. Picking another row replaces the path, so Back
    leaves the screen in one step.
  - **The table reads the day's list (ADR-160); the panel reads the order itself,**
    through `useBill`, which already keeps it current and carries the refund, void and
    reprint commands with their keys.
  - **A row is one button.** The design's per-row action is drawn as "View" inside it:
    one target a row high, not a small one at its end.
  - **The search is over what can be read off a receipt or the floor:** the invoice
    number, the order id (four letters or more, since an id is random), the table. It
    also asks for settled bills from before today by their number, listed under
    "Before today".
  - **Void acts on a payment,** as it always has: with several, a sheet asks which.
    There is no command that voids an order.
  - **Kept and placed (rule 4):** the invoice as issued (Invoice) and the order's
    history (History), in the row where the design has "Email to guest". An open order
    has Open order and Pay.
Left out, and why (rule 3): Filters (the tabs are the filters there are), Export (the
till exports nothing; the back office does), Email to guest, the cashier's and
waiter's names (an order names its people by an id this screen has no list for), the
date picker (the list is the trading day's). New Order stands where Export is drawn.
Cost: the on-screen invoice is a tap away, where the old bill screen showed it at once.
---

## ADR-162 - The refund is D09's modal, in two steps; the print queue is on Orders too
2026-10 · Status: accepted. Rebuild Task 10c. Changes ADR-084's refund sheet.
Decision:
  - **The refund is a modal over the Orders screen,** as drawn: a brick mark, "Refund
    INV...?", the choices as radio cards with what each comes to, the reason, the note
    about the Z-report and the audit trail, Cancel and the way on.
  - **Three choices.** Selected items and the whole bill are the design's; an amount
    is the till's own and is kept (rule 4). The whole bill's figure is asked of the
    server as the modal opens, so its card shows it before it is chosen.
  - **The manager's PIN is the modal's second step,** not a line of text: "Continue to
    approval", then who approves and their PIN, then "Refund" with the amount. It is
    `ApprovalSheet` drawn as a modal, so a refund is approved exactly as a void is.
  - **The last button is the tinted danger button,** not the design's filled brick:
    tokens.md keeps a filled button for the one that moves a sale forward.
  - **Reasons are tiles, not a dropdown:** five reasons fit, and a tile is one tap.
  - **A refund goes back to one payment** (ADR-084). When what is chosen is more than
    the payment picked has left, the modal says so before the PIN is asked for.
  - **Print queue is a button in the Orders header,** where the design has Filters, and
    stays in the shell (the "Not printed" pill and the profile menu, ADR-081): a
    receipt that did not print must be seen from any screen.
Left out, and why: "the kitchen and the guest are both notified" (neither is), and
"including tip" on the full refund (a credit note takes nothing off a tip).
Cost: `Modal` and `ApprovalSheet` each grew a prop. A refund is two screens of one
modal where the design draws one.
---

## ADR-163 - The waiter's home is T01, and a waiter's frame goes with them to every screen
2026-10 · Status: accepted. Rebuild Task 11.
Decision:
  - **`/waiter` is T01:** the greeting, the pills, the search, the tables as cards four
    across, and a bottom bar with Active Orders and New Order.
  - **A waiter sorts tables into four groups, not five states.** Seated and ordered are
    both "Occupied", free is "Available", paying is "Needs Bill": the design's words.
    The card's small print tells a seated table ("No order yet") from one with a
    balance. The floor (D05) keeps its five words; the two screens answer different
    questions.
  - **The fifth pill is To Clear, where the design has Reserved.** Reservations are out
    of scope (D06), and a table to clear is a state a waiter acts on.
  - **The frame follows the person, not the path.** Whoever lands on `/waiter`
    (`landingFor`) gets T01's 88px bar with Home, Orders, Menu and More on every
    screen, and no status bar, as T01 draws none. Retry now and the "Not printed" pill
    stay in the bar.
  - **More is a menu** (rule 4): Floor plan, where a party is moved and tables are
    joined until T06 is built, and Print queue.
  - **A card leads to the floor with its table picked** (`/tables?table=N`), where the
    panel seats, orders, sends and settles. Task 13 (T03) replaces where it leads.
  - **New Order starts a fresh order and opens the order screen,** which asks for its
    table and how it is served. Active Orders opens the day's orders.
  - **The greeting reads the venue's clock,** and before 05:00 it is still the evening.
Left out, and why (rule 3): the bell, Table Requests and QR Orders (nothing rings them
before phase 11), Reserved, the logo's strapline (the `Logo` primitive has none), and
the spot art (a leaf stands in its tile). "Your tables" is "the tables": no table is
assigned to a waiter.
Cost: a waiter has no clock, venue name or version on screen, and sees the link is
down only by Retry now. Two vocabularies for a table's state, one per screen.
---

## ADR-164 - A waiter's top bar says whether the server is there
2026-10 · Status: accepted. After rebuild Task 11; changes ADR-163's cost.
Decision: the status bar's connection word ("Online", "Reconnecting", "No network",
with its icon and detail) is one component, `ConnectionWord`, drawn in the status bar
on a desktop and in the top bar of a waiter's frame, which has no status bar. T01
does not draw it; offline is the default state and a waiter must not have to infer it
from Retry now.
Also settled, with the owner: More, To Clear and "the tables" stay as ADR-163 has
them; one vocabulary for a table's state is decided in Task 17, after D05's own words
are read again.
Cost: one more thing in a bar the design keeps bare. Still no clock, venue name or
version on a waiter's screen.
---

## ADR-165 - A waiter's order screen is the order screen rearranged, not a second one
2026-10 · Status: accepted. Rebuild Task 12a. Task 12 is three tasks.
Decision:
  - **T02 is D03 with its parts moved,** so `OrderScreen`, the sidebar, the grid, the
    rail and its foot each take a waiter's variant. One ticket, one item modal, one
    set of line rules for both.
  - **The top bar is the order's while an order is taken:** an arrow back to the
    tables (the design has no way back; a waiter needs one), the table, the guests and
    how it is served. The table leads to the floor, where one is picked or the party
    is moved.
  - **The rail ends in Save and Send to Kitchen.** Sending is the waiter's one green.
    Pay is "Bill and pay" under the rail's "..." until T05 is built (rule 4).
  - **Save parks the order and returns to the tables.** It is hold, as Save Order is.
  - **Clear removes every line the kitchen has not seen,** each as its own bin would,
    after a confirm. It is not drawn when it would take nothing off, and it takes
    nothing once money is on the order.
  - **Each line says its course** by the menu's name for it.
  - **Popular is where a waiter starts,** on a menu that marks anything popular; All
    Items stays in the list.
  - **Split in three:** 12a this screen, 12b confirm and send (T04), 12c Course View.
Left out, and why (rule 3): Add More (the menu is beside the rail), the rail's tabs
until 12c gives the second one something to show, and the bell. The item modal stays
D04's: T02's sheet with a photo on top is the same choices in another order.
Cost: five components carry a `waiter` branch. The sidebar is 224px, not 212, so the
search's placeholder fits.
---

## ADR-166 - A waiter reads what goes to the kitchen before it goes
2026-10 · Status: accepted. Rebuild Task 12b.
Decision:
  - **A waiter's Send to Kitchen opens T04 as a modal** over the order screen: a
    banner, the ticket, Back and "Send N items to kitchen". The cashier's Send still
    sends at once (D03 draws no step between).
  - **The preview is the plan read line by line** (`sendPreview`): what one Send takes,
    what the kitchen already has, what waits for a later course. It is the client's
    reading of the same rule the button's label uses; the server decides.
  - **Items are counted as lines,** as the design counts them.
  - **After sending: a toast, and the order stays in front,** so more can be added.
Left out, and why (rule 3): Kitchen printer or KDS screen (each item's station decides
where it goes), "Notify me when items are ready" and Undo (nothing does either), and
the KOT number (given when the ticket is made).
Cost: one more tap for every send a waiter makes.
---

## ADR-167 - Course View is the order by course, and where a waiter fires the next one
2026-10 · Status: accepted. Rebuild Task 12c. Task 12 is done.
Decision:
  - **A waiter's rail has T02's two tabs:** Current Order, with its count of lines not
    voided, and Course View. The cashier's rail has none (D03 draws none).
  - **Course View groups the lines by course,** lines with no course first. A course is
    "With the kitchen" when nothing in it waits, "To send" when the next Send takes
    some of it, "Held" when it waits to be fired. Each line says its own state, and
    Ready once the kitchen has bumped it.
  - **Fire is a button on the next course held,** named for the course. It is the same
    command as "Fire course N" under the "...", which stays for the cashier.
  - **Nothing else changes there:** quantities, removal and notes are Current Order's.
T02 names the tab and draws nothing under it, so this layout is ours, made from the
states the server already keeps (ADR-089).
Cost: two places to fire a course on a waiter's rail.
---

## ADR-168 - A waiter's Save leaves a table order open; a waiter fires from Course View
2026-10 · Status: accepted. After rebuild Task 12; changes ADR-165 and ADR-167.
Decision:
  - **Save on a waiter's table order holds nothing.** The order is kept at its table
    line by line already, so Save returns to the tables and the order stays open. An
    order with no table (takeaway, delivery) is still parked, which is how it is found
    again under Orders. The cashier's Save Order is unchanged (`saveParks`).
  - **"Fire course N" is not in a waiter's "..." menu.** Course View has the button, on
    the course it fires, by name. The cashier, who has no Course View, keeps it.
Cost: Save says nothing about items added and not sent; they wait on the order.
---

## ADR-169 - A table has its own screen for a waiter, read beside the order in front
2026-10 · Status: accepted. Rebuild Task 13a. Task 13 is two tasks. Changes ADR-163.
Decision:
  - **`/waiter/table/:id` is T03,** three columns: the table and its notes, its items,
    and where the order stands with what to do next. The top bar there is Back.
  - **It reads the table's order beside the order in front** (`useTableOrder`), as the
    floor's panel does, so looking at a table takes nothing out of the waiter's hands.
    Send, Fire and Add note act on it from here; Add Item brings it to the front.
  - **A card leads here only for a table with one order.** A free table, one to clear
    and one with several orders stay the floor's, whose panel seats, clears and asks
    which.
  - **Order Status is four steps counted from the lines and the kitchen's tickets:**
    placed, sent, in preparation, ready. "Completed" is not drawn: an order that is
    done has left this screen.
  - **Items are grouped by course by default,** each course with its state and Fire on
    the next one held; "Group by Course" turns that off. Each item wears its own state
    where the design has who rang it up.
  - **Send to Kitchen opens the confirm** (ADR-166), as on the order screen.
Left out, and why (rule 3): Guest QR orders, the "Rohan" and "QR" tabs and chips (a
line does not say who rang it up; phase 11), Preferences, Split Bill and Print KOT
(nothing does them from here), Mark as Served (no such step, ADR-090), the pencil,
the thumbnails and the per-item menu. Request Bill is "Bill and pay" and opens payment.
Cost: seating a party is on another screen from the table's own.
---

## ADR-170 - A party is moved and tables are joined from the table's screen, in a modal
2026-10 · Status: accepted. Rebuild Task 13b. Task 13 is done.
Decision:
  - **Transfer and Merge on the table's screen open T06,** a modal: what the party has
    (items, total, guests), the two modes, the other tables as tiles, Cancel and a
    button that says what it will do ("Move to T5", "Join T3", "Let go T7").
  - **The rules are the floor's (ADR-088),** asked of the same routes: a party moves to
    a free table; free tables are joined to it; a joined one is let go. Merge takes
    any number of picks and sends them in turn.
  - **A table that cannot be taken is drawn dimmed with why** (Needs Bill, To Clear,
    Occupied), not hidden, so a waiter sees the floor is full rather than short.
  - **After a transfer the screen follows the party** to its new table.
  - **The floor's own move and join modes stay** (Floor plan under More): they are the
    cashier's, and the only way for a table with several orders.
Left out, and why (rule 3): "Guest QR sessions follow the table" (phase 11) and
"Notify the kitchen of the new table number" (the kitchen reads the table from the
order, so it follows untold).
Cost: two ways to move a party, one per screen.
---

## ADR-171 - A waiter seats and clears from their own home, and Back returns to the table
2026-10 · Status: accepted. After rebuild Task 13; changes ADR-163, ADR-165 and ADR-169.
Decision:
  - **A free table's card opens "Seat Table N":** the guests, "Seat and order" in green
    (it goes on to the menu with the table's order in front) and "Seat N guests". The
    floor's panel has the same two; a waiter's usual one leads here.
  - **A card to clear asks "Clear Table N?"** and clears it.
  - **A table with several orders is still the floor's,** whose panel asks which.
  - **The order screen's back arrow returns to the order's table** (its own screen),
    and to the tables for an order with none.
Cost: seating is in two places, the waiter's home and the floor's panel.
---

## ADR-172 - The waiter's bill is one column, and a payment settles the split
2026-10 · Status: accepted. Rebuild Task 14a (T05); builds on ADR-156 and ADR-159.
Context: T05 draws the bill as one 660px column: the split, a row per guest with
Collect, how the chosen share is paid, and Close table. It has no "Complete split"
step, a QR made for the share, a Wallet tile and "Print bill".
Decision:
  - **`/pay/:id` is `WaiterBill` for whoever works from the waiter's frame,** and the
    desktop payment screen for everyone else. Same route, same ticket, same commands.
    The top bar there is bare: the bill has its own way back, to the table.
  - **The bill starts whole:** "Split between 1 guest", one row, "Whole bill". Most
    tables pay as one, and a bill must not arrive already divided by its covers.
  - **An equal split is live until a payment begins.** The server is asked again as the
    guests are counted. The first "Mark as paid" or "Cash received" settles the shares
    before the money is sent, and they stay settled if the answer is lost: a payment
    recorded unseen then shows as a bill that changed, not as shares divided again.
  - **By item and Custom open the desktop's split panel,** which takes the screen;
    what it completes comes back as the column's rows. T05 draws neither mode.
  - **A share is paid whole, in one payment,** as on the desktop. A part payment is a
    Custom split.
  - **UPI and card have no waiting step of their own.** The tablet is at the table:
    choosing the method is the wait, and "Mark as paid" records it (no gateway, ADR-159).
  - **A tip is one of three percentages or none.** A keyed tip stays the desktop's.
  - **Close table closes the order and returns to the tables,** where the table is
    cleared (ADR-171). It is offered only on a bill paid in full.
Left out, and why (rule 3): "Print bill" before payment (no bill prints until it is
paid and numbered, ADR-078; "Print receipt" appears with the invoice and is a copy
with a manager's approval), the QR, the Wallet tile, and the time a share was paid.
Cost: two payment screens over one set of logic. A split and who has paid are lost
on a reload, as on the desktop (ADR-077).
---

## ADR-173 - What one person did in a shift is added up from the order events
2026-10 · Status: accepted. Rebuild Task 14b, for T08; builds on ADR-094 and ADR-101.
Context: T08 shows a waiter their shift: tables served, orders taken, sales handled,
tips earned, tips by table. The server could say none of it for one person.
Decision:
  - **`GET /api/v1/shift/mine`** answers for whoever asks, in the open shift, and for
    nobody else. It needs only `order.open`. With no shift open it is 409.
  - **An order is theirs if they opened it; money is theirs if they took it**
    (`shift.MineOf`). One person may seat a table and another settle it: the order
    counts for the first, the sale and the tip for the second.
  - **Tables served is the orders they opened that ended at a table.** A table
    seated twice counts twice.
  - **Sales handled is what their payments settled,** less payments voided in the
    shift. Tips are apart. A refund is not taken off: it is a manager's correction.
  - **Tips are those on their own payments, before any pool** (ADR-101 shares them
    out after the close). By table they follow the party to the table it ended at;
    table 0 is the orders with no table.
  - **Nothing is stored.** It is added up on each ask from the streams of the orders
    the shift touched, as the tip report is.
Left out: guest ratings (nothing collects them before phase 11) and the hours on
shift (no timesheets; `first_at` is when they first touched an order).
Cost: a busy shift is read whole on each ask. A waiter whose tables are all opened
by the cashier reads "0 tables".
---

## ADR-174 - The shift summary is the person's own, and End shift signs them out
2026-10 · Status: accepted. Rebuild Task 14c (T08); over ADR-173.
Decision:
  - **`/waiter/shift` is T08,** one column like the bill's: who is signed in, how long
    they have been on, tables served, orders taken, sales handled, tips, and tips by
    table as bars against the largest. It is under More, "Your shift", in the waiter's
    bar. It is asked afresh each time it is opened.
  - **"Tips collected", not the design's "Tips earned".** The figure is the tips on
    the payments they took; the pool shares tips out after the close (ADR-101), so what
    they earn may differ. A line under the figures says what each one counts.
  - **"On shift" runs from the first order they touched,** or from the shift's opening
    before that. Nobody clocks in.
  - **End shift signs the person out.** Closing the shift needs the drawer count and
    `shift.manage`; a waiter has neither. The foot says so.
Left out, and why (rule 3): guest ratings (phase 11), the area beside the role (no one
is assigned an area), Print summary (nothing prints one).
Cost: "End shift" does less than its name on the desktop, where the shift is closed.
---

## ADR-175 - A cook starts a ticket: `ticket.started`, and the board has three states
2026-10 · Status: accepted. Rebuild Task 15a, for D10 and D11; extends ADR-090.
Context: the design's kitchen board has three columns, New Orders, Preparing and
Ready for pickup, with "Start preparing" and "Mark ready". The server knew a ticket
as fired or ready. `internal/domain/kitchen` already had `in_progress` between them.
Decision:
  - **`ticket.started` `{ticket_seq, station}`** moves a ticket from fired to in
    progress, and records when. `POST /api/v1/kitchen/orders/{id}/tickets/{seq}/start`,
    for a kitchen display only, answers with the board, as bump and recall do.
  - **Starting is optional.** A bump still takes a ticket straight from fired to
    ready (ADR-090): a venue that never taps Start loses nothing.
  - **A recall puts a ticket back as new** (fired), not in progress, and forgets
    when it was started: whoever takes it up again starts it again.
  - **It needs no open shift and is taken by a closed or held order,** as a bump is:
    the kitchen works after the bill.
  - **No migration.** `station_tickets` keeps two states, in the kitchen and on the
    pass; a started ticket is written there as `fired`. The board reads which it is
    from the order, as it reads the lines. Its CHECK stays as it was.
  - **`ProjectionVersion` is 8:** a ticket carries `StartedAt`.
  - The board's tickets carry `started_at`.
Left for 15d: who started it (the display has no one signed in) and the day's
figures, which the times now allow (sent to started, started to ready).
Cost: the index cannot be asked "what is in progress" without the orders.
---

## ADR-176 - The kitchen's board is three columns on paper, in the pass's type sizes
2026-10 · Status: accepted. Rebuild Task 15b (D10); over ADR-175, changes ADR-091.
Decision:
  - **Three columns a ticket crosses:** New Orders (sent, untouched, "Start
    preparing"), Preparing (a cook has it, "Mark ready", with a bar filling towards
    the venue's late time) and Ready for pickup (on the pass, while it can be
    recalled, the one just called on top). Each band has its count.
  - **A card is the design's:** an icon for how it is served, where it goes, guests
    and service, when it was placed, the timer, the order's notes, the lines.
  - **The type is the pass's, not the design's.** D10 draws 20px titles and 16px
    lines; a pass is read from three metres (ADR-091). Where it goes is 32px, a line
    26px, an action 64px tall.
  - **The column colours the card; the timer keeps its own tone** and word (calm,
    Waiting, Late), so a late ticket reads late in whichever column it sits.
  - **The board is on paper.** `main.tsx` no longer sets espresso night: D10 is the
    default, and night comes back as a choice on the device (15c).
  - **`--on-mustard` (`#40320D`)**, role `highlight-fg`, is new on the sheet: the ink
    on the Preparing band.
Kept where D10 draws nothing (rule 4): the station's name and Change station in the
header, the all-day counts in a strip under it, and Recall, now on each ready card
(it was one button for the last ticket bumped).
Left out, and why (rule 3): "Called Rohan" (nothing calls anyone, and the display
knows no waiter) and the spot art. Today's stats and Upcoming are 15d, the foot 15c.
Cost: until 15c the kitchen has no dark screen. A new ticket cannot be marked ready
from the board without Start first (the server would take it).
---

## ADR-177 - Night, sound and grouping are the kitchen display's own choices
2026-10 · Status: accepted. Rebuild Task 15c (D11, and the foot of D10); over ADR-176.
Decision:
  - **Espresso night is the same board on the ink surface,** chosen with the foot's
    Night mode. There is one layout: every colour on it is a role, and the roles turn
    with `data-surface`. `main.tsx` sets the surface before anything is drawn.
  - **The foot holds what a display is set to:** Change station (it left the header),
    Sound, Group by table and Night mode. They are kept on the device
    (`mez.kitchen.prefs` in `localStorage`), as its station is: a screen on a wall has
    no one signed in to keep them for. A display starts on paper, silent, ungrouped.
  - **Sound is two notes from the browser's own oscillator** when a ticket arrives in
    New Orders: no file, so it needs no internet and adds nothing to the bundle. A
    display turned on to a full board stays quiet. Turning Sound on is the tap a
    browser asks for before a page may make a sound.
  - **Group by table keeps an order's tickets together** in each column, at the
    place of its first. The columns are otherwise as they were.
  - **A switch says On or Off beside it,** not only by its colour and knob.
Left out, and why: Settings as a screen (a display has no other setting; the button
says "Change station"). Auto advance (nothing here should move a ticket on its own: a
ticket that left the board unmade is a lost dish). Mark all complete (no ticket is
"complete" on this board, ADR-090, and one tap that clears a pass has no undo).
Cost: a display's choices are lost with its browser's storage, and a second display
at the same station chooses for itself.
---

## ADR-178 - A station's day is added up from its ticket index, and rides on the board
2026-10 · Status: accepted. Rebuild Task 15d (D10, "Today's stats"); over ADR-090, ADR-175.
Decision:
  - **The board's response carries `stats`:** tickets sent, in the kitchen, ready,
    the mean seconds from sent to ready, and the slowest ticket by its first dish
    (`kitchen.Stats`, `Service.Board`). No route of its own: the display reads the
    board on every event at its station, so the figures are always the board's.
  - **A day is the venue's trading day** (`shift.day_start`), as on the Orders screen,
    read from `station_tickets` by one new query, `StationDay`. Only the slowest
    ticket's order is loaded, for its dish.
  - **Time runs from sent, not from started.** It is the guest's wait, and a display
    that never taps Start still has it. A ticket recalled and put back counts once.
  - **The counts are of tickets,** which is what a station is sent: an order sent to
    twice is two. The screen says "Total tickets", not the design's "Total orders".
  - **"In progress" is every ticket not yet on the pass,** new or being made: the
    index does not tell the two apart (ADR-175).
  - **The column shows from 1536px wide;** under that the same figures are one line
    under the all-day strip, so three columns keep their width at 1366.
Left out, and why (rule 3): "Upcoming (next 5)". The design lists orders about to
arrive with minutes to go; nothing knows when an order will be sent. What waits for a
later course is a list the app could make, and is rebuild 15f.
Cost: the day is read whole on each board read, capped at 2000 tickets. A ticket sent
before the display existed is in no index and no figure (ADR-090).
---

## ADR-179 - A cook signed in on a terminal has the board at /pass; the wall display stays as it is
2026-10 · Status: accepted. Rebuild Task 15e; extends ADR-090 and ADR-091, changes ADR-148.
Context: the flow map lands the kitchen role on the board after sign-in. `/kitchen`
is the wall display, another app of the bundle that pairs as a kitchen device and
has no one signed in; its API let in only that kind of device. So a cook who signed
in on a terminal was sent to `/kitchen` and asked that browser to pair again.
Decision:
  - **The kitchen's API is used two ways** (`kitchenAccess`). A device paired as a
    kitchen display gets in as before, with no session. Any other device gets in for
    a person signed in on it who holds `kitchen.view`. Nobody signed in is 401.
  - **A move made by a person carries their name:** the event's actor is theirs, not
    0. A wall display's moves are still the device's.
  - **`/pass` is the board inside the terminal app** (`kitchen/CookBoard.tsx`), the
    same `KitchenBoard` with the person's session. It sits outside the shell and the
    shift gate: D10 has its own header and foot, and the kitchen works with no shift
    open (ADR-094). The kitchen role, and a role with `kitchen.view` and no
    `order.open`, land there.
  - **Not `/kitchen`:** `main.tsx` gives that path to the wall display before the
    terminal starts, so the two cannot share it.
  - **The foot has "Lock · name"** for someone signed in, in place of the script
    line. Leaving the board puts the page back on paper: the sign-in is never dark.
  - **The station is kept on the device** here too, under the same key, and changing
    it needs no reload: a terminal's stream is not narrowed to one station.
Because: one pairing per browser was the rule for the back office (ADR-148), and a
kitchen that shares a tablet with the floor should not need a second.
Cost: every seller holds `kitchen.view` (ADR-145), so a cashier or a waiter can now
move tickets from a terminal by asking the API, though no screen of theirs offers it.
A terminal at the board hears every order event at the venue and reads the board on
each. The night, sound and grouping choices on a shared tablet are the tablet's, not
the cook's.
---

## ADR-180 - Moving a ticket is `kitchen.work`; `kitchen.view` is seeing the board
2026-10 · Status: accepted. A fix after rebuild Task 15e; changes ADR-179 and ADR-145.
Context: ADR-179 let a person signed in on a terminal use the kitchen's API with
`kitchen.view`. Every seller holds `kitchen.view` (ADR-145 gave the waiter the
cashier's permissions), so a cashier or a waiter could start, finish and recall the
kitchen's tickets by asking the API. Until then only a kitchen display could.
Decision:
  - **`kitchen.work`** is new: start a ticket, mark it ready, recall it. The three
    POST routes ask a person for it. Reading the board still asks for `kitchen.view`.
  - **A kitchen display is unchanged:** it is paired, not signed in, and does both.
  - **Who holds it:** the owner, the manager and the kitchen role. A cashier and a
    waiter keep `kitchen.view` alone.
  - **Migration 00034** gives it to roles named owner or manager, and to every role
    that holds `kitchen.view` and not `order.open`, whatever it is called: those are
    the people who land on the board (ADR-179). A role that sells is not touched.
  - **Someone who may only look gets the board without its buttons,** and a line
    saying why. The server refuses the move whatever the screen shows.
  - The back office's role form has it under "Kitchen".
Because: a permission must mean one thing. "See the kitchen" had quietly become
"clear the pass" for everyone at the till.
Cost: an owner-made role that both sells and cooks (a "Runner" with `order.open` and
`kitchen.view`) is not given `kitchen.work` by the migration: the owner ticks it.
---

## ADR-181 - "Coming next" is the courses held back for a station
2026-10 · Status: accepted. Rebuild Task 15f (D10, "Upcoming"); over ADR-089 and ADR-178.
Context: D10 lists "Upcoming (next 5)": orders about to arrive, each with minutes to
go. Nothing knows when an order will be sent. The order does know what it holds back:
lines in a course not yet fired, on an order whose earlier course has gone.
Decision:
  - **`order.Held(state, station)`** is a station's lines waiting behind a fired
    course, by course, lowest first. An order with nothing fired holds nothing (it is
    a basket still being rung up); neither does one that is parked or closed.
  - **The board's response carries `coming`,** the first five from the orders open
    longest, and `coming_more`, how many wait behind them. Each has where it is, the
    course and the dishes.
  - **The card says the course where the design has minutes:** "T4 · Mains · 1 ×
    Butter chicken". Which course tells a cook what the waiter will fire next; no
    time is promised.
  - It sits under Today's stats from 1536px wide; under that the one line has its count.
Because: a list of guesses would be worse than none on a pass. This one is exact, and
it is the thing a kitchen asks a waiter for: what is behind this table's starters.
Cost: every open order is projected on each board read to find what it holds (the
open orders list does the same; a venue has tens open, not thousands). A venue that
never uses courses sees "Nothing is waiting for a later course" and no more.
---

## ADR-182 - The back office's frame is D12's: a 72px bar and a sidebar with icons
2026-10 · Status: accepted. Rebuild Task 16a (D12, D14); over ADR-105 and ADR-148.
Context: the back office sat in `SignedInShell`, the pre-design header (logo, section
and device, a Sign out button). D12 draws the frame every management screen sits in.
Decision:
  - **The bar** is 72px: the logo, whether the server is there (D14's word, the
    terminal's `ConnectionWord`), the venue's date and time in a pill, and who is signed
    in, whose menu holds Sign out. The script line shows from 1536px wide.
  - **The sidebar** is 240px, an icon on every entry, 48px each, the current one
    terracotta. The design draws nine entries in one run; the back office has eleven, so
    they keep their groups and labels, and their `g` chords (rule 4). Its script line is
    drawn only on a screen 900px tall or more, so the entries never scroll at 768.
  - **Shared with the terminal:** `ConnectionWord`, `status-logic`, `use-disclosure` and
    `ProfileMenu` moved to `web/shared/shell`. `ProfileMenu` takes its choices as a list:
    the terminal gives it Lock, Close the shift and Print queue, the back office Sign out.
    `SignedInShell` had no other user and is deleted.
  - **Left out, because nothing answers them:** the search and the bell. Orders, Tables
    and Customers are not in the sidebar: the first two are the terminal's screens and a
    back office pairing does not open them; Customers is out of v1.
  - Sections keep their names (Stock, People, Venue settings) until their own screens are
    rebuilt (16f to 16i), so a title and its entry never disagree.
Because: one frame for both apps' top bars, and the pieces that say who and whether the
server is there are now the same code on both.
Cost: the section and device name are no longer in the bar; the sidebar says where you
are and Devices names the device. `docs/design/screens/backoffice-shell.md` describes
the old frame until Task 17.
---

## ADR-183 - The dashboard is today, from the reports that already exist
2026-10 · Status: accepted. Rebuild Task 16b (D12); over ADR-118, ADR-111 and ADR-086.
Context: D12 is today at a glance: four figures, sales through the day, top items,
active tables, staff, low stock, quick actions. The old dashboard was a list of links.
Decision:
  - **No new endpoint.** The sales report for one trading day gives the totals, the
    hours, the items and the staff; it is asked for today and again for yesterday, so
    "12% up on all of yesterday" compares two server figures and nothing is summed in
    the browser. Today so far is set against the whole of yesterday, and says so: a
    morning reads as down. The floor in service gives the tables, the stock list what is low. Each is
    read again every minute.
  - **The report now says each named person's role** (`roles`, beside `names`). It names
    staff by number and the staff list by public id, so the browser could not join them.
  - **Each card is for whoever may read it:** sales cards need `reports.view`, tables
    `order.open`, stock `stock.manage`; a shortcut shows to whoever may open its section.
  - **"Total orders" is the day's bills,** as the report counts them: an order still open
    is not in it. The average is the total over the bills.
  - **Sales overview is one green bar an hour,** 8 AM to 10 PM or wider when a bill fell
    outside, on a round axis (`axisTop`). The design stacks dine-in, takeaway and
    delivery; the report's hours are not cut by service, so there is no legend.
  - **Money keeps its minor units** ("₹800.00", where the design draws "₹24,580"). The
    axis alone is short ("₹4K"), through `formatMoneyCompact`, for round ticks only.
  - **Left out:** the "Today" pickers (the dashboard is today; Sales reports has ranges),
    item photos in the list, "View floor" (the floor is the terminal's), the spot art.
    "Add menu item" reads "Edit the menu", because that is where it goes.
  - Low stock is a chip vocabulary of its own (`stock`: Low, Out), not free text.
Because: a dashboard that adds up its own figures would one day disagree with the
report beside it.
Cost: two report reads a minute, each projecting the day's orders (ADR-118's cost,
small for a day). Staff are ranked by the bills they opened, not the tables they served.
---

## ADR-184 - The sales report says how it was served, and counts its guests
2026-10 · Status: accepted. Rebuild Task 16c1 (D15); over ADR-118 and ADR-146.
Context: D15 stacks its trend by dine-in, takeaway and delivery, draws orders by type
and a "Total guests" figure. The report grouped by channel (where an order came from),
not by service (how it is served), and counted no guests.
Decision:
  - **`by_service`** on the report: the bills by `order.State.Service`, grouped as the
    channels are. It is on the range's totals and on each period of the series.
  - **`sales.covers`**: the guests on the bills invoiced, as each order counted them.
  - Both are in `zreport.Report`, which a Z-report shares, so a Z-report built from now
    on carries them too. One issued before has no `by_service` and zero covers; the
    Z-report screen reads neither yet (16j decides whether to show them).
  - The hours of the heatmap are still not cut by service; the dashboard's bars stay one
    colour (ADR-183).
Because: the order has known how it is served since ADR-146 and nothing added it up.
Cost: no CSV sheet for service types yet. A party nobody counted adds no guests, so
"Total guests" undercounts a venue that does not key covers.
---

## ADR-185 - Sales reports is D15 on top, with the old reports under "In detail"
2026-10 · Status: accepted. Rebuild Task 16c2 (D15); over ADR-118 to ADR-123.
Context: D15 is a picture of a range: four figures, a trend, two rings, top sellers and
payments. The screen built in phase 10 was a column of tables, each with its CSV.
Decision:
  - **The top is the design's.** The title with the days counted under it; the range
    as one pill; Export; Total sales, Total orders, Average order value and Total
    guests; Sales trend stacked by service; Sales by category; Top selling items; Order
    type breakdown; Sales by payment method.
  - **The range pill opens what the old screen laid out in a row:** the quick ranges,
    From and To, and By (day, week, month). It starts at seven days, as before.
  - **Export lists every report's CSV** (ADR-119). The tables below keep their own
    Download CSV buttons.
  - **Each figure is set against the same number of days just before,** a second read
    of the report: "Up 12% on the 7 days before". Whole days against whole days, unlike
    the dashboard's today so far (ADR-183).
  - **A ring's percentages are whole and come to 100** (largest remainder); beyond six
    categories the rest are "Others". Orders by type is by bills. The ring's centre
    shows the total short ("₹2.6K"); the exact total is the first tile.
  - **Kept under "In detail" (rule 4):** the range's figures, each period, payments and
    channels, items, categories and staff in full, the heatmap, corrections and closing
    stock. Items, categories and closing stock move to 16d, staff to 16e.
  - **Left out:** All Outlets (a venue is one outlet), the order type filter (the
    server does not filter a report), item photos, and the QR codes card (phase 11).
  - The old bar chart over "Sales over time" is gone; the trend is that chart.
Because: the owner reads the picture first and the tables when a figure looks wrong.
Cost: two report reads per range. Chart colours beyond green, terracotta and mustard
are the sheet's quieter inks (the warning ink, faint, the strong hairline), by CSS
variable in a style, since a conic gradient cannot be a class.
---

## ADR-186 - A wastage report: what was written off over a range, at cost
2026-10 · Status: accepted. Rebuild Task 16d1 (D16); over ADR-111, ADR-124 and ADR-125.
Context: D16 draws "Wastage this week". Write-offs were in the ledger with a reason and
the cost they had then, and nothing added them up.
Decision:
  - **`GET /api/v1/reports/wastage?from=&to=`** (`reports.view`, as the closing stock
    report): the wasted movements keyed in a range of trading days, by stock item and
    by reason, the largest loss first, with the total.
  - **Each entry is valued at the cost it carried** (ADR-124), never today's. An entry
    with no cost is counted in `uncosted` and left out of every value, so a total is
    never read as the whole of it.
  - **An entry belongs to the trading day it was keyed in,** by the venue's zone and day
    start, placed with `shift.TradingDay`. Not the shift's day: stock written off by
    hand after the close has no shift.
  - By item, not by category as the design draws: a stock item has no category.
Because: the figure an owner wants from wastage is rupees, and the ledger already had
them.
Cost: the day start is read as it stands now, so a changed day start re-places old
entries at a day's edge. The stock service reads that setting by its key itself: the
order service imports stock, so stock cannot import the services that own the key
(a test moves the day start and watches an entry change days). No CSV yet.
---

## ADR-187 - Items and inventory is D16; the reports share their days
2026-10 · Status: accepted. Rebuild Task 16d2 (D16); over ADR-185.
Context: D16 is a tabbed panel (Item, Inventory, Staff, Customer) with item
performance, wastage and stock health. D15's own sidebar lists the same reports. The
back office has one sidebar, with one "Sales reports" entry.
Decision:
  - **The reports are tabs under that one entry:** pills above the title, Sales
    (`/sales`) and Items and inventory (`/sales/items`). D16 draws items and inventory
    on one screen, so they are one tab. Staff joins with its screen (16e); Customer is
    out of v1.
  - **The days are picked once.** The range lives above the routes (`report-range.tsx`),
    and both screens read the sales report through `use-range-sales.ts`, so moving
    between tabs keeps the days and asks the server nothing new.
  - **Item performance** is the eight items that took most: the diet mark from the menu
    as it stands, quantity, revenue, and how many sold beside the same number of days
    before. An item that did not sell then reads "New in these days".
  - **Cost and Margin are not drawn.** Nothing says what an item cost to make: a sold
    movement is tied to an order, not a dish, and a recipe is today's. Split off as 16d3.
  - **Wastage** is ADR-186's report as bars by stock item, with the total, the reason
    that cost most, and how many entries had no cost.
  - **Stock health** counts the active stock items: healthy, low (at or under the level
    with some left) and out (none left), for whoever holds `stock.manage`. "Raise
    purchase order" is "Go to stock": purchasing is out of v1.
  - **Moved here from Sales reports' "In detail":** what sold by item and by category,
    and the closing stock. Who sold it stays on Sales until 16e.
  - `DietMark` moved to `web/shared/ui`, for the terminal and the back office both.
Because: one entry in the sidebar, one range, and each report a tab away.
Cost: the tab is not in the sidebar's `g` chords. The stock health card is today's
stock whatever days are picked.
---

## ADR-188 - Staff reports is D17's table; tips are by who took the payment
2026-10 · Status: accepted. Rebuild Task 16e (D17); over ADR-101, ADR-173 and ADR-187.
Context: D17 is "Staff performance" (orders, sales, tips, rating) over customers,
regulars and feedback. The report had bills by who opened them and one tips total.
Decision:
  - **`tips_by` on the report:** each person's tips on the payments they took, the
    most first; a voided payment takes its tip back. The same rule as a waiter's own
    shift summary (ADR-173). It is in `zreport.Report`, so a Z-report built from now on
    carries it; one issued before has none.
  - **Staff is the third report tab** (`/sales/staff`), sharing the range (ADR-187).
  - **The table** is everyone who opened a bill or took a tip: role, orders (the bills
    they opened), sales, average bill and tips, the largest sales first. Someone who
    only took payments has tips and no bills: a bill counts for whoever opened it.
  - **"Removed in error"** beside it: the lines each person rang up by mistake and took
    off, which the report already had. **Tips** is the range's total.
  - **"Who sold it" moved here** from Sales reports, with its CSV.
  - **Left out:** Rating (nothing rates anyone until the guest channel, phase 11);
    Customers, Top regulars, Feedback themes and Rating spread (out of v1, D21).
    "Tips are paid out weekly" is not said: a pool is per shift (ADR-101).
Because: what an owner asks of this screen is who sold what and who is owed which tips.
Cost: tips taken is not tips paid: a pooled shift shares its tips out by the venue's
rule, and that is on the shift's Z-report. A waiter whose tables the cashier opened
shows less than they served (ADR-173's cost, unchanged).
---

## ADR-189 - A dish's cost is what the ledger moved for it; a margin only when that is whole
2026-10 · Status: accepted. Rebuild Task 16d3 (D16, Cost and Margin); over ADR-112,
ADR-124 and ADR-187. The owner chose this over "today's recipe at today's cost".
Context: D16 draws what each item cost and its margin. A sold movement carried its
order and the cost of the stock then, and not the dish: a sale added the whole order's
usage up by stock item before it wrote anything.
Decision:
  - **A sold movement keeps its dish** (`stock_movements.menu_item_id`, migration
    00035). `stock.Usage` and `stock.SaleDeltas` are by stock item and dish; a
    modifier's stock goes to its line's dish. 0 is "no dish": every movement that is
    not a sale, and every sale from before.
  - **The ledger also keeps how many of a dish it costed** (`stock_dish_sales`,
    migration 00036, append-only): a row a close, the units of each dish that used
    stock, less what the order's earlier closes recorded. A dish with no recipe adds
    none.
  - **Cost** is the dish's sold movements over the range, each at the cost it carried
    (ADR-124), stock put back taken off. Never today's recipe, never today's cost.
  - **`GET /api/v1/reports/items`** (`reports.view`) sets each item's net sales beside
    its cost and says how far the cost goes: `full`, `part` or `none`.
  - **A margin is given only for `full`:** every movement had a cost, the range has no
    sales from before dishes were kept, no stock was put back for the dish (a put-back
    is valued at the cost of the day it went back, not the day it was taken), and the
    units the ledger costed are the units sold. Anything else shows what cost is known and no margin: a dish that sold once
    before it had a recipe and once after would otherwise set one unit's cost against
    two units' sales. A walk through the dev venue showed exactly that, at 93%.
  - **Margin is net sales less cost,** net being after discounts and before tax, and a
    percentage of net in hundredths of a percent, worked out on the server in whole
    numbers. The screen's Revenue column became Net sales so the three agree.
  - An order closed before 00035 and closed again after has its stock put back under
    no dish and taken under its dishes: the stock on hand does not move.
Because: the closing stock and the wastage are valued at the cost of their day; a
margin that moved when someone edited a recipe would be the one figure here that could
not be trusted for the decision it is for.
Cost: nothing from before the migration is ever costed by dish, and the screen says
from which day costs are kept. A movement is placed on the day its order closed and a
sale on the day it was invoiced; where those differ at a day's edge the units do not
match and the row reads `part`, which is the safe side. So does a dish whose line was
voided after a reopen, even when its cost had not changed. Migration 00037 indexes the
ledger by kind and time for these reads. No CSV carries cost yet.
---

## ADR-190 - Menu management is D18: categories, the items, and the editor side by side
2026-10 · Status: accepted. Rebuild Task 16f (D18); over ADR-106 to ADR-110 and ADR-147.
Context: D18 is four columns: categories, the menu's items, the item being edited, and
its photo, tags and Save. The screen from phase 10 was a table of one category with a
modal form.
Decision:
  - **The item form is inline,** beside the list. Picking an item opens it; Add New Item
    opens the same form empty, in the category shown. The modal is gone.
  - **Categories** lists All items and Popular ahead of the categories, each with its
    count, and "+" adds one. Rename, move and retire sit under the list for the chosen
    category (rule 4).
  - **The list** has each item's photo or a plain tile, name, price and where it stands
    (a chip: Active, Sold out, Retired), and "..." for Sold out today, Retire and, in a
    single category in menu order, Move up and down. Its order is the menu's, A to Z or
    by price.
  - **Search** looks through every category by name; the filter beside it is by where
    an item stands. Both are the browser's: the editor already has the whole menu.
  - **The editor's tabs** are Choices and Kitchen and tax (the station and the course).
    A refusal for a field on the other tab switches to it. The HSN code is with the
    main fields where the venue needs one: every new item would be refused without it.
  - **Photo and tags** are beside the form from 1700px wide and under it below. Tags
    are the mark on the card, Gluten free and Popular (ADR-147).
  - **Left out:** Cost price (a dish's cost is its recipe's, ADR-189); Show in online
    ordering and Happy-hour pricing (neither exists); Nutrition; stock per menu item
    (stock is by ingredient); combos; drag to reorder. "Delete item" is Retire item: an
    item with sales is never deleted. Categories wear one icon: nothing says which
    picture a category is.
  - Choices and Stations and courses stay as views of their own, as pills (rule 4).
Because: editing a menu is going down a list and fixing each line; a modal a row made
that two clicks and a wait every time.
Cost: opening another item, or leaving the screen, drops what was typed and not saved,
without asking. `docs/design/screens/backoffice-menu.md` describes the old screen until
Task 17.
---

## ADR-191 - Inventory is D19: the stock table with its levels, and the week's wastage
2026-10 · Status: accepted. Rebuild Task 16g (D19); over ADR-111 to ADR-114, ADR-124
and ADR-186.
Context: D19 is a table of ingredients (in stock, reorder at, unit cost, supplier, a
level bar, a status) with purchase orders and the week's wastage beside it, and tabs
for purchase orders, suppliers and a wastage log. The Stock screen was a table with
five buttons a row.
Decision:
  - **The section is called Inventory,** the design's word, in the sidebar and on the
    screen. Its path stays `/stock`, and the code's word stays stock.
  - **The table** has Ingredient, In stock, Low at, Unit cost, Stock level and Status.
    The bar is on hand against three times the low level, so an ingredient at its level
    reads a third full; one with no level has no bar. The chip is Healthy, Low or Out:
    out is none left, whether or not a level is set.
  - **A row's actions are under its "..."** (Receive, Log wastage, Count, History,
    Edit), the same menu as the menu's list (`RowMenu`, shared).
  - **Stock count and Log wastage beside the title** ask which ingredient, then open
    the form the row's menu opens.
  - **Wastage this week** beside the table is ADR-186's report for the last seven
    trading days, by ingredient, with the total set against the seven days before. Less
    waste is the good direction, so a fall wears the green. It needs `reports.view`;
    without it the table has the whole width.
  - **Retired ingredients are off the table** unless "Show the N retired" is ticked.
  - **Left out:** New purchase order, Open purchase orders, and the Purchase orders
    and Suppliers tabs (purchasing is out of v1); the Supplier column; the Wastage log
    tab (the report is on Items and inventory). The terracotta button is Add ingredient.
  - Recipes stays a view of its own, as a pill beside Stock levels (rule 4).
Because: the table answers "what do I need to buy" at a glance, which the old one did
not: its only signal was the word Low.
Cost: three times the low level is our own rule for a full bar; nothing in the venue
sets it. `docs/design/screens/backoffice-stock.md` describes the old screen until Task
17.
---

## ADR-192 - Staff and roles is D20: the staff beside the role's switches
2026-10 · Status: accepted. Rebuild Task 16h (D20); over ADR-115 and ADR-145.
Context: D20 is the staff in a table (person, role, shift, state, PIN) with the picked
person's role beside it as eight switches, Reset and Save role. The People screen had
two views, a staff table and role cards, and a modal for a role.
Decision:
  - **One screen.** The staff table on the left; beside it the roles as pills and the
    chosen role's permissions as switches, saved in place. Picking a person shows their
    role and tints everyone who holds it.
  - **Every permission is a switch,** in the groups the old form had: a role grants any
    of the server's permissions, not the design's eight. A switch says On or Off as
    well as showing it. One for a permission you do not hold cannot be moved (ADR-115).
  - **The role's name is a field in the same card;** a role nobody holds can be deleted
    from it. A new role is still a modal, then it is the one shown.
  - **A row's actions are under its "..."**: Edit, New PIN, Deactivate or Bring back.
    Nobody is offered Deactivate on themselves.
  - **The section is called Staff** in the sidebar; the screen's title is the design's
    "Staff & roles". The path stays `/people`.
  - **Left out:** Roster and the Shift column (no timesheets), "on shift now", a phone
    number, and the PIN column: a PIN is kept as a hash, so none of it can be shown. The
    design's note that anything switched off asks for a manager's PIN is true of the
    corrections only, and is said there.
  - Deactivated staff are off the table unless "Show the N deactivated" is ticked.
Because: a role is read far more often than it is changed, and the question on this
screen is "what can this person do".
Cost: a change to a role that is not saved is dropped when another role is picked.
`docs/design/screens/backoffice-people.md` describes the old screen until Task 17.
---

## ADR-193 - Settings is D22's frame around the sections that exist
2026-10 · Status: accepted. Rebuild Task 16i (D22); over ADR-129 and ADR-136 to ADR-140.
Context: D22 is a 290px rail (Outlet, Taxes & charges, Printers & devices, Tables & QR
codes, Integrations, Language & theme, Backup & restore) beside every section's card on
one screen, with one Discard and Save. Settings was already a rail beside one section,
each section saving itself (ADR-129).
Decision:
  - **The frame is the design's:** the title "Settings", the rail as a card with an icon
    on every entry, the current one terracotta, and the version and licence in a box at
    its foot. The section's cards fill the rest of the width.
  - **A section is still a page that saves itself.** One Save for everything would make
    a refusal about one form hold back another, and ADR-129's reasons stand.
  - **The rail links out to the settings that are screens of their own:** Printers,
    Devices, and Tables and floor plan, each shown to whoever may open it. Leaving a
    section with unsaved changes by one of these asks first, as moving between sections
    does.
  - **Venue's fields are two across,** as the design lays out an outlet's details.
  - **The sidebar's entry is Settings,** not Venue settings.
  - **Left out:** Integrations (there are none); Language & theme (one language, and
    night is the kitchen display's own switch); QR codes (phase 11); the outlet picker
    in the bar (a venue is one outlet); an address, a phone and service hours (the
    venue keeps none); a service charge and the tax switches (tax is a profile and its
    classes, ADR-131). Backup stays "Soon" until phase 12.
Because: the sections were built to the right shape already; what the design adds is
the frame and one place to find every setting.
Cost: Printers, Devices and Floor plan are in two places, the sidebar and this rail.
`docs/design/screens/backoffice-settings.md` describes the old frame until Task 17.
---

## ADR-194 - D23 is two screens: Close the day in two steps, then the Z-report with its slip
2026-10 · Status: accepted. Rebuild Task 16j (D23); over ADR-093 to ADR-100.
Context: D23 draws the close ("Step 2 of 2": the cash reconciliation, a note, Print,
Close day) with the Z-report as a slip of paper beside it. The close was a counter and
a reveal beside a rail; the Z-report was a column of cards.
Decision:
  - **Close the day is two steps** on the order screen's app: "Step 1 of 2" counts the
    drawer, "Step 2 of 2" is the Cash reconciliation card (the opening float, Expected
    in drawer, Counted, how it came out in a tinted notice, the note), then Close the
    day. The buttons are under the step, as drawn; the shift's facts sit in a rail.
  - **The count stays blind** (ADR-093). The design shows the expected figure above the
    count; here it is on step two only, once the count is in.
  - **The slip is on the Z-report, not beside the close.** There is no report until the
    shift is closed, and a slip of figures still moving would be a guess. The close
    lands where it did, and the report it issued opens with the slip.
  - **The Z-report** (`shared/zreport`, both apps) has the slip beside the report in
    full: gross, discounts, net, each tax, total and tips; payments by method; orders,
    covers, the average order, refunds and voids; expected, counted and the variance
    in words; who closed it. The full sections are kept (rule 4).
  - **The new figures are shown where a report has them** (ADR-184, ADR-188): "By how it
    was served" among the groups, tips by who took the payment under Payments, covers
    on the slip. A report issued before them shows none of the three.
  - `averageMoney` is in `shared/money`: the average order is the one figure a screen
    works out, a total over a count, and both apps now use the same one.
  - **Left out:** cash sales, refunds and paid out as rows of the reconciliation (the
    count's answer carries the expected figure, not its parts; they are on the
    Z-report's Cash drawer); Print before the close (there is nothing to print); "lock
    terminal" (closing does not sign anyone out); the slip's monospace face (the sheet
    has none: figures are Archivo).
Because: the design's one screen mixes a step that has not happened with the paper it
produces. Two screens say the same things in the order they become true.
Cost: the slip is a second reading of figures the cards beside it already give.
`docs/design/screens/zreport.md` and `terminal-close.md` describe the old screens
until Task 17.
---

## ADR-195 - Printers, Devices and Audit take Task 16's patterns; Floor plan and Tips do not
2026-10 · Status: accepted. Rebuild Task 16k, a pass before Task 17.
Context: the design draws nothing for Audit, Floor plan, Printers, Devices or Tips, so
Task 16 left them as they were inside the new frame. Printers and Devices were lists of
cards with a row of buttons each, centred in the page; Audit had its days as two fields
in a row. Beside Inventory and Staff they read as another app.
Decision:
  - **Printers and Devices are tables in a card**, as Inventory and Staff are: a chip
    for where a thing stands, and what can be done to it under the row's "...". The
    page head is the shared one (title, a line of counts, the action at the right).
    Nothing a screen could do was taken away, and nothing was added.
  - **Devices is one table**: in service first, the revoked at the foot with nothing
    left to do to them. A new chip vocabulary, `device`: In service, Quiet (not heard
    from for a day), Revoked. "Quiet", not "Offline": the server knows when a device
    last asked for something, not whether it is switched on.
  - **A printer has no chip of its own.** The server cannot tell whether a printer is
    reachable; its last job is the evidence, so the row shows the print chip for it.
  - **What an action answered is a toast; what it was refused for is a line over the
    table**, named for the printer, as Staff does it.
  - **Audit has the reports' head**: the title block, and the days in the range pill.
    The pill is `sales/RangePill.tsx`, given its range: the reports share
    one, Audit keeps its own, since it is opened on a filter from a link.
  - **Floor plan and Tips are left alone**, but for the stray "Back office" link over
    the title on Floor plan and Printers. Floor plan is a grid editor, not a list;
    Tips is a section of Settings and already in its frame.
Because: these are the screens an owner sets a venue up on, on the first day.
Cost: `docs/design/screens/backoffice-printers.md`, `backoffice-devices.md` and
`backoffice-audit.md` describe the old layouts until Task 17.
---

## ADR-196 - Task 17 is split in eight, and a spec is written from the screen's code
2026-10 · Status: accepted. Rebuild Task 17.
Context: Task 17 asks for `docs/design/screens/*.md` rewritten from what was built,
and a walk as each role. There are about thirty screens. Twenty-one have a spec, all
describing the layouts the rebuild replaced; the waiter's six screens, the dashboard,
Open the shift and the terminal's frame have none. One session cannot read them all.
Decision:
  - **Eight parts**, 17a to 17h in `rebuild.md`: seven of specs, grouped by who uses
    the screens, and the walk last, so it is made against specs that are true.
  - **A spec is written from the code as it stands**, not from the design file and
    not from the spec before it. The design is cited under Reference by its screen
    and line, with what it draws that the screen leaves out and why, and what the
    screen keeps that the design does not draw (rule 4).
  - **A spec keeps its file name** where code cites it (`terminal-close.md` is named
    in five source files), even where the screen's own name has changed. New files
    are for screens that never had one.
  - **The frame has a spec of its own**, `terminal-shell.md`: where each role lands,
    the two frames, the paths and the shift gate. It was nowhere but in ADRs.
  - **Open the shift leaves `terminal-close.md`** for `terminal-open-shift.md`: they
    are two screens for two people at two ends of the day.
  - **No code changes in 17a to 17g.** What a spec's writing shows to be wrong goes
    into `STATE.md` for the walk.
Because: the specs are what the next phase reads before it touches a screen, and a
spec that describes the old layout is worse than none.
Cost: until 17g is done, `docs/design/screens/` is part old and part new;
`rebuild.md` says which. Comments in `shift/` still cite `terminal-close.md` for the
opening float.
---

## ADR-197 - Closing the day has one door, under the profile, for whoever may close
2026-10 · Status: accepted. Rebuild Task 17h, its first fix; found writing the specs (17b).
Context: the Orders sheet under the order screen's "..." still offered **Close the
day** to anyone, from before the rebuild. The flow map puts closing under the profile
menu, and there it asks for `shift.manage`. The drawer count itself needs only
`payment.take`, a submitted count cannot be changed, and since ADR-148 the shift gate
holds every path on every device at the close once the drawer is counted. So a cashier
could go through the sheet's door in the middle of service, submit a count, and stop
the whole venue until someone who may close arrived.
Decision:
  - **The Orders sheet has no Close the day.** The profile menu's **Close the shift**
    is the only way in, shown to someone with `shift.manage` while a shift is open.
  - **The server is unchanged.** The count still needs `payment.take`. Whether a count
    should need more is its own question; the screen no longer leads there.
  - **A venue whose cashier closes alone** switches on "Close the shift" for that role
    on Staff. It is then a choice the owner made.
Because: a door that leads somewhere a person cannot finish, and that locks everyone
else out on the way, is worse than no door.
Cost: someone with `payment.take` and not `shift.manage` can still reach
`/shift/close` by typing its path, and count there. Nothing on screen leads to it.
---

## ADR-198 - The old words are off the screens, and "ticket" is kept for the kitchen's
2026-10 · Status: accepted. Rebuild Task 17h1; what writing the specs found (17b to 17g2).
Context: the rebuild's Words table (till and register to the order screen, ticket to
order, hold to save) was applied to each screen as it was rebuilt. The messages behind
the screens were not on any screen's task: refusals in `order-messages.ts`, hints in the
back office's forms, a role's permissions in words, five of the server's own messages.
A sweep found about thirty still in the old words, one of them a note to ourselves
("Refunds come with Task 6.") shown to a cashier.
Decision:
  - **An order is an order everywhere a person reads it.** "This ticket is on hold"
    is "This order is saved"; "changed on another till" is "on another screen".
  - **"Ticket" stays where it means the kitchen's.** What a station is sent, what a
    cook starts and marks ready, what a kitchen printer prints: "Kitchen tickets",
    "Total tickets", "Start tickets and mark them ready". Those are tickets.
  - **The device is a terminal.** A device's kind reads "Terminal", as the pairing
    screen already said ("Pair this terminal"). The order screen is what is on it.
  - **A bill that owes the guest** says "A manager settles that on the Orders
    screen.", as the waiter's bill already did.
  - **Error codes, event types, routes, setting keys and file names keep their names**
    (`tickets_open`, `ticket.sent`, `ticket_late_minutes`, `use-ticket.ts`). They are
    stored or cited, and nobody reads them on a screen.
  - **Two things nothing used are deleted:** `print/PrintStatus.tsx` (the frame draws
    its own Not printed pill) and `lastBumped` (Recall is on each ready card).
Because: a word that changes between a button and the refusal under it reads as two
different things.
Cost: the printed Z-report still labels its device "Till" (`escpos/zreport.go`); a
printed layout is its own change, with a printer to check it on. The dev-only gallery
keeps its sample strings.
---

## ADR-199 - The walk is made on a fresh venue from the current build; what it found
2026-10 · Status: accepted. Rebuild Task 17h2.
Context: Task 17 ends with a walk as each role. A dev server was already running, from
a binary several commits old, on the dev venue with a day of test data in it.
Decision:
  - **The walk has its own venue**: `mez setup -demo-menu` into a scratch directory,
    served by the binary `make check` builds (the embedded, production bundles) on its
    own port. It starts where a day does, with no shift open, and leaves the dev venue
    and its server alone.
  - **Walked**, at 1920x1080 and for the waiter 1366x1024: pairing all three apps;
    Meera opening the shift, a counter order by "+" and by the item modal, Send, cash
    with a tip and change, the invoice, New Order; the wall display starting and
    readying tickets, and night; Kabir seating a table, ordering over two courses,
    Confirm and send, the table's screen, Fire, a bill split in two by UPI and cash,
    Close table, Clear, his shift summary; Arjun refunding an item on Orders, the blind
    count, a short drawer and its note, the close, Z-report 1; Ravi at `/pass`; and in
    the back office the dashboard, Z-report 1 with its exports, Sales and Staff
    reports, Audit, and the guard on Menu.
  - **A refund says who authorised it.** `RefundIssued` was built without
    `AuthorisedBy`, though its credit note carried it, so Audit read "Not recorded" for
    every refund (against ADR-117). It carries it now. Refunds already stored keep
    reading "Not recorded": events are not rewritten.
  - **The Z-report's Tax card says it is after credit notes** (`taxNote`). The domain
    is right and unchanged: Sales' Tax is the tax invoiced, the Tax card the tax
    invoiced less the tax credited. Side by side with no words they read as an error.
Because: a walk on stale code or on a venue full of test data proves little; and a
figure on a money report that looks wrong is a problem even when it is right.
Cost: the walk saw each role's spine once. It did not see card payments, a split by
item, voids, comps, discounts, transfer and merge, delivery, a reprint, or any width
under those two. `STATE.md` keeps what is still unseen.
---

## ADR-200 - STATE.md is short again: the history and the known-broken list are files of their own
2026-10 · Status: accepted. Rebuild Task 17h3.
Context: `STATE.md` says of itself that it is kept under 60 lines, since it is read
first in every session. It had grown to 831: every task's summary kept under "Before
it", and a known-broken list of some 430 lines, most of it lists of what nobody had
yet seen in a browser. A session read all of it before doing anything.
Decision:
  - **`docs/HISTORY.md`** holds what each task did, newest first, moved whole. When a
    task is done, the entry before it goes to the top of that file.
  - **`docs/KNOWN.md`** holds what is broken, left out or not seen by eye, by screen
    and by ADR, moved whole. A task edits its screen's entry there, and puts in
    `STATE.md` only what the next task must know.
  - **Neither is read at the start of a session.** `STATE.md` says where they are; a
    task reads the one entry it needs, as it reads one phase file and one screen spec.
  - **An item leaves a "not seen by eye" list only when someone has seen it.** The
    walk of 17h2 struck what it saw, fifteen places, and three statements that were
    no longer true (two about layouts since rebuilt, one about a spec since
    rewritten). Nothing was struck for being old.
  - **What wants the owner's word is in `STATE.md`**, under Open questions, a line
    each, with the detail at the top of `KNOWN.md`.
Because: the instruction in `CLAUDE.md` is that context spent on files a task does not
need is the main way this build fails, and the handoff file had become the largest of
them.
Cost: two more files to keep. `CLAUDE.md`'s table of what to read does not name them
yet; `STATE.md`'s own header does. The owner may want a row for each there.
---

## ADR-201 - The relay: frames on one socket, signed venue keys, a path per venue
2026-10 · Status: accepted. Phase 11, Task 1.
Context: ADR-008 settled that a venue dials out to a stateless relay and guests reach
it there. It left open what is spoken on the socket, how a relay that stores nothing
knows a venue, and how a guest's URL names one.
Decision:
  - **Where it lives.** `internal/relay` is the server, `internal/relay/wire` the
    framing, `cmd/relay` the binary. `wire` is its own package so the tunnel client
    (Task 2) imports the protocol and not the server. Neither imports `internal/db` or
    `internal/api`: the binary is 6.7 MB with no SQLite in it. No new dependency.
  - **Frames.** One WebSocket binary message is one frame: a type byte, an 8-byte
    stream id, a payload. A stream is one HTTP exchange. The relay sends `Request`
    (JSON head), `Body` pieces and `End`; the venue answers `Response` (JSON head),
    `Body` and `End`; either side abandons a stream with `Reset`. Bodies are raw bytes,
    32 KB a frame at most, so nothing is inflated on the way to a phone. The type
    numbers are a contract with installed venues: add, never renumber.
  - **A venue key is signed, not stored.** `mzr_<venue>.<key id>.<mac>`, the mac an
    HMAC-SHA256 over the other two under `RELAY_SECRET`. `relay issue-key VENUE_ID`
    prints one. `RELAY_REVOKED` lists venue ids and key ids to refuse, which is the one
    way to turn a key off, and takes a restart. Every refusal is the same 401.
  - **A guest's URL is `/v/<venue>/...`.** The relay strips the prefix and forwards the
    rest byte for byte, with `X-Forwarded-Prefix: /v/<venue>`, `X-Forwarded-For`,
    `-Host`, `-Proto` and `X-Request-Id` set by the relay and a guest's own copies of
    them dropped. So a table's QR code says `order.<domain>/v/<venue>/t/<code>`, not
    ADR-008's `/t/<table>`: with no table of venues, the venue has to be in the URL. A
    venue id is 1 to 63 of `a-z`, `0-9` and `-`.
  - **Every venue shares one origin, so the relay keeps each to its prefix.** A cookie
    is rewritten to `Path=/v/<venue><its path>` and loses any `Domain`; a redirect to
    `/x` becomes `/v/<venue>/x`; `Service-Worker-Allowed` and `Clear-Site-Data` are
    refused. The venue's headers are made canonical before any of this, since they
    arrive as JSON spelled as the venue likes. A venue writes paths as if at the root,
    and its pages use relative URLs.
  - **The rate limit is per venue**, a token bucket held on the venue's socket: 1200 a
    minute, 300 at once, by `RELAY_VENUE_RPM` and `RELAY_VENUE_BURST`. It protects the
    venue's machine, which is also the counter. The per-table limit is the venue's
    (Task 6), where a table is a known thing.
  - **Limits that keep one guest from hurting the rest.** 256 streams a venue, a 1 MiB
    request body, 30 s for the venue to start answering (504), and a 32-frame queue per
    guest: one who falls further behind loses the request, so the socket is never left
    unread. A response cut after its head ends the connection (`http.ErrAbortHandler`)
    so a broken download cannot read as a whole one.
  - **The newest connection for a venue wins**, and the old one is closed with 4001
    (`wire.CloseReplaced`) so a client can tell and not fight for the socket. The
    relay pings every 20 s to find a line that dropped without a close.
  - **Configured by the environment, serving plain HTTP.** TLS is the proxy's in front,
    and `RELAY_TRUST_PROXY=true` says there is one, which is the only time
    `X-Forwarded-For` is believed, and then only its last entry.
  - **A guest the relay cannot serve gets a page staff can explain** ("Ordering is
    offline ... order at the counter") when it asks for HTML, and the API's error
    envelope otherwise, with codes of the relay's own (`venue_offline`,
    `relay_rate_limited`). A venue that is not connected and one that does not exist
    get the same 503.
Because: the relay is the one part of this that strangers reach and that serves many
venues at once, so each choice is the one that needs nothing kept and lets one party
hurt only itself.
Cost: **the prefix rules are tidiness, not isolation.** Venues on one relay are one
origin to a browser, so a script served by one venue can fetch another venue's pages
as the guest, cookies attached whatever their `Path`, and read the answer; local
storage is shared too. On a relay with one venue, or venues that trust each other,
that is nothing. On a hosted relay with strangers it is cross-tenant script access,
and it must be closed before one is run: a subdomain per venue (a wildcard
certificate), or a sandboxing `Content-Security-Policy` on proxied pages, which
would also take cookies and storage away from the guest app. Until then guest
cookies must be `HttpOnly` and nothing a guest's script can read should be worth
stealing (open question 7 in `STATE.md`). Revoking a key takes a restart.
The request body is sent before the response is read, so there is no full-duplex
streaming, and a guest cannot open a WebSocket through the relay: order tracking
(Task 5) is server-sent events or polling. `go test -race` could not be run here (no
cgo), so the concurrency was checked by reading and by repeating the tests.

## ADR-202 - The tunnel client: guests are served in process, under one mount, and nothing waits on it
2026-10 · Status: accepted. Phase 11, Task 2a.
Context: Task 1 left a relay and a protocol. The venue's side has to hold the socket
open across a cafe's internet, serve guests from the same server that runs the
counter, and never be the reason the counter stops.
Decision:
  - **Task 2 is split.** 2a is the client, `relay_key`, the wiring, health and the
    test that kills the relay. 2b is the back office: a status endpoint that says
    more than one word, and where the owner sees it.
  - **A guest's request calls the server's handler in this process.** `internal/tunnel`
    is handed an `http.Handler` and builds an `http.Request` from the frames; it does
    not dial `localhost`. So a guest request goes through the same middleware, access
    log and panic recovery as any other, and the tunnel imports neither `internal/api`
    nor `internal/db`.
  - **A guest reaches the guest app's mount and nothing else, by construction.** Every
    guest path is put under `/order/`: `/t/abc` is `/order/t/abc`, and `/api/v1/orders`
    is `/order/api/v1/orders`, which is not the staff API. There is no list of allowed
    routes to keep in step. A path with `..`, `.`, an empty segment or a backslash,
    encoded or not, is refused whole (404) and not cleaned. **What a guest needs from
    the server (Task 3 on) is therefore mounted under `/order/`**, for example
    `/order/api/...`, and works the same on the venue's own wifi.
  - **`relay_key` is a new setting**, `MEZ_RELAY_KEY`. `relay_url` set with no key, or
    with something that does not start `mzr_`, stops the start with a message (ADR-019);
    the check is in `tunnel.New` and not in `internal/config`, which stays a leaf and
    lets each key be set alone. The key is never logged, and no error repeats it.
  - **`relay_url` is the relay's base**, and the client dials `<relay_url>/agent`.
  - **Nothing waits on the tunnel.** `Start` returns at once; a venue with no internet
    starts as fast as one with. A relay that cannot be reached is a log line and a
    state, never an error from `run`.
  - **Backoff** doubles from 1 s to 60 s, each wait drawn from the upper half of its
    step so that venues on a restarted relay do not return in the same second. A
    connection that lasted 30 s resets it. A 401, and close code 4001 (another machine
    holds this venue), wait 5 minutes: retrying sooner changes nothing, and for 4001
    would have two machines taking the venue from each other.
  - **Heartbeat.** The client pings every 20 s with 10 s to answer, so a line that died
    without a close is redialled and not left looking connected.
  - **A request body is whole before the handler runs**, held in memory up to the
    relay's 1 MiB; 256 requests at once, the relay's limit, enforced at this end too. A
    guest who leaves cancels the request's context. A panic the server did not recover
    resets that one stream.
  - **Health.** `checks.tunnel` is `ok` when connected, `failing` when a relay is set
    and it is not, and `unknown` when no relay is set: off is not a fault, and most
    venues never set one. `status` stays `ok` either way.
  - **The proof** is `TestTheCounterSellsWhileTheRelayIsDown` in `internal/api`: a real
    database, order service and API behind the client and a real relay on a port. A
    guest is served, the relay is killed, three sales go through on the same invoice
    series, the relay comes back on its address and guests are served again. It was
    also done by hand with `bin/relay` and `bin/mez`.
Because: the invariant is that offline is the default state, so the tunnel is built as
something the counter can ignore entirely, and the staff API is kept from guests by
where requests can land, which cannot be forgotten, and not by a rule about which
are allowed, which can.
Cost: no streamed uploads from guests. The built guest bundle names its assets as
`/order/assets/...`, which is right on the venue's wifi and wrong behind the relay's
`/v/<venue>/` prefix: Task 3 has to build it with relative URLs. A request arriving by
the tunnel is not yet marked as such for the handler; `X-Forwarded-Prefix` is there
but a device on the LAN could send it too, so Task 3 must not treat it as proof.

## ADR-203 - A hostname per venue on a shared relay
2026-10 · Status: accepted. Phase 11, Task 1b. Answers open question 7; amends ADR-201.
Context: ADR-201 gave every venue a path on one hostname and said plainly that this is
not isolation: to a browser one hostname is one site, so one venue's script could act
as another venue's guest. The owner chose a hostname per venue for the hosted relay.
Decision:
  - **`RELAY_VENUE_DOMAIN=example.com` serves each venue at `<venue>.example.com`.** The
    venue is read from the Host header: one label under the domain, a valid venue id,
    not a reserved name. The whole path is the venue's and is forwarded untouched, with
    no `X-Forwarded-Prefix`. A table's QR code says `https://<venue>.<domain>/t/<code>`.
  - **With a venue domain set, `/v/<venue>/` is not served at all.** Leaving it would
    put every venue back on one origin, at the relay's own hostname.
  - **Every other hostname is the relay's own**: `/agent` and `/healthz` answer there,
    so venues dial `relay.<domain>`. On a venue's hostname those paths are the venue's.
  - **Without a venue domain nothing changes**: path routing, with ADR-201's rules. That
    is for one venue, or venues that trust each other, where no wildcard is wanted.
  - **Reserved names** (`relay`, `www`, `api`, `admin`, `app`, `mail`, `smtp`, `ns1`,
    `ns2`, `status`, `support`, `order`) are never venues, and `issue-key` refuses them.
  - **A cookie still loses its `Domain`**, since `cafe.example.com` could otherwise set
    one for all of `example.com`. Its path and a redirect are left alone, and
    `Service-Worker-Allowed` and `Clear-Site-Data` pass: the origin is the venue's own.
Because: the browser's same-origin rule is the only isolation between tenants that does
not depend on the relay getting every header right.
Cost: a wildcard DNS record and certificate. The domain is printed on every table, so
changing it later reprints every code. Cookies set with `Domain` by a venue's own
script (not a header) are the browser's to refuse, which it does only if the domain is
on the Public Suffix List; guest sessions should be `HttpOnly` and host-only.

## ADR-204 - The relay is deployed at relay.jskin.in: one server, Cloudflare in front, Caddy for TLS
2026-10 · Status: accepted. Phase 11, Task 1c.
Context: the relay needed somewhere public to run, at the lowest cost. The owner had a
dormant Hetzner server and a dormant domain, `jskin.in`, already on Cloudflare.
Decision:
  - **The owner's existing server**, wiped and rebuilt as Ubuntu 24.04 and renamed
    `mez` (ARM, `cax21`). Key-only SSH, root's password locked, unattended upgrades.
    A Cloudflare Worker was set aside: the relay holds sockets in memory, so a Worker
    would be a second implementation in another language.
  - **Cloudflare proxies `relay.jskin.in` and `*.jskin.in`** to it. The provider's
    firewall lets 443 in from Cloudflare's addresses only, and 22 for SSH.
  - **Caddy terminates TLS with one wildcard certificate**, proved over DNS with a
    Cloudflare token that can edit that zone only. The token stays on the server for
    renewals. Caddy takes the guest's address from `CF-Connecting-IP` and gives it to
    the relay as `X-Forwarded-For`, which `RELAY_TRUST_PROXY=true` then believes.
  - **`RELAY_VENUE_DOMAIN=jskin.in`**: a venue is `<venue>.jskin.in` (ADR-203).
  - **The files are `build/relay/`**: `deploy.sh`, `compose.yml`, `Caddyfile`,
    `caddy.Dockerfile` and a README. Only `cmd/relay` and `internal/relay` are sent to
    the server and built there.
  - **The signing secret is drawn on the server** into `/opt/mez-relay/.env` and is in
    no repository, log or conversation.
  - **Proved** with `bin/mez` on the owner's PC: `checks.tunnel` went `ok`, a guest
    page loaded from `https://democafe.jskin.in/t/4` in under a second, the staff API
    path returned the guest app, mez logged the guest's own address and not
    Cloudflare's, and the venue read "offline" as soon as mez stopped.
Because: it runs what is already built, on what is already paid for, and nothing on
it is worth keeping: lose the server and a new one is one script away.
Cost: one server is one point of failure for guest ordering, never for the counter.
The secret has no backup: a rebuilt server without it means a new key for every
venue. `jskin.in` is on every table's code once printed. A test key for `democafe`
exists and is harmless; revoke it before real venues join. Cloudflare's addresses in
`Caddyfile` and the firewall are a copy and will need refreshing if they change. The
zone's SSL mode could not be read with the token; it must be Full (strict).

## ADR-205 - The tunnel in the back office: a card on Devices, and one status for health and the screen
2026-10 · Status: accepted. Phase 11, Task 2b. Task 2 is done.
Context: health says the tunnel is `ok`, `failing` or `unknown`. An owner whose guests
cannot order needs to know which kind of not working, and whether waiting will fix it.
Decision:
  - **`GET /api/v1/tunnel`**, for `device.manage`: `state` (`off`, `connecting`,
    `connected`, `waiting`, `refused`, `replaced`, `stopped`), and when there is one
    `since`, `retry_at` and `detail`, the client's own account of the last failure.
    `TunnelState` is generated into TypeScript.
  - **One source for both.** `api.WithTunnel(func() TunnelStatus)` replaces
    `WithTunnelCheck`; health's word is derived from the same status. `internal/api`
    still does not import `internal/tunnel`: `cmd/mez` maps one vocabulary to the
    other, written out, and a state it does not know is `stopped`.
  - **It is a card on Devices**, "Ordering from phones", over the list. Devices is
    where the owner looks for what is connected to the venue, and the permission is
    the same. Settings was the other candidate; nothing here can be set from a
    screen, and a Settings section that only reports would be the odd one out.
  - **Each state is a chip with a word and a sentence saying what guests can do.**
    Reconnecting says the counter is not affected. The two states waiting will not
    mend, a refused key and a second machine with the same key, say what will.
  - **`off` is "Not set up", neutral**, and is shown: it is how an owner learns the
    feature exists. It is never worded as a fault.
  - **The card reads on its own**, every 5 s while the state is moving and seldom
    once it has settled, and does not hold up the list under it.
Because: "failing" is one word for four situations with four different answers, and
the owner is the one who has to act on two of them.
Cost: the relay's address and key are still set in a file; the card can only say so.
`detail` is a Go error string, shown small for whoever is helping, not for the owner.
The card does not yet show the venue's public address, since mez does not know the
relay's venue domain: Task 6 needs it for the QR codes and should add it then. Seen by
eye in `waiting` and `connected` at 1366 and 400 wide; the other five states are
covered by the logic's tests only.

## ADR-206 - The guest's way in: a code on the table, an API under /order/api/, one bundle for wifi and relay
2026-10 · Status: accepted. Phase 11, Task 3a.
Context: Task 3 is the guest's menu and cart. Before a screen can be built the phone
needs to know which table it is at, read the menu without being a paired device, and
load the app's own files through the relay, which it could not (ADR-202's cost).
Decision:
  - **Task 3 is split.** 3a is this. 3b is the landing and the menu (M01, M02). 3c is
    the item with its choices, and the cart (M03, M04's cart, M11a).
  - **A table has a code**, twelve characters from `23456789abcdefghjkmnpqrstuvwxyz`
    (no 0, 1, i, l, o), about 59 bits, drawn without bias from `crypto/rand`. It is
    the guest's only credential. `table_codes` (migration 00038) keeps it as it is,
    not hashed: the back office has to print it again, and it guards a menu.
  - **A table has at most one live code.** `Renew` turns the old one off and makes
    the new in one transaction. A revoked row stays, so the code is never drawn
    again. A code for a table taken off the floor leads nowhere.
  - **`internal/service/tablecodes`**: `Resolve`, `List`, `Renew`, `Ensure`. A string
    that is not the shape of a code never reaches the database.
  - **The guest API is under the guest app's mount**, `/order/api/`, not `/api/v1`:
    `GET /order/api/t/{code}` (the venue's name and currency, the table's name),
    `GET /order/api/t/{code}/menu`, and `GET /order/api/photos/{name}`. It needs no
    device and no session. Every other path under `/order/api/` is a JSON 404, so a
    mistyped call is never answered with the app's page.
  - **The guest's menu is the till's with the till's parts taken out**: no stations,
    courses, tax rates, HSN codes. `sold_out` is a yes or no by the server's clock;
    the guest is not told until when. It is `no-store`, since sold out turns on the
    clock and not on the menu's version.
  - **One answer for a code that leads nowhere**, 404 `table_code_unknown`, whether it
    was never made, was replaced, or its table is gone, in words a guest can act on.
  - **300 requests a minute for each address** across the guest API.
  - **The tunnel leaves a path already under `/order/` where it is.** The bundle
    names its files and its API by the server's own paths, so the same build works on
    the venue's wifi and through the relay, and the app needs to know nothing about
    which it is in. `/t/abc` and `/order/t/abc` both reach the app.
  - **`mez table-codes`** lists each table's code and path, making any that are
    missing, run at the machine as `pair` is. It is the way to a code until Task 6
    prints them from the back office.
Because: a code under which every guest route sits is one thing to check and one
thing to revoke, and a guest API that lives where the tunnel already confines
guests cannot drift into the staff API.
Cost: the code is in the URL, so it is in browser history and in the relay's log
(the path, not the query). Anyone who photographs a sticker can read the menu from
anywhere, which is harmless, and will be able to order from anywhere once Task 4
lands unless the venue limits it: Task 6's per-table limit, and whatever ties an
order to someone being at the table, are still to be decided. The per-address limit
counts every guest behind one mobile carrier address together.

## ADR-207 - The guest's landing and menu: no router, the venue's name, and no words ahead of the feature
2026-10 · Status: accepted. Phase 11, Task 3b.
Context: M01 and M02 of the mez design, built on Task 3a's API, inside a 60 KB budget
and before a guest can order anything.
Decision:
  - **No router.** The table's code is the last segment of the path after `/t/`, so
    the same bundle works at `/order/t/<code>` and `/t/<code>`; the screen is the
    hash, so the phone's Back goes from the menu to the landing and a reload stays.
  - **The shared API client with no credentials**, and React Query as in the other
    apps: one way of saying a request failed. `problemOf` sorts a failure into the
    code leading nowhere, the venue not reached (including the relay's 502, 503, 504
    and a 429), or anything else, and every one of those screens names the counter.
  - **The filtering is pure and tested** (`menu-logic.ts`): a category, a search
    where every word must be in the name or description, and diets that narrow
    together, veg including vegan. Sold-out dishes sink to the end. A category or a
    diet nothing satisfies is not offered.
  - **The landing says whose table it is**: "Welcome to <venue>". The design shows
    only mez's name.
  - **Words do not run ahead of the feature.** The button is "See the menu" and not
    "Start Ordering" until Task 4. The cards have no Add and there is no cart bar
    until 3c. The bottom navigation waits for the screens it names.
  - **Claims are the venue's.** "Freshly made", "Fresh ingredients. Great taste." and
    the three boasts on the landing are left out; the green band carries the venue's
    name where the design has "Fresh flavours daily".
  - **A dish with no photo has no frame**, since most venues start with none.
  - **Prices are in full**, `formatMoney`, as the bill will show them.
  - **Imports are by file**, not from `shared/ui`'s index. The bundle is 31.9 KB
    gzipped of 60, with 28.1 left for the item, the cart, checkout and tracking.
Because: a guest who taps "Start Ordering" and cannot order has been told something
untrue by a screen the staff then have to explain.
Cost: the copy changes twice, now and when ordering lands; the specs say which words
go back to the design's. Seen by eye at 390 wide on the demo menu: the landing, the
whole menu, a category with a diet, and a code that leads nowhere. Not seen: a dish
with a photo, a sold-out dish, the search, the unreachable and failed states, and
anything on a real phone or through the relay with this build.

## ADR-208 - The guest's cart: ids on the phone, priced from the menu, and the till's rules for choices
2026-10 · Status: accepted. Phase 11, Task 3c. Task 3 is done.
Context: M03 and the cart half of M04, before an order can be sent anywhere.
Decision:
  - **The rules for a dish's choices are shared.** `modifier-selection.ts` moved from
    `web/terminal/src/register/` to `web/shared/`, unchanged, with its tests. The
    guest's sheet and the till's modal call the same functions, so a guest can build
    nothing a cashier could not.
  - **A cart line is ids and a count**: the dish, the chosen choices, how many. Names
    and prices are read from the menu each time the cart is shown. A stored price is
    a price that can be wrong.
  - **The cart is held against the menu each time the menu is read** (every minute,
    and when the phone comes back). A line is taken out when its dish is gone or sold
    out, a choice is gone, or its choices no longer satisfy a group; the guest is
    told by name, in a notice that stays until dismissed.
  - **It is kept in `localStorage` by table code**, so a reload keeps it and another
    table's sticker starts clean. What is read back is not trusted: anything that is
    not a line is left out, a count is held to 20 and a cart to 50 lines, and reading
    cannot throw. With storage refused the cart lasts as long as the page.
  - **The same dish with the same choices is one line.**
  - **Money is integer minor units** from the menu to the total. The total is before
    tax and says so: the guest's menu carries no tax rates, and the bill is the
    venue's to work out when the order is placed.
  - **Nothing says it sends anything.** The sheet's button is "Add to cart", not the
    design's "Add to order". The cart has no Place Order; where it would be is "To
    order, show this list to a member of staff at Table T4, or order at the
    counter." So the cart is useful today, as a list to show.
  - **Special instructions are left out** until Task 4 says how a line reaches the
    kitchen. A field that goes nowhere is worse than none.
  - **A dish with no choices is added from its card in one tap**; one with choices
    opens. Down from one in the cart removes the line. Clear is asked twice, on the
    button.
  - The bundle is 36.1 KB gzipped of 60, with 23.9 left for Tasks 4 and 5.
Because: the cart is the first thing in the guest app that holds state between
visits, so it is the first place a guest can be shown something no longer true.
Cost: Task 4 replaces the sentence at the cart's foot and decides the note field. The
figure a guest sees is the menu's arithmetic, not the server's, until then. Seen by
eye at 390 wide with a mouse pointer: the sheet (as a side panel, which is what a
fine pointer gets), adding with choices and a count, the cart's bar, the cart, a
reload keeping it, removing a line, Clear, and the empty cart. Not seen: the sheet
as a bottom sheet on a touch screen, a dish with a photo, a choose-several group
filling up, the unmet-group button, or the notice for a dish taken out.

## ADR-209 - A phone order is a request until staff accept it, once per sitting; and photo frames
2026-10 · Status: accepted, by the owner. Phase 11, before Task 4. Amends ADR-207.
Context: ADR-206 left open what ties a guest's order to someone being at the table. A
table's code is on a sticker, a sticker can be photographed, and a photographed code
works from anywhere. That is harmless for a menu and not for the kitchen.
Decision:
  - **The first order from a table in a sitting is a request.** It shows on the
    terminal as that table wanting to order, with Accept and Decline, and nothing
    reaches the kitchen until someone accepts it.
  - **After it is accepted, the table's further rounds go straight through**, until
    its bill is closed. The next sitting starts with a request again.
  - **A table staff have already seated needs no acceptance**: staff have said
    someone is there.
  - **A prepaid order goes straight to the kitchen.** In a venue that requires
    prepayment, paying is the tie.
  - **The terminal has to say a request is waiting**, with a sound and something on
    screen, or an unwatched terminal is an order that never happens. That is part of
    Task 4 and not a later nicety (the design's notifications, D13, are unbuilt).
  - **The guest sees it as it is**: "Waiting for the cafe to accept your order", then
    accepted or declined, as real events (Task 5), and a declined request says to
    ask a member of staff.
  - Task 6's limit on how many requests a table may make still applies, and a code
    can be renewed from the back office.
  - Set aside: checking the phone's location or wifi (a guest on mobile data cannot
    be checked, and the prompt is unreliable and intrusive); a PIN on a table card
    changed daily (friction for every guest, a chore for staff); a rotating code on
    a screen (hardware on every table).
  - **Photo frames (amends ADR-207):** every card has a frame for a photo once any
    dish on the menu has one, so the grid is even; a menu with no photos has none.
    `showsPhotoFrames`, asked of the whole menu and not of what a filter left.
Because: it uses what the venue already has, staff who can see whether a table is
occupied, and asks nothing new of the guest.
Cost: one tap for staff per sitting in a venue that does not prepay, and a request
nobody notices is a guest waiting. A counter-service venue with nobody on the floor
should require prepayment. Someone at the table can still order for a table they are
not at once the sitting is accepted, until the bill closes; the per-table limit is
what bounds that.

## ADR-210 - A guest's order is a round: on the table's open ticket, or kept as a request
2026-10 · Status: accepted. Phase 11, Task 4a.
Context: Task 4 is guest checkout, and is several sessions. Its first piece is the
one the rest stand on: what the server does when a phone says "these dishes, at this
table". ADR-209 had already said the first order of a sitting is a request.
Decision:
  - **Task 4 is split in seven**, listed in the phase file: 4a this, 4b the request
    on the terminal, 4c Place Order on the phone, 4d how the venue is paid, 4e UPI,
    4f the gateway adapter, 4g splitting. 4c onward was cut before reading M08 and
    M09 and may move.
  - **`POST /order/api/t/{code}/orders`** takes lines of `item_id`, `modifier_ids`
    and `qty`, and nothing else: a body with a price in it is refused, as any
    unknown field is.
  - **The server rings each line up from the menu** (`menu.Menu.Ring`, pure, in
    `internal/domain/menu`): the name, the price, each choice, the station, course
    and tax facts, as a till would have sent them. It enforces what the phone's
    sheet enforces (a group's bounds, no choice twice, none from another dish), plus
    sold out by the server's clock, and 1 to 20 of a dish. A round is 1 to 50 lines.
    These are the cart's own bounds (ADR-208).
  - **Each placement is a round**, a row in `guest_rounds` (migration 00039) with a
    public id `rnd_...`, the lines as priced, and the key it came with.
  - **A table's guests share one ticket.** With a ticket open at the table (its own
    or one it is joined to, the oldest if there are several) the round's lines are
    appended to it and sent to their stations in one command, with the round's row
    written in the same transaction. Staff opened that ticket, by seating the table
    or by accepting a request, so someone is there. One table, one ticket, one bill;
    who pays what is 4g.
  - **With no ticket open the round is a request** and stays a row: no order, no
    event, nothing in the kitchen. Accepting it (4b) is what makes it an order. So a
    request made from a photograph of the sticker, or one nobody answers, never
    reaches the order stream, the voids on the Z-report, or an invoice number.
  - **An accepted request appends the lines as they were priced when the guest
    asked**, which is why the row keeps them: the figure the guest was shown is the
    figure on the bill, whatever the menu did in between.
  - **Only the round's own lines are sent**, as a ticket for each station they
    belong to. `order.ToSend` would also send a waiter's lines still being rung up
    on the same ticket. A guest's lines go whatever course they are in: holding a
    course back is a waiter's tool, and nobody is holding this one.
  - **The guest is nobody**: actor 0 on device 0, which `Meta` already allowed for.
    The timeline shows a guest's lines as nobody's.
  - **An open shift is required, by the rule that already exists** (ADR-094): a
    line cannot be appended without one, and a request is refused the same way,
    since a request with nobody there to accept it is a guest waiting for nothing.
  - **The idempotency key is required**, where the staff API only accepts one. A
    phone on a poor connection sends again, and a second round in the kitchen is
    food nobody asked for. The key is unique in `guest_rounds`, not in
    `idempotency_keys`: a waiting round has no events for that table to point at.
    Two of the same key at once both try to write; the unique index refuses one,
    its whole transaction rolls back with its events, and it answers with the
    other's round. The same key with another cart is refused.
  - **At most five requests wait at one table.** Past that, staff have enough to
    decline.
  - **Refusals are in the guest's words**, each with its own code:
    `dish_unavailable`, `choices_invalid`, `table_not_ordering` (the ticket is on
    hold or invoiced), `requests_waiting`, `no_open_shift`. The domain's own wording
    names seqs and ids and is not shown.
  - **The answer** is the round's id, `waiting` or `sent`, each line as priced, and
    the subtotal before tax. It names no order, table or venue id.
Because: the acceptance in ADR-209 exists to keep strangers out of the kitchen, and
an order row with voided lines for every declined stranger would put them in the
books instead.
Cost: a second place where a key is remembered. A waiting round is a row nobody
sees until 4b builds the terminal's half, and it never lapses. Nothing limits how
often a table with an open ticket can send rounds to the kitchen, except 300
requests a minute an address, until Task 6. The limiter counts by address across
every limit, so a tighter one for placing orders would be spent by reading the
menu; it needs its own bucket first. The guest app does not call this yet (4c), and
it was not run through the relay or from a phone: the Go tests are the proof.

## ADR-211 - Answering a guest's request: each on its own, on the floor's version, for as long as the shift
2026-10 · Status: accepted. Phase 11, Task 4b1.
Context: ADR-210 left a waiting request as a row nobody sees. Task 4b is the staff's
half: seeing it, hearing it, and accepting or declining it. That is a server and a
screen, so it is two sessions.
Decision:
  - **Task 4b is split.** 4b1 is the server, this. 4b2 is the terminal's screen and
    its sound.
  - **Three routes on the staff API**: `GET /api/v1/guest-requests` (what is
    waiting, oldest first, each with its table by name, its lines and subtotal),
    and `POST .../{id}/accept` and `.../{id}/decline`. They ask `order.open`:
    accepting opens a ticket and adds to it, which is what that permission is. No
    new permission, so every role that can seat a table can answer one.
  - **Each request is answered on its own.** Accepting one says someone is at the
    table; it does not say every phone that asked is theirs. The table's other
    requests stay waiting, and accepting one of those adds to the same ticket.
  - **Accept finds the table's ticket or opens one**, on the `qr_table` channel,
    dine-in, and appends the request's lines **as stored**, with a ticket for each
    station, marking the request `sent` in the transaction that appends them. A
    menu price changed since the guest asked does not reach the bill.
  - **Opening and appending are two commands, not one transaction.** The order
    store opens an order with one event. If the second command never happens the
    table is seated with an empty ticket and the request is still waiting;
    accepting it again finishes on that ticket. Nothing is lost and nothing is
    sent twice.
  - **An accepted request's events are the staff member's**, not actor 0: they let
    it into the kitchen, and the timeline should name a person. A round that goes
    straight onto an open ticket is still nobody's (ADR-210). The channel of a
    ticket a guest's request opened is `qr_table`; a guest's round on a ticket a
    waiter opened leaves it `counter`.
  - **Decline marks the request**, with who and when. Nothing is voided, because
    nothing was an order.
  - **Answered twice is answered once.** Accepting an accepted request, or declining
    a declined one, returns the first answer. The other way round is refused,
    `request_decided`. Of two tills answering at once, the update that finds the
    request no longer waiting writes nothing and reports what the other did. Accepts
    are also taken one at a time in the process, so two tills cannot each open the
    table a ticket.
  - **A request lasts as long as its shift.** Nothing ends it at the close. One
    made before the open shift opened is not listed, does not count toward the
    table's five, and cannot be accepted (`request_lapsed`); it can be declined. A
    timer was set aside: how long a guest will wait is the venue's to judge, and
    the screen can show how long it has been.
  - **A request arriving or answered moves the floor version**, in its transaction,
    and nudges the hub (`svcorder.Options.OnChange`). Terminals already hear
    `floor_changed`; a table wanting to order is the floor changed in a way no
    order event shows, which is what that message is for. A fourth version and
    message were set aside as plumbing with nothing to add.
  - **The guest's phone learns the answer by its own key**: sending the round again
    returns it as `sent` or `declined`. Task 5 gives it something better to ask.
Because: the smallest thing that lets a request become a ticket, built from the
commands the till already uses, with the screen left to be built on an API that is
already tested.
Cost: a terminal that hears `floor_changed` reads the floor and the requests both,
whichever moved. A request staff ignore sits in the list until the shift closes.
Until 4b2 nothing on any screen shows a request, so a guest at an unseated table
who places an order is waiting on nobody. The lock is this process's: a second
process on the same file is not guarded, and mez does not run two.

## ADR-212 - The request on the terminal: a pill, a sheet, a chime that repeats, and a mark on the table
2026-10 · Status: accepted. Phase 11, Task 4b2. Task 4b is done.
Context: ADR-209 says the terminal has to say a request is waiting, with a sound and
something on screen, or an unwatched terminal is an order that never happens. ADR-211
built the API. The design draws a bell in the top bar and a notifications screen.
Decision:
  - **A pill in the top bar, on every screen of both frames**, there only while a
    request waits: a bell, the count and "table request(s)", in the terracotta
    tint. It is built as Not printed is, beside it, so a waiter's frame has it too.
  - **It opens a sheet**, the requests oldest first, each with its table, how long
    it has waited, its lines, the subtotal before tax, **Accept and send** and
    **Decline**. The sheet is the print queue's pattern.
  - **A request never opens the sheet by itself.** It would land on top of a
    payment being taken.
  - **Decline is asked twice, on the button**, as the guest's Clear is: it tells a
    guest no. Accept is not: it is the common answer, and a ticket opened by
    mistake can be voided.
  - **The sound is the kitchen's chime**, from the browser's oscillator: when a
    request arrives, and **again every 30 seconds while any waits**. A single chime
    in a loud room is missed, and a missed one is a guest waiting on nobody. A
    terminal opened onto requests already waiting chimes for them, where the
    kitchen's board stays quiet for a board already full.
  - **An untouched page is silent, and the pill is what says so.** A browser plays
    nothing before someone touches the page. The first tap or key anywhere is
    taken as the touch, and nobody reaches the frame without signing in, which is
    one. The case left is a terminal reloaded while signed in and not touched
    since. A banner asking for a tap was set aside: it is one more thing on
    every screen for a case that ends at the next tap.
  - **No switch for the sound.** The kitchen has one because a board can be read
    across a room; nobody is reading the till's top bar from across a room.
  - **A table with a request waiting carries a terracotta mark, a bell and a
    word, in place of its state chip**: on the floor plan ("Request" under 1536px,
    where the full words overran the tile, "Wants to order" from there), in the
    Table List, and on the waiter's card. Such a table has no ticket, so the chip
    it replaces said Free or To clear, and that a guest is asking is the more
    useful thing to know. The spoken label keeps both.
  - **One list, held above the routes** (`TerminalProvider`), read on
    `floor_changed` and on a resync. After any answer, accepted or refused, the
    requests, the floor and the open tickets are read again: a refusal usually
    means another terminal answered first.
  - **Nothing is queued while offline.** An accept sent late could send food for
    a guest who has gone.
  - **The error codes a guest or a request can be refused with are in the
    generated `ErrorCode` union**, which 4a and 4b1 had left out.
Because: the pill, the sheet and the stream nudge all existed for print jobs, and a
request is the same kind of thing: something that needs a person, on every screen,
until someone deals with it.
Cost: the chime cannot be silenced short of answering the requests. Seen by eye at
1366 wide on a demo venue, with requests placed by curl: the pill arriving without
a reload, the chime's notes starting, the sheet with two requests, Accept (the
table reads Ordered), Decline armed and confirmed, the empty sheet, and the mark on
a floor tile. Not seen: the waiter's card and frame, the Table List's row, the
sheet on touch, a refusal's words, the 30 second repeat, two requests from one
table, anything at 1536 or wider, and the sound itself, which was counted and not
heard.

## ADR-213 - Place Order on the phone: one key for one cart, the answer kept, and asked again while it waits
2026-10 · Status: accepted. Phase 11, Task 4c. Amends ADR-207 and ADR-208.
Context: the server takes a guest's order (ADR-210) and staff can answer a request
(ADR-211, ADR-212). The guest app still said "show this list to a member of staff".
Decision:
  - **The cart's foot is Place Order**, with the cart's own total on it. It sends
    the lines the cart shows, as dish ids, choice ids and counts.
  - **One key for one cart.** A key is drawn the first time a cart is sent and
    kept in `localStorage` with exactly what was sent. The same cart sent again,
    after a dropped connection or a reload, goes with the same key and is the
    same order. A cart that changed gets another key, since the server refuses a
    key that returns with something else in it. The key is forgotten once the
    venue has the order. A second tap while placing sends nothing.
  - **The cart is emptied only when the venue has the order.** Refused, it stays
    as it was, with why above the button.
  - **The answer is kept on the phone**, by table code: the key, what was sent and
    what came back. It is what the next screen shows, so a reload or a locked
    screen loses nothing. It is forgotten after three hours, and what is read back
    is not trusted. One order is remembered, the latest.
  - **A new screen, `#order`,** says how it stands in the venue's real answer:
    waiting to be accepted, with the kitchen, or not accepted. It never says an
    order nobody accepted is being made. Task 5's tracking takes it over.
  - **A waiting order asks again every 8 seconds** with its own key, and when the
    phone comes back from a locked screen, not while the page is hidden. The
    server answers a key with the order as it stands, so no new endpoint was
    needed. It stops at an answer.
  - **A strip over the menu and the cart** says how the remembered order stands
    and leads back to it, so a guest who went on browsing is still told.
  - **Each refusal in words a guest can act on.** Where the server wrote the
    sentence for a guest it is shown as written. A dish or choices that moved also
    reads the menu again, and the cart, held against the menu as before, takes the
    dish out and says so. A failure to reach the venue, by the phone or the relay,
    is said on the phone. Anything else names the counter, and nothing of the
    server's insides is shown.
  - **The design's words are back** (ADR-207, ADR-208): "Start Ordering" on the
    landing and "Add to order" on the sheet.
  - **No special instructions.** A guest's note would need a place on the kitchen
    ticket and the display, and an allergy typed into a phone that nobody reads is
    worse than none. The cart says "Allergies or a special request? Please tell a
    member of staff." It can be its own task once the kitchen shows notes from
    guests.
  - **"Your order is secure" is left out**: nothing is paid here.
  - No mutation library: one `fetch` behind the shared client. The bundle is 37.9
    KB gzipped of 60, with 22.1 left for paying and tracking.
Because: the phone's side of ordering is mostly about not sending twice and not
losing what was sent, and both come down to keeping the key with the thing it was
drawn for.
Cost: asking again is a POST every 8 seconds for each waiting phone, against the
300 a minute an address allows; Task 5 should replace it. A phone that places a
second order while its first still waits stops hearing about the first. With
storage refused, a retry after a reload is a second request, which staff see and
decline. The total on the button is the menu's arithmetic until the answer comes.
Seen by eye at 390 wide on a demo venue: the landing's button, the cart with Place
Order, the waiting screen, the phone turning to "with the kitchen" by itself after
the request was accepted, a second round going straight through, and the strip over
the menu. Not seen: a declined order, any refusal, placing over a slow or dropped
connection, a reload while waiting, the screens on a real phone or through the
relay.

## ADR-214 - The bill on the phone is the table's ticket, read by its code; and the payment mode waits for payment
2026-10 · Status: accepted. Phase 11, Task 4d. Amends ADR-210's split.
Context: 4d was cut as "how the venue is paid: a setting, prepay or pay at the table,
and the bill on the phone". Read closely, the two halves do not belong together yet.
Decision:
  - **The setting moved to 4e.** Prepay means a round is held until it is paid,
    and nothing can be paid from a phone until 4e. A setting the owner could turn
    on today would hold every guest's order for a payment that cannot be made, or
    would do nothing and say it did. It arrives with the thing it controls. Until
    then every venue is pay at the table, which is what the code already does.
  - **4d is the bill**: `GET /order/api/t/{code}/bill` and a screen at `#bill`.
  - **The bill is the table's open ticket, projected, with the till's parts taken
    out.** Each line that is on the bill (a voided one is not; a comped one is,
    marked, with what it was worth), what the charged lines come to, discounts as
    one figure, each tax component by name and rate, whether the prices already
    hold the tax, the total, what has been paid net of refunds, and what is left.
    No ids, stations, courses, reasons, staff or tenders.
  - **Nothing is added up outside the projection**, in the handler or on the
    phone, except line and order discounts into one figure with the money type's
    own addition. The phone chooses rows and words.
  - **Tax already in the prices is said as "Includes CGST (2.5%)"**, smaller, so
    the rows are not read as adding up to the total. This is the walk's open
    question 4 (ADR-199) answered for the guest's screen, where it is new; the
    till's own totals are still the owner's to decide.
  - **One guest sees the whole table's bill.** The code is the table's and so is
    the bill; a paper one would show the same. Nothing records whose phone
    ordered which line, and 4g, splitting, is where "mine" will mean something.
  - **No ticket is a bill with nothing on it, 200, not an error.** A waiting
    request is not on any bill, and the screen says so.
  - **Read on opening, every 15 seconds while shown, and on return**, never from
    an earlier look. Not on the stream: a guest has none.
  - **Where to pay is said plainly**: staff or the counter. Nothing says a guest
    can pay here.
  - **Reached from the order placed, once it is sent, and from an empty cart**, so
    a guest at a table staff seated, who ordered nothing by phone, can read it.
  - The guest's bundle is 38.8 KB gzipped of 60.
Because: a bill worked out twice is two bills. The ticket's projection is already
what the till, the invoice and the Z-report agree on.
Cost: 4e is now larger, and is to be split before it starts. Anyone holding a
table's code can read what that table has ordered and owes, from anywhere, for as
long as the code lives; ADR-206 already accepted that for the menu, and this is
more. A table with two tickets open shows the older one's bill. Once a bill
closes the phone shows nothing until Task 5's receipt. Seen by eye at 390 wide on
a demo venue with no tax: a ticket's lines, the items row, the total, and where to
pay. Not seen: tax rows of either kind, a discount, On the house, Paid and Left to
pay, the bill with nothing open, and the failures; the Go and vitest tests cover
the figures and the words for all of them.

## ADR-215 - Paying by UPI from the phone: a link the server builds, and staff confirm the payment
2026-10 · Status: accepted. Phase 11, Task 4e1.
Context: Task 4 asks for a UPI intent link and a dynamic QR, and that a confirmed
payment be the same `payment.tendered` as at the counter. A UPI link carries the
venue's address and an amount. It moves no money by itself, and when the guest's
bank pays the venue's bank nothing tells the venue's server.
Decision:
  - **Staff confirm a UPI payment**, as they already do at the till, where the
    cashier takes UPI on the venue's own code and taps Mark as paid. The guest pays
    from the phone, shows it, and a member of staff records `payment.tendered` on
    the terminal. Nothing new records a payment, so nothing new can record a false
    one. Knowing by itself that a payment arrived needs a gateway's callback, which
    is 4f, and which needs the internet the rest of mez does not.
  - **Task 4e is split in five**, listed in the phase file: 4e1 this, 4e2 the back
    office card, 4e3 the QR, 4e4 "I have paid" reaching the terminal, 4e5 the
    prepay mode.
  - **The venue's UPI address is a setting**, `payment.upi_address`, with
    `payment.upi_payee` for the name a guest's app shows, the venue's own when
    unset. `GET` and `PUT /api/v1/settings/payments`, asking `settings.edit`, on
    the settings service's own key registry.
  - **An address is checked for its shape only**: a handle, an "@", a bank's name,
    in the characters NPCI allows. Nothing offline can say whether it exists, and
    the back office card (4e2) must say so beside the field. A pay link pasted
    whole, or anything carrying "&" or "?", is refused.
  - **The link is built on the server, in the domain** (`payment.UPIAccount.Link`,
    pure): `upi://pay` with `pa`, `pn`, `am`, `cu=INR` and `tn` ("Table T5"). The
    amount is written from minor units, never through a float. Every value is
    encoded, so a payee's name or a note cannot add a parameter or change the
    address or the amount; a test tries.
  - **The bill carries it**, as `upi`: the link, the address, the payee and the
    amount, which is exactly the balance due. It is absent when the venue has no
    address, nothing is owed, or the bill is not in rupees. A failure to read the
    address costs the guest the link and not the bill.
  - **The phone shows a link drawn as the primary button**, "Pay ₹540.00 by UPI",
    the payee and address under it to check or type, and "After paying, show the
    payment to a member of staff. This bill is marked paid once they have seen
    it." It follows the server's link only if it begins `upi://pay?`.
  - **No secret and no network.** A UPI address is printed on every venue's
    counter. Nothing here needs the internet on the venue's side.
  - The guest's bundle is 39.1 KB gzipped of 60.
Because: it is what a cafe with a UPI sticker already does, with the amount and the
address filled in so neither is mistyped.
Cost: a guest can pay and leave before anyone has seen it; staff are in the same
position as with the sticker today, and 4e4 is what tells them to look. A guest
who pays twice, or pays a stale amount after another round was added, is put right
at the counter: the link is for the balance as of the last read, every 15 seconds.
Two guests at a table can each pay the whole balance; splitting is 4g. Until 4e2
the address is set only through the API. An address that is well shaped and wrong
sends guests' money to a stranger, and only a first small payment finds that out.
Seen by eye at 390 wide on a demo venue: the card with the button, the payee and
address, both sentences, and the link's text. Not seen: a UPI app opening on it, on
any phone; a phone with no UPI app; and the bill turning paid after staff record it.

## ADR-216 - The UPI address in the back office: under the venue's details, with the warning always there
2026-10 · Status: accepted. Phase 11, Task 4e2.
Context: ADR-215 made the venue's UPI address a setting with an API and no screen,
and said the screen must tell the owner the address cannot be checked.
Decision:
  - **A card, "Paid from a phone", under the venue's details** on Settings, in the
    Venue section. Where the venue is paid is a fact about who it is, like its
    legal name, and the other sections are tax and shifts. A section of its own
    was set aside for one card; the gateway (4f) and the mode (4e5) can make one.
  - **It reads and saves on its own**, as the cards of the other sections do, on
    the shared draft and frame (`useSectionDraft`, `SectionCard`): Save, Discard,
    the words a refusal shows, and the leave-without-saving prompt come with them.
    A failure to read it leaves the venue's card usable.
  - **The warning is always there, not only on a change**: "mez cannot check this
    address", why it matters, and what to do about it: pay a bill one rupee from
    the owner's own phone and see it arrive. An owner reading the card a month on
    should still be told the address was never verified by anything but that.
  - **The address is checked for its shape before it is sent**, by the same
    pattern as the server, which checks again. The message is the server's words.
  - **Where things stand is read from what is saved**, not what is typed, so the
    sentence never describes a draft. A venue not in rupees keeps the card and is
    told UPI will not be offered, since the bill's currency decides (ADR-215).
  - **No test payment from the card.** A button that opened a one rupee link on
    the office computer would open nothing: UPI apps are on phones.
Because: the one way this feature loses a venue money is a mistyped address, and
the only test of an address is a payment.
Cost: the warning is long and permanent on a screen the owner visits rarely. Seen
by eye at 1366 wide: the card under Venue, a bad address refused under its field,
a good one saved, the payee's name carried into the warning, and the standing
line. Not seen: the card at 400 wide, a venue not in rupees, the read failing, and
the leave prompt with this card's draft.

## ADR-217 - The QR code is drawn on the server, by an encoder of our own
2026-10 · Status: accepted. Phase 11, Task 4e3.
Context: a guest may want someone else's phone to pay, or the phone showing the bill
may be the one that would have to scan. That needs the pay link as a QR code. The
guest's bundle has a 60 KB budget with 39.1 used; mez's rule on dependencies is that
each needs a case.
Decision:
  - **The server draws it.** `GET /order/api/t/{code}/bill/upi.svg` answers with an
    SVG of the bill's own link, for the balance as it stands. The phone shows it as
    a picture. The bundle gains an `<img>` and no encoder: 39.4 KB.
  - **The encoder is ours, `internal/domain/qr`, about 400 lines, no dependency.**
    Bytes in, level M, versions 1 to 40, the mask that scores best. A QR code is a
    standard that will not change, the code is small enough to read whole, and a Go
    dependency is a thing an owner's antivirus may flag and we must keep current. A
    JavaScript library was set aside for the budget and because it would be tested
    less well than Go in this repository.
  - **It was checked against another encoder, module for module**: 178 codes
    against python-qrcode, every version from 1 to 40 full to the byte, a byte
    short, half full and nearly empty, and every mask. All matched.
    `TestAgainstAReferenceEncoder` does this when `QR_REFERENCE` names a program,
    and skips otherwise, so the build needs no Python.
  - **The build pins ten of those codes** by a fingerprint of the matrix
    (`golden_test.go`), chosen where the format changes: the smallest, one filled
    to the byte, version 7 (the first with a version number), 10 (a sixteen bit
    count), 14 (blocks of two lengths), 32 (the odd alignment spacing), 40. One
    code is kept whole, to read. The Reed-Solomon step is tested against the
    standard's own worked example, and the byte capacities against its table.
  - **A code the server drew was read back by a decoder**: the SVG from a running
    server, painted from its own rectangles and decoded by OpenCV, gave exactly the
    bill's link.
  - **Level M**, about 15% recoverable: the level a phone screen needs, and a
    smaller code than Q or H for the same link. A typical link is version 5, 37
    modules; the longest a venue's name allows is under version 20.
  - **The picture is only a picture**: one path of rectangles on a white ground
    with the standard's four module quiet band, served `image/svg+xml`, `nosniff`,
    with a content security policy that allows nothing, and never cached.
  - **The amount in the picture's address is for the browser only**, so the picture
    is fetched again when the balance moves. The server ignores it and draws the
    balance it has.
  - **The code is closed until the guest asks for it**, "Pay from another phone". A
    code left showing on a table is one a neighbour can scan, and it pays this bill.
  - **Nothing to pay is a 404**, `nothing_to_pay`: no ticket, no address, nothing
    owed, or not rupees.
Because: the cheapest byte in a 60 KB bundle is the one the server keeps.
Cost: four hundred lines we own. A picture is one more request, through the relay
too. The code and the sentence under it can disagree for up to 15 seconds after the
balance moves, until the bill is read again. **No phone has scanned one**: the
checks are an encoder that agrees with another and a decoder that reads it back,
which is strong evidence and not the same as a UPI app opening on it.

## ADR-218 - "I have paid" is a message to staff, kept apart from every payment
2026-10 · Status: accepted. Phase 11, Task 4e4a.
Context: a guest pays by UPI from the phone (ADR-215), and nothing tells the venue.
Staff confirm a payment by seeing it; what was missing is anything telling them to
look. That is a server and two screens, so it is two sessions.
Decision:
  - **Task 4e4 is split.** 4e4a is the server, this. 4e4b is the phone's button and
    the terminal's notice.
  - **A notice is a row, `paid_notices` (migration 00040), and nothing else.** It
    appends no event, settles no bill, and no report or Z-report reads it. A test
    holds the ticket's last seq, tenders and balance before and after. The only
    payment is still `payment.tendered`, recorded by a member of staff.
  - **`POST /order/api/t/{code}/bill/paid`** takes no body. It is refused,
    `nothing_to_pay`, when the table has no ticket, nothing is owed, or the venue
    takes no UPI: the same test the bill's pay link passes.
  - **It keeps what was owed when the guest said so**, which is the figure to look
    for on the venue's phone. The list also gives what is owed now, so a round
    added since shows as a difference and not as a mystery.
  - **One open notice a ticket**, by a unique index. Said forty times, or forty
    times at once, it is said once and the terminals are nudged once. This is the
    whole of the limit: no counter and no timer.
  - **The bill tells the phone staff have been told**, `paid_told`, read from the
    server each time, so it survives a reload and shows on every phone at the
    table. It changes no figure on the bill.
  - **Recording the payment puts the notice away without anyone doing so.** The
    list leaves out a notice whose ticket is settled or closed. Nothing is written
    for that: the row stays `open` and is never listed again.
  - **Staff can also put one away**, `POST /api/v1/paid-notices/{id}/seen`, when
    they looked and no money had come. It asks `payment.take`, records who and
    when, and changes nothing on the bill. After it the guest may say so again,
    and the bill no longer says staff were told.
  - **The notices ride on `GET /api/v1/guest-requests`**, as a second list, `paid`.
    They reach the same people by the same nudge, the floor version (ADR-211), and
    the terminal already reads that endpoint on it. Two lists and not one,
    because a request is answered and a notice is checked against a phone.
  - **A table's code is enough to say it.** Anyone holding a code can tell staff
    that table says it has paid. It costs staff a look at their phone, which is
    what they would do anyway before letting a table go.
Because: the dangerous version of this feature is the one where a tap on a guest's
phone marks a bill paid. Keeping the notice in a table no money code reads makes
that a thing that cannot happen by accident later.
Cost: a second table for guests' messages, beside `guest_rounds`. A notice whose
ticket closes stays `open` in the table for good. The listing projects each
notice's ticket to read what is owed now, a handful at most. Until 4e4b no screen
sends or shows one.

## ADR-219 - "I have paid" on the screens: the server says who was told, and the terminal leads to the payment
2026-10 · Status: accepted. Phase 11, Task 4e4b. Task 4e4 is done.
Context: ADR-218 built the notice. This is the phone's button and the terminal's
half, on the pill, sheet and chime of ADR-212.
Decision:
  - **The phone: "I have paid", a secondary button under Pay by UPI.** One tap: a
    wrong tap costs staff a look at their phone, which they would take anyway.
  - **Whether staff were told is read from the bill**, `paid_told`, never kept on
    the phone. Told, the button and the sentence above it give way to "Staff have
    been told you paid. This bill is marked paid once they have seen the payment."
    It reads the same after a reload and on the other phones at the table, and
    goes when staff put the notice away or record the payment.
  - **No figure on the bill moves for it**, and both screens say that nothing is
    paid until staff record it.
  - **A failure to tell sends the guest to staff**: "Show them the payment
    yourself." The feature exists to save that walk and must not hide it.
  - **The terminal: the same pill, sheet and chime as a request.** The pill counts
    both and words what it counts: "table request(s)", "table(s) say(s) paid", or
    "from tables" for some of each. The sheet is retitled "From the tables", with
    "Say they have paid" first, since a table waiting to leave is waiting on
    staff, and "Want to order" under it.
  - **A notice shows what the guest said they paid, and what the table owes now
    only when it differs**, in terracotta: a round added after, or part recorded.
  - **Take payment closes the sheet and opens that ticket's payment screen.** It
    records nothing. The UPI tender staff make there is the payment, and when it
    settles the bill the notice leaves the list by itself (ADR-218).
  - **No payment found is asked twice**, on the button, and puts the notice away.
    It needs `payment.take`; the list itself needs only `order.open`, so a waiter
    without it sees the notice and is refused the dismissal in words.
  - **No mark on the floor for it.** The table is already Ordered or Paying there,
    and the mark of ADR-212 means a table with no ticket.
  - The guest's bundle is 39.6 KB gzipped of 60.
Because: every piece was already built for requests; the one new idea a member of
staff has to hold is that this message is not money.
Cost: the sheet's title and the pill's words changed a task after they were
written. Take payment for a waiter opens the waiter's bill screen, which was not
looked at. Seen by eye: on the phone at 390 wide, the button, the green line after
it, and that line surviving a reload; on the terminal at 1366, the pill reading "2
from tables", the sheet with both lists, Take payment opening the table's payment
screen, and No payment found armed and confirmed. Not seen: the amount differing
from what is owed, a refusal on either screen, the waiter's frame, the chime for a
notice, and a payment recorded on the tender screen clearing the notice (the Go
tests cover that).

## ADR-220 - A gateway is two questions, and a charge becomes a payment exactly once
2026-10 · Status: accepted, the order of work by the owner. Phase 11, Task 4f1.
Context: Task 4 asks for "a pluggable gateway adapter (Razorpay/Cashfree/Stripe)
behind one interface", with a confirmed payment as the counter's own
`payment.tendered`. ADR-215 left UPI confirmed by staff, because a UPI link tells the
venue nothing. A gateway can tell it. The owner chose to build 4f before the prepay
mode, 4e5, which wants a payment that confirms itself.
Decision:
  - **4e5 follows 4f. Task 4f is split in six**, listed in the phase file: 4f1 the
    interface and the service, this; 4f2 Razorpay, its keys and a sweeper; 4f3 the
    guest's API and the phone; 4f4 unapplied charges on the terminal; 4f5 the
    webhook; 4f6 Cashfree, Stripe and the owner's choice.
  - **The interface is two questions** (`internal/gateway`): `Start` a charge and
    get the page to send the guest to, and ask a charge's `Status`. Hosted pages
    only: mez never sees a card. A provider's API, keys and names are inside its
    adapter, and nothing outside one names a provider.
  - **Only the provider's own answer says a charge was paid.** `Status` is asked
    by the server. Nothing a guest's browser carries back is believed, and the
    return address says nothing of whether they paid.
  - **A charge is a row, `gateway_charges` (migration 00041), not a payment.** The
    payment is the `payment.tendered` the charge puts on the ticket, by the order
    service's own `TenderPayment`, as actor 0 on device 0. So the invoice issued
    when a tender settles the bill, the receipt, the shift and the Z-report are the
    counter's, with nothing new to keep in step.
  - **Exactly once, by the tender's idempotency key**, `charge:<id>`. The order
    service already refuses to apply a key twice. How the charge was paid (method
    and the provider's payment id) is written to the row first, once, and the
    tender is made from the row: asked again, with whatever the provider says the
    second time, it is the same command under the same key. Twenty settles at once
    leave one tender; a test does it.
  - **Money the bill cannot take is kept, not forced on and not lost.** A charge
    is `unapplied`, with why in words, when the provider took another amount or
    currency than was asked, when the bill no longer owes that much (paid another
    way meanwhile), or when the bill is closed. It is logged as an error, listed by
    `Unapplied`, and never tried again: a person refunds it at the provider or
    records it by hand. 4f4 puts it in front of them.
  - **What may pass is tried again.** No shift open, a ticket on hold, a busy
    database, an unreachable provider: `Settle` returns the error, the charge stays
    `open`, and the next `Settle` or `Sweep` tries again. A payment that arrives
    after the close lands in the next shift, where it is recorded.
  - **A round added after a charge was started is simply still owed.** The charge
    goes on as a part payment.
  - **Starting again is the same charge** while one is open for the same ticket,
    provider and amount, and the provider is not asked. A table owing a different
    amount gets a new charge and the old is left open, since its page may still be
    on a phone; if both are paid the second is unapplied.
  - **`Sweep` settles every open charge**, so a guest who paid and closed the page
    is still recorded. 4f2 runs it on a timer.
  - **Nobody closes the bill for the guest.** A paid ticket is settled and
    invoiced and stays open for staff to close, as after a tender at the till. That
    leaves the stock movement of a close with a real staff id (KNOWN.md), and the
    table for a person to clear.
  - **A fake gateway for tests** (`internal/gateway/gatewaytest`), which does what
    a test says a provider did. Nothing here calls the internet.
Because: the two failures that cost a venue money are a payment recorded twice and
a payment recorded never. One is closed by a key the order service already checks,
the other by keeping every charge the provider took until a person has dealt with it.
Cost: a third table beside `guest_rounds` and `paid_notices`. A gateway needs the
internet at the venue, the one part of mez that does; it is for guests only, and the
till never touches it. Opening a charge and saving its row are not one transaction:
a charge can exist at the provider with no row, which nobody was given the page of.
A venue that changes provider with a charge open cannot ask the old one. Nothing is
wired: no provider, no key, no route and no screen until 4f2 and 4f3.

## ADR-221 - Razorpay by payment links, its keys outside the database, and a sweeper
2026-10 · Status: accepted. Phase 11, Task 4f2.
Context: ADR-220 left the interface with no provider behind it, no keys and nothing
running. Razorpay is the first: it is what an Indian cafe is likeliest to have.
Decision:
  - **A charge at Razorpay is a payment link** (`internal/gateway/razorpay`), from
    its Payment Links API as its own documentation gives it: `POST
    /v1/payment_links/` and `GET /v1/payment_links/{id}`, basic auth with the key
    id and secret. A link is Razorpay's hosted page at a short address, which is
    all the interface asks for: no checkout script in the guest's bundle, and no
    card anywhere near mez.
  - **Each link is for the whole amount or nothing** (`accept_partial` false), with
    our charge id as its `reference_id`, good for thirty minutes, and with
    Razorpay's own SMS, email and reminders turned off: mez has no number or
    address for a guest and must not have Razorpay invent a conversation.
  - **Status maps as**: `created` pending; `paid` paid, with `amount_paid`, and
    the method and id of the captured payment; `expired` and `cancelled` failed.
    `partially_paid`, which no link of ours allows, is answered as paid for the
    part that was, so the charge is flagged unapplied and not waited on. A status
    word mez does not know decides nothing.
  - **A method the bill has no name for is "other"**: net banking, a wallet, EMI,
    a bank transfer.
  - **Two kinds of failure, and the difference matters to what happens next.**
    Not reached, a timeout, a 5xx or a 429 is `gateway.ErrUnavailable`: nothing is
    known, ask again. A 401 or any other 4xx is `gateway.ErrRefused`, with
    Razorpay's own words: asking again gets the same answer.
  - **The keys are configuration, not settings**: `gateway`, `gateway_key_id` and
    `gateway_key_secret` in `config.toml`, the environment (`MEZ_GATEWAY_KEY_SECRET`)
    or a flag, as `relay_key` is. The settings table is in the database, the
    database is what Litestream backs up and what an owner copies to a USB stick,
    and the key secret is the key to the venue's money. The price is that the
    owner cannot set it from the back office, and whoever installs mez must.
  - **The secret goes to Razorpay as basic auth and nowhere else.** Tests hold
    that it is in no error from the adapter, the configuration or the start, and
    the real binary was run with one set and its output searched.
  - **A gateway that is set and cannot be built stops the start**, as a relay does
    (ADR-019): a name this build has no adapter for, or a key missing. Wrong keys
    are not known until the first charge, since building the adapter calls nobody.
  - **A sweeper** (`charges.Sweeper`, owned, with `Close`) settles open charges
    every 20 seconds. With none open it runs one small query and makes no call to
    the internet, which is every venue most of the time. A sweep that fails is
    said once, as a warning, and again when it recovers: the internet being down
    is not news to a venue.
  - **The adapter is tested against a stand-in server** speaking Razorpay's JSON,
    its documented examples among the answers. No test calls Razorpay.
Because: a payment link is the smallest thing Razorpay offers that is a hosted page
and can be asked about, and it needs nothing on the phone.
Cost: **no call to the real Razorpay has been made.** The adapter is built from the
documentation and agrees with a stand-in written from the same documentation; the
first real charge, with test keys, is the check that matters and is still to do.
A link's address is on Razorpay's domain, `rzp.io`. Thirty minutes is a guess at
how long a guest takes. A venue with a gateway set asks Razorpay every 20 seconds
for as long as any charge is open. Nothing starts a charge yet: no route and no
screen until 4f3.

## ADR-222 - Paying online from the phone: the venue's server asks the gateway, and the return address is the venue's own
2026-10 · Status: accepted. Phase 11, Task 4f3.
Context: ADR-220 and ADR-221 built the charge and the first gateway with nothing to
start one. This is the guest's API and the phone.
Decision:
  - **Two routes under the table's code**: `POST .../bill/charge` starts a charge
    for what is owed and answers the gateway's page; `GET .../bill/charge/{id}`
    settles it and answers how it stands. The bill says `online` when the venue has
    a gateway and something is owed. With no gateway the routes are not mounted and
    no bill mentions it.
  - **The return address is never taken from a request.** A charge is shared by a
    table: started again for the same amount, it is the same charge. If its return
    address came from the Host a request claims, one phone could start a charge
    naming an address of its choosing and the next phone at the table, paying that
    charge, would be sent there afterwards, to be told its payment failed and to
    pay again. So the address is `public_url`, a new setting, the venue's own
    account of where guests reach it, and the table's path. A test sends forged
    Host, X-Forwarded-Host, Origin and Referer and reads what the gateway was given.
  - **With `public_url` unset there is no return address**: the gateway shows its
    own page and the guest goes back to the bill themselves, where it has turned
    paid. Nothing breaks; it is one tap more.
  - **The return address has no fragment**, since a gateway adds its own words to
    the end of an address. The app, arriving at a table's bare address with a
    charge it started, opens on the bill.
  - **What a gateway adds to the return address is not read.** Whether a charge was
    paid is asked of the gateway by the venue's server (ADR-220), each time the
    phone asks how it stands.
  - **A charge is asked about by its own table's code and no other**, before the
    gateway is troubled: another table's code is a 404.
  - **A settle that decides nothing answers the charge as it was**, 200 and open:
    the gateway out of reach, or no shift open. The phone asks again; so does the
    sweep.
  - **The phone is told four things**: open, paid, failed, or taken and not on the
    bill. For the last it is told to show staff. Why the bill could not take it is
    the venue's business and is not sent.
  - **A gateway that refuses to start a charge is the owner's problem and says so
    in the log, as an error**: it is nearly always the keys. The guest is told
    paying online is not working, and to pay staff.
  - **The phone keeps which charge it started, and nothing else**, by table code,
    for 45 minutes. It asks every 4 seconds while that charge is open and the bill
    is on screen. With storage refused it forgets on leaving the page, and the
    sweep still puts the payment on the bill.
  - **Pay online is not offered while a charge this phone started is open**: a
    second payment for one bill is money to refund. "Open the payment page again"
    is offered instead.
  - **It follows a payment page's address only if it is https.**
  - **One primary on the bill**: Pay online where offered, and Pay by UPI drawn as
    the secondary under it.
  - The guest's bundle is 40.7 KB gzipped of 60.
Because: the one new way to lose a guest's money here was a redirect after paying,
and it is closed by a setting and not by trusting a header.
Cost: one more setting to get right, and a venue reached at more than one address
names one. **A real charge has still not been made.** The real binary was run with
Razorpay set and made-up keys: the bill offered Pay online, the adapter reached the
real Razorpay, was refused, and the phone and the log each said the right thing,
with no secret in the log. Seen by eye at 390 wide: the button, the line under it,
and that refusal. Not seen: the gateway's page, coming back, waiting, paid, failed,
or taken and not on the bill; the Go and vitest tests cover each against a fake.

## ADR-223 - Money on no bill stays in front of staff until a person says what they did
2026-10 · Status: accepted. Phase 11, Task 4f4.
Context: ADR-220 keeps a charge as `unapplied` when a gateway has a guest's money and
the bill could not take it, and until now only logged it. A log is read by nobody
during service.
Decision:
  - **An unapplied charge is on every terminal**, in the pill, sheet and chime of
    ADR-212, first in the sheet and in the danger colour: the pill turns to the
    danger tint whatever else it counts. It is the one thing there that is a
    guest's money gone astray.
  - **It shows what a person needs to act**: the table, the amount, how it was
    paid, why the bill could not take it in a sentence, and the gateway's id for
    the payment, which is what its dashboard is searched for and what a refund
    names.
  - **A person says which of two things they did**, refunded it at the gateway or
    recorded it on a bill by hand, and that is kept with who and when
    (`resolution`, `resolved_at`, `resolved_by`, migration 00042). The charge
    stays `unapplied`: that is what happened to it, and being dealt with is a
    second fact.
  - **Saying so moves no money and touches no bill.** mez does not refund at a
    gateway: the interface is two questions (ADR-220) and a refund is a third,
    with its own failures, for a later task if it is wanted. And it does not
    guess which bill a stray payment belongs on. A test holds the bill unchanged.
  - **Nothing else ever puts one away.** Not the shift closing, not the bill
    closing, not time. Said twice, the second is refused.
  - **Both answers are asked twice**, on the button: either takes it off every
    terminal for good.
  - **It asks `payment.take`** to say; anyone who can see requests can see it.
  - **It reaches terminals by the floor version** (ADR-211), moved when a charge
    becomes unapplied and when it is resolved, with the hub nudged
    (`charges.Options.OnChange`). A charge that is simply paid or fails moves
    nothing.
  - **The list rides on `GET /api/v1/guest-requests`**, as `unapplied`, empty at a
    venue with no gateway. `POST /api/v1/charges/{id}/resolve` takes `how`.
Because: the worst outcome of paying online is a guest charged for a bill that
shows unpaid, or charged twice. It cannot always be prevented, so it has to be
impossible to miss and impossible to lose.
Cost: the two buttons are a person's word and nothing checks it: "I refunded it"
with no refund made leaves a guest out of pocket and mez none the wiser. Nothing
reports resolved charges yet; they are in the table for whoever asks. A waiter
without `payment.take` sees the red pill and cannot clear it. Seen by eye at 1366
wide, on a charge put into a demo database by hand since no gateway works locally:
the pill in danger, the card with its sentence and reference, "I refunded it" armed
and confirmed, and the pill back to terracotta after. Not seen: a charge becoming
unapplied through a real gateway, the chime for one, the waiter's frame, a refusal.

## ADR-224 - A gateway's webhook is a hint to ask it now, and never proof
2026-10 · Status: accepted. Phase 11, Task 4f5.
Context: a paid charge reaches the bill when the guest's phone asks (ADR-222) or the
sweep does, up to 20 seconds later (ADR-221). A gateway can also call the venue the
moment it is paid. The call arrives from the internet, through the relay.
Decision:
  - **`POST /order/api/hook/{gateway}`**, under the guest's mount, because that is
    all the relay forwards: a gateway reaches the venue the way a guest does, at
    `<public_url>/order/api/hook/razorpay`. It takes no table code and no session.
  - **A notice is believed about one thing: which charge to go and ask about.**
    `charges.Notice` checks the call is the gateway's, finds the charge it names,
    and calls `Settle`, which asks the gateway itself (ADR-220). What the notice
    says of the charge's state, amount or method is never read. So a notice can
    make a payment land sooner and can make nothing land that would not have, and
    a forged one that somehow passed would cost one call to the gateway.
  - **The signature is checked anyway**, before any of the body is read, over the
    body exactly as it arrived, in constant time. For Razorpay, from its own
    documentation: `X-Razorpay-Signature` is the hex HMAC-SHA256 of the raw body
    under the webhook's own secret, which is not the API's. Events named
    `payment_link.*` name a link at `payload.payment_link.entity.id`; any other
    event is taken and does nothing.
  - **`gateway_webhook_secret` is configuration**, as the keys are (ADR-221).
    **Unset, no notice is believed**: the route answers 404 and the sweep alone
    finds out. A venue is no less correct without it, only up to 20 seconds slower
    for a guest who closed the page.
  - **An adapter may or may not take notices**: `gateway.Notified` is a second,
    optional interface, so the two questions of ADR-220 stay two.
  - **A notice that is the gateway's is answered 204 whatever came of it**: about
    a charge mez never started, about nothing, sent twice, or with the gateway
    then out of reach. A gateway told no keeps calling, and stops calling a venue
    that says no too often. One that is not the gateway's is 401 and a warning in
    the log.
  - **A notice sent twice is one payment**, by the tender's key (ADR-220). The
    event's id is not kept: there is nothing it would add.
  - A notice is at most 256 KB, and is counted against the guest API's limit by
    address like any other call there.
Because: the webhook is the one way money-shaped data reaches mez from a stranger's
address. Treating it as a doorbell and not a message means its contents cannot
matter.
Cost: a third secret to set, in two places that must agree (the gateway's dashboard
and the venue's configuration), and a webhook address to enter at the gateway by
hand. Wrong, it fails quietly: every call is a 401 and a warning, and payments still
land by the sweep. The path was not run through the relay. The real binary was sent
a signed call (204) and a forged, an unsigned and a tampered one (401 each), with no
secret in its log; **no call from the real Razorpay has been received**, so its
payload's shape here is its documentation's.

## ADR-225 - Paying first: a round held as a row, asked for the exact amount it adds to the bill
2026-10 · Status: accepted. Phase 11, Task 4e5a.
Context: Task 4 asks for "prepay-required and pay-at-table modes, per venue", and
ADR-209 says a prepaid order goes straight to the kitchen with no acceptance. With a
gateway (ADR-220) a payment can confirm itself, so a round can wait for its payment
and for nothing else. The question STATE left was what a round is between Place
Order and the payment.
Decision:
  - **Task 4e5 is split in four**, listed in the phase file: 4e5a the setting and
    the held round, this; 4e5b charging it and turning it into an order; 4e5c the
    phone; 4e5d the owner's control.
  - **It is a row, as a request is, and not a ticket.** Putting the lines on the
    ticket unsent, and charging the ticket's balance, would have reused the charge
    as it is. But most rounds that are placed and never paid are a guest changing
    their mind, and each would leave an open ticket on the floor and voided lines
    in the Z-report to get rid of it. ADR-210 kept unaccepted requests out of the
    books for that reason; an unpaid round is kept out for the same one.
  - **It is stored as a waiting round marked `prepay`** (migration 00043), and told
    as a status of its own, `unpaid`. The status column's check is not rebuilt.
  - **What it asks for is exact, and is the bill's own figure.** `order.Adding`
    projects the table's ticket with the round's lines added, or the lines as the
    first on a ticket of their own, and answers what the total rises by. It is the
    same projection that bills the lines when they are added, so there is one
    calculation. A test shows why: 5% GST on 33.00 is two halves of 0.825 that each
    round to 0.82, so the bill says 34.64 where any second sum would say 34.65.
    The amount is kept on the round as `due_minor`.
  - **A ticket that would not take the lines refuses the round before anyone is
    asked for money**: on hold, or invoiced.
  - **A seated table still pays first.** Where paying comes first it comes first
    for every round, whoever opened the ticket.
  - **An unpaid round is nothing to staff.** It is not listed, tells no terminal,
    and cannot be accepted or declined: to them it is not there. Only its payment
    moves it.
  - **The mode is a setting, `payment.mode`**, `pay_at_table` by default or
    `prepay`, on the payment settings beside the UPI address. A save that names no
    mode leaves it, so the back office card of ADR-216, which does not show it yet,
    cannot change it.
  - **It is in effect only where guests can pay online** (`Options.CanCharge`): a
    round held for a payment nobody can make is an order that never happens. The
    setting on with no gateway is the venue as it was. The settings answer says
    `can_charge`, for the card to explain.
  - **`CanCharge` is not wired in `cmd/mez` yet.** Until 4e5b can charge a held
    round, turning the mode on would hold every order unpayable. It is in effect
    only in tests.
  - **At most five unpaid rounds count against a table, for thirty minutes each.**
    They are counted apart from requests, so neither crowds the other out.
Because: the money a guest is asked for and the money the bill then shows must be
the same number from the same code, and an order nobody paid for should leave no
trace an owner has to explain.
Cost: between the asking and the paying the table's ticket can change, and what the
round adds may then differ by a paisa of rounding from what was paid; 4e5b has to
decide what is tendered then, and say so. A held round is priced against the ticket
as it stood, with a discount staff applied meanwhile not reaching it. Nothing ends
an unpaid round; it stops counting after thirty minutes and is never listed.
