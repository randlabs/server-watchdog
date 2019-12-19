// +build darwin

package directories

//------------------------------------------------------------------------------

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <stdlib.h>

char *GetHomeDirectory() {
	id path = NSHomeDirectory();
	const char *tempString = [path UTF8String];
	char *ret = malloc(strlen(tempString) + 1);
	if (ret) {
		memcpy(ret, tempString, strlen(tempString) + 1);
	}
	return ret;
}

char *GetUserApplicationSupportPath() {
	NSArray* paths = NSSearchPathForDirectoriesInDomains(NSApplicationSupportDirectory, NSUserDomainMask, YES);
	for (NSString* path in paths) {
		const char *tempString = [path UTF8String];
		char *ret = malloc(strlen(tempString) + 1);
		if (ret) {
			memcpy(ret, tempString, strlen(tempString) + 1);
		}
		return ret;
	}
	return NULL;
}

char *GetSystemApplicationSupportPath() {
	NSArray* paths = NSSearchPathForDirectoriesInDomains(NSApplicationSupportDirectory, NSLocalDomainMask, YES);
	for (NSString* path in paths) {
		const char *tempString = [path UTF8String];
		char *ret = malloc(strlen(tempString) + 1);
		if (ret) {
			memcpy(ret, tempString, strlen(tempString) + 1);
		}
		return ret;
	}
	return NULL;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func getHomeDirectory() (string, error) {
	cPath := C.GetHomeDirectory()
	if uintptr(unsafe.Pointer(cPath)) == 0 {
		return "", fmt.Errorf("unable to retrieve home directory")
	}
	defer C.free(unsafe.Pointer(cPath))

	return C.GoString(cPath), nil
}

func getAppSettingsDirectory() (string, error) {
	cPath := C.GetUserApplicationSupportPath()
	if uintptr(unsafe.Pointer(cPath)) == 0 {
		return "", fmt.Errorf("unable to retrieve application settings path")
	}
	defer C.free(unsafe.Pointer(cPath))

	return C.GoString(cPath), nil
}

func getSystemSettingsDirectory() (string, error) {
	cPath := C.GetSystemApplicationSupportPath()
	if uintptr(unsafe.Pointer(cPath)) == 0 {
		return "", fmt.Errorf("unable to retrieve system settings path")
	}
	defer C.free(unsafe.Pointer(cPath))

	return C.GoString(cPath), nil
}

