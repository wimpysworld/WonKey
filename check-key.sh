#!/bin/sh
set -eu
exec python3 - <<'PY'
import os
import pathlib
import select
import struct

interface = pathlib.Path('/sys/bus/usb/devices/1-1.2:1.0').resolve()
usb = interface.parent
if not all((usb / name).exists() for name in ('idVendor', 'idProduct')):
    raise SystemExit('USB key not found at 1-1.2. No events read.')
if ((usb / 'idVendor').read_text().strip(),
        (usb / 'idProduct').read_text().strip()) != ('af88', '6688'):
    raise SystemExit('Target identity changed. No events read.')
events = [p for p in interface.glob('*/input/input*/event*') if p.is_dir()]
if len(events) != 1:
    raise SystemExit('Expected one keyboard event node. No events read.')
node = '/dev/input/' + events[0].name
record = struct.Struct('@llHHi')
fd = os.open(node, os.O_RDONLY | os.O_NONBLOCK)
print('Listening only to ' + node + ' for the XFKEY keyboard.', flush=True)
print('Press and release the USB key. Ctrl-C stops the check.', flush=True)
try:
    finished = False
    while not finished:
        select.select([fd], [], [])
        data = os.read(fd, record.size * 32)
        if not data:
            raise SystemExit('The device disconnected.')
        if len(data) % record.size:
            raise SystemExit('Incomplete input event received.')
        for offset in range(0, len(data), record.size):
            _, _, kind, code, value = record.unpack_from(data, offset)
            if kind != 1:
                continue
            name = {183: 'KEY_F13', 28: 'KEY_ENTER'}.get(code, 'code=' + str(code))
            action = {0: 'release', 1: 'press', 2: 'repeat'}.get(value, str(value))
            print(name + ' ' + action, flush=True)
            if value == 0:
                finished = True
except KeyboardInterrupt:
    print('\nCheck stopped.')
finally:
    os.close(fd)
PY
