package file_picker

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Result struct {
	Path  string
	IsDir bool
}

// COM GUIDs
var (
	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

const (
	fosPickFolders     = 0x20
	fosForceFileSystem = 0x40
	fosNoChangeDir     = 0x8
	sigdnFileSysPath   = 0x80058000
	clsctxInprocServer = 0x1
)

// PickFileOrFolder opens a native Windows file dialog.
// First shows a file picker; if cancelled, shows a folder picker.
func PickFileOrFolder() *Result {
	if result := showDialog("Select file to send", 0); result != nil {
		return result
	}
	return showDialog("Select folder to send", fosPickFolders)
}

func showDialog(title string, extraFlags uint32) *Result {
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if err != nil {
		return nil
	}
	defer windows.CoUninitialize()

	var punk unsafe.Pointer
	hr := coCreateInstance(
		&clsidFileOpenDialog,
		clsctxInprocServer,
		&iidIFileOpenDialog,
		&punk,
	)
	if hr != 0 {
		return nil
	}

	dlg := (*iFileOpenDialog)(punk)
	defer dlg.Release()

	var opts uint32
	dlg.GetOptions(&opts)
	dlg.SetOptions(opts | fosForceFileSystem | fosNoChangeDir | extraFlags)

	titlePtr, _ := windows.UTF16PtrFromString(title)
	dlg.SetTitle(titlePtr)

	if dlg.Show(0) != 0 {
		return nil
	}

	var item *iShellItem
	if dlg.GetResult(&item) != 0 {
		return nil
	}
	defer item.Release()

	var namePtr *uint16
	if item.GetDisplayName(sigdnFileSysPath, &namePtr) != 0 {
		return nil
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(namePtr))

	path := windows.UTF16PtrToString(namePtr)
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	return &Result{Path: path, IsDir: info.IsDir()}
}

// COM vtable wrappers

type iFileOpenDialog struct {
	vtbl *iFileOpenDialogVtbl
}

type iFileOpenDialogVtbl struct {
	QueryInterface      uintptr
	AddRef              uintptr
	Release             uintptr
	Show                uintptr
	SetFileTypes        uintptr
	SetFileTypeIndex    uintptr
	GetFileTypeIndex    uintptr
	Advise              uintptr
	Unadvise            uintptr
	SetOptions          uintptr
	GetOptions          uintptr
	SetDefaultFolder    uintptr
	SetFolder           uintptr
	GetFolder           uintptr
	GetCurrentSelection uintptr
	SetFileName         uintptr
	GetFileName         uintptr
	SetTitle            uintptr
	SetOkButtonLabel    uintptr
	SetFileNameLabel    uintptr
	GetResult           uintptr
}

func (d *iFileOpenDialog) Release() {
	syscall.SyscallN(d.vtbl.Release, uintptr(unsafe.Pointer(d)))
}

func (d *iFileOpenDialog) Show(hwnd uintptr) int32 {
	r, _, _ := syscall.SyscallN(d.vtbl.Show, uintptr(unsafe.Pointer(d)), hwnd)
	return int32(r)
}

func (d *iFileOpenDialog) SetOptions(opts uint32) {
	syscall.SyscallN(d.vtbl.SetOptions, uintptr(unsafe.Pointer(d)), uintptr(opts))
}

func (d *iFileOpenDialog) GetOptions(opts *uint32) {
	syscall.SyscallN(d.vtbl.GetOptions, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(opts)))
}

func (d *iFileOpenDialog) SetTitle(title *uint16) {
	syscall.SyscallN(d.vtbl.SetTitle, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(title)))
}

func (d *iFileOpenDialog) GetResult(item **iShellItem) int32 {
	r, _, _ := syscall.SyscallN(d.vtbl.GetResult, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(item)))
	return int32(r)
}

type iShellItem struct {
	vtbl *iShellItemVtbl
}

type iShellItemVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr
	GetAttributes  uintptr
	Compare        uintptr
}

func (s *iShellItem) Release() {
	syscall.SyscallN(s.vtbl.Release, uintptr(unsafe.Pointer(s)))
}

func (s *iShellItem) GetDisplayName(sigdn uint32, name **uint16) int32 {
	r, _, _ := syscall.SyscallN(s.vtbl.GetDisplayName, uintptr(unsafe.Pointer(s)), uintptr(sigdn), uintptr(unsafe.Pointer(name)))
	return int32(r)
}

func coCreateInstance(clsid *windows.GUID, clsctx uint32, iid *windows.GUID, obj *unsafe.Pointer) int32 {
	ole32 := windows.NewLazySystemDLL("ole32.dll")
	proc := ole32.NewProc("CoCreateInstance")
	r, _, _ := proc.Call(
		uintptr(unsafe.Pointer(clsid)),
		0,
		uintptr(clsctx),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(obj)),
	)
	return int32(r)
}
