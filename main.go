// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
)

var (
	address = flag.String("address", "127.0.0.1:8080", "http service address")
	cmdPath string
)

var (
	upgrader = websocket.Upgrader{}
)

func main() {
	flag.Parse()
	if len(flag.Args()) < 1 {
		log.Fatal("must specify at least one argument")
	}

	var err error
	cmdPath, err = exec.LookPath(flag.Args()[0])
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/ws", handleWs)

	if err := http.ListenAndServe(*address, nil); err != nil {
		log.Fatal(err)
	}
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func handleWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	outr, outw, err := os.Pipe()
	if err != nil {
		internalError(ws, "stdout:", err)
		return
	}

	defer func(outr *os.File) {
		_ = outr.Close()
	}(outr)

	defer func(outw *os.File) {
		_ = outw.Close()
	}(outw)

	inr, inw, err := os.Pipe()
	if err != nil {
		internalError(ws, "stdin:", err)
		return
	}

	defer func(inr *os.File) {
		_ = inr.Close()
	}(inr)

	defer func(inw *os.File) {
		_ = inw.Close()
	}(inw)

	proc, err := os.StartProcess(cmdPath, flag.Args(), &os.ProcAttr{
		Files: []*os.File{inr, outw, outw},
	})
	if err != nil {
		internalError(ws, "start:", err)
		return
	}

	_ = inr.Close()
	_ = outw.Close()

	stdoutDone := make(chan struct{})

	go func() {
		_ = pumpStdout(ws, outr, stdoutDone)
	}()

	go func() {
		_ = ping(ws, stdoutDone)
	}()

	_ = pumpStdin(ws, inw)

	// Some commands will exit when stdin is closed.
	_ = inw.Close()

	// Other commands need a bonk on the head.
	if err := proc.Signal(os.Interrupt); err != nil {
		log.Println("inter:", err)
	}

	select {
	case <-stdoutDone:
	case <-time.After(time.Second):
		// A bigger bonk on the head.
		if err := proc.Signal(os.Kill); err != nil {
			log.Println("term:", err)
		}
		<-stdoutDone
	}

	if _, err := proc.Wait(); err != nil {
		log.Println("wait:", err)
	}
}

func pumpStdout(ws *websocket.Conn, r io.Reader, done chan struct{}) error {
	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	s := bufio.NewScanner(r)
	for s.Scan() {
		if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			return errors.Wrap(err, "failed to set write deadline")
		}
		if err := ws.WriteMessage(websocket.TextMessage, s.Bytes()); err != nil {
			return errors.Wrap(err, "failed to write message")
		}
	}

	if s.Err() != nil {
		return errors.Wrap(s.Err(), "failed to scan")
	}

	close(done)

	if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return errors.Wrap(err, "failed to set write deadline")
	}

	if err := ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		return errors.Wrap(err, "failed to write message")
	}

	time.Sleep(closeGracePeriod)

	return nil
}

func ping(ws *websocket.Conn, done chan struct{}) error {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				return errors.Wrap(err, "failed to write control")
			}
		case <-done:
			return nil
		}
	}
}

func pumpStdin(ws *websocket.Conn, w io.Writer) error {
	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	ws.SetReadLimit(maxMessageSize)
	if err := ws.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return errors.Wrap(err, "failed to set read deadline")
	}

	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			return errors.Wrap(err, "failed to read message")
		}
		message = append(message, '\n')
		if _, err := w.Write(message); err != nil {
			return errors.Wrap(err, "failed to write")
		}
	}
}

func internalError(ws *websocket.Conn, msg string, err error) {
	log.Println(msg, err)
	_ = ws.WriteMessage(websocket.TextMessage, []byte("Internal server error."))
}
