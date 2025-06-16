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

func (s *SessionService) createSessionWithOffer(offer string) (string, error) {
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

	log.Println("Session create succesfully")

	return code, nil
}

func (s *SessionService) updateAnswer(sessionID, answer string) error {
	sessionRef := s.ref.Child(sessionID)
	updates := map[string]any{
		"answer": answer,
	}
	if err := sessionRef.Update(s.client.ctx, updates); err != nil {
		return fmt.Errorf("error updating answer for session %s: %w", sessionID, err)
	}
	return nil
}

func (s *SessionService) checkForAnswer(ctx context.Context, sessionID string) (string, error) {
	// The other peer might not answer immediately so
	// we will wait a bit before checking for first time
	time.Sleep(time.Second * 5)
	log.Printf("Waiting for receiver to answer...")

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
	if err := s.deleteSession(sessionID); err != nil {
		return "", fmt.Errorf("error delete session: %s", err.Error())
	}

	return "", fmt.Errorf("timeout waiting for answer")
}

// deleteSession deletes a session by its ID
func (s *SessionService) deleteSession(sessionID string) error {
	sessionRef := s.ref.Child(sessionID)
	if err := sessionRef.Delete(s.client.ctx); err != nil {
		return fmt.Errorf("error deleting session %s: %w", sessionID, err)
	}
	return nil
}

func (s *SessionService) getOffer(sessionID string) (string, error) {
	// First check if the session exists at all
	sessionRef := s.ref.Child(sessionID)

	// Try to get any data from this path
	var sessionData Session
	if err := sessionRef.Get(s.client.ctx, &sessionData); err != nil {
		log.Printf("DEBUG: Firebase Get error: %v", err)
		return "", fmt.Errorf("error fetching session from storage for session %s: %w", sessionID, err)
	}

	return sessionData.Offer, nil
}
