package service

import (
	"context"
)

type BrokerRepository struct {
	timelineItemBroker *TimelineItemBroker
	commentBroker      *CommentBroker
	notificationBroker *NotificationBroker
}

func newBrokerRepository() *BrokerRepository {
	brokerRepository := &BrokerRepository{
		&TimelineItemBroker{
			Notifier:       make(chan TimelineItem, 1),
			NewClients:     make(chan *timelineItemClient),
			ClosingClients: make(chan *timelineItemClient),
			Clients:        make(map[string]Set[*timelineItemClient]),
		}, &CommentBroker{
			Notifier:       make(chan Comment, 1),
			NewClients:     make(chan *commentClient),
			ClosingClients: make(chan *commentClient),
			Clients:        make(map[string]Set[*commentClient]),
		}, &NotificationBroker{
			Notifier:       make(chan Notification, 1),
			NewClients:     make(chan *notificationClient),
			ClosingClients: make(chan *notificationClient),
			Clients:        make(map[string]Set[*notificationClient]),
		},
	}
	go brokerRepository.timelineItemBroker.listen()
	go brokerRepository.commentBroker.listen()
	go brokerRepository.notificationBroker.listen()
	return brokerRepository
}

type timelineItemClient struct {
	timelines chan TimelineItem
	userID    string
	ctx       context.Context
}

// A Broker holds open client connections,
// listens for incoming events on its Notifier channel
// and broadcast event data to all registered connections
type TimelineItemBroker struct {
	Notifier chan TimelineItem
	// New client connections
	NewClients chan *timelineItemClient
	// Closed client connections
	ClosingClients chan *timelineItemClient
	// Client connections registry
	Clients map[string]Set[*timelineItemClient]
}

func (broker *TimelineItemBroker) listen() {
	for {
		select {
		case s := <-broker.NewClients:
			// A new client has connected. Register their connections
			if broker.Clients[s.userID] == nil {
				broker.Clients[s.userID] = make(Set[*timelineItemClient])
			}
			broker.Clients[s.userID].Add(s)

		case s := <-broker.ClosingClients:
			// A client has dettached and we want to stop sending them messages.
			close(s.timelines)
			broker.Clients[s.userID].Remove(s)

		case timelineItem := <-broker.Notifier:
			// We got a new timelineItem from the outside! Send timelineItem to correct connected clients
			for client := range broker.Clients[timelineItem.UserID] {
				select {
				case client.timelines <- timelineItem:
				// no ops
				case <-client.ctx.Done():
					// no ops
				}
			}
		}
	}
}

type commentClient struct {
	comments chan Comment
	postID   string
	userID   *string
	ctx      context.Context
}

type CommentBroker struct {
	Notifier       chan Comment
	NewClients     chan *commentClient
	ClosingClients chan *commentClient
	Clients        map[string]Set[*commentClient]
}

func (broker *CommentBroker) listen() {
	for {
		select {
		case s := <-broker.NewClients:
			if broker.Clients[s.postID] == nil {
				broker.Clients[s.postID] = make(Set[*commentClient])
			}
			broker.Clients[s.postID].Add(s)

		case s := <-broker.ClosingClients:
			close(s.comments)
			broker.Clients[s.postID].Remove(s)

		case comment := <-broker.Notifier:
			for client := range broker.Clients[comment.PostID] {
				if !(client.userID != nil && *(client.userID) == comment.UserID) {
					select {
					case client.comments <- comment:
						// no ops
					case <-client.ctx.Done():
						// no ops
					}
				}
			}
		}
	}
}

type notificationClient struct {
	notifications chan Notification
	userID        string
	ctx           context.Context
}

type NotificationBroker struct {
	Notifier       chan Notification
	NewClients     chan *notificationClient
	ClosingClients chan *notificationClient
	Clients        map[string]Set[*notificationClient]
}

func (broker *NotificationBroker) listen() {
	for {
		select {
		case s := <-broker.NewClients:
			if broker.Clients[s.userID] == nil {
				broker.Clients[s.userID] = make(Set[*notificationClient])
			}
			broker.Clients[s.userID].Add(s)

		case s := <-broker.ClosingClients:
			close(s.notifications)
			broker.Clients[s.userID].Remove(s)

		case notification := <-broker.Notifier:
			for client := range broker.Clients[notification.UserID] {
				select {
				case client.notifications <- notification:
					// no ops
				case <-client.ctx.Done():
					// no ops
				}
			}
		}
	}
}
