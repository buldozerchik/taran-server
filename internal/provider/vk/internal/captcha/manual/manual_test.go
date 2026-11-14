package manual

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPowCaptureHandlerRequiresSecretPath(t *testing.T) {
	const path = "/local-captcha-pow/test-secret"
	pow := "v2." + base64.StdEncoding.EncodeToString([]byte(`{"hash":"fixture"}`))
	keyCh := make(chan string, 1)
	handler := powCaptureHandler(path, keyCh)

	wrongPath := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/local-captcha-pow/wrong-secret", strings.NewReader("pow="+url.QueryEscape(pow)))
	wrongPath.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wrongPathResponse := httptest.NewRecorder()
	handler.ServeHTTP(wrongPathResponse, wrongPath)
	if wrongPathResponse.Code != http.StatusNotFound {
		t.Fatalf("wrong path status = %d, want %d", wrongPathResponse.Code, http.StatusNotFound)
	}

	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader("pow="+url.QueryEscape(pow)))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("valid request status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want no CORS allow header", got)
	}
	select {
	case got := <-keyCh:
		if got != pow {
			t.Fatalf("captured PoW = %q, want %q", got, pow)
		}
	default:
		t.Fatal("valid request did not deliver PoW")
	}
}

func TestCaptchaResultRejectsUntrustedCallbacks(t *testing.T) {
	const path = "/local-captcha-result/test-secret"
	tests := []struct {
		name, method, path, host, origin, body string
		want                                   int
	}{
		{"valid", http.MethodPost, path, "localhost:8765", localCaptchaOrigin(), "token=fixture", http.StatusOK},
		{"wrong path", http.MethodPost, path + "-wrong", "localhost:8765", localCaptchaOrigin(), "token=fixture", http.StatusNotFound},
		{"wrong host", http.MethodPost, path, "evil.example:8765", localCaptchaOrigin(), "token=fixture", http.StatusNotFound},
		{"wrong origin", http.MethodPost, path, "localhost:8765", "https://evil.example", "token=fixture", http.StatusNotFound},
		{"missing origin", http.MethodPost, path, "localhost:8765", "", "token=fixture", http.StatusNotFound},
		{"wrong method", http.MethodGet, path, "localhost:8765", localCaptchaOrigin(), "token=fixture", http.StatusMethodNotAllowed},
		{"missing token", http.MethodPost, path, "localhost:8765", localCaptchaOrigin(), "", http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens := make(chan string, 1)
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, strings.NewReader(tc.body))
			req.Host = tc.host
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			res := httptest.NewRecorder()
			captchaResultHandler(path, tokens).ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("status = %d, want %d", res.Code, tc.want)
			}
			if res.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("callback must not enable cross-origin access")
			}
			if tc.want == http.StatusOK {
				select {
				case token := <-tokens:
					if token != "fixture" {
						t.Fatalf("token = %q", token)
					}
				default:
					t.Fatal("valid callback did not deliver token")
				}
			} else if len(tokens) != 0 {
				t.Fatal("rejected callback delivered a token")
			}
		})
	}
}

func TestPathProxyPreservesRelativeChunkDirectory(t *testing.T) {
	const asset = "https://sun9-1.userapi.com/captcha/assets/main.js?v=1"
	proxied := pathProxyURL(asset)
	base, err := url.Parse("http://localhost:8765" + proxied)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := url.Parse("./42.js")
	if err != nil {
		t.Fatal(err)
	}
	if got := base.ResolveReference(chunk).RequestURI(); got != "/_p/sun9-1.userapi.com/captcha/assets/42.js" {
		t.Fatalf("relative chunk resolved to %q", got)
	}
	target, err := url.Parse("https://id.vk.ru/not_robot_captcha")
	if err != nil {
		t.Fatal(err)
	}
	if html := rewriteHTMLAttrsServerSide(`<script src="`+asset+`"></script>`, target); !strings.Contains(html, proxied) {
		t.Fatalf("HTML did not use path-preserving proxy: %s", html)
	}
}

func TestPowCaptureHandlerRejectsInvalidRequests(t *testing.T) {
	const path = "/local-captcha-pow/test-secret"
	handler := powCaptureHandler(path, make(chan string, 1))

	tests := []struct {
		name   string
		method string
		body   string
		want   int
	}{
		{name: "wrong method", method: http.MethodGet, want: http.StatusMethodNotAllowed},
		{name: "malformed envelope", method: http.MethodPost, body: "pow=not-base64", want: http.StatusBadRequest},
		{name: "missing envelope", method: http.MethodPost, want: http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), tc.method, path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("status = %d, want %d", response.Code, tc.want)
			}
		})
	}
}

func TestRunCaptchaServerPresentsAfterListen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	keyCh := make(chan string, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ready")
	})
	present := func(captchaURL string) {
		response, err := http.Get(captchaURL) //nolint:gosec,noctx // local test server
		if err != nil {
			t.Errorf("present called before server was ready: %v", err)
			keyCh <- "failed"
			return
		}
		_ = response.Body.Close()
		keyCh <- "token"
	}

	got, err := runCaptchaServerAndWait(ctx, handler, localCaptchaOrigin(), keyCh, "test", present)
	if err != nil {
		t.Fatal(err)
	}
	if got != "token" {
		t.Fatalf("token = %q, want token", got)
	}
}

func TestRewriteProxyRedirectLocation(t *testing.T) {
	t.Parallel()

	targetURL, err := url.Parse("https://id.vk.ru/captcha")
	if err != nil {
		t.Fatalf("failed to parse target URL: %v", err)
	}

	testCases := []struct {
		name     string
		location string
		want     string
		ok       bool
	}{
		{
			name:     "keeps safe relative path",
			location: "/captcha?step=2",
			want:     "/captcha?step=2",
			ok:       true,
		},
		{
			name:     "rewrites same-origin absolute URL",
			location: "https://id.vk.ru/captcha?step=2",
			want:     "http://localhost:8765/captcha?step=2",
			ok:       true,
		},
		{
			name:     "blocks scheme-relative redirect",
			location: "//evil.example/captcha",
			ok:       false,
		},
		{
			name:     "blocks slash-backslash redirect",
			location: `/\evil.example/captcha`,
			ok:       false,
		},
		{
			name:     "blocks lookalike absolute host",
			location: "https://id.vk.ru.evil.example/captcha",
			ok:       false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := rewriteProxyRedirectLocation(tc.location, targetURL)
			if ok != tc.ok {
				t.Fatalf("rewriteProxyRedirectLocation() ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("rewriteProxyRedirectLocation() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsAllowedProxyHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hostname string
		want     bool
	}{
		{"id.vk.ru", true},
		{"api.vk.com", true},
		{"vk.com", true},
		{"sun9-1.userapi.com", true},
		{"ads.vk.com", false},
		{"ads.vk.ru", false},
		{"top-fwz1.mail.ru", false},
		{"sdk-api.apptracer.ru", false},
		{"stats.vk-portal.net", false},
		{"evil.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.hostname, func(t *testing.T) {
			t.Parallel()

			if got := isAllowedProxyHost(tc.hostname); got != tc.want {
				t.Fatalf("isAllowedProxyHost(%q) = %v, want %v", tc.hostname, got, tc.want)
			}
		})
	}
}
