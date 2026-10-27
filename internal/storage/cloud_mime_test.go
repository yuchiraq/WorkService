package storage

import "testing"

func TestQuickTimeSniff(t *testing.T) {
	qt := []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'q', 't', ' ', ' ', 0, 0, 0, 0, 'q', 't', ' ', ' '}
	if !CloudVideoCandidate("phone.MOV", "application/octet-stream", qt) {
		t.Fatal("legacy MOV not detected")
	}
	for _, header := range [][]byte{[]byte("<!doctype html><script>bad</script>"), {0, 0, 0, 0, 'f', 't', 'y', 'p'}, nil} {
		if CloudVideoCandidate("fake.mov", "application/octet-stream", header) {
			t.Fatal("extension trusted")
		}
	}
}
