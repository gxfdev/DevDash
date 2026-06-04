//go:build windows

package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type conPtyProcess struct {
	hpc    windows.Handle
	pi     *windows.ProcessInformation
	ptyIn  windows.Handle // write end - we write to this to send input
	ptyOut windows.Handle // read end - we read from this to get output
	pid    int
	mu     sync.Mutex
	closed bool
}

var (
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCreatePseudoConsole = modKernel32.NewProc("CreatePseudoConsole")
	procResizePseudoConsole = modKernel32.NewProc("ResizePseudoConsole")
	procClosePseudoConsole  = modKernel32.NewProc("ClosePseudoConsole")
)

type coord struct {
	X, Y int16
}

func isConPtyAvailable() bool {
	return procCreatePseudoConsole.Find() == nil &&
		procResizePseudoConsole.Find() == nil &&
		procClosePseudoConsole.Find() == nil
}

func createConPtyProcess(shell string, cols, rows int16) (*conPtyProcess, error) {
	if !isConPtyAvailable() {
		return nil, fmt.Errorf("ConPTY API not available on this Windows version")
	}

	// Lock OS thread - all Windows handle operations must be on the same thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Create two pipes: one for input, one for output
	var hConPtyInRead, hConPtyInWrite windows.Handle
	if err := windows.CreatePipe(&hConPtyInRead, &hConPtyInWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("CreatePipe(input): %w", err)
	}

	var hConPtyOutRead, hConPtyOutWrite windows.Handle
	if err := windows.CreatePipe(&hConPtyOutRead, &hConPtyOutWrite, nil, 0); err != nil {
		windows.CloseHandle(hConPtyInRead)
		windows.CloseHandle(hConPtyInWrite)
		return nil, fmt.Errorf("CreatePipe(output): %w", err)
	}

	// Create the pseudo console
	size := coord{X: cols, Y: rows}
	hpc, err := createPseudoConsole(size, hConPtyInRead, hConPtyOutWrite)
	if err != nil {
		windows.CloseHandle(hConPtyInRead)
		windows.CloseHandle(hConPtyInWrite)
		windows.CloseHandle(hConPtyOutRead)
		windows.CloseHandle(hConPtyOutWrite)
		return nil, fmt.Errorf("CreatePseudoConsole: %w", err)
	}

	// After CreatePseudoConsole, the read end of input pipe and write end of output pipe
	// are now owned by the pseudo console. They will be closed by ClosePseudoConsole.

	// Prepare startup info for the child process
	cmdArgs := buildWindowsCmdArgs(shell)
	pi, err := startProcessWithPty(hpc, cmdArgs)
	if err != nil {
		closePseudoConsole(hpc)
		windows.CloseHandle(hConPtyInWrite)
		windows.CloseHandle(hConPtyOutRead)
		return nil, fmt.Errorf("CreateProcess: %w", err)
	}

	return &conPtyProcess{
		hpc:    hpc,
		pi:     pi,
		ptyIn:  hConPtyInWrite,
		ptyOut: hConPtyOutRead,
		pid:    int(pi.ProcessId),
	}, nil
}

func createPseudoConsole(size coord, hIn, hOut windows.Handle) (windows.Handle, error) {
	var hpc windows.Handle
	packedSize := uintptr((int32(size.Y) << 16) | int32(size.X))

	ret, _, _ := procCreatePseudoConsole.Call(
		packedSize,
		uintptr(hIn),
		uintptr(hOut),
		0,
		uintptr(unsafe.Pointer(&hpc)),
	)
	if ret != 0 { // S_OK is 0
		return 0, fmt.Errorf("failed with 0x%x", ret)
	}
	return hpc, nil
}

func closePseudoConsole(hpc windows.Handle) {
	procClosePseudoConsole.Call(uintptr(hpc))
}

func resizePseudoConsole(hpc windows.Handle, size coord) error {
	packedSize := uintptr((int32(size.Y) << 16) | int32(size.X))
	ret, _, _ := procResizePseudoConsole.Call(uintptr(hpc), packedSize)
	if ret != 0 {
		return fmt.Errorf("ResizePseudoConsole failed with 0x%x", ret)
	}
	return nil
}

const _PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = uintptr(0x20016)

func startProcessWithPty(hpc windows.Handle, cmdArgs []string) (*windows.ProcessInformation, error) {
	// Use the x/sys/windows API for ProcThreadAttributeList
	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, fmt.Errorf("NewProcThreadAttributeList: %w", err)
	}
	defer attrList.Delete()

	// Add the pseudo console attribute
	if err := attrList.Update(
		_PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		unsafe.Pointer(&hpc),
		unsafe.Sizeof(hpc),
	); err != nil {
		return nil, fmt.Errorf("UpdateProcThreadAttribute: %w", err)
	}

	// Build command line
	argv0, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		argv0 = cmdArgs[0]
	}
	cmdLine := makeCmdLine(cmdArgs)

	argv0Ptr, _ := windows.UTF16PtrFromString(argv0)
	cmdLinePtr, _ := windows.UTF16PtrFromString(cmdLine)

	var si windows.StartupInfoEx
	si.ProcThreadAttributeList = attrList.List()
	si.StartupInfo.Cb = uint32(unsafe.Sizeof(si))

	var pi windows.ProcessInformation

	err = windows.CreateProcess(
		argv0Ptr,
		cmdLinePtr,
		nil, nil,
		false,
		windows.EXTENDED_STARTUPINFO_PRESENT,
		nil, nil,
		&si.StartupInfo,
		&pi,
	)
	if err != nil {
		return nil, err
	}
	return &pi, nil
}

func makeCmdLine(args []string) string {
	var s string
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += escapeArg(a)
	}
	return s
}

func escapeArg(s string) string {
	if s == "" {
		return `""`
	}
	needsQuote := false
	for _, c := range s {
		if c == ' ' || c == '\t' {
			needsQuote = true
			break
		}
	}
	if !needsQuote && !strings.Contains(s, `"`) && !strings.Contains(s, `\`) {
		return s
	}
	return `"` + s + `"`
}

func (p *conPtyProcess) Read(buf []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(p.ptyOut, buf, &n, nil)
	return int(n), err
}

func (p *conPtyProcess) Write(data []byte) (int, error) {
	var n uint32
	err := windows.WriteFile(p.ptyIn, data, &n, nil)
	return int(n), err
}

func (p *conPtyProcess) Resize(cols, rows int16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	return resizePseudoConsole(p.hpc, coord{X: cols, Y: rows})
}

func (p *conPtyProcess) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true

	closePseudoConsole(p.hpc)
	windows.CloseHandle(p.ptyIn)
	windows.CloseHandle(p.ptyOut)
	if p.pi != nil {
		windows.TerminateProcess(p.pi.Process, 1)
		windows.CloseHandle(p.pi.Process)
		windows.CloseHandle(p.pi.Thread)
	}
}

func (p *conPtyProcess) Wait(ctx context.Context) error {
	if p.pi == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := windows.WaitForSingleObject(p.pi.Process, windows.INFINITE)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *conPtyProcess) Pid() int {
	return p.pid
}

func (p *conPtyProcess) IsConPty() bool {
	return true
}
