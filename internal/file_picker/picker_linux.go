package file_picker

import "os"

// #cgo pkg-config: gtk+-3.0
// #cgo LDFLAGS: -lX11
// #include <X11/Xlib.h>
// #include <gtk/gtk.h>
// #include <stdlib.h>
//
// static int gtk_init_once() {
//     static int done = 0;
//     if (!done) {
//         XInitThreads();
//         done = gtk_init_check(NULL, NULL);
//     }
//     return done;
// }
//
// static char* show_dialog(const char* title, int pick_folder) {
//     if (!gtk_init_once()) return NULL;
//
//     GtkFileChooserAction action = pick_folder
//         ? GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER
//         : GTK_FILE_CHOOSER_ACTION_OPEN;
//
//     GtkWidget* dlg = gtk_file_chooser_dialog_new(
//         title, NULL, action,
//         "Cancel", GTK_RESPONSE_CANCEL,
//         "Send", GTK_RESPONSE_ACCEPT,
//         NULL);
//
//     char* result = NULL;
//     if (gtk_dialog_run(GTK_DIALOG(dlg)) == GTK_RESPONSE_ACCEPT) {
//         result = gtk_file_chooser_get_filename(GTK_FILE_CHOOSER(dlg));
//     }
//     gtk_widget_destroy(dlg);
//     while (gtk_events_pending()) gtk_main_iteration();
//     return result;
// }
import "C"
import "unsafe"

type Result struct {
	Path  string
	IsDir bool
}

// first shows a file picker. If cancelled, shows a folder picker.
func PickFileOrFolder(title string) *Result {
	if result := gtkDialog(title, 0); result != nil {
		return result
	}
	return gtkDialog(title, 1)
}

func gtkDialog(title string, pickFolder int) *Result {
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))

	cpath := C.show_dialog(ctitle, C.int(pickFolder))
	if cpath == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(cpath))

	path := C.GoString(cpath)
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	return &Result{
		Path:  path,
		IsDir: info.IsDir(),
	}
}
