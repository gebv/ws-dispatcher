# ws-dispatcher
websocket connections manager

example
``` golang

var wsDispatcher *Dispatcher
wsDispatcher = NewDispatcher()
wsDispatcher.Listener()

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	}
}

// in handler

var userId int64
// implement user authentication session

c, err := upgrader.Upgrade(w, r, nil)
userConn := NewUserConnection(c, &UserInfo{userId})
userConn.Listener(wsDispatcher)

//...

// TODO: example send message in thread, in connection

```
