package menus

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// jpegOf is a real JPEG of the given size.
func jpegOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, x%h, color.RGBA{R: 200, G: 90, B: 40, A: 255})
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAPhotoIsStoredByItsHash(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))
	idli := w.create(t, plain, w.item(plain, cat, "Idli", 8000, ""))
	data := jpegOf(t, 480, 360)

	before := w.version(t, plain)
	name, err := w.s.SetItemPhoto(ctx, plain, dosa, data)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}\.jpg$`).MatchString(name) {
		t.Errorf("name = %q", name)
	}
	if w.version(t, plain) != before+1 {
		t.Error("a photo is a menu edit: it bumps the version")
	}
	if got := w.tillMenu(t, plain).Items[0].Photo; got != name {
		t.Errorf("the till's dosa has photo %q", got)
	}

	// The same photo on another item is the same file.
	again, err := w.s.SetItemPhoto(ctx, plain, idli, data)
	if err != nil || again != name {
		t.Fatalf("again = %q, %v", again, err)
	}
	files, _ := os.ReadDir(w.photoDir)
	if len(files) != 1 {
		t.Errorf("%d files in the store, want one: %v", len(files), files)
	}

	f, err := w.s.OpenPhoto(name)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := f.Stat()
	_ = f.Close()
	if info.Size() != int64(len(data)) {
		t.Errorf("stored %d bytes, sent %d", info.Size(), len(data))
	}

	if err := w.s.ClearItemPhoto(ctx, plain, dosa); err != nil {
		t.Fatal(err)
	}
	if got := findItem(w.editor(t, plain), dosa).Photo; got != "" {
		t.Errorf("cleared photo = %q", got)
	}
	if _, err := os.Stat(filepath.Join(w.photoDir, name)); err != nil {
		t.Error("clearing keeps the file: idli still shows it")
	}

	if _, err := w.s.SetItemPhoto(ctx, plain, 999, data); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("a missing item: %v", err)
	}
	if err := w.s.ClearItemPhoto(ctx, plain, 999); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("clearing a missing item: %v", err)
	}
}

func TestPhotosAreChecked(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	cat := w.category(t, plain, "Food")
	dosa := w.create(t, plain, w.item(plain, cat, "Dosa", 12000, ""))

	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 200, 200))); err != nil {
		t.Fatal(err)
	}
	good := jpegOf(t, 300, 300)
	// A JPEG whose header claims 5000 x 5000: refused on the header, before
	// anything that size is decoded.
	huge := bytes.Clone(good)
	if i := bytes.Index(huge, []byte{0xFF, 0xC0}); i > 0 {
		huge[i+5], huge[i+6], huge[i+7], huge[i+8] = 0x13, 0x88, 0x13, 0x88
	} else {
		t.Fatal("no SOF0 marker in the test JPEG")
	}

	tests := []struct {
		name string
		data []byte
	}{
		{"nothing", nil},
		{"not an image", []byte("hello")},
		{"a PNG", pngBytes.Bytes()},
		{"too small", jpegOf(t, 32, 32)},
		{"too wide", jpegOf(t, MaxPhotoSide+1, 100)},
		{"a header claiming a huge image", huge},
		{"cut off after the header", good[:len(good)/3]},
		{"over the byte limit", append(bytes.Clone(good), make([]byte, MaxPhotoBytes)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := w.version(t, plain)
			_, err := w.s.SetItemPhoto(ctx, plain, dosa, tt.data)
			if fieldOf(err) != "photo" {
				t.Errorf("err = %v, want a photo refusal", err)
			}
			if w.version(t, plain) != before {
				t.Error("a refused photo must change nothing")
			}
		})
	}
	if files, _ := os.ReadDir(w.photoDir); len(files) != 0 {
		t.Errorf("refused photos left files: %v", files)
	}
}

func TestOpeningAPhotoByName(t *testing.T) {
	w := newWorld(t)
	for _, name := range []string{
		"",
		"../nofiltr.db",
		"abc.jpg",
		"ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789.jpg",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.png",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef.jpg",
	} {
		if _, err := w.s.OpenPhoto(name); !errors.Is(err, ErrPhotoNotFound) {
			t.Errorf("OpenPhoto(%q) = %v, want not found", name, err)
		}
	}
	bare, err := New(Options{DB: w.d})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bare.OpenPhoto("x"); !errors.Is(err, ErrPhotoNotFound) {
		t.Errorf("no store: %v", err)
	}
	if _, err := bare.SetItemPhoto(context.Background(), plain, 1, jpegOf(t, 100, 100)); err == nil {
		t.Error("no store: setting a photo must fail")
	}
}
