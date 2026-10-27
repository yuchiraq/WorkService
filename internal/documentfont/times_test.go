package documentfont

import (
	"encoding/binary"
	"testing"
)

func TestEmbeddingFlags(t *testing.T) {
	for _, flags := range []uint16{0, 4, 8, 2, 0x100, 0x200} {
		b := make([]byte, 38)
		binary.BigEndian.PutUint32(b, 0x10000)
		binary.BigEndian.PutUint16(b[4:6], 1)
		copy(b[12:16], "OS/2")
		binary.BigEndian.PutUint32(b[20:24], 28)
		binary.BigEndian.PutUint16(b[36:38], flags)
		err := embeddingAllowed(b)
		wantErr := flags&(2|0x100|0x200) != 0
		if (err != nil) != wantErr {
			t.Fatal(flags, err)
		}
	}
}
func TestMissingTimesDoesNotSubstitute(t *testing.T) {
	t.Setenv("WORKSERVICE_DOCUMENT_FONT_DIR", t.TempDir())
	if _, err := Times(); err == nil {
		t.Fatal("missing Times silently substituted")
	}
}
