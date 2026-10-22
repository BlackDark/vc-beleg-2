package mailimport

import (
	"strings"
	"testing"
)

const sample = "From: Max <Max@Example.com>\r\n" +
	"To: belege@example.com\r\n" +
	"Subject: Mittag\r\n" +
	"Message-ID: <abc@example.com>\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: multipart/mixed; boundary=b1\r\n\r\n" +
	"--b1\r\nContent-Type: text/plain\r\n\r\nhallo\r\n" +
	"--b1\r\nContent-Type: image/jpeg\r\nContent-Disposition: inline; filename=\"IMG_1.jpg\"\r\nContent-Transfer-Encoding: base64\r\n\r\n/9j/4AAQ\r\n" +
	"--b1\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"bon.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0x\r\n" +
	"--b1\r\nContent-Type: application/zip\r\nContent-Disposition: attachment; filename=\"x.zip\"\r\n\r\nPK\r\n" +
	"--b1--\r\n"

func TestParse(t *testing.T) {
	m, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if m.From != "max@example.com" || m.Subject != "Mittag" || m.MessageID != "abc@example.com" {
		t.Fatalf("header: %+v", m)
	}
	if len(m.Attachments) != 2 || m.Attachments[0].Filename != "IMG_1.jpg" || m.Attachments[1].Filename != "bon.pdf" || !strings.HasPrefix(string(m.Attachments[1].Data), "%PDF-1") {
		t.Fatalf("attachments: %+v", m.Attachments)
	}
}

func TestAllowed(t *testing.T) {
	list := []string{"Max@Example.com", " other@example.com"}
	if !Allowed(list, "max@example.com") || !Allowed(list, "other@example.com") || Allowed(list, "eve@example.com") || Allowed(list, "") {
		t.Fatal("allowlist")
	}
}
