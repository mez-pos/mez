package auth

import "sync"

// keyedMutex is a mutex per key, dropped when nobody holds or waits on it,
// so a key that is never seen again does not stay in memory.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedEntry
}

type keyedEntry struct {
	mu   sync.Mutex
	refs int
}

// lock takes the mutex for key and returns the function that releases it.
func (k *keyedMutex) lock(key string) (unlock func()) {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = map[string]*keyedEntry{}
	}
	e := k.locks[key]
	if e == nil {
		e = &keyedEntry{}
		k.locks[key] = e
	}
	e.refs++
	k.mu.Unlock()

	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		k.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}
