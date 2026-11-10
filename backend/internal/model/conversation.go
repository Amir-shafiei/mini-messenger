package model

type Conversation struct {
	Peer        User
	LastMessage *Message
	UnreadCount int
}
