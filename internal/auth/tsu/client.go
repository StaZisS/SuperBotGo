package tsu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type ExchangeResult struct {
	AccessToken string `json:"accessToken"`
	AccountID   string `json:"accountId"`
}

type Client struct {
	httpClient    *http.Client
	applicationID string
	secretKey     string
	baseURL       string // e.g. "https://accounts.tsu.ru"
}

func NewClient(httpClient *http.Client, applicationID, secretKey, baseURL string) *Client {
	return &Client{
		httpClient:    httpClient,
		applicationID: applicationID,
		secretKey:     secretKey,
		baseURL:       strings.TrimRight(baseURL, "/"),
	}
}

// ExchangeToken exchanges a temporary token for an AccessToken and AccountId.
func (c *Client) ExchangeToken(ctx context.Context, tempToken string) (*ExchangeResult, error) {
	form := url.Values{
		"token":         {tempToken},
		"applicationId": {c.applicationID},
		"secretKey":     {c.secretKey},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/Account/",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exchange token: status %d, body: %s", resp.StatusCode, body)
	}

	var result ExchangeResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.AccountID == "" {
		return nil, fmt.Errorf("exchange token: empty account ID in response")
	}

	return &result, nil
}

// LoginURL returns the TSU accounts login page URL for the configured application.
func (c *Client) LoginURL() string {
	return c.baseURL + "/Account/Login2/?applicationId=" + url.QueryEscape(c.applicationID)
}
