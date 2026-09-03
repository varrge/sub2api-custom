package service

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSupportTicketImageStripsMetadataAndUsesActualFormat(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	img.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 120})

	var pngBytes bytes.Buffer
	require.NoError(t, png.Encode(&pngBytes, img))
	withMetadata := injectPNGChunk(t, pngBytes.Bytes(), "tEXt", []byte("comment\x00private-canary"))
	got, err := NormalizeSupportTicketImage(withMetadata)
	require.NoError(t, err)
	require.Equal(t, "image/png", got.ContentType)
	require.Equal(t, 2, *got.Width)
	require.Equal(t, 3, *got.Height)
	require.NotContains(t, string(got.Data), "private-canary")
	decoded, format, err := image.Decode(bytes.NewReader(got.Data))
	require.NoError(t, err)
	require.Equal(t, "png", format)
	require.Equal(t, image.Rect(0, 0, 2, 3), decoded.Bounds())

	var jpegBytes bytes.Buffer
	require.NoError(t, jpeg.Encode(&jpegBytes, img, nil))
	jpegWithMetadata := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x10}, append([]byte("private-canary"), jpegBytes.Bytes()[2:]...)...)
	got, err = NormalizeSupportTicketImage(jpegWithMetadata)
	require.NoError(t, err)
	require.Equal(t, "image/jpeg", got.ContentType)
	require.NotContains(t, string(got.Data), "private-canary")
}

func TestNormalizeSupportTicketImageAcceptsWebPAndStoresPNG(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString("UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA==")
	require.NoError(t, err)
	got, err := NormalizeSupportTicketImage(raw)
	require.NoError(t, err)
	require.Equal(t, "image/png", got.ContentType)
	_, format, err := image.Decode(bytes.NewReader(got.Data))
	require.NoError(t, err)
	require.Equal(t, "png", format)
}

func TestNormalizeSupportTicketImageRejectsUnsafeInputs(t *testing.T) {
	t.Run("too large", func(t *testing.T) {
		_, err := NormalizeSupportTicketImage(make([]byte, SupportTicketMaxImageBytes+1))
		require.ErrorIs(t, err, ErrSupportTicketImageTooLarge)
	})
	t.Run("not an image", func(t *testing.T) {
		_, err := NormalizeSupportTicketImage([]byte("not-an-image"))
		require.ErrorIs(t, err, ErrSupportTicketImageInvalid)
	})
	t.Run("truncated after valid config", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))))
		_, err := NormalizeSupportTicketImage(buf.Bytes()[:40])
		require.ErrorIs(t, err, ErrSupportTicketImageInvalid)
	})
	t.Run("pixel ceiling", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1))))
		oversized := rewritePNGDimensions(t, buf.Bytes(), 5001, 5000)
		_, err := NormalizeSupportTicketImage(oversized)
		require.ErrorIs(t, err, ErrSupportTicketImageInvalid)
	})
	t.Run("normalized output ceiling", func(t *testing.T) {
		img := image.NewYCbCr(image.Rect(0, 0, 4096, 4096), image.YCbCrSubsampleRatio444)
		var state uint32 = 1
		fill := func(pixels []byte) {
			for i := range pixels {
				state = state*1664525 + 1013904223
				pixels[i] = byte(state >> 24)
			}
		}
		for _, pixels := range [][]byte{img.Y, img.Cb, img.Cr} {
			fill(pixels)
		}
		var compressed bytes.Buffer
		require.NoError(t, jpeg.Encode(&compressed, img, &jpeg.Options{Quality: 20}))
		require.LessOrEqual(t, compressed.Len(), SupportTicketMaxImageBytes)
		_, err := NormalizeSupportTicketImage(compressed.Bytes())
		require.ErrorIs(t, err, ErrSupportTicketImageTooLarge)
	})
}

func injectPNGChunk(t *testing.T, raw []byte, kind string, data []byte) []byte {
	t.Helper()
	iend := bytes.LastIndex(raw, []byte("IEND")) - 4
	require.GreaterOrEqual(t, iend, 8)
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[8+len(data):], crc32.ChecksumIEEE(chunk[4:8+len(data)]))
	return append(append(append([]byte{}, raw[:iend]...), chunk...), raw[iend:]...)
}

func rewritePNGDimensions(t *testing.T, raw []byte, width, height uint32) []byte {
	t.Helper()
	result := append([]byte{}, raw...)
	require.Equal(t, "IHDR", string(result[12:16]))
	binary.BigEndian.PutUint32(result[16:20], width)
	binary.BigEndian.PutUint32(result[20:24], height)
	binary.BigEndian.PutUint32(result[29:33], crc32.ChecksumIEEE(result[12:29]))
	return result
}
