// Package push provides Expo Push notification functionality
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	// ExpoPushURL is the Expo Push API endpoint
	ExpoPushURL = "https://exp.host/--/api/v2/push/send"

	// RequestTimeout for push requests
	RequestTimeout = 10 * time.Second
)

// Message represents an Expo push notification message
type Message struct {
	To         string         `json:"to"`
	Title      string         `json:"title,omitempty"`
	Body       string         `json:"body"`
	Data       map[string]any `json:"data,omitempty"`
	Sound      string         `json:"sound,omitempty"`    // "default" or custom
	Badge      *int           `json:"badge,omitempty"`    // iOS badge count
	Priority   string         `json:"priority,omitempty"` // "default", "normal", "high"
	CategoryId string         `json:"categoryId,omitempty"`
}

// Response represents the Expo Push API response
type Response struct {
	Data []TicketResponse `json:"data"`
}

// TicketResponse represents a single push ticket
type TicketResponse struct {
	Status  string `json:"status"` // "ok" or "error"
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
	Details struct {
		Error string `json:"error,omitempty"`
	} `json:"details,omitempty"`
}

// Service handles Expo Push notifications
type Service struct {
	client *http.Client
	logger *slog.Logger
}

// NewService creates a new push notification service
func NewService(logger *slog.Logger) *Service {
	return &Service{
		client: &http.Client{
			Timeout: RequestTimeout,
		},
		logger: logger,
	}
}

// Send sends a push notification to a single token
func (s *Service) Send(ctx context.Context, msg *Message) error {
	if msg.To == "" {
		return errors.New("push token is required")
	}

	// Validate Expo push token format
	if !isValidExpoPushToken(msg.To) {
		return fmt.Errorf("invalid Expo push token format: %s", msg.To)
	}

	// Default sound
	if msg.Sound == "" {
		msg.Sound = "default"
	}

	// Marshal message
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal push message: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", ExpoPushURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create push request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send push notification: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read push response: %w", err)
	}

	// Parse response
	var pushResp Response
	if err := json.Unmarshal(body, &pushResp); err != nil {
		s.logger.Error("failed to parse push response", "body", string(body), "error", err)
		return fmt.Errorf("failed to parse push response: %w", err)
	}

	// Check for errors
	if len(pushResp.Data) > 0 && pushResp.Data[0].Status == "error" {
		errMsg := pushResp.Data[0].Message
		if pushResp.Data[0].Details.Error != "" {
			errMsg = pushResp.Data[0].Details.Error
		}
		s.logger.Warn("push notification failed", "error", errMsg, "token", msg.To[:20]+"...")
		return fmt.Errorf("push notification failed: %s", errMsg)
	}

	s.logger.Info("push notification sent", "ticketId", pushResp.Data[0].ID)
	return nil
}

// SendBatch sends push notifications to multiple tokens
func (s *Service) SendBatch(ctx context.Context, messages []*Message) error {
	if len(messages) == 0 {
		return nil
	}

	// Filter valid tokens
	validMessages := make([]*Message, 0, len(messages))
	for _, msg := range messages {
		if msg.To != "" && isValidExpoPushToken(msg.To) {
			if msg.Sound == "" {
				msg.Sound = "default"
			}
			validMessages = append(validMessages, msg)
		}
	}

	if len(validMessages) == 0 {
		return nil
	}

	// Marshal messages
	payload, err := json.Marshal(validMessages)
	if err != nil {
		return fmt.Errorf("failed to marshal push messages: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", ExpoPushURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create push request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send push notifications: %w", err)
	}
	defer resp.Body.Close()

	// Log response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Error("push batch failed", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("push batch failed with status: %d", resp.StatusCode)
	}

	s.logger.Info("push batch sent", "count", len(validMessages))
	return nil
}

// isValidExpoPushToken checks if a token is a valid Expo push token
func isValidExpoPushToken(token string) bool {
	// Expo push tokens start with "ExponentPushToken[" or "ExpoPushToken["
	return len(token) > 20 && (token[:18] == "ExponentPushToken[" || token[:14] == "ExpoPushToken[")
}
