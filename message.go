package qord

import "github.com/amatsagu/tempest"

type Message struct {
	tempest.Message
	ShardID    uint16            `json:"-"`
	Client     *Client           `json:"-"`
	replyMsgID tempest.Snowflake `json:"-"`
}

func (msg *Message) SendReply(content Message, files []tempest.File) (tempest.Message, error) {
	m, err := msg.Client.SendMessage(msg.ChannelID, content.Message, files)
	if err == nil {
		msg.replyMsgID = m.ID
	}
	return m, err
}

func (msg *Message) SendLinearReply(content string) (tempest.Message, error) {
	m, err := msg.Client.SendMessage(msg.ChannelID, tempest.Message{Content: content}, nil)
	if err == nil {
		msg.replyMsgID = m.ID
	}
	return m, err
}

func (msg *Message) EditReply(content Message) error {
	return msg.Client.EditMessage(msg.ChannelID, msg.replyMsgID, content.Message)
}

func (msg *Message) EditLinearReply(content string) error {
	return msg.Client.EditMessage(msg.ChannelID, msg.replyMsgID, tempest.Message{Content: content})
}

func (msg *Message) DeleteReply() error {
	return msg.Client.DeleteMessage(msg.ChannelID, msg.replyMsgID)
}
