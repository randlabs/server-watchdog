package directories

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

//------------------------------------------------------------------------------

const (
	MAX_PATH = 260

	S_OK = 0

	SHGFP_TYPE_CURRENT = 0

	CSIDL_COMMON_APPDATA = 35
	CSIDL_LOCAL_APPDATA = 28
)

//------------------------------------------------------------------------------

var (
	modShell32               = windows.NewLazySystemDLL("Shell32.dll")
	procSHGetKnownFolderPath = modShell32.NewProc("SHGetKnownFolderPath")
	procSHGetFolderPathW     = modShell32.NewProc("SHGetFolderPathW")
	modOle32                 = syscall.NewLazyDLL("Ole32.dll")
	procCoTaskMemFree        = modOle32.NewProc("CoTaskMemFree")

	// {62AB5D82-FDC1-4DC3-A9DD-070D1D495D97}
	FOLDERID_ProgramData = syscall.GUID{
		Data1: 0x62AB5D82, Data2: 0xFDC1, Data3: 0x4DC3,
		Data4: [8]byte{0xA9, 0xDD, 0x07, 0x0D, 0x1D, 0x49, 0x5D, 0x97},
	}

	// {F1B32785-6FBA-4FCF-9D55-7B8E7F157091}
	FOLDERID_LocalAppData = syscall.GUID{
		Data1: 0xF1B32785, Data2: 0x6FBA, Data3: 0x4FCF,
		Data4: [8]byte{0x9D, 0x55, 0x7B, 0x8E, 0x7F, 0x15, 0x70, 0x91},
	}
)

//------------------------------------------------------------------------------

func getHomeDirectory() (string, error) {
	var s string

	t, err := syscall.OpenCurrentProcessToken()
	if err == nil {
		s, err = t.GetUserProfileDirectory()
	}
	if err != nil {
		return "", fmt.Errorf("unable to retrieve home directory")
	}
	return s, nil
}

func getAppSettingsDirectory() (string, error) {
	var s string
	var err error

	if procSHGetKnownFolderPath.Find() == nil {
		s, err = shGetKnownFolderPath(FOLDERID_LocalAppData)
	} else {
		s, err = shGetFolderPath(CSIDL_LOCAL_APPDATA)
	}
	if err != nil {
		return "", fmt.Errorf("unable to retrieve application settings path")
	}
	return s, nil
}

func getSystemSettingsDirectory() (string, error) {
	var s string
	var err error

	if procSHGetKnownFolderPath.Find() == nil {
		s, err = shGetKnownFolderPath(FOLDERID_ProgramData)
	} else {
		s, err = shGetFolderPath(CSIDL_COMMON_APPDATA)
	}
	if err != nil {
		return "", fmt.Errorf("unable to retrieve system settings path")
	}
	return s, nil
}

func shGetKnownFolderPath(folderId syscall.GUID) (string, error) {
	// Get the path to local app data folder
	var raw *uint16

	ret, _, err := procSHGetKnownFolderPath.Call(
		uintptr(unsafe.Pointer(&folderId)),
		0,
		0,
		uintptr(unsafe.Pointer(&raw)),
	)
	if ret != S_OK {
		if err == nil {
			err = syscall.EINVAL
		}
		return "", err
	}

	// defer freeing memory since this API call is managed
	defer func() {
		_, _, _ = procCoTaskMemFree.Call(uintptr(unsafe.Pointer(raw)))
	}()

	// convert UTF-16 to a Go string
	return syscall.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(raw))[:]), nil
}

func shGetFolderPath(folder uint32) (string, error) {
	var err error

	buffer := make([]uint16, MAX_PATH+1)

	ret, _, e1 := syscall.Syscall6(
		procSHGetFolderPathW.Addr(),
		5, // actual argument count
		0,
		uintptr(folder),
		0,
		SHGFP_TYPE_CURRENT,
		uintptr(unsafe.Pointer(&buffer[0])),
		0,
	)
	if ret != S_OK {
		if e1 != 0 {
			err = e1
		} else {
			err = syscall.EINVAL
		}
		return "", err
	}

	// convert UTF-16 to a Go string
	return syscall.UTF16ToString(buffer), nil
}
