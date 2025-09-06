package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"

	"github.com/buckket/go-blurhash"
	"github.com/disintegration/imaging"
	"maunium.net/go/mautrix"
)

// MatrixImageInfo is a struct for Matrix image info
type MatrixImageInfo struct {
	Mimetype      string                 `json:"mimetype,omitempty"`
	Size          int                    `json:"size,omitempty"`
	W             int                    `json:"w,omitempty"`
	H             int                    `json:"h,omitempty"`
	ThumbnailURL  string                 `json:"thumbnail_url,omitempty"`
	ThumbnailInfo *MatrixImageInfo       `json:"thumbnail_info,omitempty"`
	Additional    map[string]interface{} `json:"-"`
}

// downloadImage downloads an image from a URL and returns the image.Image, its bytes, and format
func downloadImage(url string) (image.Image, []byte, string, error) {
	log.Printf("Attempting to download image: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("HTTP request error: %v", err)
		return nil, nil, "", err
	}
	defer resp.Body.Close()

	//log.Printf("HTTP Status: %s", resp.Status)
	contentType := resp.Header.Get("Content-Type")
	//log.Printf("Content-Type: %s", contentType)

	imgBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body: %v", err)
		return nil, nil, "", err
	}
	if len(imgBytes) < 32 {
		log.Printf("Image bytes too short: %d", len(imgBytes))
	}
	// Print first 16 bytes as hex for debugging
	//log.Printf("First 16 bytes: %x", imgBytes[:min(16, len(imgBytes))])

	// Try generic image.Decode
	img, format, err := image.Decode(bytes.NewReader(imgBytes))
	if err == nil {
		//log.Printf("Decoded using image.Decode, format: %s", format)
		return img, imgBytes, format, nil
	}
	log.Printf("image.Decode failed: %v", err)

	// Skip WebP for now due to CGO dependencies

	// Try JPEG
	img, errJpeg := jpeg.Decode(bytes.NewReader(imgBytes))
	if errJpeg == nil {
		log.Printf("Decoded using jpeg.Decode")
		return img, imgBytes, "jpeg", nil
	}
	log.Printf("jpeg.Decode failed: %v", errJpeg)

	// Try PNG
	img, errPng := png.Decode(bytes.NewReader(imgBytes))
	if errPng == nil {
		log.Printf("Decoded using png.Decode")
		return img, imgBytes, "png", nil
	}
	log.Printf("png.Decode failed: %v", errPng)

	// All decoders failed
	return nil, nil, "", err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// generateThumbnail resizes the image to the given width and height
func generateThumbnail(img image.Image, width, height int) image.Image {
	return imaging.Resize(img, width, height, imaging.Lanczos)
}

// encodeImage encodes an image.Image to bytes in the given format
func encodeImage(img image.Image, format string) ([]byte, error) {
	buf := new(bytes.Buffer)
	switch format {
	case "jpeg":
		if err := jpeg.Encode(buf, img, nil); err != nil {
			return nil, err
		}
	case "png":
		if err := png.Encode(buf, img); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	return buf.Bytes(), nil
}

// calcBlurhash calculates the blurhash for an image.Image
func calcBlurhash(img image.Image) (string, error) {
	return blurhash.Encode(4, 3, img)
}

// uploadToMatrix uploads an image to Matrix and returns the MXC URL and info
func uploadToMatrix(client *mautrix.Client, filename string, imgBytes []byte, mimetype string, width, height int) (string, *MatrixImageInfo, error) {
	req := mautrix.ReqUploadMedia{
		ContentBytes: imgBytes,
		ContentType:  mimetype,
		FileName:     filename,
	}
	uploadResp, err := client.UploadMedia(req)
	if err != nil {
		return "", nil, err
	}
	info := &MatrixImageInfo{
		Mimetype: mimetype,
		Size:     len(imgBytes),
		W:        width,
		H:        height,
	}
	return uploadResp.ContentURI.String(), info, nil
}
