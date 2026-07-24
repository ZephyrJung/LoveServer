package hub

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/ZephyrJung/LoveServer/internal/session"
)

type Handler interface {
	Routes() []string
	Handle(s *session.Session, msg *Message) error
}

type Middleware func(s *session.Session, msg *Message, next func() error) error

type Hub struct {
	handlers    []Handler
	middlewares []Middleware
}

func New() *Hub {
	return &Hub{
		handlers: make([]Handler, 0),
	}
}

func (h *Hub) RegisterHandler(handler Handler) {
	h.handlers = append(h.handlers, handler)
}

func (h *Hub) Use(m Middleware) {
	h.middlewares = append(h.middlewares, m)
}

func (h *Hub) HandleMessage(s *session.Session, raw []byte) {
	msg := &Message{}
	if err := json.Unmarshal(raw, msg); err != nil {
		log.Printf("invalid message from %s: %v", s.ID, err)
		s.SendJSON(NewError("", ErrInvalidMessage, "invalid json", ""))
		return
	}

	chain := h.buildChain(s, msg, func() error {
		return h.dispatch(s, msg)
	})

	if err := chain(); err != nil {
		log.Printf("handle message error: %v", err)
	}
}

func (h *Hub) dispatch(s *session.Session, msg *Message) error {
	for _, handler := range h.handlers {
		for _, route := range handler.Routes() {
			if strings.HasPrefix(msg.Type, route) {
				return handler.Handle(s, msg)
			}
		}
	}
	return s.SendJSON(NewError(msg.Type, ErrInvalidMessage, "unknown message type: "+msg.Type, msg.ID))
}

func (h *Hub) buildChain(s *session.Session, msg *Message, final func() error) func() error {
	chain := final
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		mw := h.middlewares[i]
		next := chain
		chain = func() error {
			return mw(s, msg, next)
		}
	}
	return chain
}