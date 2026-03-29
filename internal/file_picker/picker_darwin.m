#import <Cocoa/Cocoa.h>
#include "picker_darwin.h"

PickerResult pickFileOrFolder(const char* title) {
    PickerResult result = {NULL, 0};

    NSOpenPanel* panel = [NSOpenPanel openPanel];
    [panel setCanChooseFiles:YES];
    [panel setCanChooseDirectories:YES];
    [panel setAllowsMultipleSelection:NO];
    [panel setFloatingPanel:YES];

    if (title != NULL) {
        [panel setTitle:[NSString stringWithUTF8String:title]];
    }
    [panel setPrompt:@"Send"];

    NSApplicationActivationPolicy policy = [NSApp activationPolicy];
    if (policy == NSApplicationActivationPolicyProhibited) {
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    }

    if ([panel runModal] == NSModalResponseOK) {
        NSURL* url = [[panel URLs] objectAtIndex:0];
        char buf[PATH_MAX];
        if ([url getFileSystemRepresentation:buf maxLength:PATH_MAX]) {
            result.path = strdup(buf);

            NSNumber* isDir = nil;
            [url getResourceValue:&isDir forKey:NSURLIsDirectoryKey error:nil];
            result.isDir = [isDir boolValue] ? 1 : 0;
        }
    }

    return result;
}
