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

	idFile   = 100
	idFolder = 101
)

// PickFileOrFolder shows a native dialog to choose between file and folder,
// then opens the appropriate native picker.
func PickFileOrFolder() *Result {
	switch showChoiceDialog() {
	case idFile:
		return showDialog("Select file to send", 0)
	case idFolder:
		return showDialog("Select folder to send", fosPickFolders)
	default:
		return nil
	}
}

// showChoiceDialog builds a Win32 dialog in memory with "Send File",
// "Send Folder" and "Cancel" buttons. Returns the pressed button ID.
func showChoiceDialog() int32 {
	const (
		wsPopup         = uint32(0x80000000)
		wsCaption       = uint32(0x00C00000)
		wsSysMenu       = uint32(0x00080000)
		wsChild         = uint32(0x40000000)
		wsVisible       = uint32(0x10000000)
		wsTabStop       = uint32(0x00010000)
		dsModalFrame    = uint32(0x00000080)
		dsCenter        = uint32(0x00000800)
		bsDefPushbutton = uint32(0x00000001)
		bsPushbutton    = uint32(0x00000000)
		ssLeft          = uint32(0x00000000)
		atomButton      = uint16(0x0080)
		atomStatic      = uint16(0x0082)
		wmCommand       = uintptr(0x0111)
		idCancel        = uint16(2)
	)

	var tmpl []byte
	pu16 := func(v uint16) { tmpl = append(tmpl, byte(v), byte(v>>8)) }
	pu32 := func(v uint32) { tmpl = append(tmpl, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	pi16 := func(v int16) { pu16(uint16(v)) }
	pwstr := func(s string) {
		for _, r := range s {
			pu16(uint16(r))
		}
		pu16(0)
	}
	align4 := func() {
		for len(tmpl)%4 != 0 {
			tmpl = append(tmpl, 0)
		}
	}
	addControl := func(style uint32, x, y, cx, cy int16, id, atom uint16, text string) {
		align4()
		pu32(style | wsChild | wsVisible)
		pu32(0)
		pi16(x)
		pi16(y)
		pi16(cx)
		pi16(cy)
		pu16(id)
		pu16(0xFFFF)
		pu16(atom)
		pwstr(text)
		pu16(0)
	}

	// DLGTEMPLATE header
	pu32(wsPopup | wsCaption | wsSysMenu | dsModalFrame | dsCenter)
	pu32(0)
	pu16(4) // 4 controls
	pi16(0)
	pi16(0)
	pi16(190) // width
	pi16(52)  // height
	pu16(0)   // no menu
	pu16(0)   // default class
	pwstr("Send")

	addControl(ssLeft, 7, 10, 176, 14, 0xFFFF, atomStatic, "What do you want to send?")
	addControl(bsDefPushbutton|wsTabStop, 7, 32, 58, 14, uint16(idFile), atomButton, "Send File")
	addControl(bsPushbutton|wsTabStop, 72, 32, 58, 14, uint16(idFolder), atomButton, "Send Folder")
	addControl(bsPushbutton|wsTabStop, 137, 32, 46, 14, idCancel, atomButton, "Cancel")

	cb := windows.NewCallback(func(hwnd, msg, wparam, _ uintptr) uintptr {
		if msg == wmCommand {
			id := uint16(wparam & 0xffff)
			if id == uint16(idFile) || id == uint16(idFolder) || id == idCancel {
				windows.NewLazySystemDLL("user32.dll").NewProc("EndDialog").Call(hwnd, uintptr(id))
			}
		}
		return 0
	})

	user32 := windows.NewLazySystemDLL("user32.dll")
	r, _, _ := user32.NewProc("DialogBoxIndirectParamW").Call(
		0,
		uintptr(unsafe.Pointer(&tmpl[0])),
		0,
		cb,
		0,
	)

	switch uint16(r) {
	case uint16(idFile):
		return idFile
	case uint16(idFolder):
		return idFolder
	default:
		return 0
	}
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
