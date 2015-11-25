package wsd

import (
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"time"
)

type UserInfo struct {
	UserId int64 `json:"user_id"`
}

func NewUserConnection(conn *websocket.Conn, user *UserInfo) *UserConnection {
	model := new(UserConnection)
	model.userInfo = user
	model.c = conn
	model.to = make(chan *MessageDTO)

	return model
}

type UserConnection struct {
	c        *websocket.Conn
	userInfo *UserInfo

	to chan *MessageDTO

	dispatcher *Dispatcher
}

func (c *UserConnection) Listener(dispatcher *Dispatcher) {
	dispatcher.RegistryAndListenConn(c)
	c.dispatcher = dispatcher

	go c.writePump()
	c.readPump()
}

func (c *UserConnection) readPump() {
	defer func() {
		glog.Infof("Closed to readPump '%d'...", c.userInfo.UserId)
		c.dispatcher.ConnDisconected <- c
		c.c.Close()

		glog.Infof("Closed to readPump '%d'. OK", c.userInfo.UserId)
	}()

	c.c.SetReadLimit(maxMessageSize)
	c.c.SetReadDeadline(time.Now().Add(pongWait))
	c.c.SetPongHandler(func(string) error { c.c.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		m := NewMessageInformer()
		err := c.c.ReadJSON(m)

		if err != nil {
			glog.Infof("'%+v', %s", c.userInfo, err)
			break
			// signal c.dispatcher.ConnDisconected
		}

		m.Meta.UserInfo = c.userInfo
		m.Meta.Conn = c
		c.dispatcher.fromConnections <- m
	}
}

func (c *UserConnection) write(mt int, payload []byte) error {
	c.c.SetWriteDeadline(time.Now().Add(writeWait))
	return c.c.WriteMessage(mt, payload)
}

func (c *UserConnection) writeJson(v interface{}) error {
	c.c.SetWriteDeadline(time.Now().Add(writeWait))
	return c.c.WriteJSON(v)
}

func (c *UserConnection) writePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		glog.Infof("Closed to writePump '%d'...", c.userInfo.UserId)
		c.dispatcher.ConnDisconected <- c

		ticker.Stop()
		c.c.Close()

		glog.Infof("Closed to writePump '%d'. OK", c.userInfo.UserId)
	}()

	for {
		select {
		// message to conn
		case message, ok := <-c.to:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.writeJson(message); err != nil {
				glog.Warningf("Error write to channel %s", err)
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}
