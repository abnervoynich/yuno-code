package ingestion

import (
	"fmt"
	"io"
	"strings"

	"github.com/abnervoynich/yuno-code/backend/internal/domain"
)

// Parser knows how to parse a PSP settlement file.
type Parser interface {
	Parse(r io.Reader) ([]domain.SettlementRecord, error)
}

// DetectFormat returns the PSP name and format from the filename or content hint.
func DetectFormat(filename, pspNameHint string) (pspName, format string, err error) {
	lower := strings.ToLower(filename)
	pspName = strings.ToLower(pspNameHint)

	switch {
	case pspName == "pspa" || strings.Contains(lower, "pspa"):
		return "pspa", "csv", nil
	case pspName == "pspb" || strings.Contains(lower, "pspb"):
		return "pspb", "json", nil
	case pspName == "pspc" || strings.Contains(lower, "pspc"):
		return "pspc", "custom", nil
	}

	// Fallback: detect by extension
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return "pspa", "csv", nil
	case strings.HasSuffix(lower, ".json"):
		return "pspb", "json", nil
	case strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".pipe"):
		return "pspc", "custom", nil
	}

	return "", "", fmt.Errorf("cannot detect PSP format from filename %q and hint %q", filename, pspNameHint)
}

// NewParser returns the correct Parser for the given psp name.
func NewParser(pspName string) (Parser, error) {
	switch strings.ToLower(pspName) {
	case "pspa":
		return &PSPACSVParser{}, nil
	case "pspb":
		return &PSPBJSONParser{}, nil
	case "pspc":
		return &PSPCCustomParser{}, nil
	}
	return nil, fmt.Errorf("unknown PSP: %s", pspName)
}
