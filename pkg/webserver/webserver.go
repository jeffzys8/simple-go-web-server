package webserver

import (
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"example.com/entrytask/pkg/protocol"
)

var backendAddr string = ":8080" // TODO: 地址可配置

func setCookie(w http.ResponseWriter, name string, value string, age int) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		MaxAge:   age,
	})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", 302)
		return
	}

	r.ParseForm()
	username := r.Form.Get("username")
	password := r.Form.Get("password")

	if len(username) == 0 || len(password) == 0 {
		setCookie(w, "loginFail", "", 0)
		http.Redirect(w, r, "/", 302)
		return
	}

	// TODO: 加密password

	// reqeust backend to login
	conn, err := dialBackEnd(backendAddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'loginHandler'", err)
		return
	}
	resp, err := protocol.Request(conn, protocol.Message{
		"__api__":  "login",
		"username": username,
		"password": password,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'loginHandler'", err)
		return
	}

	// check whether the login succeed based on return code
	if ret, _ := strconv.Atoi(resp["__ret__"]); ret == 0 {
		setCookie(w, "session", resp["session"], 0)
	} else {
		setCookie(w, "loginFail", "", 0)
		setCookie(w, "session", "", -1)
	}
	http.Redirect(w, r, "/", 302)
}

func updateNicknameHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: session check 合成为一个函数来调用
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", 302)
		return
	}

	session, err := r.Cookie("session")
	if err != nil {
		w.Write([]byte("Your loggin session is invalid now"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	r.ParseForm()
	nickname := r.Form.Get("nickname")

	conn, err := dialBackEnd(backendAddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'updateNicknameHandler'", err)
		return
	}
	resp, err := protocol.Request(conn, protocol.Message{
		"__api__":  "updatenickname",
		"session":  session.Value,
		"nickname": nickname,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'updateNicknameHandler'", err)
		return
	}
	if ret, _ := strconv.Atoi(resp["__ret__"]); ret != 0 {
		w.Write([]byte("Failed to update nickname"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/", 302)
}

func pictureHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		getPictureHandler(w, r)
	} else if r.Method == http.MethodPost {
		uploadPictureHandler(w, r)
	} else {
		http.Redirect(w, r, "/", 302)
	}
}

func getPictureHandler(w http.ResponseWriter, r *http.Request) {
	session, err := r.Cookie("session")
	if err != nil {
		http.Redirect(w, r, "/", 302)
		return
	}

	conn, err := dialBackEnd(backendAddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'getPictureHandler'", err)
		return
	}
	resp, err := protocol.Request(conn, protocol.Message{
		"__api__": "getpic",
		"session": session.Value,
	})
	if err != nil {
		w.Write([]byte(err.Error()))
		http.Redirect(w, r, "/", 500)
		return
	}

	if ret, _ := strconv.Atoi(resp["__ret__"]); ret == 0 {
		w.Write([]byte(resp["picture"]))
	}
}

// save the file in the backend's local file system.
// TODO: 应在webserver处直接保存图片，并发送uri给backend
func uploadPictureHandler(w http.ResponseWriter, r *http.Request) {

	session, err := r.Cookie("session")
	if err != nil {
		http.Redirect(w, r, "/", 302)
		return
	}

	r.ParseMultipartForm(32 << 20) // TODO: maxmeory should set to what?
	file, _, err := r.FormFile("picture")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'uploadPictureHandler'", err)
		return
	}
	defer file.Close()

	buf := new(strings.Builder)
	if _, err = io.Copy(buf, file); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'uploadPictureHandler'", err)
		return
	}

	conn, err := dialBackEnd(backendAddr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'uploadPictureHandler'", err)
		return
	}
	resp, err := protocol.Request(conn, protocol.Message{
		"__api__": "uploadpic",
		"session": session.Value,
		"picture": buf.String(),
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'uploadPictureHandler'", err)
		return
	}

	if ret, _ := strconv.Atoi(resp["__ret__"]); ret != 0 {
		w.WriteHeader(http.StatusInternalServerError)
		log.Fatal("Error occured with 'uploadPictureHandler'", err)
		return
	}

	http.Redirect(w, r, "/", 302)
	return
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	loginFail := false
	if _, err := r.Cookie("loginFail"); err == nil {
		loginFail = true
		setCookie(w, "loginFail", "", -1)
	}

	loginned := false
	userinfo := make(map[string]string)
	if session, err := r.Cookie("session"); err == nil {
		// request backend for personal info with session
		conn, err := dialBackEnd(backendAddr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Fatal("Error occured with 'indexHandler'", err)
			return
		}
		resp, err := protocol.Request(conn, protocol.Message{
			"__api__": "info",
			"session": session.Value,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Fatal("Error occured with 'indexHandler'", err)
			return
		}

		// check whether the login session is valid based on return code
		if ret, _ := strconv.Atoi(resp["__ret__"]); ret == 0 {
			loginned = true
			userinfo["username"] = resp["username"]
			userinfo["nickname"] = resp["nickname"]
		} else {
			setCookie(w, "session", "", -1)
		}
	}

	t, _ := template.ParseFiles("template/index.html")
	data := map[string]interface{}{
		"Loginned":  loginned,
		"LoginFail": loginFail,
		"Username":  userinfo["username"],
		"Nickname":  userinfo["nickname"],
	}
	w.WriteHeader(http.StatusOK)
	t.Execute(w, data)
}

func dialBackEnd(addr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn, nil
}

// StartWebServer starts the web service
func StartWebServer(addr string) {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/nickname", updateNicknameHandler)
	http.HandleFunc("/picture", pictureHandler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
