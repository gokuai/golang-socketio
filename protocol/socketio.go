package protocol

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const (
	open          = "0"
	msg           = "4"
	emptyMessage  = "40"
	commonMessage = "42"
	ackMessage    = "43"

	CloseMessage = "1"
	PingMessage = "2"
	PongMessage = "3"
)

var (
	ErrorWrongMessageType = errors.New("Wrong message type")
	ErrorWrongPacket      = errors.New("Wrong packet")
)

func typeToText(msgType int) (string, error) {
	switch msgType {
	case MessageTypeOpen:
		return open, nil
	case MessageTypeClose:
		return CloseMessage, nil
	case MessageTypePing:
		return PingMessage, nil
	case MessageTypePong:
		return PongMessage, nil
	case MessageTypeEmpty:
		return emptyMessage, nil
	case MessageTypeEmit, MessageTypeAckRequest:
		return commonMessage, nil
	case MessageTypeAckResponse:
		return ackMessage, nil
	}
	return "", ErrorWrongMessageType
}

func Encode(msg *Message) (string, error) {
	result, err := typeToText(msg.Type)
	if err != nil {
		return "", err
	}

	comma := false
	if msg.Namespace != "" {
		result += msg.Namespace
		comma = true
	}

	if msg.Type == MessageTypeEmpty || msg.Type == MessageTypePing ||
		msg.Type == MessageTypePong {
		return result, nil
	}

	if msg.Type == MessageTypeAckRequest || msg.Type == MessageTypeAckResponse {
		if comma {
			result += ","
			comma = false
		}
		result += strconv.Itoa(msg.AckId)
	}

	if msg.Type == MessageTypeOpen || msg.Type == MessageTypeClose {
		if comma {
			result += ","
			comma = false
		}
		return result + msg.Args, nil
	}

	if msg.Type == MessageTypeAckResponse {
		if comma {
			result += ","
			comma = false
		}
		return result + "[" + msg.Args + "]", nil
	}

	jsonMethod, err := json.Marshal(&msg.Method)
	if err != nil {
		return "", err
	}

	if comma {
		result += ","
		comma = false
	}

	return result + "[" + string(jsonMethod) + "," + msg.Args + "]", nil
}

func MustEncode(msg *Message) string {
	result, err := Encode(msg)
	if err != nil {
		panic(err)
	}

	return result
}

func getMessageType(data string) (t int, restText string, err error) {
	t = 0
	restText = data
	err = nil
	if len(data) == 0 {
		err = ErrorWrongMessageType
		return
	}
	restText = data[1:]
	switch data[0:1] {
	case open:
		return MessageTypeOpen, data[1:], nil
	case CloseMessage:
		return MessageTypeClose, data[1:], nil
	case PingMessage:
		return MessageTypePing, data[1:], nil
	case PongMessage:
		return MessageTypePong, data[1:], nil
	case msg:
		if len(data) == 1 {
			return 0, "", ErrorWrongMessageType
		}
		restText = data[2:]
		switch data[0:2] {
		case emptyMessage:
			t = MessageTypeEmpty
			return
		case commonMessage:
			t = MessageTypeAckRequest
			return
		case ackMessage:
			t = MessageTypeAckResponse
			return
		}
	}
	err = ErrorWrongMessageType
	return
}

func getNamespace(text string) (namespace string, restText string) {
	if len(text) == 0 {
		return
	}
	if text[:1] == "/" {
		pos := strings.IndexByte(text, ',')
		if pos == -1 {
			namespace = text
			restText = ""
		} else {
			namespace = text[:pos]
			if len(text) > pos + 1 {
				restText = text[pos + 1:]
			} else {
				restText = ""
			}
		}
	} else {
		restText = text
	}
	return
}

/**
Get ack id of current packet, if present
*/
func getAck(text string) (ackId int, restText string, err error) {
	if len(text) == 0 {
		return 0, "", ErrorWrongPacket
	}

	pos := strings.IndexByte(text, '[')
	if pos == -1 {
		return 0, "", ErrorWrongPacket
	}

	ack, err := strconv.Atoi(text[0:pos])
	if err != nil {
		return 0, "", err
	}

	return ack, text[pos:], nil
}

/**
Get message method of current packet, if present
*/
func getMethod(text string) (method, restText string, err error) {
	var start, end, rest, countQuote int

	for i, c := range text {
		if c == '"' {
			switch countQuote {
			case 0:
				start = i + 1
			case 1:
				end = i
				rest = i + 1
			default:
				return "", "", ErrorWrongPacket
			}
			countQuote++
		}
		if c == ',' {
			if countQuote < 2 {
				continue
			}
			rest = i + 1
			break
		}
	}

	if (end < start) || (rest >= len(text)) {
		return "", "", ErrorWrongPacket
	}

	return text[start:end], text[rest : len(text)-1], nil
}

func Decode(data string) (*Message, error) {
	var err error
	msg := &Message{}
	msg.Source = data

	var rest string
	msg.Type, rest, err = getMessageType(data)
	if err != nil {
		return nil, err
	}

	msg.Namespace, rest = getNamespace(rest)

	if msg.Type == MessageTypeOpen {
		msg.Args = rest
		return msg, nil
	}

	if msg.Type == MessageTypeClose || msg.Type == MessageTypePing ||
		msg.Type == MessageTypePong || msg.Type == MessageTypeEmpty {
		return msg, nil
	}

	ack, rest, err := getAck(rest)
	msg.AckId = ack
	if msg.Type == MessageTypeAckResponse {
		if err != nil {
			return nil, err
		}
		msg.Args = rest[1 : len(rest)-1]
		return msg, nil
	}

	if err != nil {
		msg.Type = MessageTypeEmit
		rest = data[2:]
	}

	msg.Method, msg.Args, err = getMethod(rest)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
