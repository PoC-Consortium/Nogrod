// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

package webserver

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	. "github.com/PoC-Consortium/Nogrod/pkg/logger"
	"github.com/PoC-Consortium/Nogrod/pkg/rsencoding"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const updateInterval = 5 * time.Second

type Client struct {
	c                *websocket.Conn
	msgs             chan *WebSocketMsg
	finished         chan<- *Client
	newIDs           chan uint64
	stopSubscription chan struct{}
}

type WebSocketMsg struct {
	Data []byte
	Type int
}

func NewClient(c *websocket.Conn, finished chan *Client) *Client {
	client := &Client{
		c:                c,
		msgs:             make(chan *WebSocketMsg, 4),
		newIDs:           make(chan uint64),
		stopSubscription: make(chan struct{}),
		finished:         finished}

	go client.read()
	go client.write()
	go client.subscribe()

	return client
}

func (client *Client) close() {
	client.c.Close()
	close(client.msgs)
	client.stopSubscription <- struct{}{}
	client.finished <- client
}

func (client *Client) read() {
	defer client.close()
	for {
		_, msg, err := client.c.ReadMessage()
		if err != nil {
			return
		}
		client.addSubscription(msg)
	}
}

func (client *Client) write() {
	for msg := range client.msgs {
		err := client.c.WriteMessage(msg.Type, msg.Data)
		if err != nil {
			return
		}
	}
}

func (client *Client) QueueMsg(msg *WebSocketMsg) {
	if cap(client.msgs) == len(client.msgs) {
		return
	}
	client.msgs <- msg
}

func (client *Client) genSubscriptionMsg(id uint64) []byte {
	minerInfo := GenMinerInfo(id)
	if minerInfo == nil {
		return nil
	}

	msg, err := json.Marshal(&minerInfo)
	if err != nil {
		Logger.Error("failed to encode miner info to json", zap.Error(err))
		return nil
	}
	return msg
}

func (client *Client) updateSubscription(id uint64) bool {
	msg := client.genSubscriptionMsg(id)
	if msg == nil {
		client.QueueMsg(&WebSocketMsg{
			Type: websocket.TextMessage,
			Data: []byte(`{"subscriptionFailed":"unknown account"}`)})
		return false
	}

	client.QueueMsg(&WebSocketMsg{
		Type: websocket.TextMessage,
		Data: msg})

	return true
}

func (client *Client) subscribe() {
	var update <-chan time.Time
	var id uint64
	for {
		select {
		case <-update:
			success := client.updateSubscription(id)
			if !success {
				update = make(<-chan time.Time)
			}
		case id = <-client.newIDs:
			success := client.updateSubscription(id)
			if !success {
				continue
			}
			client.QueueMsg(&WebSocketMsg{
				Type: websocket.TextMessage,
				Data: []byte(`{"subscriptionSuccess":""}`)})
			update = time.NewTicker(updateInterval).C
		case <-client.stopSubscription:
			return
		}
	}
}

func (client *Client) addSubscription(msg []byte) {
	var accountID uint64
	var err error

	trimmed := strings.TrimPrefix(strings.ToUpper(strings.Trim(string(msg), " ")), "BURST-")
	accountID, err = strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		accountID, err = rsencoding.Decode(trimmed)
		if err != nil {
			return
		}
	}

	if accountID == 0 {
		client.QueueMsg(&WebSocketMsg{
			Type: websocket.TextMessage,
			Data: []byte(`{"subscriptionFailed":"malformed numeric id or burst address"}`)})
		return
	}

	client.newIDs <- accountID
}
