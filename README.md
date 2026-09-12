# WonKey 1️⃣

One key. Your rules.

WonKey is a Linux command-line tool for the XFKEY One Key Max, model `0112`.
Use `show` to read current settings and `set` to change the key or lighting.
It uses Kong for command parsing.

On one tested unit, F13 key output and steady-blue RGB persisted after reconnect.
Other settings and devices remain unverified. Device selection requires exact descriptor matches, not only a product name.

> [!WARNING]
> `set` changes stored settings unless you use `--dry-run`. Configuration is not firmware flashing, and a settings capture is not a firmware backup.

## Get started

Requires Linux, Go 1.23 or later, and [just](https://just.systems/). Run commands from the project root.

```sh
just build
./wonkey --help
```

The build disables VCS stamping and replaces `./wonkey`.
See [AGENTS.md](AGENTS.md#build-and-test) for a temporary-output build.

Start with `show`, then use `set`. These commands access hardware. `set` changes stored settings after confirmation.

```sh
./wonkey show
./wonkey set key=f13
./wonkey set light=steady:0000ff
./wonkey set key=f13 light=steady:0000ff
./wonkey set key=f13 --dry-run    # Reads the device; no files or settings writes
```

WonKey selects the only compatible device. If multiple devices match, add `--device PHYSICAL_PATH` to `show` or `set`.
`set` reads current settings, shows the changes, and requires the exact answer `write`. A blank answer cancels.
After confirmation, it creates and validates a durable backup, checks that identity and settings still match the approved plan, then writes.
Use `--yes` to skip confirmation only. No-op and cancellation create no backup and send no settings writes.
Only explicit settings change. `key=f13` preserves trigger and modifiers. `light=steady:0000ff` explicitly sets both mode and colour.

Discovery with `./wonkey advanced devices` is optional. It reads system metadata, not offline files.
Offline planning is also optional and needs a completed apply backup. `show` does not create one:

```sh
./wonkey advanced plan ./capture key=f13 light=steady:0000ff
```

A saved plan never authorises a write. Protocol tools are under `advanced protocol`.
Old top-level commands remain hidden aliases. Legacy `apply` still requires a target and `--write`, with unchanged safety checks.
Use `show --target` for a copyable legacy target. JSON target fields keep their existing format.
Use `--json` for scripts. Human output uses colour only on capable terminals and honours `NO_COLOR` and `TERM=dumb`.
Readback verification compares all 128 bytes. It does not verify persistence after reconnect.
After a successful write, WonKey keeps the newest 10 owned backups for that model and identifier.
Legacy `apply` also saves a backup and runs retention for a no-op.

## Documentation

- [Wiki index](wiki/README.md): usage, hardware safety, and protocol reference.
- [Usage and configuration](wiki/usage.md): offline plans, supported settings, lighting, and colour.
- [Hardware operations and safety](wiki/hardware.md): inspection, permissions, backup, apply, and restore limits.
- [Protocol and evidence](wiki/protocol.md): reply encodings, offline parsers, and pinned sources.
- [Developer instructions](AGENTS.md): architecture, safety rules, and offline checks.
