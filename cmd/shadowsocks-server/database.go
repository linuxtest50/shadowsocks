package main

import (
	"database/sql"
	"fmt"
	"github.com/garyburd/redigo/redis"
	_ "github.com/go-sql-driver/mysql"
	"strconv"
	"strings"
	"time"
)

var db *sql.DB
var useRedis bool
var redisPool *redis.Pool

func initDB(url string) error {
	var err error
	db, err = sql.Open("mysql", url)
	return err
}

func initRedis(server string) error {
	var err error
	useRedis = true
	redisPool = &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}
	return err
}

func unpackCachedData(value string) (password string, bandwidth int, err error) {
	pair := strings.Split(value, "|")
	password = pair[0]
	bandwidth, err = strconv.Atoi(pair[1])
	return
}

func packCachedData(password string, bandwidth int) string {
	return fmt.Sprintf("%s|%d", password, bandwidth)
}

func getFromRedis(userID int) (have bool, password string, bandwidth int) {
	have = false
	password = ""
	bandwidth = 0
	key := fmt.Sprintf("%d", userID)
	conn := redisPool.Get()
	defer conn.Close()
	value, err := redis.String(conn.Do("GET", key))
	if err != nil {
		return
	}
	password, bandwidth, err = unpackCachedData(value)
	if err != nil {
		return
	}
	have = true
	return
}

func saveToRedis(userID int, password string, bandwidth int) {
	key := fmt.Sprintf("%d", userID)
	value := packCachedData(password, bandwidth)
	conn := redisPool.Get()
	defer conn.Close()
	status, err := conn.Do("SET", key, value, "EX", "3600")
	if debug {
		debug.Printf("%v, %v\n", status, err)
	}
}

// Database Table Format:
// table user (
//    userid int
//    password varchar(255)
//    status varchar(20)
//    bandwidth int
// )
// Status: Enabled, Disabled
//
func getPasswordAndBandwidthFromDatabase(userID int) (string, int) {
	if useRedis {
		have, password, bandwidth := getFromRedis(userID)
		if have {
			if debug {
				debug.Printf("Cache Hit for Customer: %d\n", userID)
			}
			return password, bandwidth
		}
		if debug {
			debug.Printf("Cache Miss for Customer: %d\n", userID)
		}
	}
	ssuser, err := queryDatabase(userID)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return "", -1
	}
	if ssuser == nil {
		return "", -1
	}
	if useRedis {
		saveToRedis(userID, ssuser.Password, ssuser.Bandwidth)
	}
	return ssuser.Password, ssuser.Bandwidth
}

type SSUser struct {
	UserID    int
	Password  string
	Status    string
	Bandwidth int
}

func queryDatabase(userID int) (*SSUser, error) {
	sql := fmt.Sprintf("SELECT userid, password, status, bandwidth FROM user WHERE userid='%d' AND status='Enabled';", userID)
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Commit()
	rows, err := tx.Query(sql)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, nil
	}
	defer rows.Close()
	var ret *SSUser = nil
	for rows.Next() {
		if ret != nil {
			continue
		}
		user := new(SSUser)
		row_err := rows.Scan(&user.UserID, &user.Password, &user.Status, &user.Bandwidth)
		if row_err != nil {
			return nil, err
		}
		ret = user
	}
	return ret, nil
}
