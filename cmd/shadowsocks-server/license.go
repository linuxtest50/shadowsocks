package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var LicenseLimit *LicenseConfig
var LLock sync.RWMutex = sync.RWMutex{}

func GetLicenseLimit() *LicenseConfig {
	LLock.RLock()
	defer LLock.RUnlock()
	return LicenseLimit
}

func UpdateLicenseLimit() {
	license, err := loadLicense()
	if err != nil {
		fmt.Println("Cannot Load License Exit!")
		os.Exit(1)
	}
	cfg, err := VerifyLicense(license)
	if err != nil {
		cfg = &LicenseConfig{}
	}
	LLock.Lock()
	LicenseLimit = cfg
	LLock.Unlock()
}

func LicenseChecker() {
	for {
		time.Sleep(60 * time.Second)
		UpdateLicenseLimit()
		lcfg := GetLicenseLimit()
		if lcfg.IsExpired() {
			fmt.Println("License is Expired!")
		} else {
			fmt.Println("License is Reloaded,")
		}
		fmt.Printf("Expire: %v, Max Users: %d, Max Servers: %d, Max Bandwidth: %d\n", lcfg.Expire, lcfg.MaxUsers, lcfg.MaxServers, lcfg.MaxBandwidth)
	}
}

func initLicense() error {
	license, err := loadLicense()
	if err != nil {
		return err
	}
	cfg, err := VerifyLicense(license)
	if err != nil {
		cfg = &LicenseConfig{}
	}
	LicenseLimit = cfg
	go LicenseChecker()
	return nil
}

type LicenseConfig struct {
	Expire       time.Time
	MaxBandwidth int
	MaxUsers     int
	MaxServers   int
}

func (c *LicenseConfig) IsExpired() bool {
	return time.Now().After(c.Expire)
}

func createLicenseConfig(data map[string]string) *LicenseConfig {
	ret := LicenseConfig{}
	num_server, have := data["numserver"]
	if have {
		inum_server, err := strconv.Atoi(num_server)
		if err == nil {
			ret.MaxServers = inum_server
		}
	}
	num_user, have := data["numuser"]
	if have {
		inum_user, err := strconv.Atoi(num_user)
		if err == nil {
			ret.MaxUsers = inum_user
		}
	}
	max_bw, have := data["maxbw"]
	if have {
		imax_bw, err := strconv.Atoi(max_bw)
		if err == nil {
			ret.MaxBandwidth = imax_bw
		}
	}
	expire, have := data["expire"]
	if have {
		texpire, err := time.Parse("2006-1-1", expire)
		if err == nil {
			ret.Expire = texpire
		}
	}
	return &ret
}

func parseConfig(data string) map[string]string {
	ret := make(map[string]string)
	pairs := strings.Split(data, "|")
	for _, pair := range pairs {
		kvp := strings.Split(pair, ":")
		if len(kvp) != 2 {
			continue
		}
		ret[kvp[0]] = kvp[1]
	}
	return ret
}

func VerifyLicense(license *License) (*LicenseConfig, error) {
	dlicense, err := base64.StdEncoding.DecodeString(license.License)
	if err != nil {
		return nil, err
	}
	cscfg := parseConfig(string(dlicense))
	config, have := cscfg["config"]
	if !have {
		return nil, errors.New("Do not have config")
	}
	dconfig, err := base64.StdEncoding.DecodeString(config)
	if err != nil {
		return nil, err
	}
	config = string(dconfig)
	sign, have := cscfg["sign"]
	if !have {
		return nil, errors.New("Do not have sign")
	}
	dsign, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return nil, err
	}
	// Now we should verify RSA sign
	message := config + license.FingerPrint
	verify := VerifyPKCS(message, dsign)
	if !verify {
		return nil, errors.New("License not valid")
	}
	lcfg := parseConfig(config)
	ret := createLicenseConfig(lcfg)
	return ret, nil
}

func VerifyPKCS(message string, dsign []byte) bool {
	publicKey := getPublicKey()
	data := sha256.Sum256([]byte(message))
	err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, data[:], dsign)
	if err != nil {
		return false
	}
	return true
}

func getPublicKey() *rsa.PublicKey {
	publicKeyPem := `MIIBCgKCAQEA11LzgDq5c2Ts3eSQ95wvt6Lqm5KT86X81ofTD23mZYoStX7qg4Qu
TeT3UOrjcMzGJ/zFSkU0d+A9My5zlp4fN+wozuOXQHo/bbMDG46s2fMkHxT/h+kY
sXUfJIURJ12N1FaSOhCSToIHCr9jbm7aQgECqPHTTQz1chl3BA2ggDPkD16gHxc1
Up2a6GbONE5o0h/OpFsT3qJueNR2gYfkACiBONj2yY6YINyMgKDrKvcY5/nmi5zg
HOKYis4QzQ4f3HmUyKfCrRkvWa0e+rZL6/nl0zcSk2338+zDV7zxkRa/iXxaDMee
LclgkDFsEMY/3Ytfeiiz0mV4nqKdUMsAfQIDAQAB`

	pkdata, _ := base64.StdEncoding.DecodeString(publicKeyPem)
	pubKey := &rsa.PublicKey{}
	_, err := asn1.Unmarshal(pkdata, pubKey)
	if err != nil {
		os.Exit(1)
	}
	return pubKey
}
