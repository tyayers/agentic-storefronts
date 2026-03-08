package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

const dataDir = "./data/storefronts"

var oauthConf = &oauth2.Config{
	ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
	ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	RedirectURL:  "postmessage",
	Scopes:       []string{"openid", "profile", "email"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://accounts.google.com/o/oauth2/auth",
		TokenURL: "https://oauth2.googleapis.com/token",
	},
}

func init() {
	if oauthConf.ClientID == "" {
		oauthConf.ClientID = "609874082793-0ad22eutlkcrrs0uehm8vekut6j07u2j.apps.googleusercontent.com"
	}
}

type UserSessionData struct {
	Email             string        `json:"email"`
	Name              string        `json:"name"`
	Picture           string        `json:"picture"`
	AuthorizationCode string        `json:"authorization_code"`
	Token             *oauth2.Token `json:"token"`
	IDToken           string        `json:"id_token"`
}

var sessions = make(map[string]string)
var sessionsMutex sync.Mutex

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Helper function to handle JSON responses
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// Helper to authenticate user via Google ID Token
func authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Missing or invalid Authorization header"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// In a real app, you should check audience and probably issuer.
		// For this prototype, we'll validate the token.
		payload, err := idtoken.Validate(context.Background(), tokenString, "")
		if err != nil {
			log.Printf("Token validation failed: %v", err)
			jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
			return
		}

		// Add user info to context if needed
		ctx := context.WithValue(r.Context(), "user_email", payload.Claims["email"])
		next(w, r.WithContext(ctx))
	}
}

// StorefrontsHandler manages CRUD for storefronts
func storefrontsHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to access data directory"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List all storefronts
		files, err := os.ReadDir(dataDir)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read storefronts"})
			return
		}

		var storefronts []json.RawMessage
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				data, err := os.ReadFile(filepath.Join(dataDir, file.Name()))
				if err == nil {
					storefronts = append(storefronts, data)
				}
			}
		}

		// Instead of returning null if empty, return empty array
		if storefronts == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}

		// Write array of JSON objects manually to preserve raw messages
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("["))
		for i, sf := range storefronts {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write(sf)
		}
		w.Write([]byte("]"))

	case http.MethodPost, http.MethodPut:
		// Create or Update storefront
		var sf map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&sf); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		id, ok := sf["id"].(string)
		if !ok || id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing or invalid 'id'"})
			return
		}

		// Sanitize ID to prevent directory traversal
		id = filepath.Base(filepath.Clean(id))
		filePath := filepath.Join(dataDir, id+".json")

		data, err := json.MarshalIndent(sf, "", "  ")
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process data"})
			return
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save storefront"})
			return
		}

		jsonResponse(w, http.StatusOK, sf)

	case http.MethodDelete:
		// Delete storefront
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing 'id' parameter"})
			return
		}

		// Sanitize ID
		id = filepath.Base(filepath.Clean(id))
		filePath := filepath.Join(dataDir, id+".json")

		if err := os.Remove(filePath); err != nil {
			if os.IsNotExist(err) {
				jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Storefront not found"})
			} else {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete storefront"})
			}
			return
		}

		jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		w.Header().Set("Allow", "GET, POST, PUT, DELETE")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func getGCPClient(ctx context.Context) (*http.Client, error) {
	client, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, err
	}
	return client, nil
}

func vertexProductsHandler(w http.ResponseWriter, r *http.Request) {
	projectId := r.URL.Query().Get("projectId")
	region := r.URL.Query().Get("region")

	if projectId == "" || region == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "projectId and region are required"})
		return
	}

	client, err := getGCPClient(r.Context())
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to authenticate with Google Cloud"})
		return
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/models", region, projectId, region)
	resp, err := client.Get(url)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		jsonResponse(w, resp.StatusCode, map[string]string{"error": fmt.Sprintf("GCP API error: %s", string(bodyBytes))})
		return
	}

	var data struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to parse GCP response"})
		return
	}

	// Map to common product structure
	var products []map[string]string
	for _, m := range data.Models {
		parts := strings.Split(m.Name, "/")
		shortId := parts[len(parts)-1]
		displayName := m.DisplayName
		if displayName == "" {
			displayName = shortId
		}
		products = append(products, map[string]string{"id": m.Name, "name": displayName})
	}
	if products == nil {
		products = []map[string]string{}
	}

	jsonResponse(w, http.StatusOK, products)
}

type ApigeeData struct {
	Id          string                   `json:"id"`
	Name        string                   `json:"name"`
	Versions    []map[string]interface{} `json:"versions,omitempty"`
	Deployments []map[string]interface{} `json:"deployments,omitempty"`
}

type apigeeCacheEntry struct {
	Data      []ApigeeData
	Timestamp time.Time
}

var (
	apigeeCache      = make(map[string]apigeeCacheEntry)
	apigeeCacheMutex sync.Mutex
)

func getGcpJson(client *http.Client, url string, target interface{}) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GCP API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func apigeeProductsHandler(w http.ResponseWriter, r *http.Request) {
	projectId := r.URL.Query().Get("projectId")
	region := r.URL.Query().Get("region")

	if projectId == "" || region == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "projectId and region are required"})
		return
	}

	cacheKey := fmt.Sprintf("%s:%s", projectId, region)
	apigeeCacheMutex.Lock()
	entry, ok := apigeeCache[cacheKey]
	if ok && time.Since(entry.Timestamp) < time.Hour {
		apigeeCacheMutex.Unlock()
		jsonResponse(w, http.StatusOK, entry.Data)
		return
	}
	apigeeCacheMutex.Unlock()

	client, err := getGCPClient(r.Context())
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to authenticate with Google Cloud"})
		return
	}

	deploymentsUrl := fmt.Sprintf("https://apihub.googleapis.com/v1/projects/%s/locations/%s/deployments", projectId, region)
	var depsResp struct {
		Deployments []map[string]interface{} `json:"deployments"`
	}
	getGcpJson(client, deploymentsUrl, &depsResp) // ignore error as it's optional

	apisUrl := fmt.Sprintf("https://apihub.googleapis.com/v1/projects/%s/locations/%s/apis", projectId, region)
	var apisResp struct {
		Apis []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"apis"`
	}
	if err := getGcpJson(client, apisUrl, &apisResp); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var results []ApigeeData

	for _, api := range apisResp.Apis {
		apiData := ApigeeData{
			Id:   api.Name,
			Name: api.DisplayName,
		}
		if apiData.Name == "" {
			parts := strings.Split(api.Name, "/")
			apiData.Name = parts[len(parts)-1]
		}

		versionsUrl := fmt.Sprintf("https://apihub.googleapis.com/v1/%s/versions", api.Name)
		var versionsResp struct {
			ApiVersions []map[string]interface{} `json:"apiVersions"`
		}
		if err := getGcpJson(client, versionsUrl, &versionsResp); err == nil {
			for _, vRaw := range versionsResp.ApiVersions {
				vName, _ := vRaw["name"].(string)
				specsUrl := fmt.Sprintf("https://apihub.googleapis.com/v1/%s/specs", vName)
				var specsResp struct {
					Specs []map[string]interface{} `json:"specs"`
				}
				if err := getGcpJson(client, specsUrl, &specsResp); err == nil {
					for _, sRaw := range specsResp.Specs {
						sName, _ := sRaw["name"].(string)
						contentsUrl := fmt.Sprintf("https://apihub.googleapis.com/v1/%s:contents", sName)
						var contentsResp map[string]interface{}
						if err := getGcpJson(client, contentsUrl, &contentsResp); err == nil {
							sRaw["contents"] = contentsResp
						}
					}
					vRaw["specs"] = specsResp.Specs
				}
			}
			apiData.Versions = versionsResp.ApiVersions
		}

		var apiDeps []map[string]interface{}
		for _, d := range depsResp.Deployments {
			apiVersions, _ := d["apiVersions"].([]interface{})
			for _, av := range apiVersions {
				avStr, _ := av.(string)
				if strings.HasPrefix(avStr, api.Name) {
					apiDeps = append(apiDeps, d)
					break
				}
			}
		}
		apiData.Deployments = apiDeps

		results = append(results, apiData)
	}

	if results == nil {
		results = []ApigeeData{}
	}

	apigeeCacheMutex.Lock()
	apigeeCache[cacheKey] = apigeeCacheEntry{
		Data:      results,
		Timestamp: time.Now(),
	}
	apigeeCacheMutex.Unlock()

	jsonResponse(w, http.StatusOK, results)
}

func authCallbackHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	ctx := context.Background()
	token, err := oauthConf.Exchange(ctx, req.Code)
	if err != nil {
		log.Printf("Token exchange failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to exchange token"})
		return
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "No id_token in response"})
		return
	}

	payload, err := idtoken.Validate(ctx, idToken, oauthConf.ClientID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Invalid id_token"})
		return
	}

	email := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	os.MkdirAll("./data/users", 0755)
	userData := UserSessionData{
		Email:             email,
		Name:              name,
		Picture:           picture,
		AuthorizationCode: req.Code,
		Token:             token,
		IDToken:           idToken,
	}

	fileData, _ := json.MarshalIndent(userData, "", "  ")
	os.WriteFile(filepath.Join("./data/users", email+".json"), fileData, 0644)

	sessionID := generateSessionID()
	sessionsMutex.Lock()
	sessions[sessionID] = email
	sessionsMutex.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
	})

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"user": map[string]string{
			"email":   email,
			"name":    name,
			"picture": picture,
		},
		"id_token": idToken,
	})
}

func authTokenHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "No session cookie"})
		return
	}

	sessionsMutex.Lock()
	email, ok := sessions[cookie.Value]
	sessionsMutex.Unlock()

	if !ok {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
		return
	}

	filePath := filepath.Join("./data/users", email+".json")
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "User data not found"})
		return
	}

	var userData UserSessionData
	if err := json.Unmarshal(fileData, &userData); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Invalid user data"})
		return
	}

	tokenSource := oauthConf.TokenSource(context.Background(), userData.Token)
	newToken, err := tokenSource.Token()
	if err != nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Failed to refresh token"})
		return
	}

	idToken, _ := newToken.Extra("id_token").(string)
	tokenUpdated := false

	if newToken.AccessToken != userData.Token.AccessToken {
		userData.Token = newToken
		tokenUpdated = true
	}

	if idToken != "" && idToken != userData.IDToken {
		userData.IDToken = idToken
		tokenUpdated = true
	}

	if tokenUpdated {
		newData, _ := json.MarshalIndent(userData, "", "  ")
		os.WriteFile(filePath, newData, 0644)
	}

	if userData.IDToken == "" {
		userData.IDToken = newToken.AccessToken
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"id_token": userData.IDToken,
		"user": map[string]string{
			"email":   userData.Email,
			"name":    userData.Name,
			"picture": userData.Picture,
		},
	})
}

func authLogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		sessionsMutex.Lock()
		delete(sessions, cookie.Value)
		sessionsMutex.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	jsonResponse(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func main() {
	mux := http.NewServeMux()

	// Serve landing page on / path
	landingFs := http.FileServer(http.Dir("landing"))
	mux.Handle("/", landingFs)

	// Serve static client on /manage path
	// Using http.StripPrefix to remove the /manage prefix before passing it to the file server.
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/manage/", http.StripPrefix("/manage/", fs))

	// API Endpoints
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "API is operational"})
	})

	mux.HandleFunc("/api/config", authenticate(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"projectId": os.Getenv("PROJECT_ID"),
			"region":    os.Getenv("REGION"),
		})
	}))

	mux.HandleFunc("/api/auth/google/callback", authCallbackHandler)
	mux.HandleFunc("/api/auth/session", authTokenHandler)
	mux.HandleFunc("/api/auth/logout", authLogoutHandler)

	// Protected Storefronts API
	mux.HandleFunc("/api/storefronts", authenticate(storefrontsHandler))
	mux.HandleFunc("/api/products/vertex", authenticate(vertexProductsHandler))
	mux.HandleFunc("/api/products/apigee", authenticate(apigeeProductsHandler))

	log.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
