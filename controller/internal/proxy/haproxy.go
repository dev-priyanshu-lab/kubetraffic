/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kubetraffic/controller/internal/model"
)

// HAProxy applies configuration through the HAProxy Data Plane API using the
// raw-configuration endpoint: the controller is the sole owner of the file, so
// it pushes the whole thing and lets the Data Plane API validate (haproxy -c)
// and perform a hitless reload.
type HAProxy struct {
	baseURL  string
	username string
	password string
	renderer *Renderer
	http     *http.Client
}

// HAProxyOptions configures a HAProxy proxy.
type HAProxyOptions struct {
	// BaseURL is the Data Plane API root, e.g. http://host:5555
	BaseURL  string
	Username string
	Password string
	Timeout  time.Duration
}

// NewHAProxy builds a HAProxy proxy client.
func NewHAProxy(opts HAProxyOptions) *HAProxy {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &HAProxy{
		baseURL:  strings.TrimRight(opts.BaseURL, "/"),
		username: opts.Username,
		password: opts.Password,
		renderer: NewRenderer(opts.Password),
		http:     &http.Client{Timeout: timeout},
	}
}

// GenerateConfig renders m to HAProxy configuration text.
func (h *HAProxy) GenerateConfig(m model.RoutingModel) (Config, error) {
	return h.renderer.Render(m)
}

// ValidateConfig posts the config with only_validate=true.
func (h *HAProxy) ValidateConfig(ctx context.Context, c Config) error {
	version, err := h.currentVersion(ctx)
	if err != nil {
		return err
	}
	return h.pushRaw(ctx, c.Raw, version, url.Values{"only_validate": {"true"}})
}

// ApplyConfig validates and applies c, unless the live config already matches.
func (h *HAProxy) ApplyConfig(ctx context.Context, c Config) error {
	current, version, err := h.currentConfig(ctx)
	if err != nil {
		return err
	}
	if normalize(current) == normalize(c.Raw) {
		return nil
	}
	if err := h.pushRaw(ctx, c.Raw, version, url.Values{"skip_reload": {"false"}}); err != nil {
		return err
	}
	return nil
}

func (h *HAProxy) currentVersion(ctx context.Context) (int, error) {
	_, v, err := h.currentConfig(ctx)
	return v, err
}

func (h *HAProxy) currentConfig(ctx context.Context) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		h.baseURL+"/v3/services/haproxy/configuration/raw", nil)
	if err != nil {
		return "", 0, err
	}
	req.SetBasicAuth(h.username, h.password)

	resp, err := h.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("get raw config: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("get raw config: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// v3 returns {"_version": N, "data": "..."}. Some builds return the config
	// as text with the version in the Configuration-Version header.
	var wrapped struct {
		Version int    `json:"_version"`
		Data    string `json:"data"`
	}
	if json.Unmarshal(body, &wrapped) == nil && wrapped.Data != "" {
		return wrapped.Data, wrapped.Version, nil
	}
	if v := resp.Header.Get("Configuration-Version"); v != "" {
		n, _ := strconv.Atoi(v)
		return string(body), n, nil
	}
	return string(body), 0, nil
}

func (h *HAProxy) pushRaw(ctx context.Context, cfg string, version int, extra url.Values) error {
	q := url.Values{"version": {strconv.Itoa(version)}}
	for k, vs := range extra {
		for _, v := range vs {
			q.Set(k, v)
		}
	}
	u := h.baseURL + "/v3/services/haproxy/configuration/raw?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBufferString(cfg))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.SetBasicAuth(h.username, h.password)

	resp, err := h.http.Do(req)
	if err != nil {
		return fmt.Errorf("push raw config: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusConflict:
		return fmt.Errorf("push raw config: version conflict (had %d): %s", version, snippet(body))
	default:
		return fmt.Errorf("push raw config: status %d: %s", resp.StatusCode, snippet(body))
	}
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		s = s[:400] + "…"
	}
	return s
}

// normalize makes two configs comparable despite trailing-whitespace and
// blank-line differences the Data Plane API may introduce.
func normalize(cfg string) string {
	lines := strings.Split(cfg, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimRight(l, " \t")
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
