package storage

import (
	"encoding/binary"
	"path/filepath"
	"strings"
)

// Go's standard sniffer does not recognize QuickTime's qt brand. An extension
// alone never enables inline active content; MOV bytes go through FFmpeg.
func CloudVideoCandidate(name, mime string, header []byte) bool {
	if strings.HasPrefix(mime, "video/") {
		return true
	}
	if !strings.EqualFold(filepath.Ext(name), ".mov") {
		return false
	}
	return quickTimeHeader(header)
}

func quickTimeHeader(b []byte) bool {
	for at := 0; at+8 <= len(b); {
		size := int64(binary.BigEndian.Uint32(b[at : at+4]))
		kind := string(b[at+4 : at+8])
		offset := 8
		if size == 1 {
			if at+16 > len(b) {
				return false
			}
			size = int64(binary.BigEndian.Uint64(b[at+8 : at+16]))
			offset = 16
		}
		if kind == "ftyp" && at+offset+4 <= len(b) && string(b[at+offset:at+offset+4]) == "qt  " {
			return true
		}
		if kind == "moov" || kind == "mdat" || kind == "wide" {
			return size == 0 || size >= int64(offset)
		}
		if size < int64(offset) || size > int64(len(b)-at) {
			return false
		}
		at += int(size)
	}
	return false
}
