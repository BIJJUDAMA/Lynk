package user

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
)

func IsDOCX(content []byte) bool {
	if !bytes.HasPrefix(content, []byte("PK\x03\x04")) {
		return false
	}
	r, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return false
	}
	for _, f := range r.File {
		if f.Name != "[Content_Types].xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return false
		}
		body, err := io.ReadAll(io.LimitReader(rc, 1<<20))
		_ = rc.Close()
		if err != nil {
			return false
		}
		s := strings.ToLower(string(body))
		return strings.Contains(s, "wordprocessingml")
	}
	return false
}
