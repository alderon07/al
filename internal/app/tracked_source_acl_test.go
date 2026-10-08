package app

import (
	"encoding/binary"
	"testing"
)

func TestTrackedDarwinACLDecoder(t *testing.T) {
	for _, test := range []struct {
		name         string
		kind, rights uint32
		allow        bool
	}{
		{"read", 1, 1<<1 | 1<<7, true},
		{"deny-delete", 2, 1 << 4, true},
		{"write", 1, 1 << 2, false},
		{"append", 1, 1 << 5, false},
		{"delete-child", 1, 1 << 6, false},
		{"write-security", 1, 1 << 12, false},
		{"ownership", 1, 1 << 13, false},
		{"generic-write", 1, 1 << 23, false},
		{"unknown-kind", 3, 0, false},
		{"unknown-right", 1, 1 << 31, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := trackedDarwinACLBuffer(test.kind, test.rights)
			err := validateTrackedDarwinSecurityAttributes(buffer)
			if (err == nil) != test.allow {
				t.Fatalf("ACL accepted = %v, want %v", err == nil, test.allow)
			}
		})
	}
	for _, field := range []int{0, 4, 8, 12, 48} {
		buffer := trackedDarwinACLBuffer(1, 1<<1)
		binary.LittleEndian.PutUint32(buffer[field:field+4], 0xffffffff)
		if err := validateTrackedDarwinSecurityAttributes(buffer); err == nil {
			t.Fatalf("malformed ACL field %d accepted", field)
		}
	}
	absent := make([]byte, 12)
	binary.LittleEndian.PutUint32(absent[:4], 12)
	binary.LittleEndian.PutUint32(absent[4:8], 8)
	if err := validateTrackedDarwinSecurityAttributes(absent); err != nil {
		t.Fatal(err)
	}
	volume := make([]byte, 44)
	binary.LittleEndian.PutUint32(volume[:4], 44)
	if err := validateTrackedDarwinVolumeAttributes(volume); err == nil {
		t.Fatal("unverified volume ACL support accepted")
	}
	binary.LittleEndian.PutUint32(volume[4:8], 0x400000)
	if err := validateTrackedDarwinVolumeAttributes(volume); err != nil {
		t.Fatal(err)
	}
}

func trackedDarwinACLBuffer(kind, rights uint32) []byte {
	buffer := make([]byte, 80)
	binary.LittleEndian.PutUint32(buffer[:4], uint32(len(buffer)))
	binary.LittleEndian.PutUint32(buffer[4:8], 8)
	binary.LittleEndian.PutUint32(buffer[8:12], 68)
	binary.LittleEndian.PutUint32(buffer[12:16], 0x012cc16d)
	binary.LittleEndian.PutUint32(buffer[48:52], 1)
	binary.LittleEndian.PutUint32(buffer[72:76], kind)
	binary.LittleEndian.PutUint32(buffer[76:80], rights)
	return buffer
}
