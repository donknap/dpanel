package stats

import (
	"sync"
)

type Collect struct {
	mu   sync.RWMutex
	List []*Container
}

func (s *Collect) Add(container *Container) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer(container.GetStatistics().Name); !exists {
		s.List = append(s.List, container)
		return true
	}
	return false
}

func (self *Collect) Lock() {
	self.mu.RLock()
}

func (self *Collect) Unlock() {
	self.mu.RUnlock()
}

func (self *Collect) isKnownContainer(cid string) (int, bool) {
	for i, c := range self.List {
		if usage := c.GetStatistics(); usage != nil && usage.Name == cid {
			return i, true
		}
	}
	return -1, false
}
