package file_picker

// #cgo darwin CFLAGS: -x objective-c
// #cgo darwin LDFLAGS: -framework Cocoa
// #include "picker_darwin.h"
import "C"
import "unsafe"

// Result holds the path selected by the user and whether it is a directory
type Result struct {
	Path  string
	IsDir bool
}

// PickFileOrFolder opens a native macOS dialog that allows the user
// to select either a file or a folder from a single panel.
// Returns nil if the user cancelled.
func PickFileOrFolder() *Result {
	title := C.CString("Select file or folder to send")
	defer C.free(unsafe.Pointer(title))

	r := C.pickFileOrFolder(title)
	if r.path == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(r.path))

	return &Result{
		Path:  C.GoString(r.path),
		IsDir: r.isDir == 1,
	}
}
