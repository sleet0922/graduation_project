package service

import "sync"

const e2eeGroupStateLockCount = 64

var e2eeGroupStateLocks [e2eeGroupStateLockCount]sync.Mutex

func lockE2EEGroupState(groupID uint) func() {
	lock := &e2eeGroupStateLocks[groupID%e2eeGroupStateLockCount]
	lock.Lock()
	return lock.Unlock
}
