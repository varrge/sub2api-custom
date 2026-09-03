package service

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"

	"golang.org/x/image/webp"
)

const (
	SupportTicketMaxImageBytes            = 5 << 20
	SupportTicketMaxNormalizedBytes       = 10 << 20
	SupportTicketMaxImagePixels     int64 = 25_000_000
)

// NormalizeSupportTicketImage validates actual image bytes and strips metadata
// by decoding and re-encoding the image into a supported storage format.
func NormalizeSupportTicketImage(raw []byte) (SupportTicketAttachmentInput, error) {
	if len(raw) == 0 {
		return SupportTicketAttachmentInput{}, ErrSupportTicketImageInvalid
	}
	if len(raw) > SupportTicketMaxImageBytes {
		return SupportTicketAttachmentInput{}, ErrSupportTicketImageTooLarge
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || !validSupportTicketImageDimensions(cfg.Width, cfg.Height) {
		return SupportTicketAttachmentInput{}, ErrSupportTicketImageInvalid
	}
	if format != "jpeg" && format != "png" && format != "webp" {
		return SupportTicketAttachmentInput{}, ErrSupportTicketImageInvalid
	}

	var decoded image.Image
	switch format {
	case "webp":
		decoded, err = webp.Decode(bytes.NewReader(raw))
	default:
		decoded, _, err = image.Decode(bytes.NewReader(raw))
	}
	if err != nil {
		return SupportTicketAttachmentInput{}, ErrSupportTicketImageInvalid
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != cfg.Width || bounds.Dy() != cfg.Height || !validSupportTicketImageDimensions(bounds.Dx(), bounds.Dy()) {
		return SupportTicketAttachmentInput{}, ErrSupportTicketImageInvalid
	}

	var normalized bytes.Buffer
	contentType := "image/png"
	if format == "jpeg" {
		contentType = "image/jpeg"
		err = jpeg.Encode(&normalized, decoded, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(&normalized, decoded)
	}
	if err != nil {
		return SupportTicketAttachmentInput{}, ErrSupportTicketImageInvalid
	}
	if normalized.Len() > SupportTicketMaxNormalizedBytes {
		return SupportTicketAttachmentInput{}, ErrSupportTicketImageTooLarge
	}

	width, height := bounds.Dx(), bounds.Dy()
	return SupportTicketAttachmentInput{
		ContentType: contentType,
		Data:        normalized.Bytes(),
		Width:       &width,
		Height:      &height,
	}, nil
}

func validSupportTicketImageDimensions(width, height int) bool {
	return width > 0 && height > 0 && int64(width) <= SupportTicketMaxImagePixels/int64(height)
}
