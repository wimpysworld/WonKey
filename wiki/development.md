# Development

[Wiki home](Home) · [Protocol and evidence](protocol)

WonKey is a Linux CLI for configuring supported USB macropads (`XFKEY One Key Max (0112)`, `CH57x Macropad (1189:8890, 514c:8851, 514c:8850)`, and `SayoDevice 2x3P (8089:000c)`).
It has one executable: bare `wonkey` inspects connected macropads, and `key`, `trigger`, `macro`, `mouse`, `media`, `rgb`, and `led` validate device capabilities and configure supported devices.
`cmd/wonkey` handles process entry and exit. CLI, protocol, and unified Linux device discovery across `internal/xfkey` (`0112`) and `internal/ch57` (`1189:8890`, `514c:8851`, `514c:8850`, and `8089:000c`) stay in `internal/`.

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
