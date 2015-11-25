package wsd

import (
	"encoding/json"
)

type MessageType string

var (
	MTUpdateSubscrib           = MessageType("subscription:update")
	MTThreadMessageNew         = MessageType("thread:message:new")
	MTThreadNotifyWriting      = MessageType("thread:notify:writing")
	MTThreadNotifyJoined       = MessageType("thread:notify:joined")
	MTThreadNotifyDisconnected = MessageType("thread:notify:disconnected")
)

type MessageMeta struct {
	MT          MessageType     `json:"mt"`
	To          string          `json:"to"`
	Conn        *UserConnection `json:"-"`
	UserInfo    *UserInfo       `json:"-"`
	OnlineCount int             `json:"online_count"`
}

func NewMessageDTO(messageInformer *MessageInformer, data interface{}) *MessageDTO {
	model := new(MessageDTO)
	model.Meta = messageInformer.Meta
	model.Data = data
	return model
}

type MessageDTO struct {
	Meta MessageMeta `json:"meta"`
	Data interface{} `json:"data"`
}

func NewMessageInformer() *MessageInformer {
	return new(MessageInformer)
}

type MessageInformer struct {
	Meta MessageMeta     `json:"meta"`
	Data json.RawMessage `json:"data"`
}

//

func NewMessageUpdateSubscribDTOFromRaw(raw []byte) (*MessageUpdateSubscribDTO, error) {
	model := new(MessageUpdateSubscribDTO)

	return model, json.Unmarshal(raw, model)
}

type MessageUpdateSubscribDTO struct {
	ThreadIds []string `json:"threads"`
}
