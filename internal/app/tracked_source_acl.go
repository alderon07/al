package app

import (
	"encoding/binary"
	"fmt"
)

func validateTrackedDarwinVolumeAttributes(buffer []byte) error {
	if len(buffer) < 44 || binary.LittleEndian.Uint32(buffer[:4]) != 44 || binary.LittleEndian.Uint32(buffer[4:8])&0x400000 == 0 {
		return fmt.Errorf("tracked source filesystem cannot prove native ACL support; run al untrack FILE")
	}
	return nil
}

func validateTrackedDarwinSecurityAttributes(buffer []byte) error {
	unsafe := fmt.Errorf("tracked source has unsafe or unverified ACL metadata; run al untrack FILE")
	if len(buffer) < 12 {
		return unsafe
	}
	total := uint64(binary.LittleEndian.Uint32(buffer[:4]))
	start := int64(4) + int64(int32(binary.LittleEndian.Uint32(buffer[4:8])))
	length := uint64(binary.LittleEndian.Uint32(buffer[8:12]))
	if total < 12 || total > uint64(len(buffer)) || start < 12 || uint64(start) > total || length > total-uint64(start) {
		return unsafe
	}
	if length == 0 {
		return nil
	}
	security := buffer[start : uint64(start)+length]
	if len(security) < 44 || binary.LittleEndian.Uint32(security[:4]) != 0x012cc16d {
		return unsafe
	}
	count := binary.LittleEndian.Uint32(security[36:40])
	if count == 0xffffffff {
		if len(security) != 44 {
			return unsafe
		}
		return nil
	}
	if count > 128 || uint64(len(security)) != 44+uint64(count)*24 {
		return unsafe
	}
	const readableRights = 1<<1 | 1<<3 | 1<<7 | 1<<9 | 1<<11 | 1<<20 | 1<<22 | 1<<24
	for offset := 44; offset < len(security); offset += 24 {
		flags := binary.LittleEndian.Uint32(security[offset+16 : offset+20])
		rights := binary.LittleEndian.Uint32(security[offset+20 : offset+24])
		if flags&0xf == 2 {
			continue
		}
		if flags&0xf != 1 || rights & ^uint32(readableRights) != 0 {
			return unsafe
		}
	}
	return nil
}
