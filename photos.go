package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

const (
	maxPhotoBytes  = 25 << 20
	maxPhotoPixels = 60_000_000
	photoMaxSide   = 1600
)

var photoIDRe = regexp.MustCompile(`^[a-f0-9]{16}\.(jpg|png|gif|webp)$`)

var errPhotoType = errors.New("only JPG, PNG, GIF or WebP (iPhone HEIC photos need converting to JPG first)")

// savePhoto stores an uploaded image in dir and returns its file name.
// JPEG/PNG are shrunk to photoMaxSide (phone photos are huge) with EXIF
// rotation applied; GIF and WebP are kept as they are.
func savePhoto(dir string, r io.Reader) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxPhotoBytes+1))
	if err != nil {
		return "", err
	}
	if len(raw) > maxPhotoBytes {
		return "", fmt.Errorf("too big (max %d MB)", maxPhotoBytes>>20)
	}
	var out []byte
	var ext string
	switch http.DetectContentType(raw) {
	case "image/jpeg":
		img, err := decodeChecked(raw, jpeg.Decode)
		if err != nil {
			return "", err
		}
		img = orient(shrink(img, photoMaxSide), jpegOrientation(raw))
		out, ext, err = encodeJPEG(img)
		if err != nil {
			return "", err
		}
	case "image/png":
		img, err := decodeChecked(raw, png.Decode)
		if err != nil {
			return "", err
		}
		img = shrink(img, photoMaxSide)
		if opaque(img) {
			out, ext, err = encodeJPEG(img)
		} else {
			var buf bytes.Buffer
			err = png.Encode(&buf, img)
			out, ext = buf.Bytes(), "png"
		}
		if err != nil {
			return "", err
		}
	case "image/gif":
		out, ext = raw, "gif"
	case "image/webp":
		out, ext = raw, "webp"
	default:
		return "", errPhotoType
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	name := hex.EncodeToString(b) + "." + ext
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, name), out, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

func decodeChecked(raw []byte, decode func(io.Reader) (image.Image, error)) (*image.RGBA, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("can't read image: %w", err)
	}
	if cfg.Width*cfg.Height > maxPhotoPixels {
		return nil, errors.New("image has too many pixels")
	}
	img, err := decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("can't read image: %w", err)
	}
	rgba := image.NewRGBA(img.Bounds().Sub(img.Bounds().Min))
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	return rgba, nil
}

func encodeJPEG(img *image.RGBA) ([]byte, string, error) {
	// Flatten any transparency onto white.
	bg := image.NewRGBA(img.Bounds())
	draw.Draw(bg, bg.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	draw.Draw(bg, bg.Bounds(), img, image.Point{}, draw.Over)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, bg, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "jpg", nil
}

func opaque(img *image.RGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0xff {
			return false
		}
	}
	return true
}

// shrink box-filters img so its longest side is at most maxSide.
func shrink(src *image.RGBA, maxSide int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw <= maxSide && sh <= maxSide {
		return src
	}
	dw, dh := maxSide, sh*maxSide/sw
	if sh > sw {
		dw, dh = sw*maxSide/sh, maxSide
	}
	dw, dh = max(dw, 1), max(dh, 1)
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		y0, y1 := y*sh/dh, max((y+1)*sh/dh, y*sh/dh+1)
		for x := range dw {
			x0, x1 := x*sw/dw, max((x+1)*sw/dw, x*sw/dw+1)
			var r, g, b, a, n int
			for sy := y0; sy < y1; sy++ {
				i := sy*src.Stride + x0*4
				for sx := x0; sx < x1; sx++ {
					r += int(src.Pix[i])
					g += int(src.Pix[i+1])
					b += int(src.Pix[i+2])
					a += int(src.Pix[i+3])
					i += 4
					n++
				}
			}
			j := y*dst.Stride + x*4
			dst.Pix[j], dst.Pix[j+1], dst.Pix[j+2], dst.Pix[j+3] = uint8(r/n), uint8(g/n), uint8(b/n), uint8(a/n)
		}
	}
	return dst
}

// orient applies an EXIF orientation (1–8) so the photo shows upright once
// the EXIF data is gone.
func orient(src *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		for x := range dw {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			copy(dst.Pix[y*dst.Stride+x*4:][:4], src.Pix[sy*src.Stride+sx*4:][:4])
		}
	}
	return dst
}

// jpegOrientation reads the EXIF orientation tag from a JPEG (1 if absent).
func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end of image
			return 1
		}
		size := int(binary.BigEndian.Uint16(b[i+2:]))
		if size < 2 || i+2+size > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[ifd:]))
	for k := range n {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}
