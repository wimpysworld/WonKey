# WonKey 1️⃣

One key. Your rules.

WonKey is a Linux command-line tool for the XFKEY One Key Max, model `0112`.
It inspects descriptors, captures settings queries, plans changes offline, and applies explicit key and RGB settings.
It uses only the Go standard library.

On one tested unit, F13 key output and steady-blue RGB persisted after reconnect.
Other settings and devices remain unverified. Device selection requires exact descriptor matches, not only a product name.

> [!WARNING]
> `apply --write` changes stored settings. Configuration is not firmware flashing, and a settings capture is not a firmware backup.

## Get started

Requires Linux and Go 1.23 or later. Run commands from the project root.

```sh
go build -buildvcs=false -o ./wonkey ./cmd/wonkey
./wonkey plan --capture internal/xfkey/testdata/hardware-20260911 --key f13
```

The build disables VCS stamping, so it also works without VCS metadata.
If you have [just](https://just.systems/), `just build` runs the same build.
Both build commands replace `./wonkey`. See [AGENTS.md](AGENTS.md#build-and-test) for a temporary-output build.

The plan example reads the included capture. It does not open a device or change settings.
Only explicit fields change in a plan. A saved plan never authorises a write.

## Documentation

- [Wiki index](wiki/README.md): usage, hardware safety, and protocol reference.
- [Usage and configuration](wiki/usage.md): offline plans, supported fields, and RGB modes.
- [Hardware operations and safety](wiki/hardware.md): inspection, permissions, backup, apply, and restore limits.
- [Protocol and evidence](wiki/protocol.md): reply encodings, offline parsers, and pinned sources.
- [Developer instructions](AGENTS.md): architecture, safety rules, and offline checks.
