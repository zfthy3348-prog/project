// main.go
package main

import (
    "html/template"
    "log"
    "net/http"
    "os"

    "github.com/joho/godotenv"
)

func main() {
    // تحميل متغيرات البيئة من .env (إن وُجد)
    if err := godotenv.Load(); err != nil {
        log.Println("⚠️ No .env file found, using system env")
    }

    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        data := struct {
            AppIconURL  string
            AppSize     string
            AppVersion  string
            DownloadURL string
            SiteURL     string
            SupportLink string
        }{
            AppIconURL:  os.Getenv("APP_ICON_URL"),
            AppSize:     os.Getenv("APP_SIZE"),
            AppVersion:  os.Getenv("APP_VERSION"),
            DownloadURL: os.Getenv("DOWNLOAD_URL"),
            SiteURL:     os.Getenv("SITE_URL"),
            SupportLink: os.Getenv("SUPPORT_LINK"),
        }

        tmpl := template.Must(template.ParseFiles("index.html"))
        tmpl.Execute(w, data)
    })

    log.Printf("🚀 Server running on :%s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}
