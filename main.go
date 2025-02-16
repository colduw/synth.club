package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"main/database"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/acme/autocert"
	"gorm.io/gorm"
)

type (
	apiDID struct {
		DID string `json:"did"`
	}

	apiTurnstile struct {
		Success bool `json:"success"`
	}
)

const maxReadLimit = 20 * 1024

var (
	mainTemplate     = template.Must(template.ParseFiles("./views/main.html"))
	errorTemplate    = template.Must(template.ParseFiles("./views/error.html"))
	registerTemplate = template.Must(template.ParseFiles("./views/registration.html"))

	timeoutClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	cfSecretToken string
)

func main() {
	if loadErr := godotenv.Load(); loadErr != nil {
		panic(loadErr)
	}

	v, ok := os.LookupEnv("CF_TURNSTILE_SECRET")
	if v == "" || !ok {
		panic("CF_TURNSTILE_SECRET does not exist (or is empty)")
	}
	cfSecretToken = v

	database.SetupDatabase()

	if migrationErr := database.Db().AutoMigrate(&database.CHandle{}); migrationErr != nil {
		panic(migrationErr)
	}

	sMux := http.NewServeMux()
	sMux.HandleFunc("GET /", indexPage)
	sMux.HandleFunc("GET /{username}/", redirToBsky)
	sMux.HandleFunc("GET /{username}/.well-known/atproto-did", getProtogen)
	sMux.HandleFunc("GET /{username}/.well-known/discord", getDiscord)

	sMux.HandleFunc("GET /assets/main.js", cServeFile("./assets/main.js", "text/javascript"))
	sMux.HandleFunc("GET /assets/main.css", cServeFile("./assets/main.css", "text/css"))
	sMux.HandleFunc("GET /assets/pBold.woff2", cServeFile("./assets/pBold.woff2", "font/woff2"))
	sMux.HandleFunc("GET /assets/pRegular.woff2", cServeFile("./assets/pRegular.woff2", "font/woff2"))

	sMux.HandleFunc("POST /regVerify", regVerify)
	sMux.HandleFunc("POST /doRegister", doRegister)

	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("synth.club"),
		Cache:      autocert.DirCache("certs/"),
	}

	go func() {
		httpServer := &http.Server{
			Addr:              ":80",
			Handler:           manager.HTTPHandler(nil),
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       time.Minute,
		}

		if httpListenErr := httpServer.ListenAndServe(); httpListenErr != nil {
			panic(httpListenErr)
		}
	}()

	httpsServer := &http.Server{
		Addr:              ":443",
		Handler:           sMux,
		TLSConfig:         manager.TLSConfig(),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}

	if httpsListenErr := httpsServer.ListenAndServeTLS("", ""); httpsListenErr != nil {
		panic(httpsListenErr)
	}
}

func indexPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	mainTemplate.Execute(w, nil)
}

func redirToBsky(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, fmt.Sprintf("https://bsky.app/profile/%s.synth.club", r.PathValue("username")), http.StatusFound)
}

func cServeFile(fileName, mimeType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", mimeType)
		http.ServeFile(w, r, fileName)
	}
}

func errorPage(w http.ResponseWriter, errorMessage string) {
	errorTemplate.Execute(w, map[string]string{"errorMsg": errorMessage})
}

func regVerify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxReadLimit)

	w.Header().Set("Content-Type", "application/json")

	if parseErr := r.ParseMultipartForm(maxReadLimit); parseErr != nil {
		json.NewEncoder(w).Encode(map[string]any{"isSuccess": false, "errorMessage": "Failed to parse form"})
		return
	}

	var didCount int64
	didHelper := r.FormValue("didHelper")
	database.Db().Model(&database.CHandle{}).Where("did = ?", didHelper).Count(&didCount)
	if didCount != 0 {
		json.NewEncoder(w).Encode(map[string]any{"isSuccess": false, "errorMessage": "DID is already registered"})
		return
	}

	var handleCount int64
	newHandle := strings.ToLower(r.FormValue("newHandle"))
	database.Db().Model(&database.CHandle{}).Where("handle = ?", newHandle).Count(&handleCount)
	if handleCount != 0 {
		json.NewEncoder(w).Encode(map[string]any{"isSuccess": false, "errorMessage": "Handle is already registered"})
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"isSuccess": true})
}

func doRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxReadLimit)

	if parseErr := r.ParseForm(); parseErr != nil {
		errorPage(w, "Failed to parse form")
		return
	}

	oldHandle := strings.ToLower(r.FormValue("oldHandle"))
	bReq, bReqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor="+oldHandle, http.NoBody)
	if bReqErr != nil {
		errorPage(w, "Failed to create request to Bluesky")
		return
	}

	bResp, bRespErr := timeoutClient.Do(bReq)
	if bRespErr != nil {
		errorPage(w, "Failed to do request to Bluesky")
		return
	}
	defer bResp.Body.Close()

	var apiResp apiDID
	if decodeErr := json.NewDecoder(bResp.Body).Decode(&apiResp); decodeErr != nil {
		errorPage(w, "Failed to decode response JSON")
		return
	}

	cfForm := url.Values{}
	cfForm.Set("secret", cfSecretToken)
	cfForm.Set("response", r.FormValue("cf-turnstile-response"))
	cfForm.Set("remoteip", r.Header.Get("cf-connecting-ip"))

	cfBuffer := bytes.NewBufferString(cfForm.Encode())

	cReq, cReqErr := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://challenges.cloudflare.com/turnstile/v0/siteverify", cfBuffer)
	if cReqErr != nil {
		errorPage(w, "Failed to create captcha request")
		return
	}

	cReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	cResp, cRespErr := timeoutClient.Do(cReq)
	if cRespErr != nil {
		errorPage(w, "Failed to do captcha request")
		return
	}
	defer cResp.Body.Close()

	var turnstileResult apiTurnstile
	if decodeErr := json.NewDecoder(cResp.Body).Decode(&turnstileResult); decodeErr != nil {
		errorPage(w, "Failed to decode captcha result")
		return
	}

	if !turnstileResult.Success {
		errorPage(w, "Captcha not successful")
		return
	}

	newRecord := database.CHandle{
		Handle: strings.ToLower(r.FormValue("newHandle")),
		DID:    apiResp.DID,
		DHCode: strings.ToLower(r.FormValue("dhCode")),
	}

	if createErr := database.Db().Model(&database.CHandle{}).Create(&newRecord).Error; createErr != nil {
		errorPage(w, "Failed to finalize registration")
		return
	}

	registerTemplate.Execute(w, map[string]string{"handle": newRecord.Handle})
}

func getProtogen(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	var bHandle database.CHandle
	dbErr := database.Db().Model(&database.CHandle{}).Where("handle = ?", username).First(&bHandle).Error

	if errors.Is(dbErr, gorm.ErrRecordNotFound) {
		http.Error(w, "Handle not found", http.StatusNotFound)
		return
	} else if dbErr != nil {
		http.Error(w, "Failed to get handle", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, bHandle.DID)
}

func getDiscord(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	var dHandle database.CHandle
	dbErr := database.Db().Model(&database.CHandle{}).Where("handle = ?", username).First(&dHandle).Error

	if errors.Is(dbErr, gorm.ErrRecordNotFound) {
		http.Error(w, "Handle not found", http.StatusNotFound)
		return
	} else if dbErr != nil {
		http.Error(w, "Failed to get handle", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, dHandle.DHCode)
}
