package protocol

import (
	"bufio"
	"io"
	"net"
	"strconv"
)

// Message is the message type for communication
type Message map[string]string

// Request send a `Message` type request to a connection and waits for a `Message` type response
// like a simple HTTP request
func Request(conn net.Conn, req Message) (Message, error) {
	err := sendData(conn, req)
	if err != nil {
		return Message{}, err
	}

	res, err := getData(conn)
	if err != nil {
		return Message{}, err
	}

	return res, nil
}

// HandleRequest reads a `Message` type request from a connection and send back map[string][string]response
func HandleRequest(c net.Conn, reqHandler func(Message) Message) error {
	req, err := getData(c)
	if err != nil {
		return err
	}
	resp := reqHandler(req)
	sendData(c, resp)
	return nil
}

// sendData sends a `Message` type data to a TCP connection with self-defined protocol
func sendData(c net.Conn, m Message) error {
	toSend := serialize(m)
	lenPrefix := strconv.Itoa(len(toSend)) + ";"
	toSend = append([]byte(lenPrefix), toSend...)
	cnt := len(toSend)
	for cnt > 0 {
		l, err := c.Write(toSend)
		if err != nil {
			return err
		}
		cnt -= l
	}
	return nil
}

// getData gets a `Message` type from a TCP connection with self-defined protocol
func getData(c net.Conn) (Message, error) {
	reader := bufio.NewReader(c)
	lenPrefix, err := reader.ReadBytes(';')
	if err != nil {
		return nil, err
	}

	msgLen, err := strconv.Atoi(string(lenPrefix[:len(lenPrefix)-1]))
	if err != nil {
		return nil, err
	}

	resData := make([]byte, msgLen)
	_, err = io.ReadFull(reader, resData)
	if err != nil {
		return nil, err
	}

	// resData = append(resData, line...)
	res, err := unserialize(resData)
	if err != nil {
		return nil, err
	}
	return res, nil
}
