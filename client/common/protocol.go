package common

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// Bet representa una apuesta de quiniela a enviar al servidor.
type Bet struct {
	Agency    string
	FirstName string
	LastName  string
	Document  string
	Birthdate string
	Number    string
}

// SendBet serializa la apuesta y la envía por la conexión usando un prefijo
// de 4 bytes big-endian con la longitud del mensaje para evitar short writes.
func SendBet(conn net.Conn, bet Bet) error {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		bet.Agency,
		bet.FirstName,
		bet.LastName,
		bet.Document,
		bet.Birthdate,
		bet.Number,
	)

	data := []byte(payload)
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))

	if err := sendAll(conn, header); err != nil {
		return err
	}
	return sendAll(conn, data)
}

// ReceiveResponse lee una respuesta con prefijo de longitud enviada por el servidor.
func ReceiveResponse(conn net.Conn) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}
	length := binary.BigEndian.Uint32(header)

	body := make([]byte, length)
	if _, err := io.ReadFull(conn, body); err != nil {
		return "", err
	}
	return string(body), nil
}

// sendAll escribe todos los bytes en la conexión, iterando para evitar short writes.
func sendAll(conn net.Conn, data []byte) error {
	total := 0
	for total < len(data) {
		n, err := conn.Write(data[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
}
