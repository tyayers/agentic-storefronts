package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type LoginRequest struct {
	StorefrontId string `json:"storefrontId"`
}

func userLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	email := r.PathValue("email")
	if email == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	if req.StorefrontId == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "storefrontId is required"})
		return
	}

	go processApigeeUserRegistration(email, req.StorefrontId)

	jsonResponse(w, http.StatusOK, map[string]string{"status": "success"})
}

func processApigeeUserRegistration(email, storefrontId string) {
	ctx := context.Background()

	// Load storefront
	sfFile := filepath.Join(dataDir, storefrontId+".json")
	sfData, err := os.ReadFile(sfFile)
	if err != nil {
		log.Printf("processApigeeUserRegistration: failed to read storefront %s: %v", storefrontId, err)
		return
	}

	var sf Storefront
	if err := json.Unmarshal(sfData, &sf); err != nil {
		log.Printf("processApigeeUserRegistration: failed to parse storefront %s: %v", storefrontId, err)
		return
	}

	// Collect unique Apigee organizations (project IDs)
	apigeeOrgs := make(map[string]bool)
	for _, pgConfig := range sf.ProductGroups {
		pgFile := filepath.Join(productGroupsDir, pgConfig.ProductGroupId+".json")
		pgData, err := os.ReadFile(pgFile)
		if err != nil {
			log.Printf("processApigeeUserRegistration: failed to read product group %s: %v", pgConfig.ProductGroupId, err)
			continue
		}

		var pg ProductGroup
		if err := json.Unmarshal(pgData, &pg); err != nil {
			continue
		}

		for _, source := range pg.Sources {
			if source.Type == "apigee" && source.Name != "" {
				apigeeOrgs[source.Name] = true
			}
		}
	}

	firstName := email
	if idx := strings.Index(email, "@"); idx != -1 {
		firstName = email[:idx]
	}

	devPayload := map[string]string{
		"email":     email,
		"firstName": firstName,
		"lastName":  "User",
		"userName":  firstName,
	}

	for org := range apigeeOrgs {
		url := fmt.Sprintf("https://apigee.googleapis.com/v1/organizations/%s/developers", org)
		var res interface{}
		err := doApigeeRequest(ctx, "POST", url, devPayload, &res)
		if err != nil {
			// A 409 Conflict indicates the developer already exists, which is acceptable.
			log.Printf("processApigeeUserRegistration: developer %s check/create in org %s: %v", email, org, err)
		} else {
			log.Printf("processApigeeUserRegistration: created apigee developer %s in org %s", email, org)
		}
	}
}
