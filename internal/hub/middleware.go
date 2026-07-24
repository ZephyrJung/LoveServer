package hub

import (
	"log"
	"strings"
	"time"

	"github.com/ZephyrJung/LoveServer/internal/session"
)

func LoggerMiddleware() Middleware {
	return func(s *session.Session, msg *Message, next func() error) error {
		start := time.Now()
		log.Printf("[%s] ← %s from %s", s.ID, msg.Type, s.PlayerID)
		err := next()
		log.Printf("[%s] → %s (%v)", s.ID, msg.Type, time.Since(start))
		return err
	}
}

func AuthMiddleware() Middleware {
	return func(s *session.Session, msg *Message, next func() error) error {
		if strings.HasPrefix(msg.Type, "auth.") {
			return next()
		}
		if !s.Authed {
			return s.SendJSON(NewError(msg.Type, ErrUnauthenticated, "not authenticated", msg.ID))
		}
		return next()
	}
}

func RateLimitMiddleware() Middleware {
	return func(s *session.Session, msg *Message, next func() error) error {
		return next()
	}
}