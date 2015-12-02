package wsd

import (
	"github.com/golang/glog"
	"sync"
)

func NewDispatcher(threadAccessChecker func(*UserInfo, string) bool) *Dispatcher {
	model := new(Dispatcher)
	model.fromConnections = make(chan *MessageInformer)
	model.toThread = make(chan *MessageDTO)
	model.toConnection = make(chan *MessageDTO)
	model.threads = make(map[string]*ThreadSpace)
	model.connections = make(map[*UserConnection]bool)
	// model.connectionUsers = make(map[string][]*UserConnection)
	model.connectionThreads = make(map[*UserConnection]*ConnectionThreadsManager)
	model.ConnDisconected = make(chan *UserConnection)
	model.FnThreadAccessChecker = threadAccessChecker

	return model
}

type Dispatcher struct {
	sync.RWMutex

	// All messages from connections
	fromConnections chan *MessageInformer

	// The message to thread
	toThread chan *MessageDTO

	// The messages to connection
	toConnection chan *MessageDTO

	// All active threads
	threads map[string]*ThreadSpace

	// All connections
	connections map[*UserConnection]bool

	// Connections and related threads
	connectionThreads map[*UserConnection]*ConnectionThreadsManager

	// Signal disconected
	ConnDisconected chan *UserConnection

	FnThreadAccessChecker func(*UserInfo, string) bool
}

// The messages sent to connection
func (c *Dispatcher) listenerMessagesToConnection() {
	for {
		select {
		case message := <-c.toConnection:
			for conn, _ := range c.connections {

				if conn == message.Meta.Conn {
					conn.to <- message
					break
				}
			}
		}
	}
}

// The messages sent to thread
func (c *Dispatcher) listenerMessagesToThread() {
	for {
		select {
		case message := <-c.toThread:
			threadManager, ok := c.threads[message.Meta.To]

			if !ok {
				threadManager = c.newThread(message.Meta.To)
			}

			threadManager.Inbox() <- message
		}
	}
}

func (c *Dispatcher) listenerDisconnections() {
	for {
		select {
		case userConn := <-c.ConnDisconected:
			glog.Infof("UserConn '%d' disconected...", userConn.userInfo.UserId)
			c.Lock()

			// Из списка коннектов
			delete(c.connections, userConn)
			glog.Infof("UserConn '%d' disconected... из списка коннектов", userConn.userInfo.UserId)

			// TODO: Удалить из списка коннекто пользователя (один пользователь может иметь множество коннектов)??
			// c.connectionUsers[userConn]

			// TODO: Оповещаем об отключившемся участнике

			threadsManager, exist := c.connectionThreads[userConn]
			if exist {
				glog.Infof("UserConn '%d' disconected... был участником, отключаем от каналов...", userConn.userInfo.UserId)
				threadsManager.Disconnected()
				glog.Infof("UserConn '%d' disconected... ОК", userConn.userInfo.UserId)
				delete(c.connectionThreads, userConn)
				glog.Infof("UserConn '%d' disconected... Удалили информацию из Dispatcher о каналах подключенных пользователем", userConn.userInfo.UserId)
			}
			c.Unlock()

			glog.Infof("UserConn '%d' disconected... Сигнал закрытия соединения...", userConn.userInfo.UserId)
			glog.Infof("UserConn '%d' disconected... ОК", userConn.userInfo.UserId)
		}
	}
}

func (c *Dispatcher) RegistryAndListenConn(userConn *UserConnection) {
	c.connections[userConn] = true
}

func (c *Dispatcher) newThread(threadId string) *ThreadSpace {
	c.Lock()
	// TODO: Проверять, позволено ли быть участником потока
	c.threads[threadId] = NewThreadSpace(threadId)
	c.threads[threadId].Listener()
	c.Unlock()

	return c.threads[threadId]
}

func (c *Dispatcher) Listener() {
	go c.listenerMessagesToThread()
	go c.listenerMessagesToConnection()
	go c.listenerDisconnections()

	go func() {
		for {
			select {
			case message := <-c.fromConnections:
				// All messages from connections

				glog.Infof("Сообщение от '%s': '%s'", message.Meta.Conn.userInfo, string(message.Data))

				switch message.Meta.MT {
				case MTUpdateSubscrib:
					glog.Infof("MTUpdateSubscrib")

					messageData, err := NewMessageUpdateSubscribDTOFromRaw([]byte(message.Data))

					// Invalid?
					if err != nil {
						messageError := NewMessageInformer()
						messageError.Meta.MT = MessageType("error")
						messageError.Meta.UserInfo = &UserInfo{0}
						mDto := NewMessageDTO(messageError, struct{ Message string }{"Ошибка обновления списка, " + err.Error()})

						message.Meta.Conn.to <- mDto
						break
					}

					glog.Infof("MTUpdateSubscrib. Обработали сообщение корректно")

					// Если есть в списке, оставляем
					// Если нет, то добавляем
					threadManager, exist := c.connectionThreads[message.Meta.Conn]

					if !exist {
						glog.Infof("MTUpdateSubscrib. Пользователь '%d' не является участником хотя бы в одном канале", message.Meta.Conn.userInfo.UserId)
						c.connectionThreads[message.Meta.Conn] = NewThreadManager(message.Meta.Conn, c.FnThreadAccessChecker)
						threadManager = c.connectionThreads[message.Meta.Conn]
					}

					for _, threadId := range messageData.ThreadIds {
						glog.Infof("MTUpdateSubscrib. Пользователь '%d' присоединяется к '%s' потоку", message.Meta.Conn.userInfo.UserId, threadId)
						// Убеждаемся в имеющемся канале
						// TODO: Обратный механизм очистки пустых каналов
						threadSpace, exist := c.threads[threadId]

						if !exist {

							glog.Infof("MTUpdateSubscrib. '%s' поток не был создан ранее, создается...", threadId)
							// Создаем новый поток
							threadSpace = c.newThread(threadId)
							glog.Infof("MTUpdateSubscrib. OK")
						}

						glog.Infof("MTUpdateSubscrib. Пользователь '%d' присоединяется к '%s' потоку...", message.Meta.Conn.userInfo.UserId, threadId)
						threadManager.AddThread(threadSpace)
						glog.Infof("MTUpdateSubscrib. OK")
					}

				case MTThreadMessageNew:
					// Сообщение адресованное в поток
					threadId := message.Meta.To

					glog.Infof("MTThreadMessageNew. To '%s'", threadId)

					glog.Infof("MTThreadMessageNew. Send message to thread '%s'...", threadId)
					c.toThread <- NewMessageDTO(message, struct{ Text string }{string(message.Data)})
					glog.Infof("MTThreadMessageNew. OK")
				case MTThreadNotifyWriting:
				default:
					glog.Warningf("Не известный тип сообщения '%s'", message.Meta.MT)
				}

				/*
					1. Сообщение на обновление каналов-подписчиков
					2. Сообщение в определенный канал
				*/
			}
		}
	}()
}
