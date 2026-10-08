//go:build !windows

package shell

import (
	"crypto/sha256"
	"testing"
)

func TestShadowOriginKnownAnswer(t *testing.T) {
	if got := shadowOriginFromDigest("bash", make([]byte, 32), 0, 10); got != "0e4470091b064fefd0a1574d8aa97801eab2016fe14c3fc7fe88ba518b360bfb" {
		t.Fatalf("origin vector = %s", got)
	}
}

func TestShadowGenerationHashVectors(t *testing.T) {
	approval := sha256.Sum256(nil)
	got := shadowGenerationHash("bash/v1", "linux", approval[:], []byte("# body\n"))
	if got != "06fc5ce2f0aaa98290cf5ceecb582c406e0b4249606891779927bcf8f86bc205" {
		t.Fatalf("generation vector = %s", got)
	}
}
