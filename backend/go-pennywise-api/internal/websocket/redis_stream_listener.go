package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	redisPubsubStream       = "pubsub"
	redisStreamReadBlock    = 5 * time.Second
	redisStreamReadCount    = 10
	redisStreamRetryBackoff = time.Second
)

type RedisStreamListener struct {
	redisClient *redis.Client
	hub         ConnectionHub
	stream      string
}

func NewRedisStreamListener(redisClient *redis.Client, hub ConnectionHub) *RedisStreamListener {
	return &RedisStreamListener{
		redisClient: redisClient,
		hub:         hub,
		stream:      redisPubsubStream,
	}
}

func (l *RedisStreamListener) Listen(ctx context.Context) {
	log := logger.Logger(ctx)
	lastID := "$"

	log.Info("redis websocket stream listener started", "stream", l.stream)
	for {
		streams, err := l.redisClient.XRead(ctx, &redis.XReadArgs{
			Streams: []string{l.stream, lastID},
			Count:   redisStreamReadCount,
			Block:   redisStreamReadBlock,
		}).Result()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, redis.Nil) {
				if errors.Is(err, context.Canceled) {
					log.Info("redis websocket stream listener stopped", "stream", l.stream)
					return
				}
				continue
			}

			log.Error("error while reading redis websocket stream", "stream", l.stream, "error", err)
			if !sleepOrDone(ctx, redisStreamRetryBackoff) {
				log.Info("redis websocket stream listener stopped", "stream", l.stream)
				return
			}
			continue
		}

		for _, stream := range streams {
			for _, redisMessage := range stream.Messages {
				lastID = redisMessage.ID

				message, err := websocketMessageFromRedisValues(redisMessage.Values)
				if err != nil {
					log.Warn(
						"skipping invalid redis websocket stream message",
						"stream", stream.Stream,
						"id", redisMessage.ID,
						"error", err,
					)
					continue
				}

				l.hub.Broadcast(message, nil)
			}
		}
	}
}

func websocketMessageFromRedisValues(values map[string]any) (sharedModel.Message, error) {
	eventName, err := stringValue(values, "eventName", "EventName")
	if err != nil {
		return sharedModel.Message{}, err
	}

	budgetIDValue, err := stringValue(values, "budgetId", "BudgetId")
	if err != nil {
		return sharedModel.Message{}, err
	}
	budgetID, err := parseRedisUUID(budgetIDValue)
	if err != nil {
		return sharedModel.Message{}, fmt.Errorf("invalid budgetId: %w", err)
	}

	userIDValue, err := stringValue(values, "userId", "UserId")
	if err != nil {
		return sharedModel.Message{}, err
	}
	_, err = parseRedisUUID(userIDValue)
	if err != nil {
		return sharedModel.Message{}, fmt.Errorf("invalid userId: %w", err)
	}

	convoIDValue, err := stringValue(values, "conversationId", "ConversationId")
	if err != nil {
		return sharedModel.Message{}, err
	}
	_, err = parseRedisUUID(convoIDValue)
	if err != nil {
		return sharedModel.Message{}, fmt.Errorf("invalid conversationId: %w", err)
	}

	data, ok := firstValue(values, "data", "Data")
	if !ok {
		data = nil
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return sharedModel.Message{}, errs.Wrap(
			errs.CodeInternalError,
			"error while marshalling data from redis for ws",
			err,
		)
	}
	roomID := fmt.Sprintf("%s:%s:%s", budgetIDValue, userIDValue, "chat/"+convoIDValue)
	return sharedModel.Message{
		EventName: eventName,
		Data:      dataJSON,
		BudgetID:  budgetID,
		RoomID:    &roomID,
	}, nil
}

func stringValue(values map[string]any, keys ...string) (string, error) {
	value, ok := firstValue(values, keys...)
	if !ok {
		return "", fmt.Errorf("missing field %q", keys[0])
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("field %q must be a string, got %T", keys[0], value)
	}
	if str == "" {
		return "", fmt.Errorf("field %q cannot be empty", keys[0])
	}

	return str, nil
}

func firstValue(values map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		value, ok := values[key]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func parseRedisUUID(value string) (uuid.UUID, error) {
	if id, err := uuid.Parse(value); err == nil {
		return id, nil
	}

	if len(value) == 16 {
		return uuid.FromBytes([]byte(value))
	}

	return uuid.Nil, fmt.Errorf("expected UUID string or 16 byte value")
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
