package common

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
)

// MaxPayloadSize es el límite en bytes del payload de un batch (8 KiB).
const MaxPayloadSize = 8192

// Tipos de mensaje del protocolo.
const (
	MsgTypeBatch       = byte('B')
	MsgTypeFin         = byte('F')
	MsgTypeWinnerQuery = byte('W')
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

// SerializedBetSize devuelve el tamaño en bytes de la serialización de una apuesta
// en el formato "agency|fn|ln|doc|birth|num", sin contar el separador '\n' entre apuestas.
func SerializedBetSize(bet Bet) int {
	return len(bet.Agency) + 1 +
		len(bet.FirstName) + 1 +
		len(bet.LastName) + 1 +
		len(bet.Document) + 1 +
		len(bet.Birthdate) + 1 +
		len(bet.Number)
}

// sendMessage serializa y envía un mensaje con tipo.
// Formato: [1 byte tipo][4 bytes longitud big-endian][payload]
func sendMessage(conn net.Conn, msgType byte, payload []byte) error {
	header := make([]byte, 5)
	header[0] = msgType
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if err := sendAll(conn, header); err != nil {
		return err
	}
	if len(payload) > 0 {
		return sendAll(conn, payload)
	}
	return nil
}

// SendBatch serializa un batch de apuestas y lo envía por la conexión.
// Cada apuesta se separa con '\n' en el body.
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
	return sendMessage(conn, MsgTypeBatch, payload)
}

// SendFin notifica al servidor que la agencia terminó de enviar todas sus apuestas.
func SendFin(conn net.Conn, agencyID string) error {
	return sendMessage(conn, MsgTypeFin, []byte(agencyID))
}

// SendWinnerQuery consulta al servidor la lista de ganadores de la agencia.
func SendWinnerQuery(conn net.Conn, agencyID string) error {
	return sendMessage(conn, MsgTypeWinnerQuery, []byte(agencyID))
}

// ReceiveResponse lee una respuesta del servidor con prefijo de longitud.
func ReceiveResponse(conn net.Conn) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}
	length := binary.BigEndian.Uint32(header)
	if length == 0 {
		return "", nil
	}
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
