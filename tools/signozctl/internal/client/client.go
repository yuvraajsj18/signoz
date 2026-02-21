package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	signozerrors "github.com/SigNoz/signoz/tools/signozctl/internal/errors"
)

type Client struct {
	BaseURL    string
	Token      string
	Refresh    string
	OnRotated  func(accessToken, refreshToken string) error
	HTTPClient *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) PostJSON(ctx context.Context, path string, payload any, out any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) PostRawJSON(ctx context.Context, path string, raw []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) PutRawJSON(ctx context.Context, path string, raw []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) Delete(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.doOnce(req, out, c.Token)
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}

	if resp.StatusCode == http.StatusUnauthorized && c.Token != "" && c.Refresh != "" {
		if rotated, rotateErr := c.rotateToken(req.Context()); rotateErr == nil && rotated {
			return c.do(req, out)
		}
	}
	return c.handleResponse(resp, out)
}

func (c *Client) doOnce(req *http.Request, out any, token string) (*http.Response, error) {
	r := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		r.Body = body
	} else {
		r.Body = req.Body
	}

	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}

	return c.HTTPClient.Do(r)
}

func (c *Client) rotateToken(ctx context.Context) (bool, error) {
	payload := map[string]string{"refreshToken": c.Refresh}
	var resp struct {
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		} `json:"data"`
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v2/sessions/rotate", bytes.NewReader(reqBody))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	httpResp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer httpResp.Body.Close()
	b, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return false, signozerrors.ParseAPIError(httpResp.StatusCode, b)
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return false, fmt.Errorf("failed to decode token rotate response: %w", err)
	}
	if resp.Data.AccessToken == "" || resp.Data.RefreshToken == "" {
		return false, fmt.Errorf("token rotate response missing tokens")
	}

	c.Token = resp.Data.AccessToken
	c.Refresh = resp.Data.RefreshToken
	if c.OnRotated != nil {
		if err := c.OnRotated(c.Token, c.Refresh); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (c *Client) handleResponse(resp *http.Response, out any) error {
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return signozerrors.ParseAPIError(resp.StatusCode, b)
	}
	if out == nil || len(b) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}
