# WonKey 1️⃣

One key. Your rules.

WonKey is a Linux command-line tool for the XFKEY One Key Max, model `0112`.
Use `key` for the key combination and `rgb` for lighting.

On one tested unit, F13 key output and steady-blue RGB persisted after reconnect.
Other settings and devices remain unverified. Device selection requires exact descriptor matches, not only a product name.

## Get started

Requires Linux, Go 1.23 or later, and [just](https://just.systems/). Run commands from the project root.

```sh
just build
./wonkey --help
```

The build disables VCS stamping and replaces `./wonkey`.
See [AGENTS.md](AGENTS.md#build-and-test) for a temporary-output build.
Bare `wonkey`, `--help`, `key --help`, and `rgb --help` show help without device access.

These queries read the device without creating files:

```sh
./wonkey key
./wonkey rgb
```

These commands change stored settings after confirmation:

```sh
./wonkey key f13
./wonkey key ctrl+shift+f13
./wonkey key f13 --on release
./wonkey rgb steady 0000ff
./wonkey rgb steady
./wonkey rgb off
```

A key expression is the **complete combination**. `key f13` clears all modifiers.
Supported keys are `enter` and `f13`, with optional `ctrl`, `shift`, `alt`, and `gui` modifiers.
`--on` accepts `press`, `release`, or `both`. Omit it to preserve the trigger.
RGB modes are `gradient`, `steady`, `flowing`, `flash`, `neon`, `off`, `held`, and `toggle`.
Omit the six-digit RGB value to preserve the colour. RGB commands preserve key settings.

WonKey selects a single compatible device automatically. Multiple matches require terminal selection by a displayed physical path.
Changes require a terminal. At `Save settings? [Y/n]:`, press Enter or answer `y` or `yes` to confirm (case-insensitive).
Answer `n` or `no` to cancel. Invalid input or EOF also cancels. There is no confirmation bypass.
No-op and cancellation send no settings writes and create no backup.
After confirmation, WonKey saves and validates a fresh durable backup, rejects changed device state, uploads, and compares all 128 readback bytes.
It never retries or rolls back automatically. Readback does not establish persistence after reconnect.
A settings capture is not a firmware backup or a proven restore image.

Backups use `WONKEY_CAPTURE_ROOT`, then `XFKEY_CAPTURE_ROOT`, then `$HOME/.local/state/wonkey/captures` (first non-empty value).
Directories use `YYMMDD-HHMMSS_key-KEY_rgb-MODE-COLOUR`, for example `260912-083853_key-ctrl-alt-f13_rgb-steady-0000ff`.
Names describe captured settings, not requested changes. See [capture naming and records](wiki/hardware.md#apply-safety-and-records) for fallbacks.
After a successful write, WonKey keeps the newest 10 owned backups for that model and identifier.
Human output honours `NO_COLOR`, `TERM=dumb`, and redirected output.

## Developer tools

The old `show`, `set`, `apply`, `plan`, and protocol interfaces are **not accepted by `wonkey`**.
Old scripts must use the separately built [`wonkey-dev`](wiki/development.md) executable.
It is not part of the default build or install. The consumer executable has no JSON, device-selection, storage, or confirmation-bypass flags.

## Documentation

- [Usage and configuration](wiki/usage.md): key combinations, RGB modes, and options.
- [Hardware operations and safety](wiki/hardware.md): selection, backups, permissions, and records.
- [Developer tools](wiki/development.md): separate build, saved-capture analysis, and legacy tooling.
- [Protocol and evidence](wiki/protocol.md): reply encodings and pinned sources.
- [Developer instructions](AGENTS.md): architecture, safety rules, and offline checks.
