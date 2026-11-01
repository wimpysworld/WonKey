package trisat

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// These bytes reconstruct the complete 36-byte item listing published at
// https://github.com/the-via/keyboards/issues/2169. They are not a local capture.
// Report ID 3 has 64 eight-bit Input fields and 64 eight-bit Output fields.
const candidateReportDescriptor = "\x06\x00\xff\x09\x01\xa1\x01\x85\x03\x09\x02\x15\x00\x26\x00\xff\x75\x08\x95\x40\x81\x06\x09\x02\x15\x00\x26\x00\xff\x75\x08\x95\x40\x91\x06\xc0"

// DiscoveredCandidate binds a descriptor match to one sysfs device instance.
// Obtain this value through DiscoverCandidates. Do not persist it across runs.
type DiscoveredCandidate struct {
	Candidate
	SysfsPath    string
	BusNumber    int
	DeviceNumber int
	identity     [2]uint64
}

// ProbeStatus distinguishes a raw report from a refusal or failed transfer.
type ProbeStatus string

const (
	ProbeRefused          ProbeStatus = "refused"
	ProbeDescriptorFailed ProbeStatus = "report-descriptor-failed"
	ProbeTransferFailed   ProbeStatus = "get-report-failed"
	ProbeInvalidReport    ProbeStatus = "invalid-report"
	ProbeRawReport        ProbeStatus = "raw-report-only"
	ProbeCloseFailed      ProbeStatus = "close-failed"
)

// ProbeResult preserves raw GET_REPORT bytes, including an invalid short reply.
// A valid report is not proof of settings readback or firmware identity.
type ProbeResult struct {
	Status                   ProbeStatus
	Raw                      []byte
	Count                    int
	ReportDescriptor         []byte
	ReportDescriptorCount    int
	ReportDescriptorVerified bool
	Warning                  string
}

// usbControlTransfer follows struct usbdevfs_ctrltransfer in Linux UAPI.
// Go supplies the native pointer alignment, including padding after Timeout.
type usbControlTransfer struct {
	RequestType uint8
	Request     uint8
	Value       uint16
	Index       uint16
	Length      uint16
	Timeout     uint32
	Data        unsafe.Pointer
}

// _IOWR('U', 0, sizeof(struct usbdevfs_ctrltransfer)). The combined direction
// bits are 0xc0000000 on Linux, including MIPS and PowerPC ioctl encodings.
const usbdevfsControl = uintptr(0xc0005500) | unsafe.Sizeof(usbControlTransfer{})<<16

type probeOps struct {
	readDir      func(string) ([]os.DirEntry, error)
	readFile     func(string, int) ([]byte, error)
	evalSymlinks func(string) (string, error)
	stat         func(string) (os.FileInfo, error)
	lstat        func(string, *unix.Stat_t) error
	open         func(string, int, uint32) (int, error)
	fstat        func(int, *unix.Stat_t) error
	close        func(int) error
	ioctl        func(int, uintptr, *usbControlTransfer) (int, error)
}

func systemProbeOps() probeOps {
	return probeOps{
		readDir: os.ReadDir, readFile: readProbeFile, evalSymlinks: filepath.EvalSymlinks,
		stat: os.Stat, lstat: unix.Lstat, open: unix.Open, fstat: unix.Fstat,
		close: unix.Close, ioctl: controlIN,
	}
}

// DiscoverCandidates reads cached descriptors below sysRoot, normally
// /sys/bus/usb/devices. It does not open USB device nodes or send requests.
// Callers must obtain user authority before inspecting real sysfs devices.
func DiscoverCandidates(sysRoot string) ([]DiscoveredCandidate, error) {
	return systemProbeOps().discover(sysRoot)
}

func (ops probeOps) discover(sysRoot string) ([]DiscoveredCandidate, error) {
	if !filepath.IsAbs(sysRoot) {
		return nil, fmt.Errorf("sysfs root must be absolute")
	}
	entries, err := ops.readDir(sysRoot)
	if err != nil {
		return nil, err
	}
	var candidates []DiscoveredCandidate
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 255 || !physicalUSBPath.MatchString(name) {
			continue
		}
		candidate, err := ops.inspect(sysRoot, name)
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (ops probeOps) inspect(sysRoot, physicalPath string) (DiscoveredCandidate, error) {
	var result DiscoveredCandidate
	if !filepath.IsAbs(sysRoot) || len(physicalPath) > 255 || !physicalUSBPath.MatchString(physicalPath) {
		return result, fmt.Errorf("invalid sysfs root or physical path")
	}
	path, err := ops.evalSymlinks(filepath.Join(sysRoot, physicalPath))
	if err != nil {
		return result, err
	}
	if filepath.Base(path) != physicalPath {
		return result, fmt.Errorf("sysfs path does not match the selected physical path")
	}
	before, err := ops.stat(path)
	if err != nil {
		return result, err
	}
	st, ok := before.Sys().(*syscall.Stat_t)
	if !ok || !before.IsDir() {
		return result, fmt.Errorf("sysfs device is not a directory with Linux identity")
	}
	descriptors, err := ops.readFile(filepath.Join(path, "descriptors"), len(candidateUSBDescriptor))
	if err != nil {
		return result, err
	}
	candidate, err := MatchCandidate(physicalPath, descriptors)
	if err != nil {
		return result, err
	}
	bus, err := ops.number(path, "busnum", 65535)
	if err != nil {
		return result, err
	}
	device, err := ops.number(path, "devnum", 127)
	if err != nil {
		return result, err
	}
	if !strings.HasPrefix(physicalPath, strconv.Itoa(bus)+"-") {
		return result, fmt.Errorf("bus number does not match physical path")
	}
	dev, err := ops.readFile(filepath.Join(path, "dev"), 32)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(string(dev)) != fmt.Sprintf("189:%d", (bus-1)*128+device-1) {
		return result, fmt.Errorf("sysfs device number does not match USB bus and address")
	}
	after, err := ops.stat(path)
	if err != nil {
		return result, err
	}
	if !os.SameFile(before, after) {
		return result, fmt.Errorf("sysfs device changed during inspection")
	}
	return DiscoveredCandidate{
		Candidate: candidate, SysfsPath: path, BusNumber: bus, DeviceNumber: device,
		identity: [2]uint64{st.Dev, st.Ino},
	}, nil
}

func (ops probeOps) number(path, name string, maximum int) (int, error) {
	data, err := ops.readFile(filepath.Join(path, name), 16)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSuffix(string(data), "\n")
	if value == "" || strings.Trim(value, "0123456789") != "" {
		return 0, fmt.Errorf("invalid %s", name)
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > maximum {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return n, nil
}

func readProbeFile(path string, limit int) (data []byte, err error) {
	file, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("sysfs attribute is not a regular file")
	}
	data, err = io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("sysfs attribute exceeds %d bytes", limit)
	}
	return data, nil
}

// ProbeCandidate opens one revalidated node below devRoot, normally /dev/bus/usb.
// It sends only GET_DESCRIPTOR and Input GET_REPORT, once each, with a 1000 ms
// kernel timeout per request. Linux usbfs requires O_RDWR even for control IN.
// Callers must obtain user authority before calling this function on hardware.
// A bound interface can cause EBUSY. No driver is detached to bypass that error.
// It does not reset devices, retry, or decode settings.
func ProbeCandidate(sysRoot, devRoot string, candidate DiscoveredCandidate) (ProbeResult, error) {
	return systemProbeOps().probe(sysRoot, devRoot, candidate)
}

// ProbeOutputCandidate uses the same checks and bounds as ProbeCandidate, but
// requests Output report ID 3 instead of Input. GET_REPORT remains control IN.
// Callers must obtain user authority before calling this function on hardware.
func ProbeOutputCandidate(sysRoot, devRoot string, candidate DiscoveredCandidate) (ProbeResult, error) {
	return systemProbeOps().probeReport(sysRoot, devRoot, candidate, true)
}

func (ops probeOps) revalidate(sysRoot string, candidate DiscoveredCandidate) error {
	fresh, err := ops.inspect(sysRoot, candidate.PhysicalPath)
	if err != nil {
		return err
	}
	if fresh != candidate {
		return fmt.Errorf("selected descriptors, physical path or sysfs identity changed")
	}
	return nil
}

func (ops probeOps) probe(sysRoot, devRoot string, candidate DiscoveredCandidate) (ProbeResult, error) {
	return ops.probeReport(sysRoot, devRoot, candidate, false)
}

func (ops probeOps) probeReport(sysRoot, devRoot string, candidate DiscoveredCandidate, output bool) (result ProbeResult, err error) {
	report, value := "input", uint16(0x0103)
	if output {
		report, value = "output", 0x0203
	}
	result.Status = ProbeRefused
	result.Warning = "No stock 8890 settings readback is known. Raw " + report + " is not decoded settings or firmware identification."
	if !filepath.IsAbs(devRoot) {
		return result, fmt.Errorf("USB device root must be absolute")
	}
	if err := ops.revalidate(sysRoot, candidate); err != nil {
		return result, err
	}
	node := filepath.Join(devRoot, fmt.Sprintf("%03d", candidate.BusNumber), fmt.Sprintf("%03d", candidate.DeviceNumber))
	var named unix.Stat_t
	if err := ops.lstat(node, &named); err != nil {
		return result, err
	}
	if err := checkUSBNode(named, candidate); err != nil {
		return result, err
	}
	fd, err := ops.open(node, unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return result, err
	}
	defer func() {
		if closeErr := ops.close(fd); closeErr != nil {
			err = errors.Join(err, closeErr)
			if result.Status == ProbeRawReport {
				result.Status = ProbeCloseFailed
			}
		}
	}()
	if err := ops.validateOpen(sysRoot, node, fd, candidate, named); err != nil {
		return result, err
	}
	result.Status = ProbeDescriptorFailed
	descriptor := make([]byte, len(candidateReportDescriptor))
	count, err := ops.readControl(fd, 0x81, 0x06, 0x2200, descriptor)
	result.ReportDescriptorCount = count
	if count >= 0 && count <= len(descriptor) {
		result.ReportDescriptor = descriptor[:count]
	}
	if err != nil {
		return result, fmt.Errorf("GET_DESCRIPTOR: %w", err)
	}
	if count != len(descriptor) || string(descriptor) != candidateReportDescriptor {
		return result, fmt.Errorf("interface 1 report descriptor differs from published report ID 3 layout")
	}
	result.ReportDescriptorVerified = true
	result.Status = ProbeRefused
	if err := ops.validateOpen(sysRoot, node, fd, candidate, named); err != nil {
		return result, err
	}
	result.Status = ProbeTransferFailed
	data := make([]byte, 65)
	count, err = ops.readControl(fd, 0xa1, 0x01, value, data)
	result.Count = count
	if count >= 0 && count <= len(data) {
		result.Raw = data[:count]
	}
	if err != nil {
		return result, fmt.Errorf("%s GET_REPORT: %w", report, err)
	}
	result.Status = ProbeInvalidReport
	if count != 65 {
		return result, fmt.Errorf("%s GET_REPORT returned %d bytes, want 65 including report ID", report, count)
	}
	if data[0] != 3 {
		return result, fmt.Errorf("%s GET_REPORT returned report ID %d, want 3", report, data[0])
	}
	result.Status = ProbeRawReport
	return result, nil
}

func checkUSBNode(st unix.Stat_t, candidate DiscoveredCandidate) error {
	minor := (candidate.BusNumber-1)*128 + candidate.DeviceNumber - 1
	if minor < 0 || minor > 0xfffff {
		return fmt.Errorf("USB device number is out of range")
	}
	if st.Mode&unix.S_IFMT != unix.S_IFCHR || unix.Major(st.Rdev) != 189 || unix.Minor(st.Rdev) != uint32(minor) {
		return fmt.Errorf("selected node is not the expected USB character device")
	}
	return nil
}

func (ops probeOps) validateOpen(sysRoot, node string, fd int, candidate DiscoveredCandidate, before unix.Stat_t) error {
	if err := ops.revalidate(sysRoot, candidate); err != nil {
		return err
	}
	var opened, named unix.Stat_t
	if err := ops.fstat(fd, &opened); err != nil {
		return err
	}
	if err := ops.lstat(node, &named); err != nil {
		return err
	}
	if err := checkUSBNode(opened, candidate); err != nil {
		return err
	}
	if named.Mode&unix.S_IFMT != unix.S_IFCHR || opened.Dev != before.Dev || opened.Ino != before.Ino ||
		opened.Rdev != before.Rdev || opened.Dev != named.Dev || opened.Ino != named.Ino || opened.Rdev != named.Rdev {
		return fmt.Errorf("opened USB node identity changed")
	}
	return nil
}

func (ops probeOps) readControl(fd int, requestType, request uint8, value uint16, data []byte) (int, error) {
	length := len(data)
	if length < 1 || length > 65 {
		return 0, fmt.Errorf("invalid control IN buffer size")
	}
	transfer := usbControlTransfer{
		RequestType: requestType, Request: request, Value: value, Index: 1,
		Length: uint16(length), Timeout: 1000, Data: unsafe.Pointer(&data[0]),
	}
	count, err := ops.ioctl(fd, usbdevfsControl, &transfer)
	runtime.KeepAlive(data)
	return count, err
}

func controlIN(fd int, request uintptr, transfer *usbControlTransfer) (int, error) {
	if transfer == nil {
		return 0, fmt.Errorf("missing control IN transfer")
	}
	isDescriptor := transfer.RequestType == 0x81 && transfer.Request == 6 && transfer.Value == 0x2200 && transfer.Length == 36
	isReport := transfer.RequestType == 0xa1 && transfer.Request == 1 && (transfer.Value == 0x0103 || transfer.Value == 0x0203) && transfer.Length == 65
	if request != usbdevfsControl || transfer.Index != 1 || transfer.Timeout != 1000 || transfer.Data == nil || (!isDescriptor && !isReport) {
		return 0, fmt.Errorf("refused control request outside the read-only probe")
	}
	n, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, uintptr(unsafe.Pointer(transfer)))
	runtime.KeepAlive(transfer)
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}
