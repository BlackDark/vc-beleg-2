package export

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // decode Typst's PNG output
	"os"
	"path/filepath"

	"github.com/blackdark/vc-beleg-2/internal/calc"
	"github.com/blackdark/vc-beleg-2/internal/money"
)

//go:embed templates/month.typ
var templates embed.FS

//go:embed fonts/*.ttf
var fonts embed.FS

// ExtractFonts writes the bundled fonts to dir for Typst's --font-path.
func ExtractFonts(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	entries, err := fonts.ReadDir("fonts")
	if err != nil {
		return err
	}
	for _, e := range entries {
		b, err := fonts.ReadFile("fonts/" + e.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o640); err != nil {
			return err
		}
	}
	return nil
}

// Snapshot freezes everything a closed month's report shows.
type Snapshot struct {
	Version      int             `json:"version"`
	Month        string          `json:"month"`
	GeneratedAt  string          `json:"generatedAt"`
	ReopenReason string          `json:"reopenReason"`
	Company      SnapshotCompany `json:"company"`
	EmployeeName string          `json:"employeeName"`
	Result       calc.Result     `json:"result"`
	Receipts     []SnapReceipt   `json:"receipts"`
}

// SnapshotCompany is the company as it was when the month was closed.
type SnapshotCompany struct {
	Name        string `json:"name"`
	Address     string `json:"address"`
	State       string `json:"state"`
	ChurchTaxBP int64  `json:"churchTaxBp"`
	LegalBasis  string `json:"legalBasis"`
}

// SnapReceipt is a receipt of the month with its checksum.
type SnapReceipt struct {
	ID       int64       `json:"id"`
	Number   string      `json:"number"`
	Date     string      `json:"date"`
	Merchant string      `json:"merchant"`
	Total    money.Cents `json:"total"`
	Filename string      `json:"filename"`
	Mime     string      `json:"mime"`
	Pages    int         `json:"pages"`
	SHA256   string      `json:"sha256"`
	Appended bool        `json:"appended" doc:"belongs to a counted day and is printed in the appendix"`
	File     string      `json:"file,omitempty" doc:"file name inside the Typst project"`
}

// RenderMonth compiles the report PDF. files holds the originals of the
// appended receipts by receipt ID.
func RenderMonth(ctx context.Context, t *Typst, s Snapshot, files map[int64][]byte) ([]byte, error) {
	dir, err := os.MkdirTemp("", "report-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	for i := range s.Receipts {
		r := &s.Receipts[i]
		data, ok := files[r.ID]
		if !r.Appended || !ok {
			continue
		}
		if r.Mime == "application/pdf" {
			r.File = fmt.Sprintf("r%d.pdf", r.ID)
		} else {
			// Re-rendered images carry no EXIF data (GPS positions).
			if data, err = t.reportImage(ctx, data, extOf(r.Mime)); err != nil {
				return nil, fmt.Errorf("receipt %s: %w", r.Number, err)
			}
			r.File = fmt.Sprintf("r%d.jpg", r.ID)
		}
		if err := os.WriteFile(filepath.Join(dir, r.File), data, 0o600); err != nil {
			return nil, err
		}
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "data.json"), b, 0o600); err != nil {
		return nil, err
	}
	tpl, err := templates.ReadFile("templates/month.typ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "main.typ"), tpl, 0o600); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, "out.pdf")
	if err := t.Compile(ctx, dir, "main.typ", out, "pdf", 0); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

func extOf(mime string) string {
	if mime == "image/png" {
		return ".png"
	}
	return ".jpg"
}

// reportImage renders an image upright at print resolution and re-encodes
// it as JPEG, which keeps the report small.
func (t *Typst) reportImage(ctx context.Context, data []byte, ext string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "image-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "in"+ext), data, 0o600); err != nil {
		return nil, err
	}
	src := fmt.Sprintf("#set page(width: auto, height: auto, margin: 0pt)\n#image(%q, width: 420pt)\n", "in"+ext)
	if err := os.WriteFile(filepath.Join(dir, "main.typ"), []byte(src), 0o600); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, "out.png")
	if err := t.Compile(ctx, dir, "main.typ", out, "png", 160); err != nil {
		return nil, err
	}
	png, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
