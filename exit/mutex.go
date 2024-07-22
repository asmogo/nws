package exit

import (
	"fmt"
	"sync"
)

type MutexMap struct {
	mu sync.Mutex             // a separate mutex to protect the map
	m  map[string]*sync.Mutex // map from IDs to mutexes
}

func NewMutexMap() *MutexMap {
	return &MutexMap{
		m: make(map[string]*sync.Mutex),
	}
}

func (mm *MutexMap) Lock(id string) {
	mm.mu.Lock()
	mutex, ok := mm.m[id]
	if !ok {
		mutex = &sync.Mutex{}
		mm.m[id] = mutex
	}
	mm.mu.Unlock()

	mutex.Lock()
}

func (mm *MutexMap) Unlock(id string) {
	mm.mu.Lock()
	mutex, ok := mm.m[id]
	mm.mu.Unlock()
	if !ok {
		panic(fmt.Sprintf("tried to unlock mutex for non-existent id %s", id))
	}

	mutex.Unlock()
}
