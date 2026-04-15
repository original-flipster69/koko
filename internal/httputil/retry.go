package httputil

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	var lastStatus int

	getBody := req.GetBody

	for attempt := range maxRetries {
		if attempt > 0 {
			if getBody != nil {
				body, err := getBody()
				if err != nil {
					return nil, err
				}
				req.Body = body
			}
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout {
			lastStatus = resp.StatusCode
			resp.Body.Close()
			lastErr = nil
			continue
		}

		return resp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request failed after %d retries (last status: %d)", maxRetries, lastStatus)
}
