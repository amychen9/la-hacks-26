package routes

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type fetchChatRequest struct {
	Message string `json:"message" binding:"required"`
	Context struct {
		SessionID       string `json:"sessionId"`
		MeetingType     string `json:"meetingType"`
		LocationHint    string `json:"locationHint"`
		DurationMinutes int    `json:"durationMinutes"`
		Timezone        string `json:"timezone"`
		CurrentLocation currentLocationInput `json:"currentLocation"`
	} `json:"context"`
}

type fetchDiscoveryResponse struct {
	ProtocolVersion string              `json:"protocolVersion"`
	Agent           map[string]any      `json:"agent"`
	Endpoints       map[string]string   `json:"endpoints"`
	Capabilities    []map[string]string `json:"capabilities"`
}

type fetchChatResponse struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Agent           string             `json:"agent"`
	Status          string             `json:"status"`
	Reply           string             `json:"reply"`
	Actions         []string           `json:"actions"`
	Recommendation  *recommendResponse `json:"recommendation,omitempty"`
}

func InitFetchAI(router *gin.RouterGroup) {
	fetchRouter := router.Group("/fetch")
	fetchRouter.GET("/discovery", fetchDiscovery)
	fetchRouter.GET("/health", fetchHealth)
	fetchRouter.POST("/chat", fetchChat)
}

func fetchDiscovery(c *gin.Context) {
	agentName := envOrDefault("FETCH_AGENT_NAME", "circleup-scheduling-agent")
	agentDisplayName := envOrDefault("FETCH_AGENT_DISPLAY_NAME", "CircleUp Scheduling Agent")

	c.JSON(http.StatusOK, fetchDiscoveryResponse{
		ProtocolVersion: "chat-protocol-v1",
		Agent: map[string]any{
			"name":        agentName,
			"displayName": agentDisplayName,
			"description": "Finds best meeting slots and suggests logistics from participant availability.",
			"framework":   "go-gin",
		},
		Endpoints: map[string]string{
			"chat":      "/api/fetch/chat",
			"health":    "/api/fetch/health",
			"discovery": "/api/fetch/discovery",
		},
		Capabilities: []map[string]string{
			{
				"id":          "meeting_slot_recommendation",
				"description": "Ranks the best overlap windows for a group event.",
			},
			{
				"id":          "logistics_reasoning",
				"description": "Recommends meeting mode and location fallback strategy.",
			},
		},
	})
}

func fetchHealth(c *gin.Context) {
	agentName := envOrDefault("FETCH_AGENT_NAME", "circleup-scheduling-agent")
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"protocol": "chat-protocol-v1",
		"agent":    agentName,
	})
}

func fetchChat(c *gin.Context) {
	payload := fetchChatRequest{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "message is required"})
		return
	}

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload", "message": "message cannot be empty"})
		return
	}

	if strings.TrimSpace(payload.Context.SessionID) == "" {
		agentName := envOrDefault("FETCH_AGENT_NAME", "circleup-scheduling-agent")
		c.JSON(http.StatusOK, fetchChatResponse{
			ProtocolVersion: "chat-protocol-v1",
			Agent:           agentName,
			Status:          "needs_context",
			Reply:           "I can recommend the best meeting slots once you provide context.sessionId for an existing event.",
			Actions: []string{
				"provide_session_id",
				"optionally_set_meeting_type_location_duration",
			},
		})
		return
	}

	req := recommendRequest{
		SessionID:       strings.TrimSpace(payload.Context.SessionID),
		MeetingType:     strings.TrimSpace(payload.Context.MeetingType),
		LocationHint:    strings.TrimSpace(payload.Context.LocationHint),
		DurationMinutes: payload.Context.DurationMinutes,
		Timezone:        strings.TrimSpace(payload.Context.Timezone),
		CurrentLocation: payload.Context.CurrentLocation,
	}
	resp, err := buildRecommendation(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "recommendation_failed",
			"message": err.Error(),
		})
		return
	}

	reply := "Generated ranked meeting slots with logistics suggestions."
	if resp.BestSlot == nil {
		reply = "I could not find a high-confidence overlap yet. Ask more participants to submit availability."
	}

	agentName := envOrDefault("FETCH_AGENT_NAME", "circleup-scheduling-agent")
	c.JSON(http.StatusOK, fetchChatResponse{
		ProtocolVersion: "chat-protocol-v1",
		Agent:           agentName,
		Status:          "ok",
		Reply:           reply,
		Actions: []string{
			"review_ranked_slots",
			"share_top_slot_with_participants",
			"fallback_to_backup_plan_if_needed",
		},
		Recommendation: resp,
	})
}

func envOrDefault(key string, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}
