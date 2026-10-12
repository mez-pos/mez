package config

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	// A venue in Bengaluru renders its invoices in Asia/Kolkata, and the
	// Windows machine under its counter has no zoneinfo database for Go to
	// read. Without this import time.LoadLocation succeeds on the developer's
	// machine and fails on the customer's. It costs about 450 KB in the binary.
	// See ADR-017.
	_ "time/tzdata"

	"github.com/byjoellobo/mez/internal/backup"
	"github.com/byjoellobo/mez/internal/cloud"
)

// FileName is the configuration file Load looks for beside the binary.
const FileName = "config.toml"

// envPrefix namespaces the environment: MEZ_LISTEN_ADDR, and so on.
const envPrefix = "MEZ_"

// Configuration keys. These are the names in config.toml; the environment and
// the flags are mechanical transforms of them, so there is exactly one list of
// settings in this package and adding one means touching one place.
const (
	keyListenAddr = "listen_addr"
	keyDataDir    = "data_dir"
	keyLogLevel   = "log_level"
	keyLogFormat  = "log_format"
	keyVenueID    = "venue_id"
	keyRelayURL   = "relay_url"
	keyRelayKey   = "relay_key"
	keyTimezone   = "timezone"
	keyDevOrigin  = "dev_origin"

	keyGateway          = "gateway"
	keyGatewayKeyID     = "gateway_key_id"
	keyGatewayKeySecret = "gateway_key_secret"
	keyPublicURL        = "public_url"

	keyGatewayWebhookSecret = "gateway_webhook_secret"

	keyRemoteBackoffice = "remote_backoffice"

	keyBackupSyncInterval = "backup_sync_interval"

	keyCloudURL = "cloud_url"
)

// GatewayRazorpay is the one payment gateway this build can use. The name
// is also internal/gateway/razorpay's, which this package, a leaf, does not
// import.
const GatewayRazorpay = "razorpay"

// GatewayStripe is Stripe, internal/gateway/stripe (ADR-230). It has no key
// id: gateway_key_secret is its secret key.
const GatewayStripe = "stripe"

// gateways is every gateway this build has an adapter for, as said to an
// owner who named another.
const gateways = GatewayRazorpay + " and " + GatewayStripe

// Log formats. Auto is text when stdout is a terminal and JSON otherwise, which
// is the difference between a developer reading lines and a support engineer
// grepping a file. internal/logging is what acts on these.
const (
	LogFormatAuto = "auto"
	LogFormatJSON = "json"
	LogFormatText = "text"
)

// keys is every setting this program understands. A key outside it is a typo,
// and a typo that loads silently is a port change the owner believes they made.
var keys = []string{
	keyListenAddr,
	keyDataDir,
	keyLogLevel,
	keyLogFormat,
	keyVenueID,
	keyRelayURL,
	keyRelayKey,
	keyTimezone,
	keyDevOrigin,
	keyGateway,
	keyGatewayKeyID,
	keyGatewayKeySecret,
	keyPublicURL,
	keyGatewayWebhookSecret,
	keyRemoteBackoffice,
	keyBackupSyncInterval,
	keyCloudURL,
}

var usage = map[string]string{
	keyListenAddr: "address to listen on, host:port with a numeric port",
	keyDataDir:    "directory holding the database, logs and backups",
	keyLogLevel:   "debug, info, warn or error",
	keyLogFormat:  "auto, json or text; auto is text on a terminal, json otherwise",
	keyVenueID:    "identifier for this venue, used in logs and on the relay",
	keyRelayURL:   "outbound relay endpoint; empty disables the relay",
	keyRelayKey:   "this venue's key for the relay, from the relay's operator; a secret",
	keyTimezone:   "IANA timezone for rendering, for example Asia/Kolkata",
	keyDevOrigin:  "browser origin allowed to call the API cross-site; empty disables CORS",

	keyGateway:          "payment gateway for guests paying online: razorpay or stripe, or empty for none",
	keyGatewayKeyID:     "the gateway's key id for this venue's account; stripe has none",
	keyGatewayKeySecret: "the gateway's key secret; a secret, best set in the environment",
	keyPublicURL:        "the address guests reach this venue at, https://cafe.example; where a gateway sends them back",

	keyGatewayWebhookSecret: "the secret the gateway signs its webhooks with; a secret; empty takes no webhooks",

	keyRemoteBackoffice: "true lets the owner open the back office through the relay; needs relay_url and a key issued for it",

	keyBackupSyncInterval: "how often the backup sends what changed, like 30s or 2m; from 5s to 10m; it is the most a dead machine can lose",

	keyCloudURL: "where a pairing code is exchanged; mez Cloud's own unless you run one",
}

// Config is the resolved, validated server configuration.
//
// Every field is a usable value rather than a string waiting to be parsed. A
// nonsense port or an unknown timezone stops the binary at startup, where a
// human is watching, instead of during service.
type Config struct {
	// ListenAddr is the host:port the HTTP server binds. Default ":7777".
	ListenAddr string

	// DataDir holds the database, the logs and the backups. Always absolute: a
	// Windows Service starts in system32, and a relative path there would
	// scatter a venue's sales somewhere nobody would think to back up.
	DataDir string

	// LogLevel is the minimum level written to stdout and the file log.
	LogLevel slog.Level

	// LogFormat is auto, json or text. It is a string rather than a type
	// because this package is a leaf and the handler that acts on it lives in
	// internal/logging.
	LogFormat string

	// VenueID identifies this venue in logs and to the relay. May be empty
	// until Phase 02 provisions one.
	VenueID string

	// RelayURL is the outbound tunnel endpoint. Empty means no relay, which is
	// the normal state: the register sells without it.
	RelayURL string

	// RelayKey is this venue's key for the relay. It is a secret: it is sent
	// to the relay and must not be logged. Whether one is present when a
	// relay is set is checked where the tunnel is built, not here, so that
	// each key can still be set on its own.
	RelayKey string

	// Timezone is the venue's local time, used to render timestamps and to
	// decide which business day a shift belongs to. Stored times stay UTC.
	Timezone *time.Location

	// DevOrigin is the one browser origin allowed to call this API cross-site,
	// which in practice means the Vite dev server. Empty, the default, means no
	// CORS headers at all: a shipped binary should not be reachable from a page
	// the venue did not open. See ADR-023.
	DevOrigin string

	// Gateway is the payment gateway guests pay online through, or empty
	// for none, which is the normal state: nothing at the till needs one.
	Gateway string

	// GatewayKeyID and GatewayKeySecret are this venue's keys at the
	// gateway. The secret is a secret: it goes to the gateway and must not
	// be logged. They are here and in the environment, not in the database,
	// so a backup of a venue's sales does not carry the key to its money
	// (ADR-221). Whether both are present when a gateway is set is checked
	// where the gateway is built, so that each can still be set on its own.
	GatewayKeyID     string
	GatewayKeySecret string

	// PublicURL is the address guests reach this venue at from outside:
	// its address at the relay, "https://cafe.example". It is where a
	// payment gateway sends a guest back after paying. It is the venue's to
	// say and never read from a request (ADR-222). Empty, the gateway shows
	// its own page and the guest returns to the bill themselves.
	PublicURL string

	// GatewayWebhookSecret is the secret the gateway signs its calls to
	// the venue with. It is a secret, and not the API's. Empty, no call is
	// believed and the sweep alone finds out what was paid (ADR-224).
	GatewayWebhookSecret string

	// RemoteBackoffice lets the back office, and the API it speaks to, be
	// reached through the relay, so the owner can open it away from the
	// venue (ADR-306). False, the default, the relay carries the guest app
	// and nothing else. Whether a relay is set at all, and whether its key
	// allows this, is checked where the tunnel is built, so that this key
	// can be set alone like the others.
	RemoteBackoffice bool

	// BackupSyncInterval is how often the backup sends what changed to its
	// destination, and so the most that is lost with the machine (ADR-315).
	// It is a setting of the process and not of the destination: where the
	// backup goes is the owner's to change in the back office, how often a
	// bucket is billed is whoever runs the machine's.
	BackupSyncInterval time.Duration

	// CloudURL is where a pairing code is exchanged (ADR-318): mez Cloud's
	// API unless the venue names another. Nothing is sent there until
	// someone at the venue types a code, and a venue that never does never
	// calls it.
	CloudURL string
}

// GatewayEnabled reports whether a payment gateway is configured.
func (c Config) GatewayEnabled() bool { return c.Gateway != "" }

// CORSEnabled reports whether a development origin is configured.
func (c Config) CORSEnabled() bool { return c.DevOrigin != "" }

// StreamOrigins are the browser origins allowed to open a WebSocket besides
// the one the page was served from. It is the dev origin or nothing, for
// the same reason CORS is: a shipped binary lets no other page in.
func (c Config) StreamOrigins() []string {
	if c.DevOrigin == "" {
		return nil
	}
	return []string{c.DevOrigin}
}

// LogDir is where the rotating file log lives.
func (c Config) LogDir() string { return filepath.Join(c.DataDir, "logs") }

// RelayEnabled reports whether an outbound relay is configured.
func (c Config) RelayEnabled() bool { return c.RelayURL != "" }

// Load resolves the configuration for the running binary from, in increasing
// order of precedence: defaults, config.toml beside the binary, the
// environment, and the command line.
//
// It works with no configuration present at all.
//
// When the operator passed -h the returned error wraps flag.ErrHelp; callers
// should check errors.Is and exit 0 rather than reporting a failure.
func Load() (Config, error) {
	return LoadArgs(os.Args[1:])
}

// LoadArgs is Load with the command line given rather than read. A
// subcommand parses its own flags and passes on only the ones it shares
// with the server, so that `mez setup -data-dir X` finds its database
// exactly where `mez -data-dir X` would.
func LoadArgs(args []string) (Config, error) {
	return load(args, os.LookupEnv, exeDir(), runtime.GOOS)
}

// Usage returns the flag documentation, the text -h asks for.
//
// It exists because parseFlags sends the flag package's own output to a buffer,
// so that an unknown flag comes back as an error instead of being printed from
// inside a library. That capture also swallows the usage text on -h, so the
// entrypoint asks for it here and prints it.
func Usage() string {
	var out strings.Builder
	set, _, _ := newFlagSet(&out)
	set.PrintDefaults()
	return out.String()
}

// load is the testable core of Load. Nothing below it reads process state, so
// precedence can be exercised without mutating the environment.
func load(args []string, lookup func(string) (string, bool), exeDir, goos string) (Config, error) {
	flags, explicitPath, err := parseFlags(args)
	if err != nil {
		return Config{}, err
	}

	path := explicitPath
	if path == "" {
		path = filepath.Join(exeDir, FileName)
	}
	fileValues, err := readFile(path, explicitPath != "")
	if err != nil {
		return Config{}, err
	}

	raw := defaults(lookup, goos)
	merge(raw, fileValues)
	merge(raw, fromEnv(lookup))
	merge(raw, flags)

	return build(raw)
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		// Nothing useful to do: fall back to the working directory and let
		// -config override.
		return "."
	}
	return filepath.Dir(exe)
}

// defaults is the configuration of a machine with nothing set up on it.
func defaults(lookup func(string) (string, bool), goos string) map[string]string {
	return map[string]string{
		keyListenAddr: ":7777",
		keyDataDir:    defaultDataDir(lookup, goos),
		keyLogLevel:   "info",
		keyLogFormat:  LogFormatAuto,
		keyVenueID:    "",
		keyRelayURL:   "",
		keyRelayKey:   "",
		keyTimezone:   "Local",
		keyDevOrigin:  "",

		keyGateway:          "",
		keyGatewayKeyID:     "",
		keyGatewayKeySecret: "",
		keyPublicURL:        "",

		keyGatewayWebhookSecret: "",

		keyRemoteBackoffice: "false",

		keyBackupSyncInterval: backup.DefaultSyncInterval.String(),

		keyCloudURL: cloud.DefaultURL,
	}
}

// defaultDataDir puts a venue's data where the platform expects it to survive a
// reinstall and a change of Windows user profile.
func defaultDataDir(lookup func(string) (string, bool), goos string) string {
	if goos != "windows" {
		return filepath.Join(".", "data")
	}
	base, ok := lookup("PROGRAMDATA")
	if !ok || strings.TrimSpace(base) == "" {
		base = "C:\\ProgramData"
	}
	return filepath.Join(base, "mez")
}

// merge applies one layer over another. A key present in src wins even when its
// value is empty, because "" is a real setting: it is how relay_url is turned
// off again after a file switched it on.
func merge(dst, src map[string]string) { maps.Copy(dst, src) }

// readFile reads config.toml if it is there.
//
// A missing file at the default location is normal: the binary must boot on a
// bare machine. A missing file at a path the operator typed is an error, since
// going on without it would start the server with settings they believe they
// changed.
func readFile(path string, explicit bool) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !explicit {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	defer f.Close()

	values, err := parseTOML(f)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	for k := range values {
		if !known(k) {
			return nil, fmt.Errorf("reading config %s: unknown key %q (known keys: %s)",
				path, k, strings.Join(keys, ", "))
		}
	}
	return values, nil
}

// fromEnv collects the MEZ_* variables that are actually set.
func fromEnv(lookup func(string) (string, bool)) map[string]string {
	values := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := lookup(envPrefix + strings.ToUpper(k)); ok {
			values[k] = v
		}
	}
	return values
}

// parseFlags reads the command line and returns only the flags the operator
// actually passed. An unset flag must not overwrite the file or the environment
// with its zero value, which is what fs.Visit buys over reading the pointers.
func parseFlags(args []string) (map[string]string, string, error) {
	var out strings.Builder
	set, dest, path := newFlagSet(&out)

	if err := set.Parse(args); err != nil {
		if msg := strings.TrimSpace(out.String()); msg != "" {
			return nil, "", fmt.Errorf("parsing flags: %w\n%s", err, msg)
		}
		return nil, "", fmt.Errorf("parsing flags: %w", err)
	}
	if set.NArg() > 0 {
		return nil, "", fmt.Errorf("parsing flags: unexpected argument %q", set.Arg(0))
	}

	values := make(map[string]string)
	set.Visit(func(f *flag.Flag) {
		if p, ok := dest[keyName(f.Name)]; ok {
			values[keyName(f.Name)] = *p
		}
	})
	return values, *path, nil
}

// newFlagSet registers every setting as a flag, writing into dest. One
// definition serves both parsing and Usage, so the two cannot describe
// different programs.
func newFlagSet(out *strings.Builder) (*flag.FlagSet, map[string]*string, *string) {
	set := flag.NewFlagSet("mez", flag.ContinueOnError)
	set.SetOutput(out)

	dest := make(map[string]*string, len(keys))
	for _, k := range keys {
		dest[k] = set.String(flagName(k), "", usage[k])
	}
	path := set.String("config", "", "path to "+FileName+" (default: beside the binary)")

	return set, dest, path
}

// flagName maps a config key to its flag: data_dir becomes -data-dir.
func flagName(key string) string { return strings.ReplaceAll(key, "_", "-") }

// keyName is the inverse of flagName.
func keyName(name string) string { return strings.ReplaceAll(name, "-", "_") }

func known(key string) bool { return slices.Contains(keys, key) }

// build turns resolved strings into the typed, validated Config. This is the
// system boundary: nothing past it should have to wonder whether a value
// parses.
func build(raw map[string]string) (Config, error) {
	cfg := Config{
		VenueID:  strings.TrimSpace(raw[keyVenueID]),
		RelayURL: strings.TrimSpace(raw[keyRelayURL]),
		RelayKey: strings.TrimSpace(raw[keyRelayKey]),

		GatewayKeyID:     strings.TrimSpace(raw[keyGatewayKeyID]),
		GatewayKeySecret: strings.TrimSpace(raw[keyGatewayKeySecret]),

		GatewayWebhookSecret: strings.TrimSpace(raw[keyGatewayWebhookSecret]),
	}

	var err error
	if cfg.ListenAddr, err = validAddr(raw[keyListenAddr]); err != nil {
		return Config{}, err
	}
	if cfg.DataDir, err = validDataDir(raw[keyDataDir]); err != nil {
		return Config{}, err
	}
	if cfg.LogLevel, err = validLevel(raw[keyLogLevel]); err != nil {
		return Config{}, err
	}
	if cfg.LogFormat, err = validFormat(raw[keyLogFormat]); err != nil {
		return Config{}, err
	}
	if err = validRelayURL(cfg.RelayURL); err != nil {
		return Config{}, err
	}
	if cfg.Timezone, err = validTimezone(raw[keyTimezone]); err != nil {
		return Config{}, err
	}
	if cfg.DevOrigin, err = validOrigin(raw[keyDevOrigin]); err != nil {
		return Config{}, err
	}
	if cfg.Gateway, err = validGateway(raw[keyGateway]); err != nil {
		return Config{}, err
	}
	if cfg.PublicURL, err = validPublicURL(raw[keyPublicURL]); err != nil {
		return Config{}, err
	}
	if cfg.RemoteBackoffice, err = validSwitch(keyRemoteBackoffice, raw[keyRemoteBackoffice]); err != nil {
		return Config{}, err
	}
	if cfg.BackupSyncInterval, err = validSyncInterval(raw[keyBackupSyncInterval]); err != nil {
		return Config{}, err
	}
	if cfg.CloudURL, err = validCloudURL(raw[keyCloudURL]); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validCloudURL reads where pairing codes are exchanged. Empty is mez
// Cloud's own. A code is a secret for the hour it lives and what comes back
// for it holds two that last, so the address is held to what
// internal/cloud will send them to: https, or this machine.
func validCloudURL(v string) (string, error) {
	v = strings.TrimRight(strings.TrimSpace(v), "/")
	if v == "" {
		return cloud.DefaultURL, nil
	}
	if err := cloud.CheckURL(v); err != nil {
		return "", fmt.Errorf("%s: %w", keyCloudURL, err)
	}
	return v, nil
}

// validSyncInterval reads how often the backup sends. A bare number is
// refused and not read as seconds: 30 and 30s are told apart by nobody who
// is guessing, and time.ParseDuration would call the first an error anyway,
// in words about units that say less than these.
func validSyncInterval(v string) (time.Duration, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return backup.DefaultSyncInterval, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s %q: must be a length of time with its unit, like 30s or 2m", keyBackupSyncInterval, v)
	}
	if err := backup.CheckSyncInterval(d); err != nil {
		return 0, fmt.Errorf("%s %q: %w", keyBackupSyncInterval, v, err)
	}
	return d, nil
}

// validSwitch reads a setting that is on or off. Only the two words: "1",
// "yes" and "on" are what someone types when guessing, and a guess that
// loads is a door the owner does not know is open, or believes is.
func validSwitch(key, v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		return true, nil
	case "false", "":
		return false, nil
	default:
		return false, fmt.Errorf("%s %q: must be true or false", key, v)
	}
}

func validAddr(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fmt.Errorf("%s: must not be empty, use :7777 to listen on every interface", keyListenAddr)
	}
	_, port, err := net.SplitHostPort(v)
	if err != nil {
		return "", fmt.Errorf("%s %q: want host:port, for example :7777: %w", keyListenAddr, v, err)
	}
	// Named ports resolve differently per machine, and a register that binds a
	// surprising port is a register nobody can reach. Numbers only.
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("%s %q: port must be a number between 1 and 65535", keyListenAddr, v)
	}
	return v, nil
}

func validDataDir(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fmt.Errorf("%s: must not be empty", keyDataDir)
	}
	abs, err := filepath.Abs(v)
	if err != nil {
		return "", fmt.Errorf("%s %q: %w", keyDataDir, v, err)
	}
	return abs, nil
}

func validLevel(v string) (slog.Level, error) {
	var level slog.Level
	text := strings.TrimSpace(v)
	if text == "" {
		return level, fmt.Errorf("%s: must not be empty, want debug, info, warn or error", keyLogLevel)
	}
	if err := level.UnmarshalText([]byte(strings.ToUpper(text))); err != nil {
		return level, fmt.Errorf("%s %q: want debug, info, warn or error", keyLogLevel, v)
	}
	return level, nil
}

func validFormat(v string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(v))
	switch format {
	case LogFormatAuto, LogFormatJSON, LogFormatText:
		return format, nil
	default:
		return "", fmt.Errorf("%s %q: want %s, %s or %s",
			keyLogFormat, v, LogFormatAuto, LogFormatJSON, LogFormatText)
	}
}

// validRelayURL accepts the address of a relay this venue may dial.
//
// The venue's key goes out as a bearer token on the first request of every
// connection, so the connection must be TLS: only wss and https are allowed
// for any host that is not this machine. Plain ws and http are kept for a
// relay on loopback, which is how one is run while developing it.
func validRelayURL(v string) error {
	if v == "" {
		return nil
	}
	u, err := url.Parse(v)
	if err != nil {
		return fmt.Errorf("%s %q: %w", keyRelayURL, v, err)
	}
	if u.Host == "" {
		return fmt.Errorf("%s %q: missing host", keyRelayURL, v)
	}
	switch u.Scheme {
	case "wss", "https":
	case "ws", "http":
		if !loopbackHost(u.Hostname()) {
			return fmt.Errorf("%s %q: the venue's key travels on this connection, so the scheme must be wss or https (plain %s is allowed only for a relay on this machine)",
				keyRelayURL, v, u.Scheme)
		}
	default:
		return fmt.Errorf("%s %q: scheme must be wss or https", keyRelayURL, v)
	}
	return nil
}

// loopbackHost reports whether a URL's hostname can only be this machine.
func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// validOrigin checks that dev_origin is an origin and not a URL. Browsers
// compare the Origin header by scheme, host and port exactly, so a trailing
// slash or a path is a rule that silently never matches.
func validOrigin(v string) (string, error) {
	origin := strings.TrimSpace(v)
	if origin == "" {
		return "", nil
	}
	u, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("%s %q: %w", keyDevOrigin, v, err)
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return "", fmt.Errorf("%s %q: scheme must be http or https", keyDevOrigin, v)
	case u.Host == "":
		return "", fmt.Errorf("%s %q: missing host", keyDevOrigin, v)
	case u.Path != "" || u.RawQuery != "" || u.Fragment != "":
		return "", fmt.Errorf("%s %q: want an origin with no path, for example http://localhost:5173", keyDevOrigin, v)
	}
	return u.Scheme + "://" + u.Host, nil
}

func validTimezone(v string) (*time.Location, error) {
	name := strings.TrimSpace(v)
	if name == "" || name == "Local" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%s %q: unknown timezone, want an IANA name like Asia/Kolkata: %w", keyTimezone, v, err)
	}
	return loc, nil
}

// validGateway accepts no gateway or one this build has an adapter for. A
// name it does not know stops the start: a gateway the owner believes is
// taking payments, and is not, is worse than none.
func validGateway(v string) (string, error) {
	switch v = strings.ToLower(strings.TrimSpace(v)); v {
	case "", GatewayRazorpay, GatewayStripe:
		return v, nil
	default:
		return "", fmt.Errorf("%s %q: the gateways are %s, or empty for none", keyGateway, v, gateways)
	}
}

// validPublicURL accepts nothing, or an http or https address with a host
// and no path, query or fragment: guests' paths are added to it.
func validPublicURL(v string) (string, error) {
	v = strings.TrimRight(strings.TrimSpace(v), "/")
	if v == "" {
		return "", nil
	}
	u, err := url.Parse(v)
	switch {
	case err != nil:
		return "", fmt.Errorf("%s %q: %w", keyPublicURL, v, err)
	case u.Scheme != "http" && u.Scheme != "https":
		return "", fmt.Errorf("%s %q: scheme must be http or https", keyPublicURL, v)
	case u.Host == "":
		return "", fmt.Errorf("%s %q: missing host", keyPublicURL, v)
	case u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil:
		return "", fmt.Errorf("%s %q: give the address alone, with no path after it", keyPublicURL, v)
	}
	return v, nil
}
