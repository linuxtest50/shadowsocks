package shadowsocks

import (
	"sync"
)

type UserStatistic struct {
	UserID      uint32
	BytesIn     uint64
	BytesOut    uint64
	Connections uint64
	lock        sync.Mutex
}

func init() {
	userStatisticMap = make(map[uint32]*UserStatistic)
}

var userStatisticMap map[uint32]*UserStatistic

func (us *UserStatistic) IncBytes(inBytes, outBytes int) {
	us.lock.Lock()
	defer us.lock.Unlock()
	us.BytesIn += uint64(inBytes)
	us.BytesOut += uint64(outBytes)
}

func (us *UserStatistic) IncInBytes(bytes int) {
	us.lock.Lock()
	defer us.lock.Unlock()
	us.BytesIn += uint64(bytes)
}

func (us *UserStatistic) IncOutBytes(bytes int) {
	us.lock.Lock()
	defer us.lock.Unlock()
	us.BytesOut += uint64(bytes)
}

func (us *UserStatistic) IncConnections() {
	us.lock.Lock()
	defer us.lock.Unlock()
	us.Connections += 1
}

func GetUserStatistic(userID uint32) *UserStatistic {
	us, have := userStatisticMap[userID]
	if !have {
		nus := &UserStatistic{
			UserID:      userID,
			BytesIn:     0,
			BytesOut:    0,
			Connections: 0,
		}
		userStatisticMap[userID] = nus
		return nus
	} else {
		return us
	}
}

func GetUserStatisticMap() map[uint32]*UserStatistic {
	return userStatisticMap
}
