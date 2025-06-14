package signalling

import (
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

func (s *SessionService) CheckForAnswer(sessionID string) (<-chan string, <-chan error) {
	answerChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(answerChan)
		defer close(errorChan)

		numOfCheck := 5
		firstCheck := true

		for numOfCheck > 0 {
			// The other peer might not answer immediately so
			// we will wait a bit before checking for first time
			if firstCheck {
				time.Sleep(time.Second * 2)
				firstCheck = false
			}

			numOfCheck--

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
				answerChan <- sessionData.Answer
				return
			}

			if numOfCheck > 0 {
				// Wait 5 seconds before checking again for answer
				time.Sleep(time.Second * 5)
			}
		}

		// Delete the session if no answer is received
		err := s.DeleteSession(sessionID)
		if err != nil {
			errorChan <- fmt.Errorf("error delete session: %s", err.Error())
		}

		errorChan <- fmt.Errorf("timeout waiting for answer")
	}()

	return answerChan, errorChan
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
	// Fetch offer data from storage
	var sessionData struct {
		Offer string `json:"offer"`
	}
	
	sessionRef := s.ref.Child(sessionID)
	if err := sessionRef.Get(s.client.ctx, &sessionData); err != nil {
		return "", fmt.Errorf("error fetching offer from storage for session %s: %w", sessionID, err)
	}

	return sessionData.Offer, nil
}

