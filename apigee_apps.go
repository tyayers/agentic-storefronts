package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ApigeeCredential struct {
	ConsumerKey    string `json:"consumerKey"`
	ConsumerSecret string `json:"consumerSecret"`
	ApiProducts    []struct {
		Apiproduct string `json:"apiproduct"`
	} `json:"apiProducts"`
}

type ApigeeAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ApigeeApp struct {
	AppId       string             `json:"appId"`
	Name        string             `json:"name"`
	Attributes  []ApigeeAttribute  `json:"attributes"`
	Credentials []ApigeeCredential `json:"credentials"`
}

type ApigeeAppList struct {
	App []ApigeeApp `json:"app"`
}

type ApigeeAppCreateReq struct {
	Name        string            `json:"name"`
	ApiProducts []string          `json:"apiProducts"`
	Attributes  []ApigeeAttribute `json:"attributes"`
}

type ApigeeAppUpdateReq struct {
	Name       string            `json:"name"`
	Attributes []ApigeeAttribute `json:"attributes"`
}

type ApigeeKeyUpdateReq struct {
	ApiProducts []string `json:"apiProducts"`
}

func doApigeeRequest(ctx context.Context, method, url string, body interface{}, target interface{}) error {
	client, err := getGCPClient(ctx)
	if err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("apigee API error (%d): %s", resp.StatusCode, string(respBytes))
	}

	if target != nil && resp.StatusCode != http.StatusNoContent {
		return json.NewDecoder(resp.Body).Decode(target)
	}
	return nil
}

func mapApigeeAppToApp(a ApigeeApp) App {
	app := App{
		Id:          a.AppId,
		Name:        a.Name,
		Credentials: make([]Credential, 0),
	}
	for _, attr := range a.Attributes {
		if attr.Name == "Notes" || attr.Name == "description" {
			app.Description = attr.Value
		}
	}
	for _, cred := range a.Credentials {
		c := Credential{
			ClientId:     cred.ConsumerKey,
			ClientSecret: cred.ConsumerSecret,
			Products:     make([]string, 0),
		}
		for _, p := range cred.ApiProducts {
			c.Products = append(c.Products, p.Apiproduct)
		}
		app.Credentials = append(app.Credentials, c)
	}
	return app
}

func userAppsHandler(w http.ResponseWriter, r *http.Request) {
	email := r.PathValue("email")
	storefrontId := r.PathValue("storefrontId")
	if email == "" || storefrontId == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "email and storefrontId are required"})
		return
	}

	ctx := context.Background()

	// 1. Read storefront to find Apigee organizations (project IDs)
	storefrontPath := filepath.Join(dataDir, filepath.Base(filepath.Clean(storefrontId))+".json")
	sfData, err := os.ReadFile(storefrontPath)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "storefront not found"})
		return
	}

	var sf Storefront
	if err := json.Unmarshal(sfData, &sf); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "invalid storefront data"})
		return
	}

	apigeeOrgs := make(map[string]bool)
	for _, pgConfig := range sf.ProductGroups {
		pgPath := filepath.Join(productGroupsDir, filepath.Base(filepath.Clean(pgConfig.ProductGroupId))+".json")
		pgData, err := os.ReadFile(pgPath)
		if err != nil {
			continue
		}

		var pg ProductGroup
		if err := json.Unmarshal(pgData, &pg); err == nil {
			for _, source := range pg.Sources {
				if source.Type == "apigee" && source.Name != "" {
					apigeeOrgs[source.Name] = true
				}
			}
		}
	}

	if r.Method == http.MethodGet {
		var allApps []App

		for projectId := range apigeeOrgs {
			url := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers/%s/apps?expand=true", projectId, email)
			var list ApigeeAppList
			if err := doApigeeRequest(ctx, "GET", url, nil, &list); err != nil {
				log.Printf("Failed to get apps from org %s: %v", projectId, err)
				continue
			}

			for _, a := range list.App {
				app := mapApigeeAppToApp(a)
				app.ProjectId = projectId
				allApps = append(allApps, app)
			}
		}

		if allApps == nil {
			allApps = []App{}
		}
		jsonResponse(w, http.StatusOK, allApps)
		return
	}

	if r.Method == http.MethodPost {
		var app App
		if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}

		projectId := app.ProjectId
		if projectId == "" {
			// fallback to the first discovered Apigee org or ENV
			for org := range apigeeOrgs {
				projectId = org
				break
			}
			if projectId == "" {
				projectId = os.Getenv("PROJECT_ID")
			}
		}

		if projectId == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "projectId could not be determined for app creation"})
			return
		}

		req := ApigeeAppCreateReq{
			Name:        app.Name,
			ApiProducts: make([]string, 0),
		}
		if app.Description != "" {
			req.Attributes = []ApigeeAttribute{{Name: "description", Value: app.Description}}
		}
		if len(app.Credentials) > 0 {
			req.ApiProducts = app.Credentials[0].Products
		}

		url := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers/%s/apps", projectId, email)
		var created ApigeeApp
		if err := doApigeeRequest(ctx, "POST", url, req, &created); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		mappedApp := mapApigeeAppToApp(created)
		mappedApp.ProjectId = projectId
		jsonResponse(w, http.StatusCreated, mappedApp)
		return
	}

	jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func userAppsDetailHandler(w http.ResponseWriter, r *http.Request) {
	email := r.PathValue("email")
	appName := r.PathValue("appName")
	if email == "" || appName == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "email and appName are required"})
		return
	}

	ctx := context.Background()

	if r.Method == http.MethodGet {
		projectId := r.URL.Query().Get("projectId")
		if projectId == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "projectId query parameter is required"})
			return
		}
		url := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers/%s/apps/%s", projectId, email, appName)
		var app ApigeeApp
		if err := doApigeeRequest(ctx, "GET", url, nil, &app); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		mappedApp := mapApigeeAppToApp(app)
		mappedApp.ProjectId = projectId
		jsonResponse(w, http.StatusOK, mappedApp)
		return
	}

	if r.Method == http.MethodDelete {
		projectId := r.URL.Query().Get("projectId")
		if projectId == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "projectId query parameter is required"})
			return
		}
		url := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers/%s/apps/%s", projectId, email, appName)
		if err := doApigeeRequest(ctx, "DELETE", url, nil, nil); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == http.MethodPut {
		var app App
		if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}

		projectId := app.ProjectId
		if projectId == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "projectId property in payload is required"})
			return
		}

		url := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers/%s/apps/%s", projectId, email, appName)
		req := ApigeeAppUpdateReq{Name: appName}
		if app.Description != "" {
			req.Attributes = []ApigeeAttribute{{Name: "description", Value: app.Description}}
		}

		var updatedApp ApigeeApp
		if err := doApigeeRequest(ctx, "PUT", url, req, &updatedApp); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if len(app.Credentials) > 0 && len(updatedApp.Credentials) > 0 {
			cred := app.Credentials[0]
			consumerKey := updatedApp.Credentials[0].ConsumerKey
			keyUrl := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers/%s/apps/%s/keys/%s", projectId, email, appName, consumerKey)
			keyReq := ApigeeKeyUpdateReq{ApiProducts: cred.Products}
			var updatedKey ApigeeCredential
			if err := doApigeeRequest(ctx, "POST", keyUrl, keyReq, &updatedKey); err != nil {
				jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			updatedApp.Credentials[0] = updatedKey
		}

		mappedApp := mapApigeeAppToApp(updatedApp)
		mappedApp.ProjectId = projectId
		jsonResponse(w, http.StatusOK, mappedApp)
		return
	}

	jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func userAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	email := r.PathValue("email")
	if email == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}

	storefrontId := r.PathValue("storefrontId")
	if storefrontId == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "storefrontId is required"})
		return
	}

	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ctx := context.Background()

	// 1. Read storefront to find Apigee organizations (project IDs)
	storefrontPath := filepath.Join(dataDir, filepath.Base(filepath.Clean(storefrontId))+".json")
	sfData, err := os.ReadFile(storefrontPath)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "storefront not found"})
		return
	}

	var sf Storefront
	if err := json.Unmarshal(sfData, &sf); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "invalid storefront data"})
		return
	}

	apigeeOrgs := make(map[string]bool)
	for _, pgConfig := range sf.ProductGroups {
		pgPath := filepath.Join(productGroupsDir, filepath.Base(filepath.Clean(pgConfig.ProductGroupId))+".json")
		pgData, err := os.ReadFile(pgPath)
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
			if source.Type == "apigee" && source.Name != "" {
				apigeeOrgs[source.Name] = true
			}
		}
	}

	if len(apigeeOrgs) == 0 {
		// No Apigee sources found, return empty stats
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"app":     []interface{}{},
			"product": []interface{}{},
			"model":   []interface{}{},
		})
		return
	}

	// 2. Compute timeRange (last 3 months)
	now := time.Now().UTC()
	threeMonthsAgo := now.AddDate(0, -3, 0)
	timeRange := fmt.Sprintf("%02d/%02d/%04d %02d:%02d~%02d/%02d/%04d %02d:%02d",
		threeMonthsAgo.Month(), threeMonthsAgo.Day(), threeMonthsAgo.Year(), threeMonthsAgo.Hour(), threeMonthsAgo.Minute(),
		now.Month(), now.Day(), now.Year(), now.Hour(), now.Minute())

	escapedTimeRange := strings.Replace(url.QueryEscape(timeRange), "+", "%20", -1)
	escapedFilter := strings.Replace(url.QueryEscape(fmt.Sprintf("(developer_email eq '%s')", email)), "+", "%20", -1)

	var appStats []interface{}
	var productStats []interface{}
	var modelStats []interface{}

	// Loop over discovered Apigee Orgs
	for projectId := range apigeeOrgs {
		// Get all environments for this org
		envUrl := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/environments", projectId)
		var envs []string
		if err := doApigeeRequest(ctx, "GET", envUrl, nil, &envs); err != nil {
			log.Printf("Failed to get environments for org %s: %v", projectId, err)
			continue
		}

		for _, env := range envs {
			// 1. Stats by developer_app
			appStatsUrl := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/environments/%s/stats/developer_app?select=sum(message_count),sum(dc_ai_prompt_token_count),sum(dc_ai_response_token_count)&timeUnit=day&timeRange=%s&filter=%s",
				projectId, env, escapedTimeRange, escapedFilter)

			var appStatsResp map[string]interface{}
			if err := doApigeeRequest(ctx, "GET", appStatsUrl, nil, &appStatsResp); err != nil {
				log.Printf("Failed to get app stats for org %s env %s: %v", projectId, env, err)
			} else {
				delete(appStatsResp, "metaData")
				appStats = append(appStats, appStatsResp)
			}

			// 2. Stats by api_product
			prodStatsUrl := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/environments/%s/stats/api_product?select=sum(message_count),sum(dc_ai_prompt_token_count),sum(dc_ai_response_token_count)&timeUnit=day&timeRange=%s&filter=%s",
				projectId, env, escapedTimeRange, escapedFilter)

			var prodStatsResp map[string]interface{}
			if err := doApigeeRequest(ctx, "GET", prodStatsUrl, nil, &prodStatsResp); err != nil {
				log.Printf("Failed to get product stats for org %s env %s: %v", projectId, env, err)
			} else {
				delete(prodStatsResp, "metaData")
				productStats = append(productStats, prodStatsResp)
			}

			// 3. Stats by dc_ai_model
			modelStatsUrl := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/environments/%s/stats/dc_ai_model?select=sum(dc_ai_prompt_token_count),sum(dc_ai_response_token_count),avg(dc_ai_time_first_token)&timeUnit=day&timeRange=%s&filter=%s",
				projectId, env, escapedTimeRange, escapedFilter)

			var modelStatsResp map[string]interface{}
			if err := doApigeeRequest(ctx, "GET", modelStatsUrl, nil, &modelStatsResp); err != nil {
				log.Printf("Failed to get model stats for org %s env %s: %v", projectId, env, err)
			} else {
				delete(modelStatsResp, "metaData")
				modelStats = append(modelStats, modelStatsResp)
			}
		}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"app":     appStats,
		"product": productStats,
		"model":   modelStats,
	})
}
