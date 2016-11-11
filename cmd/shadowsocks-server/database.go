package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
)

// Database Table Format:
// table user (
//    userid int
//    password varchar(255)
//    status varchar(20)
//    bandwidth int
// )
// Status: Enabled, Disabled
//
func getPasswordFromDatabase(userID int, url string) string {
	db, err := getConnection(url)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return ""
	}
	ssuser, err := queryDatabase(db, userID)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return ""
	}
	if ssuser == nil {
		return ""
	}
	return ssuser.Password
}

type SSUser struct {
	UserID    int
	Password  string
	Status    string
	Bandwidth int
}

func getConnection(url string) (*sql.DB, error) {
	db, err := sql.Open("mysql", url)
	return db, err
}

func queryDatabase(db *sql.DB, userID int) (*SSUser, error) {
	sql := fmt.Sprintf("SELECT userid, password, status, bandwidth FROM user WHERE userid='%d' AND status='Enabled';", userID)
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		return nil, nil
	}
	for rows.Next() {
		user := new(SSUser)
		row_err := rows.Scan(&user.UserID, &user.Password, &user.Status, &user.Bandwidth)
		if row_err != nil {
			return nil, err
		}
		return user, nil
	}
	return nil, nil
}
