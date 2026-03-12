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
const taxonomiesDir = "./data/taxonomies"
const productGroupsDir = "./data/product-groups"
const audiencesDir = "./data/audiences"
const imagesDir = "./data/images"
const sessionsDir = "./data/sessions"

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

// TaxonomiesHandler manages CRUD for taxonomies
func taxonomiesHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure data directory exists
	if err := os.MkdirAll(taxonomiesDir, 0755); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to access data directory"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List all taxonomies
		files, err := os.ReadDir(taxonomiesDir)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read taxonomies"})
			return
		}

		var taxonomies []json.RawMessage
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				data, err := os.ReadFile(filepath.Join(taxonomiesDir, file.Name()))
				if err == nil {
					taxonomies = append(taxonomies, data)
				}
			}
		}

		if taxonomies == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("["))
		for i, tax := range taxonomies {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write(tax)
		}
		w.Write([]byte("]"))

	case http.MethodPost, http.MethodPut:
		// Create or Update taxonomy
		var tax map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&tax); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		id, ok := tax["id"].(string)
		if !ok || id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing or invalid 'id'"})
			return
		}

		id = filepath.Base(filepath.Clean(id))
		filePath := filepath.Join(taxonomiesDir, id+".json")

		data, err := json.MarshalIndent(tax, "", "  ")
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process data"})
			return
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save taxonomy"})
			return
		}

		jsonResponse(w, http.StatusOK, tax)

	case http.MethodDelete:
		// Delete taxonomy
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing 'id' parameter"})
			return
		}

		id = filepath.Base(filepath.Clean(id))
		filePath := filepath.Join(taxonomiesDir, id+".json")

		if err := os.Remove(filePath); err != nil {
			if os.IsNotExist(err) {
				jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Taxonomy not found"})
			} else {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete taxonomy"})
			}
			return
		}

		jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		w.Header().Set("Allow", "GET, POST, PUT, DELETE")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// ProductGroupsHandler manages CRUD for product groups
func productGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(productGroupsDir, 0755); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to access data directory"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		files, err := os.ReadDir(productGroupsDir)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read product groups"})
			return
		}

		var groups []json.RawMessage
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				data, err := os.ReadFile(filepath.Join(productGroupsDir, file.Name()))
				if err == nil {
					groups = append(groups, data)
				}
			}
		}

		if groups == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("["))
		for i, g := range groups {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write(g)
		}
		w.Write([]byte("]"))

	case http.MethodPost, http.MethodPut:
		var pg map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&pg); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		id, ok := pg["id"].(string)
		if !ok || id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing or invalid 'id'"})
			return
		}

		id = filepath.Base(filepath.Clean(id))
		filePath := filepath.Join(productGroupsDir, id+".json")

		data, err := json.MarshalIndent(pg, "", "  ")
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process data"})
			return
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save product group"})
			return
		}

		jsonResponse(w, http.StatusOK, pg)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing 'id' parameter"})
			return
		}

		id = filepath.Base(filepath.Clean(id))
		filePath := filepath.Join(productGroupsDir, id+".json")

		if err := os.Remove(filePath); err != nil {
			if os.IsNotExist(err) {
				jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Product group not found"})
			} else {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete product group"})
			}
			return
		}

		jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		w.Header().Set("Allow", "GET, POST, PUT, DELETE")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

// AudiencesHandler manages CRUD for audiences
func audiencesHandler(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(audiencesDir, 0755); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to access data directory"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		files, err := os.ReadDir(audiencesDir)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read audiences"})
			return
		}

		var audiences []json.RawMessage
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
				data, err := os.ReadFile(filepath.Join(audiencesDir, file.Name()))
				if err == nil {
					audiences = append(audiences, data)
				}
			}
		}

		if audiences == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("[]"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("["))
		for i, a := range audiences {
			if i > 0 {
				w.Write([]byte(","))
			}
			w.Write(a)
		}
		w.Write([]byte("]"))

	case http.MethodPost, http.MethodPut:
		var aud map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&aud); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		id, ok := aud["id"].(string)
		if !ok || id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing or invalid 'id'"})
			return
		}

		id = filepath.Base(filepath.Clean(id))
		filePath := filepath.Join(audiencesDir, id+".json")

		data, err := json.MarshalIndent(aud, "", "  ")
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process data"})
			return
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save audience"})
			return
		}

		jsonResponse(w, http.StatusOK, aud)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing 'id' parameter"})
			return
		}

		id = filepath.Base(filepath.Clean(id))
		filePath := filepath.Join(audiencesDir, id+".json")

		if err := os.Remove(filePath); err != nil {
			if os.IsNotExist(err) {
				jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Audience not found"})
			} else {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete audience"})
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

func getVertexProducts(ctx context.Context, projectId, region string) ([]Product, error) {
	client, err := getGCPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to authenticate with Google Cloud")
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/models", region, projectId, region)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GCP API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var data struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Description string `json:"description"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("Failed to parse GCP response")
	}

	var products []Product
	for _, m := range data.Models {
		parts := strings.Split(m.Name, "/")
		shortId := parts[len(parts)-1]
		displayName := m.DisplayName
		if displayName == "" {
			displayName = shortId
		}
		products = append(products, Product{
			Id:                 m.Name,
			Name:               displayName,
			DisplayName:        displayName,
			Description:        m.Description,
			DisplayDescription: m.Description,
			Type:               "vertex",
		})
	}
	if products == nil {
		products = []Product{}
	}

	return products, nil
}

func vertexProductsHandler(w http.ResponseWriter, r *http.Request) {
	projectId := r.URL.Query().Get("projectId")
	region := r.URL.Query().Get("region")

	if projectId == "" || region == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "projectId and region are required"})
		return
	}

	products, err := getVertexProducts(r.Context(), projectId, region)
	if err != nil {
		// Try to return appropriate status codes
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "GCP API error") {
			status = http.StatusBadGateway // or whatever makes sense
		}
		jsonResponse(w, status, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, products)
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

func getApigeeProducts(ctx context.Context, projectId, region string) ([]Product, error) {
	cacheKey := fmt.Sprintf("%s:%s", projectId, region)
	apigeeCacheMutex.Lock()
	entry, ok := apigeeCache[cacheKey]
	if ok && time.Since(entry.Timestamp) < time.Hour {
		apigeeCacheMutex.Unlock()
		return entry.Data, nil
	}
	apigeeCacheMutex.Unlock()

	client, err := getGCPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to authenticate with Google Cloud")
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
			Description string `json:"description"`
			ApiStyle    struct {
				EnumValues struct {
					Values []struct {
						DisplayName string `json:"displayName"`
					} `json:"values"`
				} `json:"enumValues"`
			} `json:"apiStyle"`
		} `json:"apis"`
	}
	if err := getGcpJson(client, apisUrl, &apisResp); err != nil {
		return nil, err
	}

	var results []Product

	for _, api := range apisResp.Apis {
		apiDisplayName := api.DisplayName
		if apiDisplayName == "" {
			parts := strings.Split(api.Name, "/")
			apiDisplayName = parts[len(parts)-1]
		}

		apiStyle := ""
		if len(api.ApiStyle.EnumValues.Values) > 0 {
			apiStyle = api.ApiStyle.EnumValues.Values[0].DisplayName
		}

		versionsUrl := fmt.Sprintf("https://apihub.googleapis.com/v1/%s/versions", api.Name)
		var versionsResp struct {
			ApiVersions []map[string]interface{} `json:"versions"`
		}
		if err := getGcpJson(client, versionsUrl, &versionsResp); err == nil {
			for _, vRaw := range versionsResp.ApiVersions {
				vName, _ := vRaw["name"].(string)
				vDisplayName, _ := vRaw["displayName"].(string)
				if vDisplayName == "" {
					parts := strings.Split(vName, "/")
					vDisplayName = parts[len(parts)-1]
				}

				// Combine names and descriptions
				productName := fmt.Sprintf("%s (%s)", apiDisplayName, vDisplayName)
				productDesc := api.Description
				vDesc, _ := vRaw["description"].(string)
				if vDesc != "" {
					if productDesc != "" {
						productDesc += "\n\n" + vDesc
					} else {
						productDesc = vDesc
					}
				}

				versionProduct := Product{
					Id:                 vName, // Use version name as the unique ID
					Name:               productName,
					DisplayName:        productName,
					Description:        productDesc,
					DisplayDescription: productDesc,
					Type:               "apigee",
					Style:              apiStyle,
					DisplayStyle:       apiStyle,
				}

				// Get specs for this version
				specsUrl := fmt.Sprintf("https://apihub.googleapis.com/v1/%s/specs", vName)
				var specsResp struct {
					Specs []map[string]interface{} `json:"specs"`
				}
				if err := getGcpJson(client, specsUrl, &specsResp); err == nil {
					// Just grab the first spec contents for now
					for _, sRaw := range specsResp.Specs {
						sName, _ := sRaw["name"].(string)
						contentsUrl := fmt.Sprintf("https://apihub.googleapis.com/v1/%s:contents", sName)
						var contentsResp map[string]interface{}
						if err := getGcpJson(client, contentsUrl, &contentsResp); err == nil {
							if contents, ok := contentsResp["contents"].(string); ok {
								versionProduct.SpecContents = contents
								break // Got a spec, stop looking for more specs for this version
							}
						}
					}
				}

				// Get deployment for this version
				for _, d := range depsResp.Deployments {
					apiVersions, _ := d["apiVersions"].([]interface{})
					for _, av := range apiVersions {
						avStr, _ := av.(string)
						if avStr == vName { // Exact match for the version
							if endpoints, ok := d["endpoints"].([]interface{}); ok && len(endpoints) > 0 {
								if epMap, ok := endpoints[0].(map[string]interface{}); ok {
									if uri, ok := epMap["uri"].(string); ok && uri != "" {
										versionProduct.Endpoint = uri
									}
								}
							}
							if versionProduct.Endpoint == "" {
								if uri, ok := d["deploymentUri"].(string); ok && uri != "" {
									versionProduct.Endpoint = uri
								}
							}
							break
						}
					}
				}

				results = append(results, versionProduct)
			}
		} else {
			// If we couldn't get versions, fallback to creating a product for the API itself
			apiData := Product{
				Id:                 api.Name,
				Name:               apiDisplayName,
				DisplayName:        apiDisplayName,
				Description:        api.Description,
				DisplayDescription: api.Description,
				Type:               "apigee",
				Style:              apiStyle,
				DisplayStyle:       apiStyle,
			}
			results = append(results, apiData)
		}
	}

	if results == nil {
		results = []Product{}
	}

	apigeeCacheMutex.Lock()
	entryToCache := apigeeCacheEntry{
		Data:      results,
		Timestamp: time.Now(),
	}
	apigeeCache[cacheKey] = entryToCache

	if err := os.MkdirAll("./data/products", 0755); err == nil {
		if fileData, err := json.MarshalIndent(entryToCache, "", "  "); err == nil {
			fileName := fmt.Sprintf("./data/products/apigee_%s_%s.json", projectId, region)
			os.WriteFile(fileName, fileData, 0644)
		}
	}
	apigeeCacheMutex.Unlock()

	return results, nil
}

func apigeeProductsHandler(w http.ResponseWriter, r *http.Request) {
	projectId := r.URL.Query().Get("projectId")
	region := r.URL.Query().Get("region")

	if projectId == "" || region == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "projectId and region are required"})
		return
	}

	products, err := getApigeeProducts(r.Context(), projectId, region)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "GCP API error") {
			status = http.StatusBadGateway
		}
		jsonResponse(w, status, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, products)
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

	os.MkdirAll(sessionsDir, 0755)
	sessionData := map[string]string{
		"id":    sessionID,
		"email": email,
	}
	sessionFile, _ := json.MarshalIndent(sessionData, "", "  ")
	os.WriteFile(filepath.Join(sessionsDir, sessionID+".json"), sessionFile, 0644)

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

	sessionID := filepath.Base(filepath.Clean(cookie.Value))
	sessionFilePath := filepath.Join(sessionsDir, sessionID+".json")

	sessionFileData, err := os.ReadFile(sessionFilePath)
	if err != nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid session"})
		return
	}

	var sessionData map[string]string
	if err := json.Unmarshal(sessionFileData, &sessionData); err != nil {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid session data"})
		return
	}

	email, ok := sessionData["email"]
	if !ok || email == "" {
		jsonResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid session data"})
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
		sessionID := filepath.Base(filepath.Clean(cookie.Value))
		sessionFilePath := filepath.Join(sessionsDir, sessionID+".json")
		os.Remove(sessionFilePath)
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

func storefrontProductsHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		// Fallback just in case
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 4 {
			name = parts[3]
		}
	}

	if name == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing storefront name"})
		return
	}

	// Sanitize name
	name = filepath.Base(filepath.Clean(name))
	filePath := filepath.Join(dataDir, name+".json")

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Storefront not found"})
		} else {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read storefront"})
		}
		return
	}

	var sf Storefront
	if err := json.Unmarshal(fileData, &sf); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Invalid storefront data"})
		return
	}

	var allProducts []Product
	for _, pgConfig := range sf.ProductGroups {
		// Read product group
		pgFile := filepath.Join(productGroupsDir, pgConfig.ProductGroupId+".json")
		pgData, err := os.ReadFile(pgFile)
		if err != nil {
			log.Printf("Failed to read product group %s: %v", pgConfig.ProductGroupId, err)
			continue
		}

		var pg ProductGroup
		if err := json.Unmarshal(pgData, &pg); err != nil {
			log.Printf("Failed to parse product group %s: %v", pgConfig.ProductGroupId, err)
			continue
		}

		for _, source := range pg.Sources {
			var products []Product
			var fetchErr error

			if source.Type == "vertex" {
				products, fetchErr = getVertexProducts(r.Context(), source.Name, source.Region)
			} else if source.Type == "apigee" {
				products, fetchErr = getApigeeProducts(r.Context(), source.Name, source.Region)
			} else if source.Type == "manual" {
				// Handle manual products if necessary
				// Not fully implemented in getVertexProducts/getApigeeProducts yet
			}

			if fetchErr != nil {
				log.Printf("Failed to fetch products for source %s: %v", source.Name, fetchErr)
				continue
			}

			if source.Autodetect {
				allProducts = append(allProducts, products...)
			} else {
				selectedMap := make(map[string]SelectedProduct)
				for _, sp := range source.SelectedProducts {
					selectedMap[sp.Id] = sp
				}
				for _, p := range products {
					if sp, ok := selectedMap[p.Id]; ok {
						if sp.DisplayName != "" {
							p.DisplayName = sp.DisplayName
						}
						if sp.DisplayDescription != "" {
							p.DisplayDescription = sp.DisplayDescription
						}
						if sp.DisplayStyle != "" {
							p.DisplayStyle = sp.DisplayStyle
						}
						if sp.Image != "" {
							p.Image = sp.Image
						}
						allProducts = append(allProducts, p)
					}
				}
			}
		}
	}

	if allProducts == nil {
		allProducts = []Product{}
	}

	jsonResponse(w, http.StatusOK, allProducts)
}

func imagesHandler(w http.ResponseWriter, r *http.Request) {
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to access images directory"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List images
		files, err := os.ReadDir(imagesDir)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read images directory"})
			return
		}

		var images []string
		for _, file := range files {
			if !file.IsDir() {
				ext := strings.ToLower(filepath.Ext(file.Name()))
				if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".svg" || ext == ".webp" {
					images = append(images, "/images/"+file.Name())
				}
			}
		}

		if images == nil {
			images = []string{}
		}

		jsonResponse(w, http.StatusOK, images)

	case http.MethodPost:
		// Upload image
		// 10 MB limit
		r.ParseMultipartForm(10 << 20)
		file, handler, err := r.FormFile("image")
		if err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Error retrieving file from form"})
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(handler.Filename))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".svg" && ext != ".webp" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid file type"})
			return
		}

		// Sanitize filename to prevent directory traversal or weird chars
		filename := filepath.Base(filepath.Clean(handler.Filename))
		// Optional: prepend timestamp or random string to avoid overwriting
		filename = fmt.Sprintf("%d-%s", time.Now().Unix(), filename)

		dst, err := os.Create(filepath.Join(imagesDir, filename))
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error creating file on server"})
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error saving file"})
			return
		}

		jsonResponse(w, http.StatusOK, map[string]string{"url": "/images/" + filename})

	default:
		w.Header().Set("Allow", "GET, POST")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
	}
}

func main() {
	if files, err := os.ReadDir("./data/products"); err == nil {
		apigeeCacheMutex.Lock()
		loadedCount := 0
		for _, file := range files {
			if !file.IsDir() && strings.HasPrefix(file.Name(), "apigee_") && strings.HasSuffix(file.Name(), ".json") {
				// Parse projectId and region from filename: apigee_{projectId}_{region}.json
				nameParts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(file.Name(), "apigee_"), ".json"), "_")
				if len(nameParts) >= 2 {
					// Handle cases where projectId might contain underscores by taking the last part as region
					regionStr := nameParts[len(nameParts)-1]
					projectIdStr := strings.Join(nameParts[:len(nameParts)-1], "_")
					cacheKey := fmt.Sprintf("%s:%s", projectIdStr, regionStr)

					fileData, err := os.ReadFile(filepath.Join("./data/products", file.Name()))
					if err == nil {
						var entry apigeeCacheEntry
						if err := json.Unmarshal(fileData, &entry); err == nil {
							apigeeCache[cacheKey] = entry
							loadedCount++
						} else {
							log.Printf("Failed to unmarshal apigee cache file %s: %v", file.Name(), err)
						}
					}
				}
			}
		}
		apigeeCacheMutex.Unlock()
		if loadedCount > 0 {
			log.Printf("Loaded %d Apigee product cache entries from local files", loadedCount)
		}
	}

	// Pre-cache Apigee products on startup if project and region are provided
	projectId := os.Getenv("PROJECT_ID")
	region := os.Getenv("REGION")
	if projectId != "" && region != "" {
		log.Printf("Pre-caching Apigee products for project: %s, region: %s", projectId, region)
		go func() {
			_, err := getApigeeProducts(context.Background(), projectId, region)
			if err != nil {
				log.Printf("Failed to pre-cache Apigee products: %v", err)
			} else {
				log.Println("Successfully pre-cached Apigee products")
			}
		}()
	}

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
	mux.HandleFunc("/api/taxonomies", authenticate(taxonomiesHandler))
	mux.HandleFunc("/api/product-groups", authenticate(productGroupsHandler))
	mux.HandleFunc("/api/audiences", authenticate(audiencesHandler))
	mux.HandleFunc("/api/images", authenticate(imagesHandler))
	mux.HandleFunc("/api/storefronts/{name}/products", storefrontProductsHandler)
	mux.HandleFunc("/api/products/vertex", authenticate(vertexProductsHandler))
	mux.HandleFunc("/api/products/apigee", authenticate(apigeeProductsHandler))

	// Serve uploaded images statically
	imagesFs := http.FileServer(http.Dir("./data/images"))
	mux.Handle("/images/", http.StripPrefix("/images/", imagesFs))

	log.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
