package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type PageData struct {
	AppIconURL  string
	AppSize     string
	AppVersion  string
	DownloadURL string
	SiteURL     string
	SupportLink string
}

func main() {
	// تحميل .env إذا كان موجوداً
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		data := PageData{
			AppIconURL:  os.Getenv("APP_ICON_URL"),
			AppSize:     os.Getenv("APP_SIZE"),
			AppVersion:  os.Getenv("APP_VERSION"),
			DownloadURL: os.Getenv("DOWNLOAD_URL"),
			SiteURL:     os.Getenv("SITE_URL"),
			SupportLink: os.Getenv("SUPPORT_LINK"),
		}

		tmpl, err := template.ParseFiles("index.html")
		if err != nil {
			http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, data); err != nil {
			log.Println("Template execute error:", err)
		}
	})

	log.Printf("Server running on :%s", port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
