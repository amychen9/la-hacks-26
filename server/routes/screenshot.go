package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ParsedTimeBlock struct {
	StartISO   string  `json:"startIso"`
	EndISO     string  `json:"endIso"`
	Confidence float64 `json:"confidence"`
	SourceText string  `json:"sourceText,omitempty"`
}

type ParseScreenshotRequest struct {
	ImageURL string `json:"imageUrl" binding:"required"`
	Timezone string `json:"timezone" binding:"required"`
	Duration int    `json:"duration,omitempty"`
}

type ParseScreenshotResponse struct {
	SchemaVersion string            `json:"schemaVersion"`
	Availability  []ParsedTimeBlock `json:"availability"`
	IfNeeded      []ParsedTimeBlock `json:"ifNeeded"`
	Confidence    float64           `json:"confidence"`
	Warnings      []string          `json:"warnings"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func InitScreenshot(router *gin.RouterGroup) {
	screenshotRouter := router.Group("/screenshot")
	screenshotRouter.POST("/parse", parseScreenshot)
}

// @Summary Parse availability from a schedule screenshot URL
// @Tags screenshot
// @Accept json
// @Produce json
// @Param payload body ParseScreenshotRequest true "Screenshot parse request"
// @Success 200 {object} ParseScreenshotResponse
// @Failure 400 {object} map[string]string
// @Router /screenshot/parse [post]
func parseScreenshot(c *gin.Context) {
	payload := ParseScreenshotRequest{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_payload",
			"message": "Expected imageUrl and timezone",
		})
		return
	}

	if !strings.HasPrefix(payload.ImageURL, "http://") && !strings.HasPrefix(payload.ImageURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_image_url",
			"message": "imageUrl must be an http(s) URL",
		})
		return
	}

	response, err := parseScreenshotWithAnthropic(payload)
	if err != nil {
		ocrResponse, ocrErr := parseScreenshotWithPyTesseract(payload)
		if ocrErr == nil && (len(ocrResponse.Availability) > 0 || len(ocrResponse.IfNeeded) > 0) {
			ocrResponse.Warnings = append(ocrResponse.Warnings, fmt.Sprintf("Claude parsing unavailable (%s). Used pytesseract OCR fallback.", err.Error()))
			c.JSON(http.StatusOK, ocrResponse)
			return
		}

		fallback := getDemoFallbackResponse(payload.Timezone)
		if ocrErr != nil {
			fallback.Warnings = append(fallback.Warnings, fmt.Sprintf("Claude unavailable (%s). OCR fallback failed (%s). Using demo parser output.", err.Error(), ocrErr.Error()))
		} else {
			fallback.Warnings = append(fallback.Warnings, fmt.Sprintf("Claude unavailable (%s). OCR returned no slots. Using demo parser output.", err.Error()))
		}
		c.JSON(http.StatusOK, fallback)
		return
	}

	c.JSON(http.StatusOK, response)
}

func getDemoFallbackResponse(timezone string) *ParseScreenshotResponse {
	_ = timezone
	return &ParseScreenshotResponse{
		SchemaVersion: "v1",
		Availability: []ParsedTimeBlock{
			{
				StartISO:   "2026-04-27T13:00:00-07:00",
				EndISO:     "2026-04-27T15:00:00-07:00",
				Confidence: 0.82,
				SourceText: "Mon 1pm-3pm",
			},
			{
				StartISO:   "2026-04-28T10:00:00-07:00",
				EndISO:     "2026-04-28T12:00:00-07:00",
				Confidence: 0.78,
				SourceText: "Tue 10am-12pm",
			},
		},
		IfNeeded: []ParsedTimeBlock{
			{
				StartISO:   "2026-04-29T16:00:00-07:00",
				EndISO:     "2026-04-29T17:00:00-07:00",
				Confidence: 0.63,
				SourceText: "Wed around 4pm",
			},
		},
		Confidence: 0.79,
		Warnings: []string{
			"Demo parser output. Verify times before creating the event.",
		},
	}
}


func parseScreenshotWithAnthropic(payload ParseScreenshotRequest) (*ParseScreenshotResponse, error) {
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not configured")
	}

	model := strings.TrimSpace(os.Getenv("ANTHROPIC_MODEL"))
	if model == "" {
		model = "claude-3-5-sonnet-latest"
	}

	prompt := fmt.Sprintf(`Extract free-time availability from this calendar screenshot.
Timezone: %s
Return ONLY JSON with this exact shape:
{
  "schemaVersion":"v1",
  "availability":[{"startIso":"RFC3339","endIso":"RFC3339","confidence":0.0,"sourceText":"..."}],
  "ifNeeded":[{"startIso":"RFC3339","endIso":"RFC3339","confidence":0.0,"sourceText":"..."}],
  "confidence":0.0,
  "warnings":["..."]
}
Rules:
- confidence values must be between 0 and 1
- endIso must be later than startIso
- no markdown fences
- no text outside JSON
- identify likely FREE windows from visible empty areas between busy events
- if exact minute is unclear, round to nearest 30 minutes
- prefer returning 2-6 best candidate free windows instead of empty arrays`, payload.Timezone)

	requestBody := map[string]interface{}{
		"model":      model,
		"max_tokens": 1200,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": prompt,
					},
					{
						"type": "image",
						"source": map[string]interface{}{
							"type": "url",
							"url":  payload.ImageURL,
						},
					},
				},
			},
		},
	}

	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to encode Claude request")
	}

	client := &http.Client{Timeout: 12 * time.Second}
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewBuffer(rawBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create Claude request")
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Claude request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading Claude response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 400 {
			snippet = snippet[:400]
		}
		return nil, fmt.Errorf("Claude returned status %d: %s", resp.StatusCode, snippet)
	}

	claudeResp := anthropicResponse{}
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("failed to parse Claude response envelope")
	}

	extractedText := ""
	for _, content := range claudeResp.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			extractedText = content.Text
			break
		}
	}
	if extractedText == "" {
		return nil, fmt.Errorf("no structured parser output received from Claude")
	}

	jsonText := extractJSONObject(extractedText)
	parsed := ParseScreenshotResponse{}
	if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
		return nil, fmt.Errorf("invalid parser JSON returned by Claude")
	}

	if parsed.SchemaVersion == "" {
		parsed.SchemaVersion = "v1"
	}
	if parsed.Availability == nil {
		parsed.Availability = []ParsedTimeBlock{}
	}
	if parsed.IfNeeded == nil {
		parsed.IfNeeded = []ParsedTimeBlock{}
	}
	if parsed.Warnings == nil {
		parsed.Warnings = []string{}
	}
	if parsed.Confidence < 0 || parsed.Confidence > 1 {
		return nil, fmt.Errorf("invalid parser confidence value from Claude")
	}

	if err := validateParsedBlocks(parsed.Availability); err != nil {
		return nil, err
	}
	if err := validateParsedBlocks(parsed.IfNeeded); err != nil {
		return nil, err
	}
	if len(parsed.Availability) == 0 && len(parsed.IfNeeded) == 0 {
		return nil, fmt.Errorf("no time blocks detected from screenshot")
	}

	return &parsed, nil
}

func parseScreenshotWithPyTesseract(payload ParseScreenshotRequest) (*ParseScreenshotResponse, error) {
	scriptPath := filepath.Join("scripts", "screenshot_ocr_fallback.py")

	inputPayload := map[string]string{
		"imageUrl": payload.ImageURL,
		"timezone": payload.Timezone,
	}
	rawInput, err := json.Marshal(inputPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode OCR payload")
	}

	cmd := exec.Command("python3", scriptPath)
	cmd.Stdin = bytes.NewReader(rawInput)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("python OCR command failed: %s", errMsg)
	}

	parsed := ParseScreenshotResponse{}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("failed parsing OCR JSON output")
	}

	if parsed.SchemaVersion == "" {
		parsed.SchemaVersion = "v1"
	}
	if parsed.Availability == nil {
		parsed.Availability = []ParsedTimeBlock{}
	}
	if parsed.IfNeeded == nil {
		parsed.IfNeeded = []ParsedTimeBlock{}
	}
	if parsed.Warnings == nil {
		parsed.Warnings = []string{}
	}
	if parsed.Confidence < 0 || parsed.Confidence > 1 {
		parsed.Confidence = 0
		parsed.Warnings = append(parsed.Warnings, "OCR returned invalid confidence; reset to 0.")
	}

	if err := validateParsedBlocks(parsed.Availability); err != nil && len(parsed.Availability) > 0 {
		return nil, fmt.Errorf("invalid OCR availability blocks: %s", err.Error())
	}
	if err := validateParsedBlocks(parsed.IfNeeded); err != nil && len(parsed.IfNeeded) > 0 {
		return nil, fmt.Errorf("invalid OCR ifNeeded blocks: %s", err.Error())
	}

	return &parsed, nil
}

func validateParsedBlocks(blocks []ParsedTimeBlock) error {
	for _, block := range blocks {
		start, err := time.Parse(time.RFC3339, block.StartISO)
		if err != nil {
			return fmt.Errorf("invalid startIso value: %s", block.StartISO)
		}
		end, err := time.Parse(time.RFC3339, block.EndISO)
		if err != nil {
			return fmt.Errorf("invalid endIso value: %s", block.EndISO)
		}
		if !end.After(start) {
			return fmt.Errorf("endIso must be later than startIso")
		}
		if block.Confidence < 0 || block.Confidence > 1 {
			return fmt.Errorf("block confidence must be between 0 and 1")
		}
	}
	return nil
}

func extractJSONObject(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start != -1 && end != -1 && end > start {
		return trimmed[start : end+1]
	}

	return text
}
