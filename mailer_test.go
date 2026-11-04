package smtp

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"

	"github.com/go-mailx/mailx"
	gomail "github.com/wneessen/go-mail"
)

// mimePart is a parsed MIME part. Multipart parts have children, leaf parts
// have a decoded body.
type mimePart struct {
	mediaType string
	header    map[string][]string
	body      []byte
	children  []*mimePart
}

func parseMessage(t *testing.T, raw []byte) *mimePart {
	t.Helper()
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	return parsePart(t, msg.Header, msg.Body)
}

func parsePart(t *testing.T, header map[string][]string, body io.Reader) *mimePart {
	t.Helper()
	contentType := firstHeader(header, "Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse Content-Type %q: %v", contentType, err)
	}
	p := &mimePart{mediaType: mediaType, header: header}
	if strings.HasPrefix(mediaType, "multipart/") {
		r := multipart.NewReader(body, params["boundary"])
		for {
			child, err := r.NextRawPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("read part of %s: %v", mediaType, err)
			}
			p.children = append(p.children, parsePart(t, child.Header, child))
		}
		return p
	}
	switch strings.ToLower(firstHeader(header, "Content-Transfer-Encoding")) {
	case "base64":
		body = base64.NewDecoder(base64.StdEncoding, body)
	case "quoted-printable":
		body = quotedprintable.NewReader(body)
	}
	if p.body, err = io.ReadAll(body); err != nil {
		t.Fatalf("read body of %s: %v", mediaType, err)
	}
	return p
}

func firstHeader(header map[string][]string, key string) string {
	for k, v := range header {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func (p *mimePart) find(mediaType string) *mimePart {
	if p.mediaType == mediaType {
		return p
	}
	for _, c := range p.children {
		if found := c.find(mediaType); found != nil {
			return found
		}
	}
	return nil
}

func newTestMail(t *testing.T) *mailInstance {
	t.Helper()
	m := &mailInstance{msg: gomail.NewMsg()}
	if err := m.From("sender@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := m.To([]string{"recipient@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Subject("test"); err != nil {
		t.Fatal(err)
	}
	return m
}

func writeTestMail(t *testing.T, m *mailInstance) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := m.msg.WriteTo(&buf); err != nil {
		t.Fatalf("write message: %v", err)
	}
	return buf.Bytes()
}

const (
	htmlBody = `<p>Hello <img src="cid:logo"></p>`
	textBody = "Hello world"
)

func assertAlternative(t *testing.T, root *mimePart) {
	t.Helper()
	alt := root.find("multipart/alternative")
	if alt == nil {
		t.Fatalf("no multipart/alternative part, top-level type is %s", root.mediaType)
	}
	if len(alt.children) != 2 {
		t.Fatalf("multipart/alternative has %d parts, want 2", len(alt.children))
	}
	text, html := alt.children[0], alt.children[1]
	if text.mediaType != "text/plain" || html.mediaType != "text/html" {
		t.Fatalf("multipart/alternative parts are %s, %s; want text/plain, text/html",
			text.mediaType, html.mediaType)
	}
	if string(text.body) != textBody {
		t.Errorf("text body = %q, want %q", text.body, textBody)
	}
	if string(html.body) != htmlBody {
		t.Errorf("HTML body = %q, want %q", html.body, htmlBody)
	}
}

func TestHTMLAndTextBody(t *testing.T) {
	m := newTestMail(t)
	if err := m.HtmlBody(htmlBody); err != nil {
		t.Fatal(err)
	}
	if err := m.TextBody(textBody); err != nil {
		t.Fatal(err)
	}
	assertAlternative(t, parseMessage(t, writeTestMail(t, m)))
}

func TestTextAndHTMLBody(t *testing.T) {
	m := newTestMail(t)
	if err := m.TextBody(textBody); err != nil {
		t.Fatal(err)
	}
	if err := m.HtmlBody(htmlBody); err != nil {
		t.Fatal(err)
	}
	assertAlternative(t, parseMessage(t, writeTestMail(t, m)))
}

func TestSingleBody(t *testing.T) {
	tests := map[string]struct {
		set       func(*mailInstance, string) error
		body      string
		mediaType string
	}{
		"html": {(*mailInstance).HtmlBody, htmlBody, "text/html"},
		"text": {(*mailInstance).TextBody, textBody, "text/plain"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			m := newTestMail(t)
			if err := tt.set(m, tt.body); err != nil {
				t.Fatal(err)
			}
			root := parseMessage(t, writeTestMail(t, m))
			if root.mediaType != tt.mediaType {
				t.Fatalf("top-level type = %s, want %s", root.mediaType, tt.mediaType)
			}
			if string(root.body) != tt.body {
				t.Errorf("body = %q, want %q", root.body, tt.body)
			}
		})
	}
}

var (
	logoPNG = []byte("\x89PNG\r\n\x1a\nnot really a png")
	logo    = mailx.InlineAttachment{ContentID: "logo", ContentType: "image/png", Data: logoPNG}
)

func assertInlineLogo(t *testing.T, related *mimePart) {
	t.Helper()
	img := related.find("image/png")
	if img == nil {
		t.Fatal("multipart/related has no image/png part")
	}
	if got := firstHeader(img.header, "Content-ID"); got != "<logo>" {
		t.Errorf("Content-ID = %q, want %q", got, "<logo>")
	}
	disposition, params, err := mime.ParseMediaType(firstHeader(img.header, "Content-Disposition"))
	if err != nil {
		t.Fatalf("parse Content-Disposition: %v", err)
	}
	if disposition != "inline" {
		t.Errorf("Content-Disposition = %q, want inline", disposition)
	}
	if params["filename"] != "logo.png" {
		t.Errorf("filename = %q, want logo.png", params["filename"])
	}
	if !bytes.Equal(img.body, logoPNG) {
		t.Errorf("image data = %q, want %q", img.body, logoPNG)
	}
}

func findRelated(t *testing.T, root *mimePart) *mimePart {
	t.Helper()
	related := root.find("multipart/related")
	if related == nil {
		t.Fatalf("no multipart/related part, top-level type is %s", root.mediaType)
	}
	return related
}

func TestInlineAttachmentWithHTMLBody(t *testing.T) {
	m := newTestMail(t)
	if err := m.HtmlBody(htmlBody); err != nil {
		t.Fatal(err)
	}
	if err := m.AddInlineAttachment(logo); err != nil {
		t.Fatal(err)
	}
	related := findRelated(t, parseMessage(t, writeTestMail(t, m)))

	html := related.find("text/html")
	if html == nil {
		t.Fatal("multipart/related has no text/html part")
	}
	if string(html.body) != htmlBody {
		t.Errorf("HTML body = %q, want %q", html.body, htmlBody)
	}
	assertInlineLogo(t, related)
}

func TestInlineAttachmentWithHTMLAndTextBody(t *testing.T) {
	m := newTestMail(t)
	if err := m.HtmlBody(htmlBody); err != nil {
		t.Fatal(err)
	}
	if err := m.TextBody(textBody); err != nil {
		t.Fatal(err)
	}
	if err := m.AddInlineAttachment(logo); err != nil {
		t.Fatal(err)
	}
	related := findRelated(t, parseMessage(t, writeTestMail(t, m)))

	assertAlternative(t, related)
	assertInlineLogo(t, related)
}

func TestInlineFileName(t *testing.T) {
	tests := map[string]string{
		"image/png":                 "logo.png",
		"image/jpeg":                "logo.jpg",
		"image/png; name=\"x.png\"": "logo.png",
		"application/x-unknown-foo": "logo",
		"not a media type;;":        "logo",
	}
	for contentType, want := range tests {
		got := inlineFileName(mailx.InlineAttachment{ContentID: "logo", ContentType: contentType})
		if got != want {
			t.Errorf("inlineFileName(%q) = %q, want %q", contentType, got, want)
		}
	}
}
