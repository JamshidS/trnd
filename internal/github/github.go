// Package github wraps the few GitHub REST API calls the app needs.
package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var apiURL = "https://api.github.com"

// ErrNoToken is returned by calls that require authentication.
var ErrNoToken = errors.New("no GitHub token: set GITHUB_TOKEN or run `gh auth login`")

// Token returns a GitHub token from GITHUB_TOKEN, GH_TOKEN, or the gh CLI.
func Token() string {
	for _, k := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if t := strings.TrimSpace(os.Getenv(k)); t != "" {
			return t
		}
	}
	if _, err := exec.LookPath("gh"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// Client is a minimal GitHub API client.
type Client struct {
	HTTP  *http.Client
	Token string
}

// NewClient returns a Client with a sane timeout.
func NewClient(token string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, Token: token}
}

func (c *Client) do(ctx context.Context, method, path, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, apiURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return c.HTTP.Do(req)
}

// Readme returns the raw README markdown of owner/name.
func (c *Client) Readme(ctx context.Context, fullName string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/repos/"+fullName+"/readme", "application/vnd.github.raw")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", errors.New("this repository has no README")
	case http.StatusForbidden, http.StatusTooManyRequests:
		return "", errors.New("GitHub API rate limit hit; set GITHUB_TOKEN for higher limits")
	default:
		return "", fmt.Errorf("fetching README: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return string(b), err
}

// Star stars owner/name for the authenticated user.
func (c *Client) Star(ctx context.Context, fullName string) error {
	if c.Token == "" {
		return ErrNoToken
	}
	resp, err := c.do(ctx, http.MethodPut, "/user/starred/"+fullName, "application/vnd.github+json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("starring %s: %s", fullName, resp.Status)
	}
	return nil
}
