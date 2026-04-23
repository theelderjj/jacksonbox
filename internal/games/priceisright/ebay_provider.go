package priceisright

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jj/trivia/internal/engine"
)

const (
	ebayOAuthURL  = "https://api.ebay.com/identity/v1/oauth2/token"
	ebayBrowseURL = "https://api.ebay.com/buy/browse/v1/item_summary/search"
)

var ebayAuthState struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

type ebayTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type ebaySearchResponse struct {
	ItemSummaries []struct {
		ItemID        string   `json:"itemId"`
		Title         string   `json:"title"`
		BuyingOptions []string `json:"buyingOptions"`
		Image         struct {
			ImageURL string `json:"imageUrl"`
		} `json:"image"`
		Price struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"price"`
	} `json:"itemSummaries"`
}

func ebayProduct(ctx context.Context, state *engine.GameState, round, thresholdCents int) (Product, bool) {
	clientID := strings.TrimSpace(os.Getenv("EBAY_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("EBAY_CLIENT_SECRET"))
	if clientID == "" || clientSecret == "" {
		return Product{}, false
	}

	token, err := ebayToken(ctx, clientID, clientSecret)
	if err != nil {
		return Product{}, false
	}

	keywords := ebayKeywords()
	start := hashSeed(state.RoomID, round) % len(keywords)
	httpClient := &http.Client{Timeout: 8 * time.Second}
	for offset := 0; offset < len(keywords); offset++ {
		keyword := keywords[(start+offset)%len(keywords)]
		items, err := ebaySearch(ctx, httpClient, token, keyword)
		if err != nil {
			continue
		}
		filtered := make([]Product, 0, len(items))
		for _, item := range items {
			if item.PriceCents <= thresholdCents {
				continue
			}
			filtered = append(filtered, item)
		}
		if len(filtered) == 0 {
			continue
		}
		index := hashSeed(state.RoomID+keyword, round) % len(filtered)
		return filtered[index], true
	}
	return Product{}, false
}

func ebayToken(ctx context.Context, clientID, clientSecret string) (string, error) {
	ebayAuthState.mu.Lock()
	defer ebayAuthState.mu.Unlock()

	if ebayAuthState.token != "" && time.Until(ebayAuthState.expiresAt) > 30*time.Second {
		return ebayAuthState.token, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", "https://api.ebay.com/oauth/api_scope")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ebayOAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))

	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("ebay oauth failed: %s", strings.TrimSpace(string(body)))
	}

	var payload ebayTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("missing ebay access token")
	}
	ebayAuthState.token = payload.AccessToken
	ebayAuthState.expiresAt = time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second)
	return ebayAuthState.token, nil
}

func ebaySearch(ctx context.Context, client *http.Client, token, keyword string) ([]Product, error) {
	query := url.Values{}
	query.Set("q", keyword)
	query.Set("limit", "50")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ebayBrowseURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-EBAY-C-MARKETPLACE-ID", "EBAY_US")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("ebay search failed: %s", strings.TrimSpace(string(body)))
	}

	var payload ebaySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]Product, 0, len(payload.ItemSummaries))
	for _, item := range payload.ItemSummaries {
		if item.ItemID == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Image.ImageURL) == "" {
			continue
		}
		if !containsFixedPrice(item.BuyingOptions) {
			continue
		}
		value, err := strconv.ParseFloat(item.Price.Value, 64)
		if err != nil || item.Price.Currency != "USD" {
			continue
		}
		out = append(out, Product{
			ID:         item.ItemID,
			Name:       item.Title,
			PriceCents: int(value * 100),
			ImageURL:   item.Image.ImageURL,
			Accent:     "#cbd5e1",
		})
	}
	return out, nil
}

func containsFixedPrice(options []string) bool {
	for _, option := range options {
		if option == "FIXED_PRICE" {
			return true
		}
	}
	return false
}

func ebayKeywords() []string {
	return []string{
		"camera",
		"drone",
		"monitor",
		"chair",
		"espresso machine",
		"bike",
		"projector",
		"gaming laptop",
		"headphones",
		"blender",
		"safe",
		"telescope",
	}
}
