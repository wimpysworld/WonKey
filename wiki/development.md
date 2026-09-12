# Development

[Wiki home](Home) · [Protocol and evidence](protocol)

WonKey has one executable with three commands: `key`, `rgb`, and `restore`.
`cmd/wonkey` handles process entry and exit. CLI, protocol, Linux device access, and capture storage stay in `internal/xfkey`.

## Offline checks

Run the [build and test checks](https://github.com/wimpysworld/WonKey/blob/main/AGENTS.md#build-and-test) from the project root.
They use synthetic transports and temporary captures, without connected hardware.

The [protocol reference](protocol) records framing, configuration fields, and pinned source evidence.
The developer executable, legacy command interfaces, and capture/event helper scripts are removed.

## Evidence fixtures

Keep `internal/xfkey/testdata/hardware-20260911` byte-for-byte unchanged, including warnings, paths, and genuine hardware identifiers.
Copy fixtures to temporary directories before corruption tests.

The authentic fixture confirms query framing. It lacks an authentic `result.json` completion record and is not a restore source.
Do not fabricate completion records or modify evidence to match changed output.
Query captures do not prove upload effects, restore behaviour, or persistence after reconnect.

Use synthetic complete captures to test restore, including failed transactions and different unknown configuration bytes.
See [hardware safety and records](hardware#apply-safety-and-records) before changing transaction order or capture completion.
