package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blackdark/vc-beleg-2/internal/api"
	"github.com/blackdark/vc-beleg-2/internal/app"
	"github.com/blackdark/vc-beleg-2/internal/folderimport"
	"github.com/blackdark/vc-beleg-2/internal/mailimport"
)

// Config is read from MAHLZEIT_* environment variables. Secrets can also be
// read from a file named by the variable with a _FILE suffix.
type Config struct {
	Listen         string
	DataDir        string
	BaseURL        string
	SecureCookies  bool
	SetupToken     string
	PasswordLogin  bool
	OIDC           *api.OIDCConfig
	Typst          string
	MaxUploadBytes int64
	MaxPixels      int64
	MaxPDFPages    int
	LogLevel       slog.Level
	Extract        ExtractConfig
	Mail           *mailimport.Config
	Folder         *folderimport.Config
}

func env(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return strings.TrimSpace(v)
	}
	return def
}

func secret(name string) (string, error) {
	if p := env(name+"_FILE", ""); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("%s_FILE: %w", name, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return env(name, ""), nil
}

func list(name string) []string {
	var out []string
	for _, s := range strings.Split(env(name, ""), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// LoadConfig reads and validates the configuration; invalid combinations
// fail at startup.
func LoadConfig() (*Config, error) {
	c := &Config{
		Listen:  env("MAHLZEIT_LISTEN", ":8080"),
		DataDir: env("MAHLZEIT_DATA_DIR", "/data"),
		BaseURL: strings.TrimRight(env("MAHLZEIT_BASE_URL", "http://localhost:8080"), "/"),
		Typst:   env("MAHLZEIT_TYPST", "typst"),
	}
	var errs []error
	u, err := url.Parse(c.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, errors.New("MAHLZEIT_BASE_URL must be an absolute http(s) URL"))
	} else {
		c.SecureCookies = u.Scheme == "https"
	}
	if c.SetupToken, err = secret("MAHLZEIT_SETUP_TOKEN"); err != nil {
		errs = append(errs, err)
	}
	c.PasswordLogin, err = strconv.ParseBool(env("MAHLZEIT_PASSWORD_LOGIN", "true"))
	if err != nil {
		errs = append(errs, errors.New("MAHLZEIT_PASSWORD_LOGIN must be true or false"))
	}
	if issuer := env("MAHLZEIT_OIDC_ISSUER", ""); issuer != "" {
		o := &api.OIDCConfig{
			Issuer: issuer, ClientID: env("MAHLZEIT_OIDC_CLIENT_ID", ""), Label: env("MAHLZEIT_OIDC_LABEL", "Pocket ID"),
			GroupsClaim: env("MAHLZEIT_OIDC_GROUPS_CLAIM", "groups"), Scopes: list("MAHLZEIT_OIDC_SCOPES"),
			Policy: app.OIDCPolicy{AllowedGroups: list("MAHLZEIT_OIDC_ALLOWED_GROUPS"), AllowedEmails: list("MAHLZEIT_OIDC_ALLOWED_EMAILS"), AllowedSubjects: list("MAHLZEIT_OIDC_ALLOWED_SUBJECTS")},
		}
		if o.ClientSecret, err = secret("MAHLZEIT_OIDC_CLIENT_SECRET"); err != nil {
			errs = append(errs, err)
		}
		if o.ClientID == "" {
			errs = append(errs, errors.New("MAHLZEIT_OIDC_CLIENT_ID is required with MAHLZEIT_OIDC_ISSUER"))
		}
		c.OIDC = o
	}
	if !c.PasswordLogin && c.OIDC == nil {
		errs = append(errs, errors.New("enable at least one login mode (MAHLZEIT_PASSWORD_LOGIN or MAHLZEIT_OIDC_ISSUER)"))
	}
	mb, err := strconv.Atoi(env("MAHLZEIT_MAX_UPLOAD_MB", "25"))
	if err != nil || mb < 1 || mb > 25 {
		errs = append(errs, errors.New("MAHLZEIT_MAX_UPLOAD_MB must be between 1 and 25"))
	}
	c.MaxUploadBytes = int64(mb) << 20
	px, err := strconv.ParseInt(env("MAHLZEIT_MAX_MEGAPIXELS", "40"), 10, 64)
	if err != nil || px < 1 || px > 40 {
		errs = append(errs, errors.New("MAHLZEIT_MAX_MEGAPIXELS must be between 1 and 40"))
	}
	c.MaxPixels = px * 1_000_000
	if c.MaxPDFPages, err = strconv.Atoi(env("MAHLZEIT_MAX_PDF_PAGES", "50")); err != nil || c.MaxPDFPages < 1 || c.MaxPDFPages > 50 {
		errs = append(errs, errors.New("MAHLZEIT_MAX_PDF_PAGES must be between 1 and 50"))
	}
	c.Extract, err = extractConfig()
	errs = append(errs, err)
	c.Mail, err = mailConfig()
	errs = append(errs, err)
	c.Folder, err = folderConfig(c.MaxUploadBytes)
	errs = append(errs, err)
	if err := c.LogLevel.UnmarshalText([]byte(env("MAHLZEIT_LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, errors.New("MAHLZEIT_LOG_LEVEL must be debug, info, warn or error"))
	}
	return c, errors.Join(errs...)
}

func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// ExtractConfig selects the LLM that reads receipts.
type ExtractConfig struct {
	Provider string // claude, gemini or openai
	Enabled  bool
	APIKey   string
	Model    string
	BaseURL  string
	Fallback bool // Claude only
}

// extractConfig reads the receipt reader settings. Claude without a key
// means no reading: date, total and merchant are typed by hand.
func extractConfig() (ExtractConfig, error) {
	x := ExtractConfig{
		Provider: env("MAHLZEIT_EXTRACT_PROVIDER", "claude"),
		Model:    env("MAHLZEIT_EXTRACT_MODEL", ""),
		BaseURL:  env("MAHLZEIT_EXTRACT_BASE_URL", ""),
	}
	var errs []error
	var err error
	if x.Fallback, err = strconv.ParseBool(env("MAHLZEIT_EXTRACT_FALLBACK", "true")); err != nil {
		errs = append(errs, errors.New("MAHLZEIT_EXTRACT_FALLBACK must be true or false"))
	}
	if x.BaseURL != "" {
		if u, err := url.Parse(x.BaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, errors.New("MAHLZEIT_EXTRACT_BASE_URL must be an absolute http(s) URL"))
		}
	}
	keyVar := map[string]string{"claude": "ANTHROPIC_API_KEY", "gemini": "GEMINI_API_KEY", "openai": "OPENAI_API_KEY"}[x.Provider]
	if keyVar == "" {
		return x, errors.Join(append(errs, errors.New("MAHLZEIT_EXTRACT_PROVIDER must be claude, gemini or openai"))...)
	}
	if x.APIKey, err = secret(keyVar); err != nil {
		errs = append(errs, err)
	}
	switch x.Provider {
	case "claude":
		x.Enabled = x.APIKey != ""
	case "gemini":
		x.Enabled = true
		if x.APIKey == "" {
			errs = append(errs, errors.New("GEMINI_API_KEY is required with MAHLZEIT_EXTRACT_PROVIDER=gemini"))
		}
	case "openai":
		// Local servers such as Ollama need no key.
		x.Enabled = true
		if x.Model == "" {
			errs = append(errs, errors.New("MAHLZEIT_EXTRACT_MODEL is required with MAHLZEIT_EXTRACT_PROVIDER=openai"))
		}
	}
	return x, errors.Join(errs...)
}

// mailConfig reads the IMAP settings; no host means no mail import.
func mailConfig() (*mailimport.Config, error) {
	host := env("MAHLZEIT_IMAP_HOST", "")
	if host == "" {
		return nil, nil
	}
	var errs []error
	m := &mailimport.Config{
		Host: host, Username: env("MAHLZEIT_IMAP_USERNAME", ""),
		Folder: env("MAHLZEIT_IMAP_FOLDER", "INBOX"), ProcessedFolder: env("MAHLZEIT_IMAP_PROCESSED_FOLDER", "Processed"),
		AllowedSenders: list("MAHLZEIT_IMAP_ALLOWED_SENDERS"),
	}
	var err error
	if m.Port, err = strconv.Atoi(env("MAHLZEIT_IMAP_PORT", "993")); err != nil || m.Port < 1 || m.Port > 65535 {
		errs = append(errs, errors.New("MAHLZEIT_IMAP_PORT must be a port number"))
	}
	if m.Password, err = secret("MAHLZEIT_IMAP_PASSWORD"); err != nil {
		errs = append(errs, err)
	}
	if m.Delete, err = strconv.ParseBool(env("MAHLZEIT_IMAP_DELETE", "false")); err != nil {
		errs = append(errs, errors.New("MAHLZEIT_IMAP_DELETE must be true or false"))
	}
	if m.Interval, err = time.ParseDuration(env("MAHLZEIT_IMAP_INTERVAL", "5m")); err != nil || m.Interval < time.Minute {
		errs = append(errs, errors.New("MAHLZEIT_IMAP_INTERVAL must be a duration of at least 1m"))
	}
	if m.Username == "" || m.Password == "" {
		errs = append(errs, errors.New("MAHLZEIT_IMAP_USERNAME and MAHLZEIT_IMAP_PASSWORD are required with MAHLZEIT_IMAP_HOST"))
	}
	if len(m.AllowedSenders) == 0 {
		errs = append(errs, errors.New("MAHLZEIT_IMAP_ALLOWED_SENDERS is required with MAHLZEIT_IMAP_HOST"))
	}
	return m, errors.Join(errs...)
}

// folderConfig reads the WebDAV folder settings; no URL means no folder
// import.
func folderConfig(maxBytes int64) (*folderimport.Config, error) {
	raw := env("MAHLZEIT_WEBDAV_URL", "")
	if raw == "" {
		return nil, nil
	}
	var errs []error
	f := &folderimport.Config{
		URL: strings.TrimRight(raw, "/"), Username: env("MAHLZEIT_WEBDAV_USERNAME", ""),
		ProcessedFolder: strings.Trim(env("MAHLZEIT_WEBDAV_PROCESSED_FOLDER", "Verarbeitet"), "/"), MaxBytes: maxBytes,
	}
	if u, err := url.Parse(f.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		errs = append(errs, errors.New("MAHLZEIT_WEBDAV_URL must be an absolute http(s) URL without credentials"))
	}
	var err error
	if f.Password, err = secret("MAHLZEIT_WEBDAV_PASSWORD"); err != nil {
		errs = append(errs, err)
	}
	if f.Delete, err = strconv.ParseBool(env("MAHLZEIT_WEBDAV_DELETE", "false")); err != nil {
		errs = append(errs, errors.New("MAHLZEIT_WEBDAV_DELETE must be true or false"))
	}
	if f.Interval, err = time.ParseDuration(env("MAHLZEIT_WEBDAV_INTERVAL", "5m")); err != nil || f.Interval < time.Minute {
		errs = append(errs, errors.New("MAHLZEIT_WEBDAV_INTERVAL must be a duration of at least 1m"))
	}
	if !f.Delete && (f.ProcessedFolder == "" || strings.Contains(f.ProcessedFolder, "..")) {
		errs = append(errs, errors.New("MAHLZEIT_WEBDAV_PROCESSED_FOLDER must name a subfolder"))
	}
	return f, errors.Join(errs...)
}
