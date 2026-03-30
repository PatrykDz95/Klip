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
// #define CHOICE_FILE   1
// #define CHOICE_FOLDER 2
//
// static int ask_file_or_folder() {
//     if (!gtk_init_once()) return 0;
//
//     GtkWidget* dlg = gtk_message_dialog_new(
//         NULL, GTK_DIALOG_MODAL, GTK_MESSAGE_QUESTION, GTK_BUTTONS_NONE,
//         "What do you want to send?");
//     gtk_window_set_title(GTK_WINDOW(dlg), "Send");
//     gtk_dialog_add_button(GTK_DIALOG(dlg), "Send File",   CHOICE_FILE);
//     gtk_dialog_add_button(GTK_DIALOG(dlg), "Send Folder", CHOICE_FOLDER);
//     gtk_dialog_add_button(GTK_DIALOG(dlg), "Cancel",      GTK_RESPONSE_CANCEL);
//
//     gint response = gtk_dialog_run(GTK_DIALOG(dlg));
//     gtk_widget_destroy(dlg);
//     while (gtk_events_pending()) gtk_main_iteration();
//
//     return (response == CHOICE_FILE || response == CHOICE_FOLDER) ? (int)response : 0;
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

func PickFileOrFolder() *Result {
	switch int(C.ask_file_or_folder()) {
	case 1:
		return gtkDialog("Select file to send", 0)
	case 2:
		return gtkDialog("Select folder to send", 1)
	default:
		return nil
	}
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
