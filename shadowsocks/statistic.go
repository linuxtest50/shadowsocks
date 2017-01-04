package shadowsocks

type UserStatistic struct {
	UserID      uint32
	BytesIn     uint64
	BytesOut    uint64
	Connections uint64
}

func init() {
	userStatisticMap = make(map[uint32]*UserStatistic)
}

const (
	OpIncConnections uint8 = 1
	OpIncBytesOut    uint8 = 2
	OpIncBytesIn     uint8 = 3
)

type UserStaticOp struct {
	Op     uint8
	UserID uint32
	Value  int
}

type UserStatisticService struct {
	Queue chan UserStaticOp
}

var userStatisticMap map[uint32]*UserStatistic
var userStatisticService *UserStatisticService

func CreateUserStatisticService() {
	userStatisticService = &UserStatisticService{
		Queue: make(chan UserStaticOp, 100),
	}
	go userStatisticService.RunUserStaticOpServer()
}

func GetUserStatisticService() *UserStatisticService {
	return userStatisticService
}

func (s *UserStatisticService) RunUserStaticOpServer() {
	for {
		op := <-s.Queue
		userStat := GetUserStatistic(op.UserID)
		switch op.Op {
		case OpIncConnections:
			userStat.IncConnections()
		case OpIncBytesOut:
			userStat.IncOutBytes(op.Value)
		case OpIncBytesIn:
			userStat.IncInBytes(op.Value)
		}
	}
}

func (s *UserStatisticService) IncConnections(userID uint32) {
	op := UserStaticOp{
		Op:     OpIncConnections,
		UserID: userID,
		Value:  1,
	}
	s.Queue <- op
}

func (s *UserStatisticService) IncInBytes(userID uint32, value int) {
	op := UserStaticOp{
		Op:     OpIncBytesIn,
		UserID: userID,
		Value:  value,
	}
	s.Queue <- op
}

func (s *UserStatisticService) IncOutBytes(userID uint32, value int) {
	op := UserStaticOp{
		Op:     OpIncBytesOut,
		UserID: userID,
		Value:  value,
	}
	s.Queue <- op
}

func (us *UserStatistic) IncBytes(inBytes, outBytes int) {
	us.BytesIn += uint64(inBytes)
	us.BytesOut += uint64(outBytes)
}

func (us *UserStatistic) IncInBytes(bytes int) {
	us.BytesIn += uint64(bytes)
}

func (us *UserStatistic) IncOutBytes(bytes int) {
	us.BytesOut += uint64(bytes)
}

func (us *UserStatistic) IncConnections() {
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
