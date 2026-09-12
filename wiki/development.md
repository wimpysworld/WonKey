# Developer tools

[Wiki index](README.md) · [Protocol and evidence](protocol.md)

`wonkey-dev` contains protocol tools, saved-capture analysis, and the legacy CLI.
It is separate from the consumer `wonkey` executable and is not part of the default build or install.
Both entry points keep their logic in `internal/xfkey` and share the same guarded transaction.

## Separate build

From the project root:

```sh
go build -buildvcs=false -o wonkey-dev ./cmd/wonkey-dev
./wonkey-dev --help
```

For a build that does not replace a local binary, use a fresh temporary output directory as described in [AGENTS.md](../AGENTS.md#build-and-test).
The fixed-path `capture-settings.sh` wrapper builds `cmd/wonkey-dev` into its private temporary directory.
It does not use the consumer parser. Do not execute the wrapper without separate hardware and permission authority.

## Offline analysis

These examples read a completed backup, not a device:

```sh
./wonkey-dev advanced plan "$capture" key=f13
./wonkey-dev advanced plan "$capture" light=steady:0000ff --json
```

Planning checks the completion marker, identity, raw replies, and reconstructed configuration.
The labelled hardware fixture has no authentic `result.json` completion marker, so it is not accepted as a complete backup.
Do not add a fabricated completion marker to authentic evidence. A saved plan never authorises a live write.

`wonkey-dev protocol preview` deliberately constructs a complete replacement and requires `--replace-all`.
It remains offline and is not an apply input. See [offline protocol operations](protocol.md#existing-offline-operations) for parser examples.
Protocol commands always produce JSON.

## Legacy live tooling

The developer binary retains `show`, `set`, `advanced`, `devices`, `plan`, `apply`, and protocol/capture aliases.
This preserves explicit access to the old tooling, not compatibility in `wonkey`.
Legacy `set key=f13` preserves modifiers. Public `wonkey key f13` clears them.

Developer `show --target` supplies a copyable target. Legacy `apply TARGET key=f13 --write` requires the target and explicit write gate.
Targets bind path, model, identifier, and version. Encoded `wonkey-target-v1:` targets and old flags remain accepted here only.
Developer `apply` creates a backup before confirmation, including no-op and cancelled transactions.
Developer `set` previews and confirms before creating its backup. `set --dry-run` reads hardware but saves no files.
`--yes` skips legacy confirmation only, never backup or validation. `--json` keeps prompts and diagnostics on stderr.

Legacy settings assignments are `key=`, `trigger=`, `modifiers=`, `lighting=`, `colour=`, and `light=mode:RGB`.
Friendly settings flags and the old numeric settings remain accepted. Do not combine assignments with flags or friendly values with numeric compatibility values.
Numeric values are trigger `1`–`3`, modifiers `0`–`15`, RGB mode `0`–`7`, and channels `0`–`255`.
Unspecified legacy settings stay unchanged. Duplicate or overlapping settings fail.

Legacy `--capture-root` overrides the environment fallbacks. Otherwise, storage uses `WONKEY_CAPTURE_ROOT`, then `XFKEY_CAPTURE_ROOT`, then `$HOME/.local/state/wonkey/captures`.
Read [hardware operations and safety](hardware.md) before discovery, queries, or writes. No example grants permission to execute a live operation.
