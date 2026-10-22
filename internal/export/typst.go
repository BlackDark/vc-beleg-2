// Package export renders claim PDFs with Typst and builds the evidence ZIP.
package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// Typst runs the Typst CLI with a deadline, fixed templates and no network.
type Typst struct {
	Bin     string
	FontDir string
	Timeout time.Duration
}

// Compile compiles main.typ in dir to out. format is "pdf" or "png".
func (t *Typst) Compile(ctx context.Context, dir, main, out, format string, ppi int) error {
	timeout := t.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"compile", "--root", dir, "--format", format}
	if t.FontDir != "" {
		args = append(args, "--font-path", t.FontDir, "--ignore-system-fonts")
	}
	if format == "png" {
		args = append(args, "--ppi", strconv.Itoa(ppi), "--pages", "1")
	}
	args = append(args, filepath.Join(dir, main), out)
	cmd := exec.CommandContext(ctx, t.Bin, args...)
	cmd.Env = []string{"TYPST_PACKAGE_PATH=" + filepath.Join(dir, ".no-packages"), "HOME=" + dir}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("typst: timeout")
		}
		return &CompileError{Output: stderr.String()}
	}
	return nil
}

// CompileError carries Typst's diagnostics.
type CompileError struct{ Output string }

func (e *CompileError) Error() string { return "typst: " + e.Output }

var pageHint = regexp.MustCompile(`only has (\d+) page`)

// PDFInfo probes a PDF with Typst: it returns the page count or an error
// for encrypted, malformed or unsupported files.
func (t *Typst) PDFInfo(ctx context.Context, pdf []byte) (int, error) {
	dir, err := os.MkdirTemp("", "probe-*")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "in.pdf"), pdf, 0o600); err != nil {
		return 0, err
	}
	src := "#set page(width: auto, height: auto, margin: 0pt)\n#image(\"in.pdf\", page: 100000)\n"
	if err := os.WriteFile(filepath.Join(dir, "main.typ"), []byte(src), 0o600); err != nil {
		return 0, err
	}
	err = t.Compile(ctx, dir, "main.typ", filepath.Join(dir, "out.pdf"), "pdf", 0)
	var ce *CompileError
	if errors.As(err, &ce) {
		if m := pageHint.FindStringSubmatch(ce.Output); m != nil {
			return strconv.Atoi(m[1])
		}
		return 0, fmt.Errorf("unreadable PDF")
	}
	if err != nil {
		return 0, err
	}
	return 100000, nil
}

// Preview renders the first page of an image or PDF as a PNG of the given
// width in points. Rendering strips metadata such as EXIF/GPS.
func (t *Typst) Preview(ctx context.Context, file []byte, ext string, widthPt int) ([]byte, error) {
	dir, err := os.MkdirTemp("", "preview-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	name := "in" + ext
	if err := os.WriteFile(filepath.Join(dir, name), file, 0o600); err != nil {
		return nil, err
	}
	page := ""
	if ext == ".pdf" {
		page = ", page: 1"
	}
	src := fmt.Sprintf("#set page(width: auto, height: auto, margin: 0pt)\n#image(%q, width: %dpt%s)\n", name, widthPt, page)
	if err := os.WriteFile(filepath.Join(dir, "main.typ"), []byte(src), 0o600); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, "out.png")
	if err := t.Compile(ctx, dir, "main.typ", out, "png", 96); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}
