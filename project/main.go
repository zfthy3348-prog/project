package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const fallbackIconURL = "data:image/svg+xml,%3Csvg xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22 width%3D%22200%22 height%3D%22200%22 viewBox%3D%220 0 200 200%22%3E%3Crect width%3D%22200%22 height%3D%22200%22 rx%3D%2228%22 fill%3D%22%232b6cf0%22%2F%3E%3Ctext x%3D%22100%22 y%3D%22128%22 text-anchor%3D%22middle%22 font-size%3D%2290%22 font-family%3D%22sans-serif%22 fill%3D%22white%22%3EB%3C%2Ftext%3E%3C%2Fsvg%3E"

type PageData struct {
	AppIconURL      string
	FallbackIconURL string
	AppSize         string
	AppVersion      string
	DownloadURL     string
	SiteURL         string
	SupportLink     string
	CSPNonce        string
}

func main() {
	// Production deployments should provide configuration through environment
	// variables. No shell commands or user-controlled input are executed.
	cfg := PageData{
		AppIconURL:      safeHTTPSURL(os.Getenv("APP_ICON_URL"), fallbackIconURL),
		FallbackIconURL: fallbackIconURL,
		AppSize:         cleanText(os.Getenv("APP_SIZE"), 64),
		AppVersion:      cleanVersion(os.Getenv("APP_VERSION")),
		DownloadURL:     safeHTTPSURL(os.Getenv("DOWNLOAD_URL"), ""),
		SiteURL:         safeHTTPSURL(os.Getenv("SITE_URL"), ""),
		SupportLink:     safeSupportURL(os.Getenv("SUPPORT_LINK")),
	}

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

		// Never trust a nonce from a request. Generate it on the server.
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
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}

	log.Printf("server listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject malformed/control characters in the request target.
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
			"img-src 'self' https: data:; "+
			"style-src 'nonce-"+nonce+"'; "+
			"script-src 'nonce-"+nonce+"'")

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
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
		// Keep mailto support links limited to a simple address form.
		addr := strings.TrimPrefix(value, "mailto:")
		if strings.ContainsAny(addr, "?#%") || !strings.Contains(addr, "@") {
			return "#"
		}
		return "mailto:" + addr
	}

	return safeHTTPSURL(value, "#")
}

func cleanText(value string, max int) string {
	value = strings.TrimSpace(value)
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
	return b.String()
}

func cleanVersion(value string) string {
	value = cleanText(value, 32)
	if value == "" {
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
	if value == "" {
		return false
	}
	if len(value) > 5 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	var n int
	for _, r := range value {
		n = n*10 + int(r-'0')
	}
	return n >= 1 && n <= 65535
}
