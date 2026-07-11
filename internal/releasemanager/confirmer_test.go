package releasemanager

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestTTYConfirmerRejectsPipedInputWithoutPrompting(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := write.WriteString("2.0.0\n"); err != nil {
		t.Fatal(err)
	}
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	var output bytes.Buffer

	confirmed, err := (TTYConfirmer{Input: read, Output: &output}).Confirm(context.Background(), "2.0.0")
	if err == nil || !strings.Contains(err.Error(), "TTY") {
		t.Fatalf("Confirm error = %v, want an explicit TTY rejection", err)
	}
	if confirmed {
		t.Fatal("piped input unexpectedly confirmed a release change")
	}
	if output.Len() != 0 {
		t.Fatalf("non-TTY confirmation wrote a misleading prompt: %q", output.String())
	}
}
