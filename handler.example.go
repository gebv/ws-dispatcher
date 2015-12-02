package wsd_example

import (
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"net/http"
	"strconv"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		//
		return true
	},
	Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
		glog.Errorln("Error Websocket, %s, (status=%d)", reason, status)
	},
}

var wsDispatcher *Dispatcher

func initDispatcher() {
	wsDispatcher = NewDispatcher()
	wsDispatcher.Listener()
}

func ExmapleHandler(w http.ResponseWriter, r *http.Request) {
	// implement user authentication session
	userIdRaw := r.URL.Query().Get("User ID")
	userId, _ := strconv.ParseInt(userIdRaw, 10, 64)
	// if !ok {
	// 	glog.Errorln("Клиент не задан")
	// 	return
	// }

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		glog.Errorln("upgrade:", err)
		return
	}
	defer c.Close()

	userConn := NewUserConnection(c, &UserInfo{userId})
	glog.Infof("User '%d' connection...", userId)
	userConn.Listener(wsDispatcher)
	glog.Infof("User '%d' disconnection", userId)
}
