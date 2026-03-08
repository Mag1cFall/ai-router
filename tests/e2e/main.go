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

	run("Claude→Claude (直通)", func() error {
		return testClaude("/v1/messages", "claude-sonnet-4-5-thinking", false)
	})
	run("Claude→Claude (流式)", func() error {
		return testClaude("/v1/messages", "claude-sonnet-4-5-thinking", true)
	})

	run("OpenAI→Claude (转译)", func() error {
		return testOpenAI("/v1/chat/completions", "claude-sonnet-4-5-thinking", false)
	})
	run("OpenAI→Claude (流式转译)", func() error {
		return testOpenAI("/v1/chat/completions", "claude-sonnet-4-5-thinking", true)
	})

	run("OpenAI→OpenAI (直通)", func() error {
		return testOpenAI("/v1/chat/completions", "gpt-5.4", false)
	})
	run("OpenAI→OpenAI (流式)", func() error {
		return testOpenAI("/v1/chat/completions", "gpt-5.4", true)
	})

	run("Claude→OpenAI (转译)", func() error {
		return testClaude("/v1/messages", "gpt-5.4", false)
	})

	run("OpenAI→Gemini (转译)", func() error {
		return testOpenAI("/v1/chat/completions", "gemini-2.5-flash-lite", false)
	})
	run("OpenAI→Gemini (流式转译)", func() error {
		return testOpenAI("/v1/chat/completions", "gemini-2.5-flash-lite", true)
	})

	run("Claude→Gemini (转译)", func() error {
		return testClaude("/v1/messages", "gemini-2.5-flash-lite", false)
	})

	fmt.Printf("\n════════════════════════════════\n")
	fmt.Printf("  PASSED: %d  FAILED: %d  TOTAL: %d\n", passed, failed, passed+failed)
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
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  %s\n", body)
	return assertStatus(resp, 200)
}

func testProviders() error {
	resp, err := http.Get(baseURL + "/api/providers")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  %s\n", body)
	return nil
}

func testRoutes() error {
	resp, err := http.Get(baseURL + "/api/routes")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  %s\n", body)
	return nil
}

func testClaude(path, model string, stream bool) error {
	payload := map[string]any{
		"model":      model,
		"max_tokens": 64,
		"messages":   []map[string]string{{"role": "user", "content": "Say hi in one word."}},
	}
	if stream {
		payload["stream"] = true
	}
	resp, err := postJSON(baseURL+path, payload)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Status: %d  Content-Type: %s\n", resp.StatusCode, resp.Header.Get("Content-Type"))
	if stream {
		printStreamLines(body, 6)
	} else {
		fmt.Printf("  %s\n", truncate(string(body), 300))
	}
	return assertStatus(resp, 200)
}

func testOpenAI(path, model string, stream bool) error {
	payload := map[string]any{
		"model":      model,
		"max_tokens": 64,
		"messages":   []map[string]string{{"role": "user", "content": "Say hi in one word."}},
	}
	if stream {
		payload["stream"] = true
	}
	resp, err := postJSON(baseURL+path, payload)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Status: %d  Content-Type: %s\n", resp.StatusCode, resp.Header.Get("Content-Type"))
	if stream {
		printStreamLines(body, 6)
	} else {
		fmt.Printf("  %s\n", truncate(string(body), 300))
		if resp.StatusCode == 200 {
			var result map[string]any
			if err := json.Unmarshal(body, &result); err != nil {
				return fmt.Errorf("invalid JSON: %v", err)
			}
			if _, ok := result["choices"]; !ok {
				return fmt.Errorf("expected 'choices' in response")
			}
		}
	}
	return assertStatus(resp, 200)
}

func postJSON(url string, payload any) (*http.Response, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return http.Post(url, "application/json", bytes.NewReader(data))
}

func assertStatus(resp *http.Response, expected int) error {
	if resp.StatusCode != expected {
		return fmt.Errorf("expected status %d, got %d", expected, resp.StatusCode)
	}
	return nil
}

func printStreamLines(body []byte, max int) {
	lines := strings.Split(string(body), "\n")
	shown := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && shown < max {
			fmt.Printf("  %s\n", truncate(line, 120))
			shown++
		}
	}
	total := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			total++
		}
	}
	if total > shown {
		fmt.Printf("  ... (%d more lines)\n", total-shown)
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
