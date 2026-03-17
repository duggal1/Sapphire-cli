package gemini

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"charm.land/fantasy"
)

type callUAKey struct{}

func withCallUA(ctx context.Context, call fantasy.Call) context.Context {
	if ua, ok := callUserAgent(call.UserAgent); ok {
		return context.WithValue(ctx, callUAKey{}, ua)
	}
	return ctx
}

func withObjectCallUA(ctx context.Context, call fantasy.ObjectCall) context.Context {
	if ua, ok := callUserAgent(call.UserAgent); ok {
		return context.WithValue(ctx, callUAKey{}, ua)
	}
	return ctx
}

func defaultUserAgent(version string) string {
	return fmt.Sprintf("Charm-Fantasy/%s (https://charm.land/fantasy)", version)
}

func resolveHeaders(headers map[string]string, explicitUA, defaultUA string) map[string]string {
	out := make(map[string]string, len(headers)+1)
	var uaKeys []string

	for k, v := range headers {
		out[k] = v
		if strings.EqualFold(k, "User-Agent") {
			uaKeys = append(uaKeys, k)
		}
	}

	switch {
	case explicitUA != "":
		for _, k := range uaKeys {
			delete(out, k)
		}
		out["User-Agent"] = explicitUA
	case len(uaKeys) > 0:
		val := out[uaKeys[0]]
		for _, k := range uaKeys {
			delete(out, k)
		}
		out["User-Agent"] = val
	default:
		out["User-Agent"] = defaultUA
	}

	return out
}

func callUserAgent(callUA string) (string, bool) {
	if callUA != "" {
		return callUA, true
	}
	return "", false
}

func wrapHTTPClient(c *http.Client) *http.Client {
	if c == nil {
		c = http.DefaultClient
	}
	transport := c.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		Transport:     &uaTransport{base: transport},
		CheckRedirect: c.CheckRedirect,
		Jar:           c.Jar,
		Timeout:       c.Timeout,
	}
}

type uaTransport struct {
	base http.RoundTripper
}

func (t *uaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if ua, ok := req.Context().Value(callUAKey{}).(string); ok && ua != "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", ua)
	}
	return t.base.RoundTrip(req)
}
