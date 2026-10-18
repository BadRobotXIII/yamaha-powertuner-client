package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"image/png"
	"os"
	"testing"
)

// Exercise the initial portrait layout with Fyne's software/test driver.
// No window is launched and no Read action or network request is performed.
func TestPortraitPreview(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("Preview")
	defer w.Close()
	w.SetContent(makeUI(w))
	w.Resize(fyne.NewSize(480, 800))
	frame := w.Canvas().Capture()
	if frame.Bounds().Dx() < 400 || frame.Bounds().Dy() < 700 {
		t.Fatal("portrait canvas unexpectedly small")
	}
	if err := os.MkdirAll("build", 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("build/portrait-preview.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, frame); err != nil {
		t.Fatal(err)
	}
}
