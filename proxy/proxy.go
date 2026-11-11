// Package proxy resolves the proxy for outgoing HTTP requests from the
// subscription-manager proxy configuration in /etc/rhsm/rhsm.conf.
package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/RedHatInsights/host-metering/config"
	"golang.org/x/net/http/httpproxy"
)

// ProxyFunc returns the proxy resolver for outgoing HTTP requests. A proxy
// configured in /etc/rhsm/rhsm.conf takes priority, mirroring
// subscription-manager; HTTP_PROXY/HTTPS_PROXY in the service environment are
// consulted only when rhsm.conf configures no proxy. Targets matching the RHSM
// no_proxy list or the NO_PROXY environment variable, and localhost and
// loopback targets, are never sent to the rhsm.conf proxy.
func ProxyFunc(cfg *config.Config) func(*http.Request) (*url.URL, error) {
	if cfg.ProxyUrl == "" {
		return http.ProxyFromEnvironment
	}
	proxyUrl, err := url.Parse(cfg.ProxyUrl)
	if err != nil || proxyUrl.Host == "" {
		// Never fall back to the environment resolver here: that silently
		// sends traffic unproxied when rhsm.conf is misconfigured.
		return func(*http.Request) (*url.URL, error) {
			return nil, fmt.Errorf("invalid proxy URL %q", cfg.ProxyUrl)
		}
	}
	if cfg.ProxyUser != "" {
		if cfg.ProxyPassword != "" {
			proxyUrl.User = url.UserPassword(cfg.ProxyUser, cfg.ProxyPassword)
		} else {
			proxyUrl.User = url.User(cfg.ProxyUser)
		}
	}
	// httpproxy is the exported copy of the resolver net/http uses for
	// HTTP_PROXY/HTTPS_PROXY/NO_PROXY. Delegating keeps no_proxy matching
	// identical to the documented net/http semantics.
	envNoProxy := os.Getenv("NO_PROXY")
	if envNoProxy == "" {
		envNoProxy = os.Getenv("no_proxy")
	}
	resolve := (&httpproxy.Config{
		HTTPProxy:  proxyUrl.String(),
		HTTPSProxy: proxyUrl.String(),
		NoProxy:    strings.Trim(cfg.NoProxy+","+envNoProxy, ","),
	}).ProxyFunc()
	return func(req *http.Request) (*url.URL, error) {
		return resolve(req.URL)
	}
}
