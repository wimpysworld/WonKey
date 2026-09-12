# WonKey documentation

WonKey plans key and RGB changes offline and applies explicit settings to a descriptor-verified device on Linux.
Start with the [project overview and build instructions](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run every command example from the project root, not from `wiki/`.

| Page | Use it to |
|---|---|
| [Usage and configuration](usage.md) | Plan partial changes, check field limits, and choose an RGB mode. |
| [Hardware operations and safety](hardware.md) | Inspect the mapping, grant temporary access, back up settings, apply changes, and check restore limits. |
| [Protocol and evidence](protocol.md) | Parse replies offline and check encodings, captured identifiers, and source evidence. |

`set` and legacy `apply --write` can change stored settings. `set --dry-run` only reads settings.
Read the hardware safety conditions before any live operation.
Configuration is not firmware flashing. Captures prove neither upload effects nor persistence.

For development and offline checks, read [AGENTS.md](https://github.com/wimpysworld/WonKey/blob/main/AGENTS.md).
