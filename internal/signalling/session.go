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

// SessionService handles session operations - manages current session
type SessionService struct {
	client *firebaseClient
	ref    *db.Ref

	currentSession *Session
}

func (s *SessionService) CreateSession() (string, error) {
	// This code will be displayed to user, it is also the session ID
	code, err := utils.GenerateCode(8)
	if err != nil {
		return "", fmt.Errorf("error generating session code %s", err)
	}

	newSession := &Session{
		ID: code,
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

	s.currentSession = newSession
	return code, nil
}

func (s *SessionService) GetSession(sessionID string) error {
	var sessionData struct {
		SessionID string `json:"sessionId"`
		Offer     string `json:"offer"`
		Answer    string `json:"answer"`
	}

	sessionRef := s.ref.Child(sessionID)
	if err := sessionRef.Get(s.client.ctx, &sessionData); err != nil {
		return fmt.Errorf("error getting session %s: %w", sessionID, err)
	}

	session := &Session{
		ID:     sessionData.SessionID,
		Offer:  sessionData.Offer,
		Answer: sessionData.Answer,
	}

	s.currentSession = session
	return nil
}

func (s *SessionService) UpdateOffer(offer string) error {
	if s.currentSession == nil {
		return fmt.Errorf("no current session to update offer")
	}

	s.currentSession.Offer = offer
	updates := map[string]any{
		"offer": offer,
	}
	return s.updateSession(updates)
}

func (s *SessionService) UpdateAnswer(answer string) error {
	if s.currentSession == nil {
		return fmt.Errorf("no current session to update answer")
	}

	s.currentSession.Answer = answer
	updates := map[string]any{
		"answer": answer,
	}
	return s.updateSession(updates)
}

func (s *SessionService) CheckForAnswer() (<-chan string, <-chan error) {
	answerChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(answerChan)
		defer close(errorChan)

		if s.currentSession == nil {
			errorChan <- fmt.Errorf("no current session to check for answer")
			return
		}

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
			sessionRef := s.ref.Child(s.currentSession.ID)
			if err := sessionRef.Get(s.client.ctx, &sessionData); err != nil {
				log.Println(err.Error())
				continue
			}

			if sessionData.Answer != "" {
				s.currentSession.Answer = sessionData.Answer // Update local state
				answerChan <- sessionData.Answer
				return
			}

			if numOfCheck > 0 {
				// Wait 5 seconds before checking again for answer
				time.Sleep(time.Second * 5)
			}
		}

		// Delete the session if no answer is received
		err := s.DeleteCurrentSession()
		if err != nil {
			errorChan <- fmt.Errorf("error delete session: %s", err.Error())
		}

		errorChan <- fmt.Errorf("timeout waiting for answer")
	}()

	return answerChan, errorChan
}

// This should be call by sender after file transfering has completed
func (s *SessionService) DeleteCurrentSession() error {
	if s.currentSession == nil {
		return fmt.Errorf("no current session to delete")
	}

	sessionRef := s.ref.Child(s.currentSession.ID)
	if err := sessionRef.Delete(s.client.ctx); err != nil {
		return fmt.Errorf("error deleting session %s: %w", s.currentSession.ID, err)
	}

	s.currentSession = nil
	return nil
}

func (s *SessionService) GetOffer() (string, error) {
	if s.currentSession == nil {
		return "", fmt.Errorf("no current session to get offer from")
	}

	// Fetch fresh offer data from storage
	var sessionData struct {
		Offer string `json:"offer"`
	}
	sessionRef := s.ref.Child(s.currentSession.ID)
	if err := sessionRef.Get(s.client.ctx, &sessionData); err != nil {
		return "", fmt.Errorf("error fetching offer from storage: %w", err)
	}

	// Update local state with fresh data
	s.currentSession.Offer = sessionData.Offer

	return sessionData.Offer, nil
}

func (s *SessionService) GetSessionID() (string, error) {
	if s.currentSession == nil {
		return "", fmt.Errorf("no current session")
	}

	return s.currentSession.ID, nil
}

// updateSession is a helper method for updating session data
func (s *SessionService) updateSession(updates map[string]any) error {
	if s.currentSession == nil {
		return fmt.Errorf("no current session to update")
	}

	sessionRef := s.ref.Child(s.currentSession.ID)
	if err := sessionRef.Update(s.client.ctx, updates); err != nil {
		return fmt.Errorf("error updating session %s: %w", s.currentSession.ID, err)
	}
	return nil
}
