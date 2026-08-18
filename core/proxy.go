package core

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"nexus-api/models"
)

type ChatRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []any  `json:"messages"`
}

// defaultBaseURL returns the default API base URL for a given provider.
func defaultBaseURL(provider string) string {
	switch provider {
	case "anthropic":
		return "https://api.anthropic.com"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	default:
		return "https://api.openai.com"
	}
}

// detectProvider maps a model name to a likely provider.
func detectProvider(model string) string {
	m := strings.ToLower(model)
	if strings.Contains(m, "claude") {
		return "anthropic"
	}
	if strings.Contains(m, "gemini") {
		return "gemini"
	}
	return "openai"
}

// ProxyHandler is the main LLM proxy endpoint (OpenAI-compatible).
func ProxyHandler(c *gin.Context) {
	uid := c.GetUint("userID")
	pool := c.GetString("pool")
	nexusKeyID := c.GetUint("nexusKeyID")

	// Load user
	var user models.User
	if err := models.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}
	if user.Status != "active" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is " + user.Status})
		return
	}

	// Check total quota
	if user.TokensTotal >= 0 && user.TokensUsed >= user.TokensTotal {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Total token quota exceeded"})
		return
	}
	// Check 5h quota with window reset
	now := time.Now().Unix()
	if user.Tokens5h >= 0 {
		windowStart := user.Tokens5hReset
		if now-windowStart > 5*3600 {
			// Reset window
			models.DB.Model(&user).Updates(map[string]interface{}{
				"tokens5h_used":  0,
				"tokens5h_reset": now,
			})
			user.Tokens5hUsed = 0
			user.Tokens5hReset = now
		}
		if user.Tokens5hUsed >= user.Tokens5h {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "5-hour token quota exceeded"})
			return
		}
	}
	// Check weekly quota
	// Use beginning of current week (Monday)
	y, w := time.Unix(now, 0).ISOWeek()
	if user.TokensWeekly >= 0 {
		wy, ww := time.Unix(user.TokensWeekReset, 0).ISOWeek()
		if y != wy || w != ww {
			models.DB.Model(&user).Updates(map[string]interface{}{
				"tokens_week_used":  0,
				"tokens_week_reset": now,
			})
			user.TokensWeekUsed = 0
			user.TokensWeekReset = now
		}
		if user.TokensWeekUsed >= user.TokensWeekly {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Weekly token quota exceeded"})
			return
		}
	}

	// Check monthly quota
	// Use beginning of current month
	m := time.Unix(now, 0).Month()
	if user.TokensMonthly >= 0 {
		wm := time.Unix(user.TokensMonthReset, 0).Month()
		if m != wm {
			models.DB.Model(&user).Updates(map[string]interface{}{
				"tokens_month_used":  0,
				"tokens_month_reset": now,
			})
			user.TokensMonthUsed = 0
			user.TokensMonthReset = now
		}
		if user.TokensMonthUsed >= user.TokensMonthly {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Monthly token quota exceeded"})
			return
		}
	}

	// Read body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot read request body"})
		return
	}

	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Easter egg check
	easterEggModels := models.GetSetting("easter_egg_models", "free_llm_chat")
	for _, m := range strings.Split(easterEggModels, ",") {
		if strings.TrimSpace(m) == req.Model {
			handleEasterEgg(c, req.Stream, req.Model)
			return
		}
	}

	// Check user model permissions
	if user.AllowedModels != "*" {
		allowed := false
		for _, m := range strings.Split(user.AllowedModels, ",") {
			if strings.TrimSpace(m) == req.Model {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "Model not allowed for your account"})
			return
		}
	}

	// Select upstream key pool
	provider := detectProvider(req.Model)
	var keys []models.APIKey
	if pool == "private" {
		models.DB.Where("user_id = ? AND status = 'active' AND provider = ?", uid, provider).
			Order("weight desc").Find(&keys)
	} else {
		models.DB.Where("user_id = 0 AND status = 'active' AND provider = ?", provider).
			Order("weight desc").Find(&keys)
	}

	if len(keys) == 0 {
		// Try any provider in pool as fallback
		if pool == "private" {
			models.DB.Where("user_id = ? AND status = 'active'", uid).Order("weight desc").Find(&keys)
		} else {
			models.DB.Where("user_id = 0 AND status = 'active'").Order("weight desc").Find(&keys)
		}
	}

	if len(keys) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No available upstream API keys"})
		return
	}

	// Attempt proxy with smart routing fallback
	var lastErr string
	for _, apiKey := range keys {
		// Check key model permissions
		if apiKey.AllowedModels != "*" {
			allowed := false
			for _, m := range strings.Split(apiKey.AllowedModels, ",") {
				if strings.TrimSpace(m) == req.Model {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		baseURL := apiKey.BaseURL
		if baseURL == "" {
			baseURL = defaultBaseURL(apiKey.Provider)
		}
		
		baseURL = strings.TrimRight(baseURL, "/")
		targetURL := baseURL
		if !strings.HasSuffix(baseURL, "/v1") {
			targetURL += "/v1"
		}
		targetURL += "/chat/completions"
		proxyReq, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(body))
		if err != nil {
			lastErr = "Failed to create proxy request"
			continue
		}

		// Copy safe headers
		for k, v := range c.Request.Header {
			k2 := strings.ToLower(k)
			if k2 == "authorization" || k2 == "x-api-key" || k2 == "anthropic-version" {
				continue // we set these ourselves
			}
			proxyReq.Header[k] = v
		}

		// Set provider-specific auth
		if apiKey.Provider == "anthropic" {
			proxyReq.Header.Set("x-api-key", apiKey.Key)
			proxyReq.Header.Set("anthropic-version", "2023-06-01")
			proxyReq.Header.Del("Authorization")
		} else {
			proxyReq.Header.Set("Authorization", "Bearer "+apiKey.Key)
		}

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(proxyReq)
		if err != nil {
			lastErr = "Upstream connection failed"
			models.DB.Model(&apiKey).UpdateColumn("error_count", apiKey.ErrorCount+1)
			continue
		}

		// Handle 429 with smart routing
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			if apiKey.RoutingEnabled && user.RoutingEnabled {
				lastErr = "Rate limited (429), trying next key"
				models.DB.Model(&apiKey).UpdateColumn("error_count", apiKey.ErrorCount+1)
				continue
			}
			// Routing disabled – pass 429 through
			c.Status(resp.StatusCode)
			c.Writer.Write([]byte(`{"error":"Rate limit exceeded"}`))
			return
		}

		// Handle context length overflow with smart routing
		if resp.StatusCode == http.StatusBadRequest && apiKey.RoutingEnabled && user.RoutingEnabled {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			bodyStr := strings.ToLower(string(respBody))
			if strings.Contains(bodyStr, "context") && (strings.Contains(bodyStr, "length") || strings.Contains(bodyStr, "overflow") || strings.Contains(bodyStr, "too long") || strings.Contains(bodyStr, "maximum")) {
				lastErr = "Context length exceeded, trying next key"
				continue
			}
			// Not a context error – pass through
			for k, v := range resp.Header {
				c.Writer.Header()[k] = v
			}
			c.Writer.WriteHeader(resp.StatusCode)
			c.Writer.Write(respBody)
			return
		}

		// Success – stream/copy response back
		models.DB.Model(&apiKey).Updates(map[string]interface{}{
			"total_calls": apiKey.TotalCalls + 1,
			"last_used":   now,
		})
		models.DB.Model(&models.NexusKey{}).Where("id = ?", nexusKeyID).Updates(map[string]interface{}{
			"total_calls": models.NexusKey{}.TotalCalls + 1,
			"last_used":   now,
		})

		for k, v := range resp.Header {
			c.Writer.Header()[k] = v
		}
		c.Writer.WriteHeader(resp.StatusCode)

		// Count tokens from streaming or non-streaming response
		var tokensUsed int64
		if req.Stream {
			tokensUsed = streamAndCount(c.Writer, resp.Body, len(body))
		} else {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			c.Writer.Write(respBody)
			tokensUsed = extractTokensFromResponse(respBody, len(body))
		}

		// Update user token usage
		if tokensUsed > 0 {
			updateUserTokenUsage(&user, tokensUsed)
			recordUsageStat(uid, req.Model, provider, tokensUsed)
			// Update key total tokens
			models.DB.Model(&apiKey).UpdateColumn("total_tokens", apiKey.TotalTokens+tokensUsed)
		}
		return
	}

	c.JSON(http.StatusServiceUnavailable, gin.H{"error": fmt.Sprintf("All upstream keys exhausted: %s", lastErr)})
}

// streamAndCount proxies an SSE stream and tries to count tokens from the final usage chunk.
func streamAndCount(w gin.ResponseWriter, body io.ReadCloser, inputBodyLen int) int64 {
	defer body.Close()
	var totalTokens int64
	var outputBytes int
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		outputBytes += len(line)
		w.Write([]byte(line + "\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		// Try to extract usage from stream chunks
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if usage, ok := chunk["usage"].(map[string]interface{}); ok {
					if tt, ok := usage["total_tokens"].(float64); ok {
						totalTokens = int64(tt)
					}
				}
			}
		}
	}
	if totalTokens == 0 {
		totalTokens = int64(inputBodyLen/3 + outputBytes/3)
	}
	return totalTokens
}

// extractTokensFromResponse parses token usage from a non-streaming JSON response.
func extractTokensFromResponse(body []byte, inputBodyLen int) int64 {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return int64(inputBodyLen/3 + len(body)/3)
	}
	usage, ok := resp["usage"].(map[string]interface{})
	if !ok {
		return int64(inputBodyLen/3 + len(body)/3)
	}
	if tt, ok := usage["total_tokens"].(float64); ok {
		return int64(tt)
	}
	return int64(inputBodyLen/3 + len(body)/3)
}

// updateUserTokenUsage increments all quota counters for a user.
func updateUserTokenUsage(user *models.User, tokens int64) {
	models.DB.Model(user).Updates(map[string]interface{}{
		"tokens_used":       user.TokensUsed + tokens,
		"tokens_week_used":  user.TokensWeekUsed + tokens,
		"tokens_month_used": user.TokensMonthUsed + tokens,
		"tokens5h_used":     user.Tokens5hUsed + tokens,
	})
}

// recordUsageStat creates or updates a UsageStat record for today/this hour.
func recordUsageStat(userID uint, model, provider string, tokens int64) {
	now := time.Now()
	date := now.Format("2006-01-02")
	hour := now.Hour()

	var stat models.UsageStat
	err := models.DB.Where("user_id = ? AND model_name = ? AND date = ? AND hour = ?", userID, model, date, hour).First(&stat).Error
	if err != nil {
		stat = models.UsageStat{
			UserID: userID, ModelName: model, Provider: provider,
			Date: date, Hour: hour, TokensUsed: tokens, CallCount: 1,
		}
		models.DB.Create(&stat)
	} else {
		models.DB.Model(&stat).Updates(map[string]interface{}{
			"tokens_used": stat.TokensUsed + tokens,
			"call_count":  stat.CallCount + 1,
		})
	}
}

// handleEasterEgg streams the configured easter egg message character by character.
func handleEasterEgg(c *gin.Context, stream bool, model string) {
	msg := models.GetSetting("easter_egg_message", "我喜欢你喵！")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	// Always stream, character by character
	now := time.Now().Unix()
	for i, r := range []rune(msg) {
		chunk := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-egg%d", i),
			"object":  "chat.completion.chunk",
			"created": now,
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"delta":         map[string]string{"content": string(r)},
					"finish_reason": nil,
				},
			},
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(60 * time.Millisecond)
	}
	// Final done chunk
	doneChunk := map[string]interface{}{
		"id":      "chatcmpl-egg-done",
		"object":  "chat.completion.chunk",
		"created": now,
		"model":   model,
		"choices": []map[string]interface{}{
			{"index": 0, "delta": map[string]string{}, "finish_reason": "stop"},
		},
	}
	data, _ := json.Marshal(doneChunk)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}
