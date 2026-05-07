package service

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mohammadpnp/content-moderator/internal/adapter/inbound/websocket"
	"github.com/mohammadpnp/content-moderator/internal/domain/entity"
	"github.com/mohammadpnp/content-moderator/internal/domain/port/outbound"
)

const redisChannel = "moderation:notifications"

type NotificationBridge struct {
	broker      outbound.MessageBroker
	broadcaster outbound.RealtimeBroadcaster
}

func NewNotificationBridge(broker outbound.MessageBroker, broadcaster outbound.RealtimeBroadcaster) *NotificationBridge {
	return &NotificationBridge{
		broker:      broker,
		broadcaster: broadcaster,
	}
}

func (b *NotificationBridge) Start(ctx context.Context) error {
	return b.broker.SubscribeNotifications(ctx, func(notif *entity.Notification) error {
		rt := websocket.RealtimeNotification{
			UserID:    notif.UserID,
			ContentID: notif.ContentID,
			Type:      string(notif.Type),
			Message:   notif.Message,
		}
		payload, err := json.Marshal(rt)
		if err != nil {
			log.Printf("Bridge marshal error: %v", err)
			return err
		}
		return b.broadcaster.Publish(ctx, redisChannel, payload)
	})
}
