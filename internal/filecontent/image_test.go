package filecontent_test

import (
	"testing"

	"github.com/subzone/Agentctl/internal/filecontent"
)

func TestIsImagePNG(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !filecontent.IsImage("x.png", png) {
		t.Fatal("expected png detection")
	}
}

func TestReadImageBytesPNG(t *testing.T) {
	// Minimal PNG header only — enough for mime detection, not a valid image file for display.
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	img, err := filecontent.ReadImageBytes("photo.png", png)
	if err != nil {
		t.Fatal(err)
	}
	if img.MimeType != "image/png" {
		t.Fatalf("mime: %s", img.MimeType)
	}
}
