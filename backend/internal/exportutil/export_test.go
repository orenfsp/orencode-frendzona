package exportutil

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func zipFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("файл не является корректным zip/OOXML: %v", err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func TestXLSXContainsRowsAndEscapesXML(t *testing.T) {
	data, err := XLSX("Численность", [][]string{{"Год", "Организация"}, {"2026", `ОГУ <"Лицей"> & Ко`}})
	if err != nil {
		t.Fatal(err)
	}
	files := zipFiles(t, data)
	if _, ok := files["[Content_Types].xml"]; !ok {
		t.Fatal("в xlsx нет [Content_Types].xml")
	}
	all := strings.Join(mapValues(files), "")
	for _, want := range []string{"Организация", "2026", "&lt;", "&amp;"} {
		if !strings.Contains(all, want) {
			t.Errorf("в xlsx не найдено %q", want)
		}
	}
	if strings.Contains(all, `<"Лицей">`) {
		t.Error("спецсимволы XML не экранированы")
	}
}

func TestDOCXContainsParagraphs(t *testing.T) {
	data, err := DOCX("Справка", []string{"Первая строка", "Вторая & строка"})
	if err != nil {
		t.Fatal(err)
	}
	files := zipFiles(t, data)
	doc, ok := files["word/document.xml"]
	if !ok {
		t.Fatal("в docx нет word/document.xml")
	}
	for _, want := range []string{"Справка", "Первая строка", "Вторая &amp; строка"} {
		if !strings.Contains(doc, want) {
			t.Errorf("в docx не найдено %q", want)
		}
	}
}

func TestFileName(t *testing.T) {
	if got := FileName("protocol", 7); got != "protocol-7.xlsx" {
		t.Fatalf("получили %q", got)
	}
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
