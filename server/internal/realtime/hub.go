package realtime

import (
	"sync"
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[int]map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[int]map[chan Event]struct{})}
}

func (h *Hub) Subscribe(userID int) (<-chan Event, func()) {
	channel := make(chan Event, 16)
	h.mu.Lock()
	if h.subscribers[userID] == nil {
		h.subscribers[userID] = make(map[chan Event]struct{})
	}
	h.subscribers[userID][channel] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subscribers[userID], channel)
			if len(h.subscribers[userID]) == 0 {
				delete(h.subscribers, userID)
			}
			h.mu.Unlock()
			close(channel)
		})
	}
	return channel, cancel
}

func (h *Hub) Publish(userID int, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for channel := range h.subscribers[userID] {
		select {
		case channel <- event:
		default:
			// A slow client must not block the write path. Replace the oldest
			// queued event with a resync signal so clients refetch authoritative
			// notification/feed state instead of remaining stale.
			select {
			case <-channel:
			default:
			}
			select {
			case channel <- Event{Type: "stream.resync"}:
			default:
			}
		}
	}
}
