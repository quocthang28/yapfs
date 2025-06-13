package signalling

import (
	"context"
	"fmt"

	"yapfs/internal/config"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/db"
	"google.golang.org/api/option"
)

type firebaseClient struct {
	db  *db.Client
	ctx context.Context
}

func NewClient(ctx context.Context, cfg *config.FirebaseConfig) (*firebaseClient, error) {
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

	return &firebaseClient{
		db:  client,
		ctx: ctx,
	}, nil
}

func (c *firebaseClient) NewSessionService() *SessionService {
	return &SessionService{
		client: c,
		ref:    c.db.NewRef("sessions"),
	}
}
