# WonKey 1️⃣

One key. Your rules.

WonKey is a Linux command-line tool for the XFKEY One Key Max, model `0112`.
It finds device paths, reads settings without files, plans changes from apply backups, and applies explicit key and lighting settings.
It uses Kong for command parsing.

On one tested unit, F13 key output and steady-blue RGB persisted after reconnect.
Other settings and devices remain unverified. Device selection requires exact descriptor matches, not only a product name.

> [!WARNING]
> `apply --write` changes stored settings. Configuration is not firmware flashing, and a settings capture is not a firmware backup.

## Get started

Requires Linux, Go 1.23 or later, and [just](https://just.systems/). Run commands from the project root.

```sh
just build
./wonkey --help
./wonkey devices
```

The build disables VCS stamping and replaces `./wonkey`.
See [AGENTS.md](AGENTS.md#build-and-test) for a temporary-output build.

Use `show` to read current settings in memory and get a target token. `show` creates no files.
A target token binds the device path and verified identity for `apply`. Apply creates and validates a durable pre-write backup.
Use `plan` offline with an apply backup. A saved plan never authorises a write.
After a successful apply or no-op, WonKey keeps the newest 10 owned backups for that model and identifier.

## Documentation

- [Wiki index](wiki/README.md): usage, hardware safety, and protocol reference.
- [Usage and configuration](wiki/usage.md): offline plans, supported settings, lighting, and colour.
- [Hardware operations and safety](wiki/hardware.md): inspection, permissions, backup, apply, and restore limits.
- [Protocol and evidence](wiki/protocol.md): reply encodings, offline parsers, and pinned sources.
- [Developer instructions](AGENTS.md): architecture, safety rules, and offline checks.
