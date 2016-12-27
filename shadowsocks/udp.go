package shadowsocks

import (
	"bytes"
	"errors"
	"net"
)

type UDPConn struct {
	*net.UDPConn
	*Cipher
	readBuf []byte
	// writeBuf []byte
	// for shadowsocks-go
	natlist     NATlist
	UserID      uint32
	WriteBucket *Bucket
	ReadBucket  *Bucket
}

func UDPDecryptData(n int, data []byte, cipher *Cipher, output []byte) (int, []byte, error) {
	if (n - 4) < cipher.info.ivLen {
		return 0, nil, errors.New("Cannot decrypt")
	}
	iv := make([]byte, cipher.info.ivLen)
	copy(iv, data[4:4+cipher.info.ivLen])
	if err := cipher.initDecrypt(iv); err != nil {
		return 0, nil, err
	}
	cipher.decrypt(output[0:n-cipher.info.ivLen-4], data[cipher.info.ivLen+4:n])
	ret := n - cipher.info.ivLen - 4
	return ret, iv, nil
}

func NewUDPConn(c *net.UDPConn, cipher *Cipher) *UDPConn {
	return &UDPConn{
		UDPConn: c,
		Cipher:  cipher,
		readBuf: leakyBuf.Get(),
		// for thread safety
		// writeBuf: leakyBuf.Get(),
		// for shadowsocks-go
		natlist: NATlist{conns: map[string]*CachedUDPConn{}},
	}
}

func (c *UDPConn) GetIv() (iv []byte) {
	iv = make([]byte, len(c.iv))
	copy(iv, c.iv)
	return
}

func (c *UDPConn) GetKey() (key []byte) {
	key = make([]byte, len(c.key))
	copy(key, c.key)
	return
}

func (c *UDPConn) GetUserStatistic() *UserStatistic {
	return GetUserStatistic(c.UserID)
}

func (c *UDPConn) Close() error {
	leakyBuf.Put(c.readBuf)
	return c.UDPConn.Close()
}

func (c *UDPConn) Read(b []byte) (n int, err error) {
	buf := c.readBuf
	n, err = c.UDPConn.Read(buf[0:])
	if err != nil {
		return
	}

	iv := buf[:c.info.ivLen]
	if err = c.initDecrypt(iv); err != nil {
		return
	}
	c.decrypt(b[0:n-c.info.ivLen], buf[c.info.ivLen:n])
	n = n - c.info.ivLen
	if c.ota {
		key := c.GetKey()
		authData := b[n-10 : n]
		authHmacSha1 := HmacSha1(append(iv, key...), b[:n-10])
		if !bytes.Equal(authData, authHmacSha1) {
			err = errors.New("[udp]auth failed")
			return
		}
		n = n - 10
	}
	return
}

func (c *UDPConn) ReadFrom(b []byte) (n int, src net.Addr, err error) {
	n, src, err = c.UDPConn.ReadFrom(c.readBuf[0:])
	if err != nil {
		return
	}
	if n < c.info.ivLen {
		return 0, nil, errors.New("[udp]read error: cannot decrypt")
	}
	iv := make([]byte, c.info.ivLen)
	copy(iv, c.readBuf[:c.info.ivLen])
	if err = c.initDecrypt(iv); err != nil {
		return
	}
	c.decrypt(b[0:n-c.info.ivLen], c.readBuf[c.info.ivLen:n])
	n = n - c.info.ivLen
	return
}

// Maybe some thread safe issue with Write and encryption
func (c *UDPConn) Write(b []byte) (n int, err error) {
	dataStart := 0

	var iv []byte
	iv, err = c.initEncrypt()
	if err != nil {
		return
	}
	// Put initialization vector in buffer, do a single write to send both
	// iv and data.
	cipherData := make([]byte, len(b)+len(iv))
	copy(cipherData, iv)
	dataStart = len(iv)

	c.encrypt(cipherData[dataStart:], b)
	n, err = c.UDPConn.Write(cipherData)
	return
}

func (c *UDPConn) WriteTo(b []byte, dst net.Addr) (n int, err error) {
	var iv []byte
	iv, err = c.initEncrypt()
	if err != nil {
		return
	}
	// Put initialization vector in buffer, do a single write to send both
	// iv and data.
	cipherData := make([]byte, len(b)+len(iv))
	copy(cipherData, iv)
	dataStart := len(iv)

	c.encrypt(cipherData[dataStart:], b)
	n, err = c.UDPConn.WriteTo(cipherData, dst)
	return
}

func (c *UDPConn) WriteToUDP(b []byte, dst *net.UDPAddr, auth bool) (n int, err error) {
	var iv []byte
	iv, err = c.initEncrypt()
	if err != nil {
		return
	}
	// Put initialization vector in buffer, do a single write to send both
	// iv and data.
	dataLen := len(b) + len(iv)
	if auth {
		dataLen += 10
	}
	cipherData := make([]byte, dataLen)
	copy(cipherData, iv)
	dataStart := len(iv)
	if auth {
		key := c.GetKey()
		authHmacSha1 := HmacSha1(append(iv, key...), b)
		c.encrypt(cipherData[dataStart:], append(b, authHmacSha1...))
	} else {
		c.encrypt(cipherData[dataStart:], b)
	}
	n, err = c.UDPConn.WriteToUDP(cipherData, dst)
	if n > 0 {
		c.GetUserStatistic().IncOutBytes(n)
		if c.WriteBucket != nil {
			c.WriteBucket.WaitMaxDuration(int64(n), RateLimitWaitMaxDuration)
		}
	}
	return
}

func (c *UDPConn) WriteWithUserID(b []byte, userID []byte) (n int, err error) {
	var iv []byte
	iv, err = c.initEncrypt()
	if err != nil {
		return
	}
	// Put initialization vector in buffer, do a single write to send both
	// iv and data.
	dataLen := len(b) + len(iv) + 4
	if c.ota {
		dataLen += 10
	}
	cipherData := make([]byte, dataLen)
	copy(cipherData, userID)
	copy(cipherData[4:], iv)
	dataStart := len(iv) + 4
	if c.ota {
		key := c.GetKey()
		authHmacSha1 := HmacSha1(append(iv, key...), b)
		c.encrypt(cipherData[dataStart:], append(b, authHmacSha1...))
	} else {
		c.encrypt(cipherData[dataStart:], b)
	}

	n, err = c.UDPConn.Write(cipherData)
	if n > 0 {
		c.GetUserStatistic().IncOutBytes(n)
		if c.WriteBucket != nil {
			c.WriteBucket.WaitMaxDuration(int64(n), RateLimitWaitMaxDuration)
		}
	}
	return
}

func (c *UDPConn) IsOta() bool {
	return c.ota
}
