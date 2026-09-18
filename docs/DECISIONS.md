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
