package user

import (
	"archive/zip"
	"bytes"
	"testing"
)

func zipWith(files map[string]string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			panic(err)
		}
	}
	_ = w.Close()
	return buf.Bytes()
}

func TestIsDOCX_RejectsPlainZip(t *testing.T) {
	plain := zipWith(map[string]string{"readme.txt": "hello"})
	if !bytes.HasPrefix(plain, []byte("PK\x03\x04")) {
		t.Fatal("fixture must look like ZIP")
	}
	if IsDOCX(plain) {
		t.Fatal("generic ZIP must not pass as DOCX")
	}
}

func TestIsDOCX_AcceptsContentTypesWordprocessingml(t *testing.T) {
	docx := zipWith(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"/>`,
	})
	if !IsDOCX(docx) {
		t.Fatal("expected OOXML wordprocessingml zip to pass")
	}
}
