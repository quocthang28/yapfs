package signalling

import (
	"context"
	"fmt"
	"log"
	"time"
	"yapfs/pkg/utils"

	"firebase.google.com/go/v4/db"
)

// Session represents a signaling session data
// Support vanilla ICE for now,
// TODO: support trickle ICE, add support for expire/time out and status(create, connected, failed)
type Session struct {
	ID     string `json:"sessionId"`
	Offer  string `json:"offer"`
	Answer string `json:"answer"`
}

// SessionService handles session operations
type SessionService struct {
	client *firebaseClient
	ref    *db.Ref
}

func (s *SessionService) CreateSession() (string, error) {
	// This code will be displayed to user, it is also the session ID
	code, err := utils.GenerateCode(8)
	if err != nil {
		return "", fmt.Errorf("error generating session code %s", err)
	}

	sessionRef := s.ref.Child(code)
	sessionData := map[string]any{
		"sessionId": code,
		"offer":     "",
		"answer":    "",
	}
	err = sessionRef.Set(s.client.ctx, sessionData)
	if err != nil {
		return "", fmt.Errorf("error creating session %s", err)
	}

	return code, nil
}

func (s *SessionService) CreateSessionWithOffer(offer string) (string, error) {
	// This code will be displayed to user, it is also the session ID
	code, err := utils.GenerateCode(8)
	if err != nil {
		return "", fmt.Errorf("error generating session code %s", err)
	}

	sessionRef := s.ref.Child(code)
	sessionData := map[string]any{
		"sessionId": code,
		"offer":     offer,
		"answer":    "",
	}
	err = sessionRef.Set(s.client.ctx, sessionData)
	if err != nil {
		return "", fmt.Errorf("error creating session %s", err)
	}

	return code, nil
}

func (s *SessionService) UpdateAnswer(sessionID, answer string) error {
	sessionRef := s.ref.Child(sessionID)
	updates := map[string]any{
		"answer": answer,
	}
	if err := sessionRef.Update(s.client.ctx, updates); err != nil {
		return fmt.Errorf("error updating answer for session %s: %w", sessionID, err)
	}
	return nil
}

func (s *SessionService) CheckForAnswer(ctx context.Context, sessionID string) (string, error) {
	// The other peer might not answer immediately so
	// we will wait a bit before checking for first time
	time.Sleep(time.Second * 2)
	for i := 0; i < 10; i++ {
		// Refresh session data from storage
		var sessionData struct {
			Answer string `json:"answer"`
		}
		sessionRef := s.ref.Child(sessionID)
		if err := sessionRef.Get(s.client.ctx, &sessionData); err != nil {
			log.Println(err.Error())
			continue
		}

		if sessionData.Answer != "" {
			return sessionData.Answer, nil
		}

		// Wait 5 seconds before checking again for answer (except on last iteration)
		if i < 9 {
			select {
			case <-time.After(time.Second * 5):
				// Continue polling
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}

	// Delete the session if no answer is received
	if err := s.DeleteSession(sessionID); err != nil {
		return "", fmt.Errorf("error delete session: %s", err.Error())
	}

	return "", fmt.Errorf("timeout waiting for answer")
}

// DeleteSession deletes a session by its ID
func (s *SessionService) DeleteSession(sessionID string) error {
	sessionRef := s.ref.Child(sessionID)
	if err := sessionRef.Delete(s.client.ctx); err != nil {
		return fmt.Errorf("error deleting session %s: %w", sessionID, err)
	}
	return nil
}

func (s *SessionService) GetOffer(sessionID string) (string, error) {
	log.Printf("DEBUG: Attempting to get offer for session ID: %s", sessionID)
	log.Printf("DEBUG: Firebase ref path: %s", s.ref.Path)
	log.Printf("DEBUG: Full ref path will be: %s/%s", s.ref.Path, sessionID)

	// First check if the session exists at all
	sessionRef := s.ref.Child(sessionID)
	
	// Try to get any data from this path
	var sessionData map[string]interface{}
	if err := sessionRef.Get(s.client.ctx, &sessionData); err != nil {
		log.Printf("DEBUG: Firebase Get error: %v", err)
		return "", fmt.Errorf("error fetching session from storage for session %s: %w", sessionID, err)
	}

	log.Printf("DEBUG: Full session data for %s: %+v", sessionID, sessionData)
	log.Printf("DEBUG: Session data is nil: %v", sessionData == nil)
	log.Printf("DEBUG: Session data length: %d", len(sessionData))

	// If session data is empty, let's check if we can list all sessions
	var allSessions map[string]interface{}
	if err := s.ref.Get(s.client.ctx, &allSessions); err != nil {
		log.Printf("DEBUG: Error fetching all sessions: %v", err)
	} else {
		log.Printf("DEBUG: All sessions in database: %+v", allSessions)
	}

	if len(sessionData) == 0 {
		return "", fmt.Errorf("session %s not found in database", sessionID)
	}

	// Extract offer from the map
	offerValue, exists := sessionData["offer"]
	if !exists {
		return "", fmt.Errorf("offer field does not exist for session %s", sessionID)
	}

	offer, ok := offerValue.(string)
	if !ok {
		return "", fmt.Errorf("offer field is not a string for session %s, got type %T", sessionID, offerValue)
	}

	if offer == "" {
		return "", fmt.Errorf("offer is empty for session %s", sessionID)
	}

	log.Printf("DEBUG: Retrieved offer for session %s, length: %d", sessionID, len(offer))
	return offer, nil
}

