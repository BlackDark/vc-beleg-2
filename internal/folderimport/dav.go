package folderimport

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// Entry is a file or folder in the watched folder.
type Entry struct {
	Href    string // absolute URL, escaped
	Name    string // decoded base name
	Size    int64  // -1 when the server does not report it
	ModTime time.Time
	IsDir   bool
}

// Dav is a minimal WebDAV client. Servers differ in which properties they
// return (Bitrix24 omits getcontentlength on some entries), so missing
// properties are tolerated instead of failing the whole listing.
type Dav struct {
	client    *http.Client
	base      *url.URL // the watched folder, with a trailing slash
	username  string
	password  string
	userAgent string
}

// DefaultUserAgent names a known WebDAV client. Bitrix24 serves its web
// portal page instead of the file to clients it does not recognise.
const DefaultUserAgent = "Microsoft-WebDAV-MiniRedir/10.0.19045 (Mahlzeit)"

// NewDav creates a client for the folder at rawURL; an empty userAgent
// takes DefaultUserAgent.
func NewDav(rawURL, username, password, userAgent string) (*Dav, error) {
	u, err := url.Parse(strings.TrimRight(rawURL, "/") + "/")
	if err != nil {
		return nil, err
	}
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	return &Dav{client: &http.Client{Timeout: 2 * time.Minute}, base: u, username: username, password: password, userAgent: userAgent}, nil
}

const propfindBody = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:getcontentlength/><d:getlastmodified/></d:prop></d:propfind>`

type multistatus struct {
	Responses []struct {
		Href     string `xml:"href"`
		Propstat []struct {
			Status string `xml:"status"`
			Prop   struct {
				ResourceType struct {
					Collection *struct{} `xml:"collection"`
				} `xml:"resourcetype"`
				Length   string `xml:"getcontentlength"`
				Modified string `xml:"getlastmodified"`
			} `xml:"prop"`
		} `xml:"propstat"`
	} `xml:"response"`
}

// List returns the entries directly in the folder, without the folder itself.
func (d *Dav) List(ctx context.Context) ([]Entry, error) {
	req, err := d.request(ctx, "PROPFIND", d.base.String(), strings.NewReader(propfindBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	resp, err := d.do(req, http.StatusMultiStatus)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var ms multistatus
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&ms); err != nil {
		return nil, fmt.Errorf("unexpected PROPFIND answer: %w", err)
	}
	var out []Entry
	for _, r := range ms.Responses {
		href, err := d.base.Parse(strings.TrimSpace(r.Href))
		if err != nil || !d.inside(href) {
			continue // unparsable, the folder itself, or elsewhere
		}
		e := Entry{Href: href.String(), Name: path.Base(strings.TrimRight(href.Path, "/")), Size: -1, IsDir: strings.HasSuffix(href.Path, "/")}
		for _, ps := range r.Propstat {
			if !strings.Contains(ps.Status, " 200") {
				continue // properties the server does not have
			}
			if ps.Prop.ResourceType.Collection != nil {
				e.IsDir = true
			}
			if n, err := strconv.ParseInt(strings.TrimSpace(ps.Prop.Length), 10, 64); err == nil {
				e.Size = n
			}
			if t, err := http.ParseTime(strings.TrimSpace(ps.Prop.Modified)); err == nil {
				e.ModTime = t
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// Open fetches a file.
func (d *Dav) Open(ctx context.Context, href string) (io.ReadCloser, error) {
	req, err := d.request(ctx, http.MethodGet, href, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.do(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	// A login or error page instead of the file means the download failed;
	// the file must stay in the folder.
	if strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		resp.Body.Close()
		return nil, errors.New("GET: the server answered with a web page instead of the file (login or redirect?)")
	}
	return resp.Body, nil
}

// Mkdir creates a subfolder of the watched folder; an existing one is fine.
func (d *Dav) Mkdir(ctx context.Context, name string) error {
	req, err := d.request(ctx, "MKCOL", d.child(name+"/"), nil)
	if err != nil {
		return err
	}
	resp, err := d.do(req, http.StatusCreated, http.StatusMethodNotAllowed)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Move moves a file to dest, a path relative to the watched folder, without
// overwriting.
func (d *Dav) Move(ctx context.Context, href, dest string) error {
	req, err := d.request(ctx, "MOVE", href, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Destination", d.child(dest))
	req.Header.Set("Overwrite", "F")
	resp, err := d.do(req, http.StatusCreated, http.StatusNoContent)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// Delete removes a file.
func (d *Dav) Delete(ctx context.Context, href string) error {
	req, err := d.request(ctx, http.MethodDelete, href, nil)
	if err != nil {
		return err
	}
	resp, err := d.do(req, http.StatusOK, http.StatusNoContent)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// inside reports whether href is an entry of the watched folder. Requests
// carry the credentials, so an answer must not point them anywhere else.
func (d *Dav) inside(href *url.URL) bool {
	return href.Scheme == d.base.Scheme && href.Host == d.base.Host &&
		strings.HasPrefix(href.Path, d.base.Path) && strings.TrimRight(href.Path, "/") != strings.TrimRight(d.base.Path, "/")
}

// child is the escaped URL of a path below the watched folder.
func (d *Dav) child(rel string) string {
	return d.base.ResolveReference(&url.URL{Path: strings.TrimLeft(rel, "/")}).String()
}

func (d *Dav) request(ctx context.Context, method, target string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", d.userAgent)
	if d.username != "" || d.password != "" {
		req.SetBasicAuth(d.username, d.password)
	}
	return req, nil
}

// do sends req and accepts only the given status codes; error messages carry
// the status, never the body.
func (d *Dav) do(req *http.Request, ok ...int) (*http.Response, error) {
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	for _, s := range ok {
		if resp.StatusCode == s {
			return resp, nil
		}
	}
	resp.Body.Close()
	return nil, errors.New(req.Method + ": " + resp.Status)
}
