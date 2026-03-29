package license

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiURL    = "https://api.lemonsqueezy.com/v1/licenses"
	storeID   = 329375
	productID = 928458
)

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type APIResponse struct {
	Activated  bool      `json:"activated"`
	Valid      bool      `json:"valid"`
	Error      *string   `json:"error"`
	LicenseKey Info      `json:"license_key"`
	Instance   *Instance `json:"instance"`
	Meta       Meta      `json:"meta"`
}

type Info struct {
	ID              int     `json:"id"`
	Status          string  `json:"status"`
	Key             string  `json:"key"`
	ActivationLimit int     `json:"activation_limit"`
	ActivationUsage int     `json:"activation_usage"`
	CreatedAt       string  `json:"created_at"`
	ExpiresAt       *string `json:"expires_at"`
}

type Instance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type Meta struct {
	StoreID       int    `json:"store_id"`
	ProductID     int    `json:"product_id"`
	ProductName   string `json:"product_name"`
	VariantID     int    `json:"variant_id"`
	VariantName   string `json:"variant_name"`
	CustomerID    int    `json:"customer_id"`
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
}

func (c *Client) Activate(licenseKey, instanceName string) (*APIResponse, error) {
	data := url.Values{
		"license_key":   {licenseKey},
		"instance_name": {instanceName},
	}

	resp, err := c.post(apiURL+"/activate", data)
	if err != nil {
		return nil, err
	}

	if err := c.verifyProduct(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) Validate(licenseKey, instanceID string) (*APIResponse, error) {
	data := url.Values{
		"license_key": {licenseKey},
		"instance_id": {instanceID},
	}

	resp, err := c.post(apiURL+"/validate", data)
	if err != nil {
		return nil, err
	}

	if err := c.verifyProduct(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) post(endpoint string, data url.Values) (*APIResponse, error) {
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}(resp.Body)

	var apiResp APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Error != nil {
		return &apiResp, fmt.Errorf("API error: %s", *apiResp.Error)
	}

	return &apiResp, nil
}

func (c *Client) verifyProduct(resp *APIResponse) error {
	if resp.Meta.StoreID != storeID {
		return fmt.Errorf("invalid store ID: %d", resp.Meta.StoreID)
	}
	if resp.Meta.ProductID != productID {
		return fmt.Errorf("invalid product ID: %d", resp.Meta.ProductID)
	}
	return nil
}
