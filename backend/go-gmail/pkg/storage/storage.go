package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gmail-transactions/pkg/config"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Service struct {
	config          *config.Config
	firestoreClient *firestore.Client
}

func getFirestoreClient(config *config.Config) *firestore.Client {
	projectId := config.ProjectID
	log.Printf("Setting up firestore client %v", projectId)
	ctx := context.Background()
	credsFile := config.GoogleCloudSecretsFile
	opt := option.WithCredentialsFile(credsFile)
	firestoreClient, err := firestore.NewClient(ctx, projectId, opt)
	if err != nil {
		log.Fatalf("Error while setting up firestore client: %v", err.Error())
	}
	return firestoreClient
}

func NewService(config *config.Config) *Service {
	return &Service{config: config, firestoreClient: getFirestoreClient(config)}
}

func (s *Service) GetRefreshToken(email string) (string, error) {
	defer s.firestoreClient.Close()

	ctx := context.Background()

	collection := s.firestoreClient.Collection("users")
	query := collection.Where("email", "==", email)
	iter := query.Documents(ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		return "", err
	}
	refreshToken := doc.Data()["refresh_token"].(string)
	return refreshToken, nil
}

func (s *Service) GetPrevHistoryId(email string) (uint64, error) {
	defer s.firestoreClient.Close()

	ctx := context.Background()

	log.Printf("GetPrevHistoryId: %v", email)
	collection := s.firestoreClient.Collection("gmailHistoryIds")
	query := collection.Where("email", "==", email)
	iter := query.Documents(ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		return 0, err
	}
	// if doc == nil || !doc.Exists() {
	// 	return 0, nil
	// }
	historyId, ok := doc.Data()["historyId"].(int64)
	if !ok {
		return 0, errors.New("Cannot convert to int64")
	}
	var uintHistoryID uint64
	if historyId >= 0 {
		uintHistoryID = uint64(historyId)
	} else {
		return 0, errors.New("Negative historyId")
	}
	return uintHistoryID, nil
}

func (s *Service) UpdateHistoryId(email string, historyId uint64) error {
	defer s.firestoreClient.Close()

	ctx := context.Background()

	collection := s.firestoreClient.Collection("gmailHistoryIds")
	query := collection.Where("email", "==", email)
	docsToUpdate := query.Documents(ctx)
	for {
		doc, err := docsToUpdate.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		loc, err := time.LoadLocation("Asia/Kolkata")
		if err != nil {
			return fmt.Errorf("Error while loading timezone: %w", err)
		}
		updateData := []firestore.Update{
			{Path: "historyId", Value: int64(historyId)},
			{Path: "lastUpdatedAt", Value: time.Now().In(loc).Format("January 02, 2006 15:04:05")},
		}
		_, err = doc.Ref.Update(ctx, updateData)
		if err != nil {
			return fmt.Errorf("Error while updating history ID: %w", err)
		}
	}
	return nil
}
