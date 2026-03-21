package common

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
)

const maxPayloadSize = 8192 // 8 KiB

// Bet representa una apuesta de quiniela a enviar al servidor.
type Bet struct {
	Agency    string
	FirstName string
	LastName  string
	Document  string
	Birthdate string
	Number    string
}

// SendBatch serializa un batch de apuestas y lo envía por la conexión.
// Cada apuesta se separa con '\n' en el body. El protocolo usa un prefijo
// de 4 bytes big-endian con la longitud total para evitar short writes.
// Retorna error si el payload supera los 8 KiB.
func SendBatch(conn net.Conn, bets []Bet) error {
	lines := make([]string, len(bets))
	for i, bet := range bets {
		lines[i] = fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			bet.Agency,
			bet.FirstName,
			bet.LastName,
			bet.Document,
			bet.Birthdate,
			bet.Number,
		)
	}

	payload := []byte(strings.Join(lines, "\n"))
	if len(payload) > maxPayloadSize {
		return fmt.Errorf("payload de %d bytes supera el límite de %d bytes", len(payload), maxPayloadSize)
	}

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))

	if err := sendAll(conn, header); err != nil {
		return err
	}
	return sendAll(conn, payload)
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
