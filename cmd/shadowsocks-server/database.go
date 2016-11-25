package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func initDB(url string) error {
	var err error
	db, err = sql.Open("mysql", url)
	return err
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
	ssuser, err := queryDatabase(userID)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return "", -1
	}
	if ssuser == nil {
		return "", -1
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
