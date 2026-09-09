/*
Copyright 2026 The KubeTraffic Authors.
SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/kubetraffic/controller/internal/model"
)

// BaseConfig is the controller-owned skeleton. Every render reproduces it in
// full: a raw-config push to the Data Plane API replaces the entire file, so the
// global/program/userlist/defaults sections must always be present or we would
// lose the Data Plane API itself on the next reload.
//
// {{DATAPLANE_PASSWORD}} is substituted at construction time from the mounted
// credential so the literal is never baked into the binary or an image layer.
const BaseConfig = `global
    master-worker
    log stdout format raw local0 info
    stats socket /var/run/haproxy.sock mode 660 level admin expose-fd listeners
    hard-stop-after 15s

userlist haproxy-dataplaneapi
    user admin insecure-password {{DATAPLANE_PASSWORD}}

program api
    command dataplaneapi --host 0.0.0.0 --port 5555 --haproxy-bin /usr/local/sbin/haproxy --config-file /usr/local/etc/haproxy/haproxy.cfg --reload-cmd /usr/local/etc/haproxy/reload.sh --restart-cmd /usr/local/etc/haproxy/restart.sh --reload-delay 5 --transaction-dir /tmp/haproxy --userlist haproxy-dataplaneapi --scheme http
    no option start-on-reload

defaults
    mode http
    log global
    option httplog
    option dontlognull
    retries 2
    timeout connect 5s
    timeout client 30s
    timeout server 30s
    timeout http-request 10s

frontend stats
    bind :8404
    http-request use-service prometheus-exporter if { path /metrics }
    stats enable
    stats uri /stats
    stats refresh 10s

`

var configTmpl = template.Must(template.New("haproxy").Funcs(template.FuncMap{}).Parse(
	`frontend kubetraffic
    bind :8080
    http-request set-var(txn.host) req.hdr(host),lower,field(1,:)
{{- range .Rules }}
    use_backend {{ .BackendName }} if { var(txn.host) -m str {{ $.Host }} } { path -m beg {{ .Path }} }
{{- end }}
    default_backend kubetraffic_no_route

backend kubetraffic_no_route
    http-request return status 503 content-type "text/plain" string "kubetraffic: no route configured\n"
{{ range .Rules }}
backend {{ .BackendName }}
    balance roundrobin
    option httpchk GET /healthz
{{- range .Servers }}
    server {{ .Name }} {{ .Address }}:{{ .Port }} weight {{ .Weight }} check inter 5s fall 3 rise 2
{{- end }}
{{ end -}}
`))

// Renderer produces HAProxy configuration text from a RoutingModel.
type Renderer struct {
	dataplanePassword string
}

// NewRenderer returns a Renderer that injects the given Data Plane API password
// into the base config.
func NewRenderer(dataplanePassword string) *Renderer {
	return &Renderer{dataplanePassword: dataplanePassword}
}

// Render produces the full configuration for m.
func (r *Renderer) Render(m model.RoutingModel) (Config, error) {
	if m.Host == "" {
		return Config{}, fmt.Errorf("routing model has no host")
	}

	// Stable ordering so identical models render byte-identical configs.
	rules := append([]model.Rule(nil), m.Rules...)
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Path != rules[j].Path {
			// longest path first: more specific prefixes win in HAProxy order
			return len(rules[i].Path) > len(rules[j].Path)
		}
		return rules[i].BackendName < rules[j].BackendName
	})

	var body strings.Builder
	if err := configTmpl.Execute(&body, model.RoutingModel{Host: m.Host, Rules: rules}); err != nil {
		return Config{}, fmt.Errorf("render haproxy config: %w", err)
	}

	base := strings.ReplaceAll(BaseConfig, "{{DATAPLANE_PASSWORD}}", r.dataplanePassword)
	return Config{Raw: base + body.String()}, nil
}
