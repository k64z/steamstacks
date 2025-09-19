package steamsession

import (
	"github.com/k64z/steamstacks/protocol"
)

type Option func(s *Session)

func WithPlatformType(platformType PlatformType) Option {
	return func(s *Session) {
		s.platformType = protocol.EAuthTokenPlatformType(platformType)
	}
}

func WithPersistence(persistence Persistence) Option {
	return func(s *Session) {
		s.persistence = protocol.ESessionPersistence(persistence)
	}
}
