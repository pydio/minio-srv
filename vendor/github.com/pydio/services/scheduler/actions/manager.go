package actions

import (
	"sync"
)

var (
	defaultManager *ActionsManager
)

// Global Access to default Manager
func GetActionsManager() *ActionsManager {

	if defaultManager == nil {
		defaultManager = &ActionsManager{
			registeredActions: make(map[string]Concrete),
		}
	}
	return defaultManager

}

type ActionsManager struct {
	registeredActions map[string]Concrete
	lock              sync.Mutex
}

func (m *ActionsManager) Register(name string, a Concrete) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.registeredActions[name] = a
}

func (m *ActionsManager) ActionById(actionId string) (ConcreteAction, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	generator, ok := m.registeredActions[actionId]
	if ok {
		return generator(), true
	} else {
		return nil, false
	}
}
