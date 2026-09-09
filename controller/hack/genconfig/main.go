// Command genconfig prints the HAProxy bootstrap config the controller expects
// (zero routes). Keep deployments/haproxy/configmap.yaml in sync with its output.
package main

import (
	"flag"
	"fmt"

	"github.com/kubetraffic/controller/internal/model"
	"github.com/kubetraffic/controller/internal/proxy"
)

func main() {
	pw := flag.String("password", "kubetraffic-dev-not-secret", "data plane API password")
	flag.Parse()
	cfg, err := proxy.NewRenderer(*pw).Render(model.RoutingModel{Host: "bootstrap.local"})
	if err != nil {
		panic(err)
	}
	fmt.Print(cfg.Raw)
}
