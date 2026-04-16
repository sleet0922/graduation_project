package rtc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"sort"
	"time"
)

const (
	Version                    = "001" // Token 版本号
	PrivPublishStream   uint16 = 0     // 发布流：开关
	privPublishAudio    uint16 = 1     // 发布音频
	privPublishVideo    uint16 = 2     // 发布视频
	privPublishData     uint16 = 3     // 发布数据
	PrivSubscribeStream uint16 = 4     // 订阅流：看别人嗒
)

type AccessToken struct {
	appID      string
	appKey     string
	roomID     string
	userID     string
	issuedAt   uint32
	nonce      uint32 // 随机数，防重放
	expireAt   uint32
	privileges map[uint16]uint32 //权限类型-过期时间
}

func NewAccessToken(appID, appKey, roomID, userID string) *AccessToken {
	return &AccessToken{
		appID:      appID,
		appKey:     appKey,
		roomID:     roomID,
		userID:     userID,
		issuedAt:   uint32(time.Now().Unix()),
		nonce:      randomNonce(),
		privileges: make(map[uint16]uint32),
	}
}

// 转换为Unix时间戳
func toUnix(t time.Time) uint32 {
	if t.IsZero() {
		return 0
	}
	return uint32(t.Unix())
}

// 设置过期时间
func (t *AccessToken) ExpireTime(expireAt time.Time) {
	t.expireAt = toUnix(expireAt)
}

// 添加权限
func (t *AccessToken) AddPrivilege(privilege uint16, expireAt time.Time) {
	ts := toUnix(expireAt)
	t.privileges[privilege] = ts
	if privilege == PrivPublishStream {
		t.privileges[privPublishAudio] = ts
		t.privileges[privPublishVideo] = ts
		t.privileges[privPublishData] = ts
	}
}

// 序列化Token
func (t *AccessToken) Serialize() string {
	msg := t.packMsg()
	signature := sign(t.appKey, msg)
	content := append(packBytes(msg), packBytes(signature)...)
	return Version + t.appID + base64.StdEncoding.EncodeToString(content)
}

// 打包消息数据
func (t *AccessToken) packMsg() []byte {
	msg := make([]byte, 0, 64)
	msg = append(msg, packUint32(t.nonce)...)
	msg = append(msg, packUint32(t.issuedAt)...)
	msg = append(msg, packUint32(t.expireAt)...)
	msg = append(msg, packString(t.roomID)...)
	msg = append(msg, packString(t.userID)...)
	msg = append(msg, packMapUint32(t.privileges)...)
	return msg
}

// 签名
func sign(appKey string, msg []byte) []byte {
	mac := hmac.New(sha256.New, []byte(appKey))
	mac.Write(msg)
	return mac.Sum(nil)
}

// 生成随机数
func randomNonce() uint32 {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return uint32(time.Now().UnixNano()%99999999) + 1
	}
	nonce := binary.LittleEndian.Uint32(buf[:]) % 99999999
	if nonce == 0 {
		return 1
	}
	return nonce
}

func packUint16(v uint16) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, v)
	return buf
}

func packUint32(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}

func packString(v string) []byte {
	return packBytes([]byte(v))
}

func packBytes(v []byte) []byte {
	buf := packUint16(uint16(len(v)))
	return append(buf, v...)
}

func packMapUint32(m map[uint16]uint32) []byte {
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, int(key))
	}
	sort.Ints(keys)

	buf := packUint16(uint16(len(m)))
	for _, key := range keys {
		buf = append(buf, packUint16(uint16(key))...)
		buf = append(buf, packUint32(m[uint16(key)])...)
	}
	return buf
}
