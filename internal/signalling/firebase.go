package signalling

import (
	"context"
	"fmt"
	"log"
	"time"

	"yapfs/internal/config"
	"yapfs/pkg/utils"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"google.golang.org/api/option"
)

type FirebaseClient struct {
	db  *db.Client
	ctx context.Context
	ref *db.Ref
}

func NewFirebaseClient(ctx context.Context, cfg *config.FirebaseConfig) (*FirebaseClient, error) {
	opt := option.WithCredentialsFile(cfg.CredentialsPath)

	firebaseConfig := &firebase.Config{
		DatabaseURL: cfg.DatabaseURL,
	}

	app, err := firebase.NewApp(ctx, firebaseConfig, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing Firebase app: %w", err)
	}

	client, err := app.Database(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting database client: %w", err)
	}

	return &FirebaseClient{
		db:  client,
		ctx: ctx,
		ref: client.NewRef("sessions"),
	}, nil
}

// Session represents a signaling session data
// Support vanilla ICE for now,
// TODO: support trickle ICE, add support for expire/time out and status(create, connected, failed)
type Session struct {
	ID     string `json:"sessionId"`
	Offer  string `json:"offer"`
	Answer string `json:"answer"`
}

func (f *FirebaseClient) CreateSession(ctx context.Context, offer string) (string, error) {
	code, err := utils.GenerateCode(8)
	if err != nil {
		return "", fmt.Errorf("error generating session code: %w", err)
	}

	sessionRef := f.ref.Child(code)
	sessionData := map[string]any{
		"sessionId": code,
		"offer":     offer,
		"answer":    "",
	}
	err = sessionRef.Set(f.ctx, sessionData)
	if err != nil {
		return "", fmt.Errorf("error creating session: %w", err)
	}

	log.Println("Session created successfully")
	return code, nil
}

func (f *FirebaseClient) UpdateAnswer(ctx context.Context, sessionID, answer string) error {
	// First check if session exists
	var sessionData Session
	
	sessionRef := f.ref.Child(sessionID)
	if err := sessionRef.Get(f.ctx, &sessionData); err != nil {
		return fmt.Errorf("error checking session existence for %s: %w", sessionID, err)
	}

	if sessionData.ID == "" {
		return fmt.Errorf("session %s not found", sessionID)
	}

	updates := map[string]any{
		"answer": answer,
	}
	if err := sessionRef.Update(f.ctx, updates); err != nil {
		return fmt.Errorf("error updating answer for session %s: %w", sessionID, err)
	}
	return nil
}

func (f *FirebaseClient) WaitForAnswer(ctx context.Context, sessionID string) (string, error) {
	// First verify the session exists
	var initialCheck Session

	sessionRef := f.ref.Child(sessionID)
	if err := sessionRef.Get(f.ctx, &initialCheck); err != nil {
		return "", fmt.Errorf("error checking session existence for %s: %w", sessionID, err)
	}

	if initialCheck.ID == "" {
		return "", fmt.Errorf("session %s not found", sessionID)
	}

	time.Sleep(time.Second * 5)
	log.Printf("Waiting for receiver to answer...")

	for i := range 10 {
		var sessionData struct {
			Answer string `json:"answer"`
		}
		if err := sessionRef.Get(f.ctx, &sessionData); err != nil {
			log.Println(err.Error())
			continue
		}

		if sessionData.Answer != "" {
			return sessionData.Answer, nil
		}

		if i < 9 {
			select {
			case <-time.After(time.Second * 5):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}

	if err := f.DeleteSession(ctx, sessionID); err != nil {
		return "", fmt.Errorf("error deleting session: %w", err)
	}

	return "", fmt.Errorf("timeout waiting for answer")
}

func (f *FirebaseClient) DeleteSession(ctx context.Context, sessionID string) error {
	// Check if session exists before attempting deletion
	var sessionData Session

	sessionRef := f.ref.Child(sessionID)
	if err := sessionRef.Get(f.ctx, &sessionData); err != nil {
		return fmt.Errorf("error checking session existence for %s: %w", sessionID, err)
	}

	if sessionData.ID == "" {
		// Session doesn't exist, but this is not an error for cleanup operations
		log.Printf("Session %s not found, skipping deletion", sessionID)
		return nil
	}

	if err := sessionRef.Delete(f.ctx); err != nil {
		return fmt.Errorf("error deleting session %s: %w", sessionID, err)
	}
	return nil
}

func (f *FirebaseClient) GetOffer(ctx context.Context, sessionID string) (string, error) {
	sessionRef := f.ref.Child(sessionID)

	var sessionData Session
	if err := sessionRef.Get(f.ctx, &sessionData); err != nil {
		return "", fmt.Errorf("error fetching session from storage for session %s: %w", sessionID, err)
	}

	// Validate that the session actually exists and has an offer
	if sessionData.ID == "" || sessionData.Offer == "" {
		return "", fmt.Errorf("session %s not found or has no offer", sessionID)
	}

	return sessionData.Offer, nil
}
