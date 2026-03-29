#ifndef PICKER_DARWIN_H
#define PICKER_DARWIN_H

#include <stdlib.h>

typedef struct {
    char* path;  // NULL if cancelled
    int isDir;   // 1 if directory, 0 if file
} PickerResult;

PickerResult pickFileOrFolder(const char* title);

#endif
