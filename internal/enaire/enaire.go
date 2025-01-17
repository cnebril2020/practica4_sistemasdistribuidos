// NO MODIFICAR ESTE ARCHIVO
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

var (
	buf    bytes.Buffer
	logger = log.New(&buf, "logger: ", log.Lshortfile)
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8000")
	if err != nil {
		logger.Fatal(err)
	}
	iniciar(conn)
	for i := 0; i < 10; i++ {
		operando(conn)
	}
	terminar(conn)
	conn.Close()
}

func iniciar(dst net.Conn) {
	time.Sleep(1 * time.Second)
	setSeparator()
	fmt.Println("Iniciando operación en: " + dst.RemoteAddr().String())
	Send2conn(dst, 0)
	time.Sleep(1 * time.Second)
}

func terminar(dst net.Conn) {
	setSeparator()
	Send2conn(dst, 0)
	time.Sleep(1 * time.Second)
	fmt.Println("Terminando operación en: " + dst.RemoteAddr().String())
}

func operando(dst net.Conn) {
	setSeparator()
	Send2conn(dst, getRand())
	fmt.Println("Operando en: " + dst.RemoteAddr().String())
	tiempo := getRand() + 1
	fmt.Println("Tiempo: " + strconv.Itoa(tiempo) + "seg.")
	time.Sleep(time.Duration(tiempo) * time.Second)
}

func Send2conn(dst net.Conn, number int) {
	msg := strconv.Itoa(number)
	fmt.Println("Msg enviado: " + msg)
	r := strings.NewReader(msg + "\n")
	io.Copy(dst, r)
}

func getRand() (num int) {
	num = int(time.Now().UTC().UnixNano()) % 10
	if num == 0 {
		num = 9
	}
	return
}

func setSeparator() {
	fmt.Println("------------------")
}
