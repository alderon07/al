package app

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func validateTrackedSourceMetadata(fd int) error {
	volume := unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT, Volattr: unix.ATTR_VOL_INFO | unix.ATTR_VOL_ATTRIBUTES}
	buffer := make([]byte, 4096)
	if err := readTrackedDarwinAttributes(fd, &volume, buffer); err != nil {
		return err
	}
	if err := validateTrackedDarwinVolumeAttributes(buffer); err != nil {
		return err
	}
	security := unix.Attrlist{Bitmapcount: unix.ATTR_BIT_MAP_COUNT, Commonattr: unix.ATTR_CMN_EXTENDED_SECURITY}
	clear(buffer)
	if err := readTrackedDarwinAttributes(fd, &security, buffer); err != nil {
		return err
	}
	return validateTrackedDarwinSecurityAttributes(buffer)
}

func readTrackedDarwinAttributes(fd int, attributes *unix.Attrlist, buffer []byte) error {
	_, _, errno := syscall.Syscall6(unix.SYS_FGETATTRLIST, uintptr(fd), uintptr(unsafe.Pointer(attributes)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), unix.FSOPT_REPORT_FULLSIZE, 0)
	runtime.KeepAlive(attributes)
	runtime.KeepAlive(buffer)
	if errno != 0 {
		return fmt.Errorf("cannot verify tracked source ACL metadata; run al untrack FILE: %w", errno)
	}
	return nil
}
