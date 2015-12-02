package wsd

import (
	// "fmt"
	"github.com/golang/glog"
	"sync"
)

func NewThreadManager(conn *UserConnection, accessCkecker func(*UserInfo, string) bool) *ConnectionThreadsManager {
	model := new(ConnectionThreadsManager)
	model.Conn = conn
	model.Threads = make(map[string]*ThreadSpace)
	model.AccessChecker = accessCkecker

	return model
}

type ConnectionThreadsManager struct {
	sync.Mutex
	Conn          *UserConnection
	Threads       map[string]*ThreadSpace
	AccessChecker func(*UserInfo, string) bool
}

func (c *ConnectionThreadsManager) Disconnected() {
	glog.Infof("Удаление информации о каналлах для '%d' пользователя", c.Conn.userInfo.UserId)

	for _, thread := range c.Threads {
		thread.RemoveParticipant() <- c.Conn
	}
}

func (c *ConnectionThreadsManager) AddThread(thread *ThreadSpace) {
	if _, exist := c.Threads[thread.Id]; !exist {

		if !c.AccessChecker(c.Conn.userInfo, thread.Id) {
			glog.Warningf("Нет доступа '%+v' к thread id '%s'", c.Conn.userInfo, thread.Id)
			return
		}

		thread.AddParticipant() <- c.Conn

		c.Lock()
		c.Threads[thread.Id] = thread
		c.Unlock()
	}
}

func (c *ConnectionThreadsManager) RemoveThread(thread *ThreadSpace) {
	if _, exist := c.Threads[thread.Id]; exist {
		c.Lock()
		delete(c.Threads, thread.Id)
		c.Unlock()

		thread.RemoveParticipant() <- c.Conn
	}
}

func (c *ConnectionThreadsManager) IsParticipant(thread *ThreadSpace) bool {
	_, isParticipant := c.Threads[thread.Id]

	return isParticipant
}

func NewThreadSpace(id string) *ThreadSpace {
	model := new(ThreadSpace)
	// model.Id = fmt.Sprintf("thread:%s", id)
	model.Id = id
	model.participants = make(map[*UserConnection]bool)
	model.addParticipant = make(chan *UserConnection)
	model.removeParticipant = make(chan *UserConnection)
	model.inbox = make(chan *MessageDTO)

	return model
}

type ThreadSpace struct {
	sync.Mutex

	Id                string
	addParticipant    chan *UserConnection
	removeParticipant chan *UserConnection
	participants      map[*UserConnection]bool
	inbox             chan *MessageDTO
}

func (c *ThreadSpace) AddParticipant() chan<- *UserConnection {
	return (chan<- *UserConnection)(c.addParticipant)
}

func (c *ThreadSpace) RemoveParticipant() chan<- *UserConnection {
	return (chan<- *UserConnection)(c.removeParticipant)
}

func (c *ThreadSpace) Inbox() chan<- *MessageDTO {
	return (chan<- *MessageDTO)(c.inbox)
}

func (c *ThreadSpace) listenerMessages() {
	for {
		select {
		case m := <-c.inbox:
			glog.Infof("Сообщение в потоке '%s' будет разослано '%d' пользователям. '%+v'", c.Id, len(c.participants), m.Data)

			for participant, _ := range c.participants {
				m.Meta.OnlineCount = len(c.participants)
				participant.to <- m
			}
		}
	}
}

func (c *ThreadSpace) listenerParticipants() {
	for {
		select {
		case conn := <-c.addParticipant:
			m := NewMessageInformer()
			m.Meta.MT = MTThreadNotifyJoined
			m.Meta.To = c.Id
			m.Meta.Conn = conn
			mDto := NewMessageDTO(m, conn.userInfo)

			c.Inbox() <- mDto

			c.Lock()
			c.participants[conn] = true
			c.Unlock()
		case conn := <-c.removeParticipant:
			if _, exist := c.participants[conn]; exist {
				c.Lock()
				delete(c.participants, conn)
				c.Unlock()

				m := NewMessageInformer()
				m.Meta.MT = MTThreadNotifyDisconnected
				m.Meta.To = c.Id
				m.Meta.Conn = conn

				mDto := NewMessageDTO(m, conn.userInfo)

				c.Inbox() <- mDto
			}
		}
	}
}

func (c *ThreadSpace) Listener() {
	go c.listenerMessages()
	go c.listenerParticipants()
}
