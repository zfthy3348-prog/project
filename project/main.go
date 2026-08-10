package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type GalleryItem struct {
	ImageURL string
	Alt      string
}

type Version struct {
	Year      string
	Version   string
	Size      string
	Notes     string
	AppURL    string
	ZipURL    string
	Published bool
}

type PageData struct {
	AppIconURL    string
	AppName       string
	Tagline       string
	AppSize       string
	AppVersion    string
	DownloadURL   string
	SiteURL       string
	SupportLink   string
	VideoURL      string
	Gallery       []GalleryItem
	Versions      []Version
	PremiumZipURL string
	PremiumAppURL string
	PremiumPrice  string
	PremiumLabel  string
	CSPNonce      string
}

func main() {
	cfg := loadConfig()

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		nonce, err := newNonce()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		data := cfg
		data.CSPNonce = nonce
		setSecurityHeaders(w, nonce)

		if err := tmpl.Execute(w, data); err != nil {
			log.Printf("template execution failed: %v", err)
		}
	})

	port := os.Getenv("PORT")
	if !validPort(port) {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           securityMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}

	log.Printf("server listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}

func loadConfig() PageData {
	appIcon := safeHTTPSURL(os.Getenv("APP_ICON_URL"), "")
	gallery := parseGallery(os.Getenv("GALLERY_URLS"))
	versions := parseVersions(os.Getenv("VERSIONS_JSON"))

	if len(gallery) == 0 && appIcon != "" {
		gallery = []GalleryItem{{ImageURL: appIcon, Alt: "Biscelor"}}
	}

	if len(versions) == 0 {
		versions = []Version{{
			Year:      "2026",
			Version:   cleanVersion(os.Getenv("APP_VERSION")),
			Size:      cleanText(os.Getenv("APP_SIZE"), 32, "—"),
			Notes:     "الإصدار الحالي",
			AppURL:    safeHTTPSURL(os.Getenv("DOWNLOAD_URL"), ""),
			ZipURL:    "",
			Published: true,
		}}
	}

	return PageData{
		AppIconURL:    appIcon,
		AppName:       cleanText(os.Getenv("APP_NAME"), 80, "Biscelor"),
		Tagline:       cleanText(os.Getenv("APP_TAGLINE"), 160, "نظام التصميم الاحترافي"),
		AppSize:       cleanText(os.Getenv("APP_SIZE"), 32, "—"),
		AppVersion:    cleanVersion(os.Getenv("APP_VERSION")),
		DownloadURL:   safeHTTPSURL(os.Getenv("DOWNLOAD_URL"), ""),
		SiteURL:       safeHTTPSURL(os.Getenv("SITE_URL"), ""),
		SupportLink:   safeSupportURL(os.Getenv("SUPPORT_LINK")),
		VideoURL:      safeHTTPSURL(os.Getenv("VIDEO_URL"), ""),
		Gallery:       gallery,
		Versions:      versions,
		PremiumZipURL: safeHTTPSURL(os.Getenv("PREMIUM_ZIP_URL"), ""),
		PremiumAppURL: safeHTTPSURL(os.Getenv("PREMIUM_APP_URL"), ""),
		PremiumPrice:  cleanText(os.Getenv("PREMIUM_PRICE"), 40, "50%"),
		PremiumLabel:  cleanText(os.Getenv("PREMIUM_LABEL"), 80, "الباقة المميزة"),
	}
}

func parseGallery(value string) []GalleryItem {
	var out []GalleryItem
	for _, raw := range strings.Split(value, ",") {
		u := safeHTTPSURL(strings.TrimSpace(raw), "")
		if u != "" {
			out = append(out, GalleryItem{ImageURL: u, Alt: "لقطة من تطبيق Biscelor"})
		}
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func parseVersions(value string) []Version {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	var raw []struct {
		Year      string `json:"year"`
		Version   string `json:"version"`
		Size      string `json:"size"`
		Notes     string `json:"notes"`
		AppURL    string `json:"app_url"`
		ZipURL    string `json:"zip_url"`
		Published bool   `json:"published"`
	}
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		log.Printf("invalid VERSIONS_JSON: %v", err)
		return nil
	}

	out := make([]Version, 0, len(raw))
	for _, item := range raw {
		year := cleanYear(item.Year)
		if year == "" {
			continue
		}
		out = append(out, Version{
			Year:      year,
			Version:   cleanVersion(item.Version),
			Size:      cleanText(item.Size, 32, "—"),
			Notes:     cleanText(item.Notes, 240, ""),
			AppURL:    safeHTTPSURL(item.AppURL, ""),
			ZipURL:    safeHTTPSURL(item.ZipURL, ""),
			Published: item.Published,
		})
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func cleanYear(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 4 {
		return ""
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	n, _ := strconv.Atoi(value)
	if n < 2000 || n > 2100 {
		return ""
	}
	return value
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.IndexFunc(r.URL.RequestURI(), func(r rune) bool {
			return r < 0x20 || r == 0x7f
		}) >= 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setSecurityHeaders(w http.ResponseWriter, nonce string) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; "+
			"base-uri 'none'; "+
			"object-src 'none'; "+
			"frame-ancestors 'none'; "+
			"form-action 'none'; "+
			"connect-src 'none'; "+
			"font-src 'self'; "+
			"img-src https: data:; "+
			"media-src https:; "+
			"style-src 'nonce-"+nonce+"'; "+
			"script-src 'nonce-"+nonce+"'")

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func newNonce() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

func safeHTTPSURL(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	u, err := url.ParseRequestURI(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fallback
	}
	if strings.ContainsAny(value, "\r\n\t<>\"'`") {
		return fallback
	}
	return u.String()
}

func safeSupportURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "#"
	}
	if strings.ContainsAny(value, "\r\n\t<>\"'`") {
		return "#"
	}
	if strings.HasPrefix(value, "mailto:") {
		addr := strings.TrimPrefix(value, "mailto:")
		if strings.ContainsAny(addr, "?#%") || !strings.Contains(addr, "@") {
			return "#"
		}
		return "mailto:" + addr
	}
	return safeHTTPSURL(value, "#")
}

func cleanText(value string, max int, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if len(value) > max {
		value = value[:max]
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}

func cleanVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "—"
	}
	if len(value) > 32 {
		return "—"
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && r != '.' && r != '-' && r != '_' {
			return "—"
		}
	}
	return value
}

func validPort(value string) bool {
	if value == "" || len(value) > 5 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	n, _ := strconv.Atoi(value)
	return n >= 1 && n <= 65535
}
