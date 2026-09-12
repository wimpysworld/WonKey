# WonKey 1️⃣

One key. Your rules.

WonKey is a Linux command-line tool for the XFKEY One Key Max, model `0112`.
Use `key` for the key combination, `rgb` for lighting, and `restore` for saved settings.

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
Bare `wonkey`, `--help`, and help for each command show help without device access.

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
./wonkey restore
```

A key expression is the **complete combination**. `key f13` clears all modifiers.
Supported keys are `enter` and `f13`, with optional `ctrl`, `shift`, `alt`, and `gui` modifiers.
`--on` accepts `press`, `release`, or `both`. Omit it to preserve the trigger.
RGB modes are `gradient`, `steady`, `flowing`, `flash`, `neon`, `off`, `held`, and `toggle`.
Omit the six-digit RGB value to preserve the colour. RGB commands preserve key settings.

WonKey selects a single compatible device automatically. Multiple matches require terminal selection by a displayed physical path.
Changes require a terminal. At `Save settings? [Y/n]: `, press Enter to accept the default Yes.
You can also enter `y` or `yes`, ignoring letter case. `n`, `no`, any other answer, or EOF cancels.
Blank input still cancels device and capture selection. There is no confirmation bypass.
No-op and cancellation send no settings writes and create no backup.
After confirmation, WonKey saves and validates a fresh durable backup, rejects changed device state, uploads, and compares all 128 readback bytes.
It never retries or rolls back automatically. Readback does not establish persistence after reconnect.
A settings capture is not a firmware backup.

`restore` lists compatible backups for selection, newest first, excluding settings that already match.
Use `restore /path/to/capture-directory` for a specific backup, including one in an old storage location.
Restore changes the saved key, modifiers, trigger, RGB mode, and colour. It preserves all other current bytes.
A complete backup remains a restore source even if its later transaction failed.

Backups use `$XDG_STATE_HOME/wonkey/captures` when `XDG_STATE_HOME` is absolute and non-empty.
Otherwise, backups use `$HOME/.local/state/wonkey/captures`. The fallback requires an absolute, non-empty `HOME`.
WonKey has no custom storage configuration. It ignores `WONKEY_CAPTURE_ROOT` and `XFKEY_CAPTURE_ROOT`.
Directories use `YYMMDD-HHMMSS_key-KEY_rgb-MODE-COLOUR`, for example `260912-083853_key-ctrl-alt-f13_rgb-steady-0000ff`.
Names describe captured settings, not requested changes. See [capture naming and records](wiki/hardware.md#apply-safety-and-records) for fallbacks.
After a successful write, WonKey keeps the newest 10 owned backups for that model and identifier.
Human output honours `NO_COLOR`, `TERM=dumb`, and redirected output.

## Documentation

- [Usage and configuration](wiki/usage.md): key combinations, RGB modes, and options.
- [Hardware operations and safety](wiki/hardware.md): selection, backups, permissions, and records.
- [Development](wiki/development.md): architecture, offline checks, and evidence fixtures.
- [Protocol and evidence](wiki/protocol.md): reply encodings and pinned sources.
- [Developer instructions](AGENTS.md): architecture, safety rules, and offline checks.
