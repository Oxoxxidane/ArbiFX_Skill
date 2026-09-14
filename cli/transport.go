package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"
)

func encode(body object) ([]byte, *cliError) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, input("Cannot encode the request as JSON.")
	}
	if len(b) > maxBody {
		return nil, input("The UTF-8 JSON request exceeds the server's 1 MiB limit.")
	}
	return b, nil
}
func decode(b []byte) (object, error) {
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("not utf8")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var result object
	if err := d.Decode(&result); err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	if result == nil {
		return nil, fmt.Errorf("not object")
	}
	return result, nil
}

// Use a fresh connection per command: no proxies, redirects, or POST retries.
// Timeouts stop client waiting, not AE main-thread scripts or queued Start operations.
func request(target string, port int, body object, timeout time.Duration) (object, *cliError) {
	b, e := encode(body)
	if e != nil {
		return nil, e
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/command", port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, input("Cannot create the HTTP request.")
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	tr := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		kind := "transport"
		msg := "HTTP request incomplete; check the port and host. The operation may have executed; do not replay automatically."
		if errors.Is(err, context.DeadlineExceeded) {
			kind = "timeout"
			msg = "Response timeout; server execution is not cancelled. Read back state before considering another request."
		}
		return nil, failure(kind, 3, msg, object{"target": target, "port": port, "outcome": "unknown"})
	}
	defer response.Body.Close()
	// Bound response memory; project/instances have no server-side pagination.
	const maxResponse = 64 * 1024 * 1024
	content, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil {
		return nil, failure("transport", 3, "Response read failed; the operation outcome is unknown.", object{"outcome": "unknown"})
	}
	if len(content) > maxResponse {
		return nil, failure("protocol", 3, "Response exceeds the CLI's 64 MiB limit.", nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, failure("http", 3, "HTTP status is not 2xx; redirects were not followed.", object{"http_status": response.StatusCode})
	}
	result, err := decode(content)
	if err != nil {
		return nil, failure("protocol", 3, "The server did not return a valid UTF-8 JSON object.", nil)
	}
	ok, exists := result["ok"].(bool)
	if !exists {
		return nil, failure("protocol", 3, "Response is missing a boolean ok field.", nil)
	}
	if !ok {
		return nil, failure("api", 4, "ArbiFX rejected the command.", object{"response": result})
	}
	if _, exists = result["data"]; !exists {
		return nil, failure("protocol", 3, "Successful response is missing data.", nil)
	}
	if body["cmd"] == "script" {
		if data, ok := result["data"].(map[string]any); ok {
			if msg, ok := data["error"].(string); ok && msg != "" {
				return nil, failure("script", 4, "ExtendScript execution failed.", object{"response": result})
			}
		}
	}
	return result, nil
}
