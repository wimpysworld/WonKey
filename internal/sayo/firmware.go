package sayo

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// FirmwareCDNBaseURL is the official SayoDevice firmware update CDN root.
const FirmwareCDNBaseURL = "https://a.sayobot.cn/firmware/update"

// FirmwareManifestCmds holds optional command overrides in a vendor config.json manifest.
type FirmwareManifestCmds struct {
	GetBootVersion byte `json:"get_boot_version"`
	CheckFW        byte `json:"check_fw"`
	WriteData      byte `json:"write_data"`
	ExecuteApp     byte `json:"execute_app"`
	EraseFlash     byte `json:"erase_flash"`
	Verify         byte `json:"verify"`
}

// FirmwareManifest describes a SayoDevice PID's vendor firmware update manifest (config.json).
type FirmwareManifest struct {
	VID             int                     `json:"vid"`
	PID             int                     `json:"pid"`
	PID2            int                     `json:"pid2"`
	AddrLen         int                     `json:"addr_len"`
	BootInfoAddr    int                     `json:"boot_info_addr"`
	AddrByteOrderBE int                     `json:"addr_byte_order_be"`
	EncryptFlag     int                     `json:"encrypt_flag"`
	EraseFlash      int                     `json:"erase_flash"`
	HIDChecksum     *int                    `json:"hid_checksum"`
	HIDErrForSize   int                     `json:"hid_err_for_size"`
	Cmd             FirmwareManifestCmds    `json:"cmd"`
	Data            []FirmwareManifestEntry `json:"data"`
}

// FirmwareManifestEntry describes a single hardware model's firmware image in config.json.
type FirmwareManifestEntry struct {
	Model     string `json:"model"`
	ModelCode uint16 `json:"model_code"`
	ROMSize   int    `json:"rom_size"`
	Version   uint16 `json:"version"`
	FilePath  string `json:"file_path"`
}

// UseChecksum reports whether bootloader HID packets use the 8-bit V1 sum checksum.
// When hid_checksum is omitted in config.json (such as PID 10), it defaults to 1 (true).
func (m FirmwareManifest) UseChecksum() bool {
	if m.HIDChecksum == nil {
		return true
	}
	return *m.HIDChecksum != 0
}

// VerifyCmd returns the bootloader VERIFY command opcode (0x05 when checksums are enabled, else 0x06).
func (m FirmwareManifest) VerifyCmd() byte {
	if m.Cmd.Verify != 0 {
		return m.Cmd.Verify
	}
	if m.UseChecksum() {
		return 0x05
	}
	return 0x06
}

// FindEntry locates the manifest entry matching a device's 16-bit model_code.
func (m FirmwareManifest) FindEntry(modelCode uint16) (FirmwareManifestEntry, bool) {
	for _, entry := range m.Data {
		if entry.ModelCode == modelCode {
			return entry, true
		}
	}
	return FirmwareManifestEntry{}, false
}

// ManifestURL returns the config.json URL for a SayoDevice application or bootloader PID.
func ManifestURL(baseURL string, pid uint16) string {
	return fmt.Sprintf("%s/%d/config.json", strings.TrimRight(baseURL, "/"), ManifestPID(pid))
}

// FirmwareBinaryURL validates filePath and returns the full firmware binary URL for pid.
func FirmwareBinaryURL(baseURL string, pid uint16, filePath string) (string, error) {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") || strings.Contains(trimmed, "..") || strings.Contains(trimmed, "://") || strings.ContainsAny(trimmed, "\\?#") {
		return "", fmt.Errorf("invalid firmware file_path %q in manifest", filePath)
	}
	return fmt.Sprintf("%s/%d/%s", strings.TrimRight(baseURL, "/"), ManifestPID(pid), trimmed), nil
}

// defaultHTTPGet downloads up to maxBytes from rawURL over HTTPS.
func defaultHTTPGet(rawURL string, maxBytes int) (data []byte, err error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return nil, fmt.Errorf("refusing non-HTTPS firmware URL %q", rawURL)
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("refusing non-HTTPS redirect")
			}
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req) // #nosec G107 -- URL scheme validated above and redirects restricted to HTTPS
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("response from %s exceeds %d bytes", rawURL, maxBytes)
	}
	return data, nil
}

// FetchFirmwareManifest downloads and parses the official SayoDevice config.json for pid.
func FetchFirmwareManifest(pid uint16) (FirmwareManifest, error) {
	return FetchFirmwareManifestFrom(FirmwareCDNBaseURL, pid, defaultHTTPGet)
}

// FetchFirmwareManifestFrom downloads and parses config.json using a caller-supplied HTTP getter.
func FetchFirmwareManifestFrom(baseURL string, pid uint16, httpGet func(string, int) ([]byte, error)) (FirmwareManifest, error) {
	url := ManifestURL(baseURL, pid)
	raw, err := httpGet(url, 1<<20)
	if err != nil {
		return FirmwareManifest{}, fmt.Errorf("fetch firmware manifest: %w", err)
	}
	var manifest FirmwareManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return FirmwareManifest{}, fmt.Errorf("decode firmware manifest: %w", err)
	}
	return manifest, nil
}

// FetchFirmwareBinary downloads the official SayoDevice firmware binary for pid and filePath.
func FetchFirmwareBinary(pid uint16, filePath string) ([]byte, error) {
	return FetchFirmwareBinaryFrom(FirmwareCDNBaseURL, pid, filePath, defaultHTTPGet)
}

// FetchFirmwareBinaryFrom downloads a firmware binary using a caller-supplied HTTP getter.
func FetchFirmwareBinaryFrom(baseURL string, pid uint16, filePath string, httpGet func(string, int) ([]byte, error)) ([]byte, error) {
	url, err := FirmwareBinaryURL(baseURL, pid, filePath)
	if err != nil {
		return nil, err
	}
	data, err := httpGet(url, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("fetch firmware binary: %w", err)
	}
	return data, nil
}

// ValidateFirmwareBinary checks that fwBytes has a valid SayoDevice length, fits within entry.ROMSize,
// and satisfies optional manifest command opcode and address-length bounds.
func ValidateFirmwareBinary(fwBytes []byte, entry FirmwareManifestEntry, manifests ...FirmwareManifest) error {
	if len(fwBytes) < 64 || len(fwBytes)%16 != 0 {
		return fmt.Errorf("invalid SayoDevice firmware binary length %d", len(fwBytes))
	}
	if entry.ROMSize > 0 && len(fwBytes) > entry.ROMSize+16 {
		return fmt.Errorf("firmware binary length %d exceeds ROM limit %d", len(fwBytes), entry.ROMSize+16)
	}
	for _, m := range manifests {
		if m.Cmd.GetBootVersion != 0 ||
			(m.Cmd.CheckFW != 0 && m.Cmd.CheckFW != 1) ||
			(m.Cmd.WriteData != 0 && m.Cmd.WriteData != 2) ||
			(m.Cmd.ExecuteApp != 0 && m.Cmd.ExecuteApp != 3) ||
			(m.Cmd.EraseFlash != 0 && m.Cmd.EraseFlash != 4) ||
			(m.Cmd.Verify != 0 && m.Cmd.Verify != 5 && m.Cmd.Verify != 6) {
			return fmt.Errorf("unsupported bootloader command opcodes in manifest: %+v", m.Cmd)
		}
		if m.AddrLen == 2 && len(fwBytes) > 0xFFFF {
			return fmt.Errorf("firmware binary length %d exceeds 16-bit address limit", len(fwBytes))
		}
	}
	return nil
}

// QueryInfoPacketV2 returns the signed 64-byte HID v2 Info (0x00) query packet.
func QueryInfoPacketV2() Packet {
	pkt := Packet{0x21, 0x12, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00}
	SignPacketV2(&pkt)
	return pkt
}

// QueryInfoPacketV1 returns the signed 64-byte HID v1 Info (0x00) query packet.
func QueryInfoPacketV1() Packet {
	pkt := Packet{0x02, 0x00, 0x00}
	SignPacketV1(&pkt)
	return pkt
}

// BootloaderEntryPacketV2 returns the signed 64-byte HID v2 bootloader entry packet (cmd 0x0E).
func BootloaderEntryPacketV2() Packet {
	pkt := Packet{0x21, 0x12, 0x00, 0x00, 0x08, 0x00, 0x0e, 0x00, 0x96, 0x72, 0xff, 0x00}
	SignPacketV2(&pkt)
	return pkt
}

// BootloaderEntryPacketV1 returns the signed 64-byte HID v1 bootloader entry packet (cmd 0xFF).
func BootloaderEntryPacketV1() Packet {
	pkt := Packet{0x02, 0xff, 0x02, 0x72, 0x96}
	SignPacketV1(&pkt)
	return pkt
}

func signBootPacket(pkt *Packet, useChecksum bool) {
	if useChecksum {
		SignPacketV1(pkt)
	}
}

func bootStatusOK(rx Packet, errForSize int, allowOne bool) bool {
	status := rx[1]
	if errForSize != 0 {
		status = rx[2]
	}
	return status == 0x00 || (allowOne && status == 0x01)
}

func putBootUint32(dst []byte, v uint32, be bool) {
	if be {
		binary.BigEndian.PutUint32(dst, v)
	} else {
		binary.LittleEndian.PutUint32(dst, v)
	}
}

// BootGetVersionPacket returns the 64-byte bootloader GET_BOOT_VERSION (0x00) packet.
func BootGetVersionPacket(useChecksum bool) Packet {
	pkt := Packet{0x02, 0x00, 0x00}
	signBootPacket(&pkt, useChecksum)
	return pkt
}

// BootCheckFWPacket returns the 64-byte bootloader CHECK_FW (0x01) packet.
func BootCheckFWPacket(useChecksum bool) Packet {
	pkt := Packet{0x02, 0x01, 0x00}
	signBootPacket(&pkt, useChecksum)
	return pkt
}

// BootEraseFlashPacket returns the 64-byte bootloader ERASE_FLASH (0x04) packet.
func BootEraseFlashPacket(totalBytes uint32, be, useChecksum bool) Packet {
	pkt := Packet{0x02, 0x04, 0x04}
	putBootUint32(pkt[3:7], totalBytes, be)
	signBootPacket(&pkt, useChecksum)
	return pkt
}

// BootWriteDataPacket returns a 64-byte bootloader WRITE_DATA (0x02) packet for one <=32-byte chunk.
func BootWriteDataPacket(addr uint32, addrLen int, be bool, encryptFlag int, chunk []byte, useChecksum bool) Packet {
	var pkt Packet
	pos := 3
	if addrLen == 2 {
		if be {
			binary.BigEndian.PutUint16(pkt[pos:pos+2], uint16(addr&0xffff))
		} else {
			binary.LittleEndian.PutUint16(pkt[pos:pos+2], uint16(addr&0xffff))
		}
		pos += 2
	} else {
		putBootUint32(pkt[pos:pos+4], addr, be)
		pos += 4
	}
	if encryptFlag == 1 {
		pkt[pos] = 0x01
		pos++
	}
	copy(pkt[pos:], chunk)
	pos += len(chunk)
	pkt[0] = 0x02
	pkt[1] = 0x02
	pkt[2] = byte((pos - 3) & 255)
	signBootPacket(&pkt, useChecksum)
	return pkt
}

// BootVerifyPacket returns the 64-byte bootloader VERIFY (0x05 or 0x06) packet.
func BootVerifyPacket(verifyCmd byte, verifyBytes uint32, be, useChecksum bool) Packet {
	pkt := Packet{0x02, verifyCmd, 0x04}
	putBootUint32(pkt[3:7], verifyBytes, be)
	signBootPacket(&pkt, useChecksum)
	return pkt
}

// BootExecuteAppPacket returns the 64-byte bootloader EXECUTE_APP (0x03) packet.
func BootExecuteAppPacket(useChecksum bool) Packet {
	pkt := Packet{0x02, 0x03, 0x00}
	signBootPacket(&pkt, useChecksum)
	return pkt
}

// FirmwareProgressFunc reports cumulative firmware bytes written and total firmware size.
type FirmwareProgressFunc func(written, total int)

// FlashFirmware enters bootloader mode (when in app mode), waits for USB re-enumeration,
// and executes the bootloader erase, write, verify, and execute-app sequence.
func FlashFirmware(sysRoot, devRoot string, candidate DiscoveredCandidate, proto ProtocolVersion, inBootloader bool, manifest FirmwareManifest, entry FirmwareManifestEntry, fwBytes []byte, onProgress FirmwareProgressFunc) (ApplyResult, error) {
	return systemProbeOps().flashFirmware(sysRoot, devRoot, candidate, proto, inBootloader, manifest, entry, fwBytes, onProgress)
}

func (ops probeOps) flashFirmware(sysRoot, devRoot string, candidate DiscoveredCandidate, proto ProtocolVersion, inBootloader bool, manifest FirmwareManifest, entry FirmwareManifestEntry, fwBytes []byte, onProgress FirmwareProgressFunc) (result ApplyResult, err error) {
	if !filepath.IsAbs(devRoot) {
		return result, fmt.Errorf("USB device root must be absolute")
	}
	if err := ValidateFirmwareBinary(fwBytes, entry, manifest); err != nil {
		return result, err
	}
	if err := ops.revalidate(sysRoot, candidate); err != nil {
		return result, err
	}
	useChecksum := manifest.UseChecksum()
	var bootFD int
	if !inBootloader {
		if err := ops.enterBootloader(sysRoot, devRoot, candidate, proto, &result); err != nil {
			return result, err
		}
		bootFD, err = ops.waitForBootloaderHidraw(sysRoot, devRoot, candidate, manifest, &result)
		if err != nil {
			return result, err
		}
	} else {
		bootFD, err = ops.openBootloaderHidraw(sysRoot, devRoot, candidate)
		if err != nil {
			return result, err
		}
	}
	defer func() {
		if closeErr := ops.close(bootFD); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	result.WriteAttempted = true
	if inBootloader {
		if err := ops.bootStepGetVersion(bootFD, useChecksum, &result); err != nil {
			return result, err
		}
	}
	return ops.flashBootloader(bootFD, manifest, entry, fwBytes, result, onProgress)
}

func (ops probeOps) openValidatedHidraw(sysRoot, devRoot string, candidate DiscoveredCandidate) (int, ProtocolVersion, error) {
	hidrawName, _, proto, wantMajor, wantMinor, err := ops.locateSayoHidraw(candidate)
	if err != nil {
		return -1, 0, err
	}
	node := filepath.Join(hidrawRoot(devRoot), hidrawName)
	var named unix.Stat_t
	if err := ops.lstat(node, &named); err != nil {
		return -1, 0, err
	}
	if err := checkHidrawNode(named, wantMajor, wantMinor); err != nil {
		return -1, 0, err
	}
	fd, err := ops.open(node, unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, 0, err
	}
	if err := ops.validateHidrawOpen(sysRoot, node, fd, candidate, named, wantMajor, wantMinor); err != nil {
		_ = ops.close(fd)
		return -1, 0, err
	}
	return fd, proto, nil
}

func (ops probeOps) openBootloaderHidraw(sysRoot, devRoot string, candidate DiscoveredCandidate) (int, error) {
	fd, proto, err := ops.openValidatedHidraw(sysRoot, devRoot, candidate)
	if err != nil {
		return -1, err
	}
	if proto != ProtocolV1 {
		_ = ops.close(fd)
		return -1, fmt.Errorf("interface %d is still using HID v2 application report descriptor", candidate.VendorInterface)
	}
	return fd, nil
}

func (ops probeOps) enterBootloader(sysRoot, devRoot string, candidate DiscoveredCandidate, proto ProtocolVersion, result *ApplyResult) (err error) {
	fd, _, err := ops.openValidatedHidraw(sysRoot, devRoot, candidate)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := ops.close(fd); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	result.WriteAttempted = true
	if proto == ProtocolV2 {
		pkt := BootloaderEntryPacketV2()
		if n, wErr := ops.writeHidraw(fd, pkt[:]); wErr != nil || n != 64 {
			if wErr != nil {
				return fmt.Errorf("write v2 bootloader entry packet: %w", wErr)
			}
			return fmt.Errorf("write v2 bootloader entry packet wrote %d bytes, want 64", n)
		}
		result.ReportsWritten++
		return nil
	}
	pkt := BootloaderEntryPacketV1()
	for i := range 2 {
		if n, wErr := ops.writeHidraw(fd, pkt[:]); wErr != nil || n != 64 {
			if wErr != nil {
				return fmt.Errorf("write v1 bootloader entry packet %d: %w", i+1, wErr)
			}
			return fmt.Errorf("write v1 bootloader entry packet %d wrote %d bytes, want 64", i+1, n)
		}
		result.ReportsWritten++
	}
	return nil
}

func (ops probeOps) waitForBootloaderHidraw(sysRoot, devRoot string, prev DiscoveredCandidate, m FirmwareManifest, result *ApplyResult) (int, error) {
	useChecksum := m.UseChecksum()
	attempts := ops.bootAttempts
	if attempts <= 0 {
		attempts = 50
	}
	sleepFn := ops.sleep
	if sleepFn == nil {
		sleepFn = time.Sleep
	}
	for range attempts {
		sleepFn(100 * time.Millisecond)
		candidates, err := ops.discover(sysRoot)
		if err != nil {
			continue
		}
		for _, c := range candidates {
			if c.PhysicalPath != prev.PhysicalPath {
				continue
			}
			if m.PID2 > 0 && int(c.PID) != m.PID2 {
				continue
			}
			fd, err := ops.openBootloaderHidraw(sysRoot, devRoot, c)
			if err != nil {
				continue
			}
			if err := ops.bootStepGetVersion(fd, useChecksum, result); err != nil {
				_ = ops.close(fd)
				continue
			}
			return fd, nil
		}
	}
	return -1, fmt.Errorf("timed out waiting for SayoDevice bootloader USB re-enumeration")
}

func (ops probeOps) flashBootloader(fd int, m FirmwareManifest, entry FirmwareManifestEntry, fwBytes []byte, result ApplyResult, onProgress FirmwareProgressFunc) (ApplyResult, error) {
	useChecksum := m.UseChecksum()
	be := m.AddrByteOrderBE != 0
	if err := ops.bootStepCheckFW(fd, m.HIDErrForSize, entry.ModelCode, useChecksum, &result); err != nil {
		return result, err
	}
	if m.EraseFlash != 0 {
		if err := ops.bootStepEraseFlash(fd, uint32(len(fwBytes)), be, m.HIDErrForSize, useChecksum, &result); err != nil { // #nosec G115 -- validated <= 4 MiB
			return result, err
		}
	}
	if err := ops.bootStepWriteData(fd, m, fwBytes, useChecksum, &result, onProgress); err != nil {
		return result, err
	}
	if err := ops.bootStepVerify(fd, m, uint32(len(fwBytes)-16), be, useChecksum, &result); err != nil { // #nosec G115 -- validated >= 64 and <= 4 MiB
		return result, err
	}
	execPkt := BootExecuteAppPacket(useChecksum)
	if n, err := ops.writeHidraw(fd, execPkt[:]); err != nil || n != 64 {
		if err != nil {
			return result, fmt.Errorf("write EXECUTE_APP: %w", err)
		}
		return result, fmt.Errorf("write EXECUTE_APP wrote %d bytes, want 64", n)
	}
	result.ReportsWritten++
	return result, nil
}

func (ops probeOps) bootStepGetVersion(fd int, useChecksum bool, result *ApplyResult) error {
	pkt := BootGetVersionPacket(useChecksum)
	rx, err := ops.exchangeBootPacket(fd, pkt, 1000, "GET_BOOT_VERSION", result)
	if err != nil {
		return err
	}
	if rx[0] != 0x02 || rx[1] != 0x02 {
		return fmt.Errorf("device is not in bootloader mode (GET_BOOT_VERSION reply %02x %02x)", rx[0], rx[1])
	}
	return nil
}

func (ops probeOps) bootStepCheckFW(fd, errForSize int, wantModelCode uint16, useChecksum bool, result *ApplyResult) error {
	pkt := BootCheckFWPacket(useChecksum)
	rx, err := ops.exchangeBootPacket(fd, pkt, 1000, "CHECK_FW", result)
	if err != nil {
		return err
	}
	if !bootStatusOK(rx, errForSize, true) {
		return fmt.Errorf("bootloader CHECK_FW reported error status")
	}
	gotModel := binary.BigEndian.Uint16(rx[5:7])
	if gotModel != wantModelCode {
		return fmt.Errorf("bootloader model_code 0x%04x does not match expected 0x%04x", gotModel, wantModelCode)
	}
	return nil
}

func (ops probeOps) bootStepEraseFlash(fd int, totalBytes uint32, be bool, errForSize int, useChecksum bool, result *ApplyResult) error {
	pkt := BootEraseFlashPacket(totalBytes, be, useChecksum)
	rx, err := ops.exchangeBootPacket(fd, pkt, 5000, "ERASE_FLASH", result)
	if err != nil {
		return err
	}
	if !bootStatusOK(rx, errForSize, false) {
		return fmt.Errorf("bootloader ERASE_FLASH failed")
	}
	return nil
}

func (ops probeOps) bootStepWriteData(fd int, m FirmwareManifest, fwBytes []byte, useChecksum bool, result *ApplyResult, onProgress FirmwareProgressFunc) error {
	be := m.AddrByteOrderBE != 0
	if onProgress != nil {
		onProgress(0, len(fwBytes))
	}
	for offset := 0; offset < len(fwBytes); {
		chunk := fwBytes[offset:min(offset+32, len(fwBytes))]
		pkt := BootWriteDataPacket(uint32(offset), m.AddrLen, be, m.EncryptFlag, chunk, useChecksum) // #nosec G115 -- validated <= 4 MiB
		rx, err := ops.exchangeBootPacket(fd, pkt, 1000, fmt.Sprintf("WRITE_DATA at 0x%x", offset), result)
		if err != nil {
			return err
		}
		if !bootStatusOK(rx, m.HIDErrForSize, false) {
			return fmt.Errorf("bootloader WRITE_DATA failed at offset 0x%x", offset)
		}
		offset += len(chunk)
		if onProgress != nil {
			onProgress(offset, len(fwBytes))
		}
	}
	return nil
}

func (ops probeOps) bootStepVerify(fd int, m FirmwareManifest, verifyBytes uint32, be, useChecksum bool, result *ApplyResult) error {
	pkt := BootVerifyPacket(m.VerifyCmd(), verifyBytes, be, useChecksum)
	rx, err := ops.exchangeBootPacket(fd, pkt, 2000, "VERIFY", result)
	if err != nil {
		return err
	}
	if !bootStatusOK(rx, m.HIDErrForSize, true) {
		return fmt.Errorf("bootloader VERIFY failed")
	}
	return nil
}

func (ops probeOps) exchangeBootPacket(fd int, pkt Packet, timeoutMs int, stepName string, result *ApplyResult) (Packet, error) {
	if n, err := ops.writeHidraw(fd, pkt[:]); err != nil || n != 64 {
		if err != nil {
			return Packet{}, fmt.Errorf("write %s: %w", stepName, err)
		}
		return Packet{}, fmt.Errorf("write %s wrote %d bytes, want 64", stepName, n)
	}
	result.ReportsWritten++
	var rx Packet
	n, err := ops.readHidraw(fd, rx[:], timeoutMs)
	if err != nil {
		return Packet{}, fmt.Errorf("read %s reply: %w", stepName, err)
	}
	if n != 64 || rx[0] != 0x02 {
		return Packet{}, fmt.Errorf("invalid %s reply", stepName)
	}
	return rx, nil
}
