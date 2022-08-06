// Copyright (c) Mainflux
// SPDX-Licence-Identifier: Apache-2.0

package api

import "github.com/gorilla/websocket"

type connReq struct {
	thingKey string
	chanID   string
	subtopic string
	conn     *websocket.Conn
}