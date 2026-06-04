package handlers

import (
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
)

// readUpload reads a multipart file fully and returns its bytes, content-type
// and extension (without leading dot).
func readUpload(fh *multipart.FileHeader) (content []byte, contentType, ext string, err error) {
	f, err := fh.Open()
	if err != nil {
		return nil, "", "", err
	}
	defer f.Close()

	content, err = io.ReadAll(f)
	if err != nil {
		return nil, "", "", err
	}

	contentType = fh.Header.Get("Content-Type")
	ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(fh.Filename)), ".")
	return content, contentType, ext, nil
}
