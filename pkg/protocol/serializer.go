package protocol

import (
	"bytes"
	"errors"
	"strconv"
)

func serialize(m Message) []byte {
	r := []byte{}
	for k, v := range m {
		r = append(r, []byte(k)...) // key
		r = append(r, ';')
		r = append(r, []byte(strconv.Itoa(len(v)))...) // the len of the value
		r = append(r, ';')
		r = append(r, []byte(v)...) // value
	}

	return r
}

func unserialize(data []byte) (Message, error) {

	msg := make(Message)
	cur := 0

	for cur < len(data) {
		// getting key
		index := bytes.IndexByte(data[cur:], ';')
		if index == -1 {
			return Message{}, errors.New("Failed to unserialize with 'key' field")
		}
		k := string(data[cur : cur+index])
		cur += index + 1

		// getting len
		index = bytes.IndexByte(data[cur:], ';')
		if index == -1 {
			return Message{}, errors.New("Failed to unserialize with 'data len' field")
		}
		l, err := strconv.Atoi(string(data[cur : cur+index]))
		if err != nil || l < 0 {
			return Message{}, errors.New("Failed to unserialize with 'data len' field")
		}
		cur += index + 1

		// saving k,v
		if cur+l >= len(data) {
			return Message{}, errors.New("Failed to unserialize with 'value' field")
		}
		msg[k] = string(data[cur : cur+l])
		cur += l
	}

	return msg, nil
}
