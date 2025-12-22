package git

import (
	"fmt"
)

func (s *Service) LocalChanges() (LocalChanges, error) {
	if s.backend == nil {
		return LocalChanges{}, fmt.Errorf("repository root not set")
	}
	return s.backend.LocalChangesStatus()
}
