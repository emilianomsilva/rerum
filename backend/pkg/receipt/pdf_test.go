package receipt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFTextAccents(t *testing.T) {
	sample := "Locação nº 01 – Imóvel Indaiá (IPTU quitado)"
	escaped := pdfText(sample)
	if strings.Contains(escaped, "Locacao") {
		t.Fatalf("accents were stripped unexpectedly: %s", escaped)
	}
	// Check that octal escapes for ã, ç, nº, etc. are generated
	if !strings.Contains(escaped, `\343`) || !strings.Contains(escaped, `\347`) {
		t.Fatalf("expected octal escapes for ã and ç, got: %s", escaped)
	}

	tempDir := t.TempDir()
	pdfFile := filepath.Join(tempDir, "recibo.pdf")
	lines := []string{
		"Locador: João Silva",
		"Locatário: Maria da Conceição",
		"Valor: R$ 1.500,00",
		"Referência: Aluguel de habitação",
	}
	if err := Write(pdfFile, lines); err != nil {
		t.Fatalf("Write PDF failed: %v", err)
	}
	content, err := os.ReadFile(pdfFile)
	if err != nil {
		t.Fatalf("read PDF failed: %v", err)
	}
	if !strings.HasPrefix(string(content), "%PDF-1.4") {
		t.Fatalf("invalid PDF header: %s", string(content[:20]))
	}
}
