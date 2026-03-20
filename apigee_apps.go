package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	if email == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}

	projectId := os.Getenv("PROJECT_ID")
	ctx := context.Background()

	if r.Method == http.MethodGet {
		url := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers/%s/apps?expand=true", projectId, email)
		var list ApigeeAppList
		if err := doApigeeRequest(ctx, "GET", url, nil, &list); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		apps := make([]App, 0)
		for _, a := range list.App {
			apps = append(apps, mapApigeeAppToApp(a))
		}
		jsonResponse(w, http.StatusOK, apps)
		return
	}

	if r.Method == http.MethodPost {
		var app App
		if err := json.NewDecoder(r.Body).Decode(&app); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
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

		jsonResponse(w, http.StatusCreated, mapApigeeAppToApp(created))
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

	projectId := os.Getenv("PROJECT_ID")
	ctx := context.Background()

	if r.Method == http.MethodGet {
		url := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers/%s/apps/%s", projectId, email, appName)
		var app ApigeeApp
		if err := doApigeeRequest(ctx, "GET", url, nil, &app); err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		jsonResponse(w, http.StatusOK, mapApigeeAppToApp(app))
		return
	}

	if r.Method == http.MethodDelete {
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

		jsonResponse(w, http.StatusOK, mapApigeeAppToApp(updatedApp))
		return
	}

	jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}
