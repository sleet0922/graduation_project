package service

import "sync"

// Identity changes must not interleave with message acceptance. A read lock
// lets normal sends proceed concurrently while a public-key rotation gets an
// atomic window for its key update and associated group rotations.
var e2eeIdentityStateLock sync.RWMutex

func lockE2EEIdentityRead() func() {
	e2eeIdentityStateLock.RLock()
	return e2eeIdentityStateLock.RUnlock
}

func lockE2EEIdentityWrite() func() {
	e2eeIdentityStateLock.Lock()
	return e2eeIdentityStateLock.Unlock
}
