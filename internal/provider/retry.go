package provider

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	baseBackoff = 2 * time.Second
	maxBackoff  = 30 * time.Second
)

func withRetry(ctx context.Context, client *http.Client, req *http.Request, maxAttempts int) (*http.Response, error) {
	var lastErr error
	var lastStatus int
	var nextWait time.Duration

	getBody := req.GetBody

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if getBody != nil {
				body, err := getBody()
				if err != nil {
					return nil, err
				}
				req.Body = body
			}
			wait := nextWait
			if wait == 0 {
				wait = expBackoff(attempt)
			}
			nextWait = 0
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			lastStatus = 0
			continue
		}

		if isRetryStatus(resp.StatusCode) {
			lastStatus = resp.StatusCode
			lastErr = nil
			nextWait = parseRetryAfter(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request failed after %d attempts (last status: %d)", maxAttempts, lastStatus)
}

func isRetryStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

func expBackoff(attempt int) time.Duration {
	exp := baseBackoff << (attempt - 1)
	if exp <= 0 || exp > maxBackoff {
		exp = maxBackoff
	}
	half := exp / 2
	jitter := time.Duration(rand.Int63n(int64(half) + 1))
	return half + jitter
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		d := time.Duration(secs) * time.Second
		if d > maxBackoff {
			d = maxBackoff
		}
		return d
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > maxBackoff {
			d = maxBackoff
		}
		if d > 0 {
			return d
		}
	}
	return 0
}
