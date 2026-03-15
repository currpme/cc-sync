package webdav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
)

type Client struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

func New(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		client:   &http.Client{},
	}
}

func (c *Client) join(remotePath string) string {
	return c.baseURL + "/" + strings.TrimLeft(remotePath, "/")
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	return c.client.Do(req)
}

func (c *Client) Stat(ctx context.Context, remotePath string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.join(remotePath), nil)
	if err != nil {
		return false, err
	}
	resp, err := c.do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Errorf("stat %s: %s", remotePath, resp.Status)
	}
	return true, nil
}

func (c *Client) ReadFile(ctx context.Context, remotePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.join(remotePath), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not found")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("get %s: %s", remotePath, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) EnsureDir(ctx context.Context, remotePath string) error {
	parts := strings.Split(strings.Trim(strings.TrimSpace(remotePath), "/"), "/")
	cur := ""
	for _, part := range parts {
		cur = path.Join(cur, part)
		req, err := http.NewRequestWithContext(ctx, "MKCOL", c.join(cur), nil)
		if err != nil {
			return err
		}
		resp, err := c.do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusConflict {
			continue
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("mkcol %s: %s", cur, resp.Status)
		}
	}
	return nil
}

func (c *Client) WriteFile(ctx context.Context, remotePath string, data []byte) error {
	dir := path.Dir(remotePath)
	if dir != "." {
		if err := c.EnsureDir(ctx, dir); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.join(remotePath), bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("put %s: %s", remotePath, resp.Status)
	}
	return nil
}

func (c *Client) DeleteFile(ctx context.Context, remotePath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.join(remotePath), nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete %s: %s", remotePath, resp.Status)
	}
	return nil
}

type propfindResponse struct {
	XMLName   xml.Name       `xml:"multistatus"`
	Responses []responseNode `xml:"response"`
}

type responseNode struct {
	Href string `xml:"href"`
}

func (c *Client) List(ctx context.Context, remotePath string) ([]string, error) {
	body := `<?xml version="1.0" encoding="utf-8" ?><propfind xmlns="DAV:"><allprop/></propfind>`
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", c.join(remotePath), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "infinity")
	req.Header.Set("Content-Type", "application/xml")
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("propfind %s: %s", remotePath, resp.Status)
	}
	var parsed propfindResponse
	if err := xml.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	prefix := strings.TrimRight(c.join(remotePath), "/")
	out := make([]string, 0, len(parsed.Responses))
	for _, respNode := range parsed.Responses {
		href := strings.TrimRight(respNode.Href, "/")
		href = strings.TrimPrefix(href, prefix)
		href = strings.TrimPrefix(href, "/")
		if href == "" {
			continue
		}
		out = append(out, href)
	}
	return out, nil
}
