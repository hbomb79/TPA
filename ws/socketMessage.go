package ws

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type socketMessageType int

const (
	Update socketMessageType = iota
	Command
	Response
	ErrorResponse
	Welcome
)

// SocketMessage is a struct that allows us to define the
// command that has been passed through the web socket.
// The Id field can be used when replying to this message
// so the receiving client is aware of which message this reply
// is for. Origin is much for the same - it allows us to
// send the reply to the websocket attached to the client
// with the matching UUID
type SocketMessage struct {
	Title  string                 `json:"title"`
	Body   map[string]interface{} `json:"arguments"`
	Id     int                    `json:"id"`
	Type   socketMessageType      `json:"type"`
	Origin *uuid.UUID             `json:"-"`
	Target *uuid.UUID             `json:"-"`
}

func (message *SocketMessage) ValidateArguments(required map[string]string) error {
	const ERR_FMT = "failed to validate key '%v' with type '%v' - %v"

	for key, value := range required {
		if v, ok := message.Body[key]; ok {
			// get the string interpretation of the
			// given value - this method is only used to
			// test for primitve types currently. Perhaps with go
			// generics this method could be expanded to test for
			// interface implementation too
			givenValue := fmt.Sprintf("%v", v)
			switch value {
			case "number", "int":
				var i int
				_, err := fmt.Sscanf(givenValue, "%d", &i)
				if err != nil {
					return errors.New(fmt.Sprintf(ERR_FMT, key, value, givenValue))
				}
			case "string":
				break
			default:
				return errors.New(fmt.Sprint(ERR_FMT, key, value, "unknown type"))
			}
		} else {
			// Error, missing key
			return errors.New(fmt.Sprintf("failed to validate key '%v' - key is missing", key))
		}
	}

	return nil
}

// FormReply is a method on a SocketMessage that will
// return a NEW message that has the same origin/id as
// the original message, but with a new (caller provided) title,
// type, and arguments.
func (message *SocketMessage) FormReply(replyTitle string, replyBody map[string]interface{}, replyType socketMessageType) *SocketMessage {
	if replyBody != nil {
		replyBody["command"] = message.Body
	}

	return &SocketMessage{
		Title:  replyTitle,
		Body:   replyBody,
		Type:   replyType,
		Id:     message.Id,
		Target: message.Origin,
	}
}
