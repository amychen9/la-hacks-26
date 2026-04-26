package routes

import (
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"schej.it/server/db"
	"schej.it/server/models"
)

type participantInput struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Availability []string `json:"availability"`
	IfNeeded     []string `json:"ifNeeded"`
}

type recommendRequest struct {
	SessionID       string             `json:"sessionId"`
	MeetingType     string             `json:"meetingType"`
	LocationHint    string             `json:"locationHint"`
	DurationMinutes int                `json:"durationMinutes"`
	Timezone        string             `json:"timezone"`
	Participants    []participantInput `json:"participants"`
}

type chatRequest struct {
	Message string `json:"message"`
	Context struct {
		SessionID       string `json:"sessionId"`
		MeetingType     string `json:"meetingType"`
		LocationHint    string `json:"locationHint"`
		DurationMinutes int    `json:"durationMinutes"`
		Timezone        string `json:"timezone"`
	} `json:"context"`
}

type rankedSlot struct {
	StartISO       string  `json:"startIso"`
	EndISO         string  `json:"endIso"`
	Score          float64 `json:"score"`
	AvailableCount int     `json:"availableCount"`
	IfNeededCount  int     `json:"ifNeededCount"`
}

type logisticsSuggestion struct {
	MeetingMode string `json:"meetingMode"`
	Suggestion  string `json:"suggestion"`
	Backup      string `json:"backup"`
}

type recommendResponse struct {
	BestSlot            *rankedSlot         `json:"bestSlot"`
	RankedSlots         []rankedSlot        `json:"rankedSlots"`
	LogisticsSuggestion logisticsSuggestion `json:"logisticsSuggestion"`
	Reasoning           []string            `json:"reasoning"`
	AgentTrace          []string            `json:"agentTrace"`
}

type participantAvail struct {
	ID       string
	Name     string
	AvailSet map[int64]bool
	IfSet    map[int64]bool
}

func InitAgent(router *gin.RouterGroup) {
	agentRouter := router.Group("/agent")
	agentRouter.POST("/recommend", recommend)
	agentRouter.POST("/chat", chat)
}

func recommend(c *gin.Context) {
	payload := recommendRequest{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload"})
		return
	}

	resp, err := buildRecommendation(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recommendation_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func chat(c *gin.Context) {
	payload := chatRequest{}
	if err := c.Bind(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_payload"})
		return
	}

	req := recommendRequest{
		SessionID:       payload.Context.SessionID,
		MeetingType:     payload.Context.MeetingType,
		LocationHint:    payload.Context.LocationHint,
		DurationMinutes: payload.Context.DurationMinutes,
		Timezone:        payload.Context.Timezone,
	}
	resp, err := buildRecommendation(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chat_failed", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Generated recommendation from multi-agent orchestration.",
		"result":  resp,
	})
}

func buildRecommendation(payload recommendRequest) (*recommendResponse, error) {
	if strings.TrimSpace(payload.SessionID) == "" {
		return nil, errString("sessionId is required")
	}
	if payload.DurationMinutes <= 0 {
		payload.DurationMinutes = 60
	}
	if payload.MeetingType == "" {
		payload.MeetingType = "study"
	}

	event := db.GetEventByEitherId(payload.SessionID)
	if event == nil {
		return nil, errString("session not found")
	}
	participants := hydrateParticipants(event, payload.Participants)
	if len(participants) == 0 {
		return nil, errString("no participant availability found")
	}

	timeIncrement := 15
	if event.TimeIncrement != nil && *event.TimeIncrement > 0 {
		timeIncrement = *event.TimeIncrement
	}
	steps := int(math.Ceil(float64(payload.DurationMinutes) / float64(timeIncrement)))
	if steps < 1 {
		steps = 1
	}

	candidates, trace := runAvailabilityAgent(participants, steps, timeIncrement)
	ranked := runConsensusAgent(candidates, timeIncrement)
	if len(ranked) == 0 {
		return &recommendResponse{
			BestSlot:    nil,
			RankedSlots: []rankedSlot{},
			LogisticsSuggestion: logisticsSuggestion{
				MeetingMode: "virtual",
				Suggestion:  "No high-confidence overlap found yet. Ask more participants to submit availability.",
				Backup:      "Collect additional responses and rerun recommendations.",
			},
			Reasoning:  []string{"No overlapping windows detected for requested duration."},
			AgentTrace: append(trace, "ConsensusAgent: produced 0 ranked slots"),
		}, nil
	}

	logistics := runLogisticsAgent(payload.MeetingType, payload.LocationHint)
	best := ranked[0]
	reasoning := []string{
		"Highest overlap across participants",
		"Minimized if-needed dependencies",
		"Selected earliest high-scoring slot",
	}

	agentTrace := append(trace, "ConsensusAgent: ranked candidate slots", "LogisticsAgent: generated location/mode recommendation")
	return &recommendResponse{
		BestSlot:            &best,
		RankedSlots:         ranked,
		LogisticsSuggestion: logistics,
		Reasoning:           reasoning,
		AgentTrace:          agentTrace,
	}, nil
}

func hydrateParticipants(event *models.Event, fallback []participantInput) []participantAvail {
	responses := db.GetEventResponses(event.Id.Hex())
	out := make([]participantAvail, 0)
	if len(responses) > 0 {
		for _, er := range responses {
			if er.Response == nil {
				continue
			}
			out = append(out, participantAvail{
				ID:       er.UserId,
				Name:     er.Response.Name,
				AvailSet: toSet(er.Response.Availability),
				IfSet:    toSet(er.Response.IfNeeded),
			})
		}
		return out
	}

	for _, p := range fallback {
		availSet := make(map[int64]bool)
		ifSet := make(map[int64]bool)
		for _, t := range p.Availability {
			if ts, err := time.Parse(time.RFC3339, t); err == nil {
				availSet[ts.UnixMilli()] = true
			}
		}
		for _, t := range p.IfNeeded {
			if ts, err := time.Parse(time.RFC3339, t); err == nil {
				ifSet[ts.UnixMilli()] = true
			}
		}
		out = append(out, participantAvail{
			ID:       p.ID,
			Name:     p.Name,
			AvailSet: availSet,
			IfSet:    ifSet,
		})
	}

	return out
}

func toSet(arr []primitive.DateTime) map[int64]bool {
	set := make(map[int64]bool)
	for _, dt := range arr {
		set[dt.Time().UnixMilli()] = true
	}
	return set
}

type candidateSlot struct {
	StartMS        int64
	AvailableCount int
	IfNeededCount  int
	Score          float64
}

func runAvailabilityAgent(participants []participantAvail, steps int, incrementMinutes int) ([]candidateSlot, []string) {
	startCandidates := make(map[int64]bool)
	for _, p := range participants {
		for t := range p.AvailSet {
			startCandidates[t] = true
		}
		for t := range p.IfSet {
			startCandidates[t] = true
		}
	}

	incMs := int64(incrementMinutes * 60 * 1000)
	candidates := make([]candidateSlot, 0)

	for start := range startCandidates {
		availableCount := 0
		ifNeededCount := 0

		for _, p := range participants {
			allAvailable := true
			needsIf := false
			for i := 0; i < steps; i++ {
				t := start + int64(i)*incMs
				if p.AvailSet[t] {
					continue
				}
				if p.IfSet[t] {
					needsIf = true
					continue
				}
				allAvailable = false
				break
			}
			if allAvailable {
				if needsIf {
					ifNeededCount++
				} else {
					availableCount++
				}
			}
		}

		if availableCount+ifNeededCount == 0 {
			continue
		}

		score := float64(availableCount) - (0.35 * float64(ifNeededCount))
		candidates = append(candidates, candidateSlot{
			StartMS:        start,
			AvailableCount: availableCount,
			IfNeededCount:  ifNeededCount,
			Score:          score,
		})
	}

	trace := []string{
		"AvailabilityAgent: generated candidate windows from participant availabilities",
	}
	return candidates, trace
}

func runConsensusAgent(candidates []candidateSlot, incrementMinutes int) []rankedSlot {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		if candidates[i].AvailableCount != candidates[j].AvailableCount {
			return candidates[i].AvailableCount > candidates[j].AvailableCount
		}
		if candidates[i].IfNeededCount != candidates[j].IfNeededCount {
			return candidates[i].IfNeededCount < candidates[j].IfNeededCount
		}
		return candidates[i].StartMS < candidates[j].StartMS
	})

	max := 3
	if len(candidates) < max {
		max = len(candidates)
	}
	out := make([]rankedSlot, 0, max)
	for i := 0; i < max; i++ {
		start := time.UnixMilli(candidates[i].StartMS).UTC()
		end := start.Add(time.Duration(incrementMinutes) * time.Minute)
		out = append(out, rankedSlot{
			StartISO:       start.Format(time.RFC3339),
			EndISO:         end.Format(time.RFC3339),
			Score:          round2(candidates[i].Score),
			AvailableCount: candidates[i].AvailableCount,
			IfNeededCount:  candidates[i].IfNeededCount,
		})
	}
	return out
}

func runLogisticsAgent(meetingType string, locationHint string) logisticsSuggestion {
	place := strings.TrimSpace(locationHint)
	if place == "" {
		place = "your area"
	}
	switch strings.ToLower(strings.TrimSpace(meetingType)) {
	case "work":
		return logisticsSuggestion{
			MeetingMode: "virtual",
			Suggestion:  "Create a Google Meet link and include agenda notes.",
			Backup:      "Book a quiet coworking table near " + place + ".",
		}
	case "social":
		return logisticsSuggestion{
			MeetingMode: "in_person",
			Suggestion:  "Pick a casual cafe or food spot near " + place + ".",
			Backup:      "If travel is hard, switch to a quick video call.",
		}
	default:
		return logisticsSuggestion{
			MeetingMode: "in_person",
			Suggestion:  "Reserve a library study room near " + place + ".",
			Backup:      "Use a virtual room if anyone cannot make it in-person.",
		}
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

type simpleErr string

func (e simpleErr) Error() string { return string(e) }

func errString(msg string) error {
	return simpleErr(msg)
}
