package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const baseURL = "http://localhost:8446"

func main() {
	passed := 0
	failed := 0

	run := func(name string, fn func() error) {
		fmt.Printf("\n━━━ %s ━━━\n", name)
		start := time.Now()
		if err := fn(); err != nil {
			fmt.Printf("  ❌ FAIL (%v): %v\n", time.Since(start).Round(time.Millisecond), err)
			failed++
		} else {
			fmt.Printf("  ✅ PASS (%v)\n", time.Since(start).Round(time.Millisecond))
			passed++
		}
	}

	run("Healthz", testHealthz)
	run("API Providers", testProviders)
	run("API Routes", testRoutes)
	run("API Logs", testLogs)
	run("Claude Direct (non-stream)", testClaudeDirect)
	run("Claude Direct (stream)", testClaudeStream)
	run("OpenAI→Claude (non-stream)", testOpenAIToClaude)
	run("OpenAI→Claude (stream)", testOpenAIToClaudeStream)

	fmt.Printf("\n════════════════════════════════\n")
	fmt.Printf("  PASSED: %d  FAILED: %d\n", passed, failed)
	fmt.Printf("════════════════════════════════\n")

	if failed > 0 {
		os.Exit(1)
	}
}

func testHealthz() error {
	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  %s\n", body)
	return nil
}

func testProviders() error {
	resp, err := http.Get(baseURL + "/api/providers")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  %s\n", body)
	return nil
}

func testRoutes() error {
	resp, err := http.Get(baseURL + "/api/routes")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  %s\n", body)
	return nil
}

func testLogs() error {
	resp, err := http.Get(baseURL + "/api/logs")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  %s\n", truncate(string(body), 200))
	return nil
}

func testClaudeDirect() error {
	payload := map[string]any{
		"model":      "claude-sonnet-4-5-thinking",
		"max_tokens": 128,
		"messages":   []map[string]string{{"role": "user", "content": "Say hello in exactly one word."}},
	}
	resp, err := postJSON(baseURL+"/v1/messages", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Status: %d\n", resp.StatusCode)
	fmt.Printf("  %s\n", truncate(string(body), 500))
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func testClaudeStream() error {
	payload := map[string]any{
		"model":      "claude-sonnet-4-5-thinking",
		"max_tokens": 128,
		"stream":     true,
		"messages":   []map[string]string{{"role": "user", "content": "Say hi in one word."}},
	}
	resp, err := postJSON(baseURL+"/v1/messages", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	fmt.Printf("  Status: %d  Content-Type: %s\n", resp.StatusCode, ct)
	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(string(body), "\n")
	shown := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && shown < 8 {
			fmt.Printf("  %s\n", truncate(line, 120))
			shown++
		}
	}
	if shown < len(lines) {
		fmt.Printf("  ... (%d more lines)\n", len(lines)-shown)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func testOpenAIToClaude() error {
	payload := map[string]any{
		"model":      "claude-sonnet-4-5-thinking",
		"max_tokens": 128,
		"messages":   []map[string]string{{"role": "user", "content": "Say hello in exactly one word."}},
	}
	resp, err := postJSON(baseURL+"/v1/chat/completions", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Status: %d\n", resp.StatusCode)
	fmt.Printf("  %s\n", truncate(string(body), 500))
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}
	if _, ok := result["choices"]; !ok {
		return fmt.Errorf("expected 'choices' in OpenAI response")
	}
	return nil
}

func testOpenAIToClaudeStream() error {
	payload := map[string]any{
		"model":      "claude-sonnet-4-5-thinking",
		"max_tokens": 128,
		"stream":     true,
		"messages":   []map[string]string{{"role": "user", "content": "Say hi."}},
	}
	resp, err := postJSON(baseURL+"/v1/chat/completions", payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	fmt.Printf("  Status: %d  Content-Type: %s\n", resp.StatusCode, ct)
	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(string(body), "\n")
	shown := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && shown < 8 {
			fmt.Printf("  %s\n", truncate(line, 120))
			shown++
		}
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func postJSON(url string, payload any) (*http.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return http.Post(url, "application/json", bytes.NewReader(data))
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
