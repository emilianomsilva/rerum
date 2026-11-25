package receipt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Write(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var content strings.Builder
	content.WriteString("BT\n/F1 16 Tf\n50 790 Td\n(" + pdfText("RECIBO DE ALUGUEL") + ") Tj\n/F1 11 Tf\n")
	for _, line := range lines {
		content.WriteString("0 -28 Td\n(" + pdfText(line) + ") Tj\n")
	}
	content.WriteString("0 -50 Td\n(" + pdfText("Assinatura do locador: __________________________________") + ") Tj\nET\n")
	stream := content.String()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
	}
	var output bytes.Buffer
	output.WriteString("%PDF-1.4\n%RERUM\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, output.Len())
		fmt.Fprintf(&output, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := output.Len()
	fmt.Fprintf(&output, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&output, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&output, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return os.WriteFile(path, output.Bytes(), 0o600)
}

func pdfText(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '(':
			b.WriteString(`\(`)
		case ')':
			b.WriteString(`\)`)
		default:
			if r >= 32 && r < 127 {
				b.WriteRune(r)
			} else if code, ok := cp1252Rune(r); ok {
				fmt.Fprintf(&b, `\%03o`, code)
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

func cp1252Rune(r rune) (byte, bool) {
	switch r {
	case 'á':
		return 0xE1, true
	case 'à':
		return 0xE0, true
	case 'ã':
		return 0xE3, true
	case 'â':
		return 0xE2, true
	case 'é':
		return 0xE9, true
	case 'ê':
		return 0xEA, true
	case 'í':
		return 0xED, true
	case 'ó':
		return 0xF3, true
	case 'ô':
		return 0xF4, true
	case 'õ':
		return 0xF5, true
	case 'ú':
		return 0xFA, true
	case 'ü':
		return 0xFC, true
	case 'ç':
		return 0xE7, true
	case 'Á':
		return 0xC1, true
	case 'À':
		return 0xC0, true
	case 'Ã':
		return 0xC3, true
	case 'Â':
		return 0xC2, true
	case 'É':
		return 0xC9, true
	case 'Ê':
		return 0xCA, true
	case 'Í':
		return 0xCD, true
	case 'Ó':
		return 0xD3, true
	case 'Ô':
		return 0xD4, true
	case 'Õ':
		return 0xD5, true
	case 'Ú':
		return 0xDA, true
	case 'Ü':
		return 0xDC, true
	case 'Ç':
		return 0xC7, true
	case '–':
		return 0x96, true
	case '—':
		return 0x97, true
	case '«':
		return 0xAB, true
	case '»':
		return 0xBB, true
	case 'º':
		return 0xBA, true
	case 'ª':
		return 0xAA, true
	default:
		return 0, false
	}
}
