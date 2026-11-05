# Development

[Wiki home](Home) · [Protocol and evidence](protocol)

WonKey has one executable: bare `wonkey` inspects connected devices, and `key`, `trigger`, `macro`, `mouse`, `media`, `rgb`, and `led` validate device capabilities and configure an XFKEY One Key Max (`0112`).
`cmd/wonkey` handles process entry and exit. CLI, protocol, and unified Linux device discovery across XFKEY One Key Max (`0112`) and CH57 (`1189:8890`) stay in `internal/xfkey` and `internal/ch57`.

## Offline checks

Run the [build and test checks](https://github.com/wimpysworld/WonKey/blob/main/AGENTS.md#build-and-test) from the project root.
They use synthetic transports without connected hardware.

The [protocol reference](protocol) records framing, configuration fields, and pinned source evidence.
The developer executable, legacy command interfaces, and helper scripts are removed.

## Evidence fixtures

Keep `internal/xfkey/testdata/hardware-20260911` byte-for-byte unchanged, including warnings, paths, and genuine hardware identifiers.
Copy fixtures to temporary directories before corruption tests.

The authentic fixture confirms query framing.
Do not modify evidence to match changed output.
Query fixtures do not prove upload effects or persistence after reconnect.

See [hardware safety](hardware#apply-safety) before changing transaction order.
