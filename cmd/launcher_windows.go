//go:build windows

package cmd

import (
	"errors"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func launchedFromExplorer() bool {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()

	processNames := map[uint32]string{}
	parentProcessIDs := map[uint32]uint32{}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return false
	}
	for {
		processNames[entry.ProcessID] = windows.UTF16ToString(entry.ExeFile[:])
		parentProcessIDs[entry.ProcessID] = entry.ParentProcessID
		err = windows.Process32Next(snapshot, &entry)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			break
		}
		if err != nil {
			return false
		}
	}

	return hasExplorerParent(uint32(os.Getpid()), parentProcessIDs, processNames)
}

func hasExplorerParent(processID uint32, parentProcessIDs map[uint32]uint32, processNames map[uint32]string) bool {
	parentProcessID := parentProcessIDs[processID]
	return parentProcessID != 0 && strings.EqualFold(processNames[parentProcessID], "explorer.exe")
}
