package main

import (
	"time"

	"golang.org/x/oauth2"
)

type UserSessionData struct {
	Email             string        `json:"email"`
	Name              string        `json:"name"`
	Picture           string        `json:"picture"`
	AuthorizationCode string        `json:"authorization_code"`
	Token             *oauth2.Token `json:"token"`
	IDToken           string        `json:"id_token"`
}

type Product struct {
	Id                 string `json:"id"`
	Name               string `json:"name"`
	DisplayName        string `json:"displayName,omitempty"`
	Description        string `json:"description,omitempty"`
	DisplayDescription string `json:"displayDescription,omitempty"`
	Endpoint           string `json:"endpoint,omitempty"`
	SpecContents       string `json:"specContents,omitempty"`
	Type               string `json:"type,omitempty"`
	AuthType           string `json:"authType,omitempty"`
	Style              string `json:"style,omitempty"`
	DisplayStyle       string `json:"displayStyle,omitempty"`
	Image              string `json:"image,omitempty"`
}

type SelectedProduct struct {
	Id                 string   `json:"id"`
	DisplayName        string   `json:"displayName,omitempty"`
	DisplayDescription string   `json:"displayDescription,omitempty"`
	DisplayStyle       string   `json:"displayStyle,omitempty"`
	Image              string   `json:"image,omitempty"`
	Categories         []string `json:"categories"`
	Tags               []string `json:"tags"`
}

type Source struct {
	Name              string            `json:"name"`
	Type              string            `json:"type"`
	Region            string            `json:"region"`
	Autodetect        bool              `json:"autodetect"`
	SelectedProducts  []SelectedProduct `json:"selectedProducts"`
	AllUsers          bool              `json:"allUsers"`
	SelectedAudiences []string          `json:"selectedAudiences"`
}

type Qualifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Audience struct {
	Id          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Qualifiers  []Qualifier `json:"qualifiers"`
}

type ProductGroup struct {
	Id       string   `json:"id"`
	Name     string   `json:"name"`
	Taxonomy string   `json:"taxonomy"`
	Sources  []Source `json:"sources"`
}

type ProductGroupConfig struct {
	ProductGroupId    string   `json:"productGroupId"`
	AllUsers          bool     `json:"allUsers"`
	SelectedAudiences []string `json:"selectedAudiences"`
}

type Storefront struct {
	Id            string               `json:"id"`
	Name          string               `json:"name"`
	AuthType      string               `json:"authType"`
	AuthApiKey    *string              `json:"authApiKey"`
	AuthDomain    *string              `json:"authDomain"`
	ThemeId       string               `json:"themeId"`
	SupportMcp    bool                 `json:"supportMcp"`
	ProductGroups []ProductGroupConfig `json:"productGroups"`
}

type Category struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Tag struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Taxonomy struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
	Tags       []string `json:"tags"`
}

type Theme struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
	GithubRepo string   `json:"githubRepo"`
	Images     []string `json:"images"`
}

type apigeeCacheEntry struct {
	Data      []Product
	Timestamp time.Time
}
