package publication

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func responseBytes(client *http.Client, request *http.Request) ([]byte, error) {
	download := *client
	download.CheckRedirect = func(next *http.Request, previous []*http.Request) error {
		if client.CheckRedirect != nil {
			if err := client.CheckRedirect(next, previous); err != nil {
				return err
			}
		}
		if len(previous) >= 10 { // Preserve net/http's default redirect bound.
			return errors.New("stopped after 10 redirects")
		}
		for _, prior := range previous {
			same, err := assetAuthority(prior.URL.String(), next.URL.String())
			if err != nil {
				return err
			}
			if !same {
				next.Header.Del("Authorization")
				next.Header.Del("Job-Token")
			}
		}
		return nil
	}
	response, err := download.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("release asset download failed with HTTP %d", response.StatusCode)
	}
	return io.ReadAll(response.Body)
}

func requestWithoutRedirects(client *http.Client, request *http.Request) (*http.Response, error) {
	once := *client
	once.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return once.Do(request)
}

func authority(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("invalid HTTP URL: %s", raw)
	}
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".") + ":" + port, nil
}

func assetAuthority(api, asset string) (bool, error) {
	selected, err := authority(api)
	if err != nil {
		return false, err
	}
	target, err := authority(asset)
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(selected, "https:") && !strings.HasPrefix(target, "https:") {
		return false, fmt.Errorf("release asset URL would downgrade HTTPS")
	}
	return selected == target, nil
}
