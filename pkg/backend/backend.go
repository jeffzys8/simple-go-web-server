package backend

import (
	"context"
	"database/sql"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	"example.com/entrytask/pkg/protocol"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	uuid "github.com/satori/go.uuid"
)

const (
	redisAddr     = "localhost:6379"
	redisPassword = ""
	redisInfoDB   = 0
)

const (
	mysqlLoginStr = "root:testtest@/entrytask"
)

const (
	uploadPicDir = "/tmp/entrytask/"
)

const (
	sessionExpireTime = 120 * time.Second
)

var redisConn *redis.Client

// TODO: session 机制应放在webserver
// TODO: 现在仅支持username, nickname两个info，需要做成一个struct以便拓展
// TODO: 需要使用context包装请求

// use req["username"] & req["password"] to login and return session
// if succeed, save session record(user, session) in redis (with timeout)
func loginHandler(req protocol.Message) protocol.Message {
	db, err := sql.Open("mysql", mysqlLoginStr)
	if err != nil {
		panic(err)
	}

	var password, nickname string
	err = db.QueryRow("SELECT password, nickname FROM users WHERE username=?", req["username"]).Scan(&password, &nickname)

	if err == sql.ErrNoRows {
		return protocol.Message{"__ret__": "1"}
	} else if err != nil {
		panic(err) // TODO: 不应该挂掉整个服务
	} else if password != req["password"] { // TODO: 避免明文 & 避免重放攻击
		return protocol.Message{"__ret__": "1"}
	}
	// TODO: timing attack

	// generate random sessionid, use it as a key to specific userinfo in redis
	sessionID := uuid.NewV4().String()
	// TODO: context: Go HTTP标准库

	// TODO: 一行语句不做过多事情
	err = redisConn.HSet(context.TODO(), sessionID, map[string]interface{}{
		"username": req["username"],
		"nickname": nickname,
	}).Err()
	if err != nil {
		panic(err)
	}

	// TODO: simplify
	if err = redisConn.Expire(context.TODO(), sessionID, sessionExpireTime).Err(); err != nil {
		panic(err)
	}
	return protocol.Message{"__ret__": "0", "session": sessionID}
}

// use req["session"] to get personal info
func infoHandler(req protocol.Message) protocol.Message {

	// check session first
	hasSession, err := redisConn.Exists(context.TODO(), req["session"]).Result()
	if err != nil {
		panic(err)
	} else if hasSession == 0 {
		return protocol.Message{"__ret__": "1"}
	}

	// retrieve info from session
	userInfo, err := redisConn.HGetAll(context.TODO(), req["session"]).Result()
	if err != nil {
		panic(err)
	}
	return protocol.Message{
		"__ret__":  "0",
		"username": userInfo["username"],
		"nickname": userInfo["nickname"],
	}
}

func updateNicknameHandler(req protocol.Message) protocol.Message {
	hasSession, err := redisConn.Exists(context.TODO(), req["session"]).Result()
	if err != nil {
		panic(err)
	} else if hasSession == 0 {
		return protocol.Message{"__ret__": "1"}
	}

	// retrieve info from session
	userInfo, err := redisConn.HGetAll(context.TODO(), req["session"]).Result()
	if err != nil {
		panic(err)
	}

	// update mysql
	db, err := sql.Open("mysql", mysqlLoginStr)
	if err != nil {
		panic(err)
	}
	err = db.QueryRow("UPDATE users SET nickname=? WHERE username=?", req["nickname"], userInfo["username"]).Err()
	if err != nil && err != sql.ErrNoRows {
		panic(err)
	}

	// update redis
	err = redisConn.HSet(context.TODO(), req["session"], "nickname", req["nickname"]).Err()
	if err != nil {
		panic(err)
	}
	return protocol.Message{"__ret__": "0"}
}

func getPictureHandler(req protocol.Message) protocol.Message {
	hasSession, err := redisConn.Exists(context.TODO(), req["session"]).Result()
	if err != nil {
		panic(err)
	} else if hasSession == 0 {
		return protocol.Message{"__ret__": "1"}
	}

	userInfo, err := redisConn.HGetAll(context.TODO(), req["session"]).Result()
	if err != nil {
		panic(err)
	}

	filename := uploadPicDir + userInfo["username"]
	if _, err := os.Stat(filename); err != nil {
		return protocol.Message{"__ret__": "1"}
	}
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	return protocol.Message{
		"__ret__": "0",
		"picture": string(bytes),
	}
}

func uploadPictureHandler(req protocol.Message) protocol.Message {
	hasSession, err := redisConn.Exists(context.TODO(), req["session"]).Result()
	if err != nil {
		panic(err)
	} else if hasSession == 0 {
		return protocol.Message{"__ret__": "1"}
	}
	userInfo, err := redisConn.HGetAll(context.TODO(), req["session"]).Result()
	if err != nil {
		panic(err)
	}
	f, err := os.Create(uploadPicDir + userInfo["username"])
	if err != nil {
		panic(err)
	}
	_, err = io.WriteString(f, req["picture"])
	if err != nil {
		panic(err)
	}

	return protocol.Message{"__ret__": "0"}
}

// TODO: 多人协作难以管理，看Go HTTP标准库
func requestHandler(req protocol.Message) protocol.Message {
	switch req["__api__"] {
	case "login":
		return loginHandler(req)
	case "info":
		return infoHandler(req)
	case "updatenickname":
		return updateNicknameHandler(req)
	case "getpic":
		return getPictureHandler(req)
	case "uploadpic":
		return uploadPictureHandler(req)
	default:
		return protocol.Message{}
	}
}

func handleConnection(c net.Conn) {
	defer c.Close()
	err := protocol.HandleRequest(c, requestHandler)
	if err != nil {
		panic(err)
	}
}

// StartBackend starts the backend service
// TODO: use context to wrap up
func StartBackend(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// handle error
	}
	redisConn = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisInfoDB,
	})
	defer redisConn.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			// handle error
		}
		go handleConnection(conn) // TODO: 短连接性能
	}
}
