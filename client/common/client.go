package common

import (
	"bufio"
	"net"
	"os"
	"strings"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

type ClientConfig struct {
	ID            string
	ServerAddress string
	DataFilePath  string
	MaxBatchSize  int
}

type Client struct {
	config       ClientConfig
	conn         net.Conn
	shuttingDown bool
}

func NewClient(config ClientConfig) *Client {
	return &Client{config: config}
}

func (c *Client) createClientSocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
		return err
	}

	c.conn = conn
	log.Infof("action: connect | result: success | client_id: %v", c.config.ID)
	return nil
}

func (c *Client) closeConnection() {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Errorf("action: close_socket | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
		} else {
			log.Infof("action: close_socket | result: success | client_id: %v",
				c.config.ID,
			)
		}
		c.conn = nil
	}
}

func (c *Client) Shutdown() {
	log.Infof("action: shutdown_signal_received | result: success | client_id: %v", c.config.ID)
	c.shuttingDown = true
	c.closeConnection()
}

// StartClientLoop gestiona el flujo completo del cliente en una única conexión:
// 1. Envía todos los batches de apuestas al servidor.
// 2. Notifica al servidor que terminó con un mensaje de (FIN).
// 3. Envía la consulta de ganadores y se bloquea hasta recibir la respuesta.
//    El servidor mantiene la conexión abierta hasta que el sorteo esté listo.
func (c *Client) StartClientLoop() {
	if err := c.createClientSocket(); err != nil {
		return
	}
	defer c.closeConnection()

	if err := c.sendAllBets(); err != nil {
		return
	}

	if err := SendFin(c.conn, c.config.ID); err != nil {
		log.Errorf("action: send_fin | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	if err := SendWinnerQuery(c.conn, c.config.ID); err != nil {
		log.Errorf("action: send_winner_query | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	// El servidor bloquea hasta que todas las agencias terminaron (barrera).
	// El cliente espera la respuesta sin reconectar.
	response, err := ReceiveResponse(c.conn)
	if err != nil {
		log.Errorf("action: receive_winners | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	count := 0
	if response != "" {
		count = len(strings.Split(response, ","))
	}
	log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %d", count)
}

// sendAllBets lee el CSV de apuestas y las envía al servidor en batches.
// Retorna error si falla el envío o la recepción de respuesta.
func (c *Client) sendAllBets() error {
	file, err := os.Open(c.config.DataFilePath)
	if err != nil {
		log.Errorf("action: open_file | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	batch := make([]Bet, 0, c.config.MaxBatchSize)
	currentPayloadSize := 0

	// Procesa el archivo línea a línea, acumulando apuestas en batches.
	// Manda el batch cuando se alcanza maxAmount apuestas o cuando agregar
	// la siguiente apuesta superaría el límite de 8KB.
	for !c.shuttingDown {
		hasMore := scanner.Scan()
		if hasMore {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) != 5 {
				log.Errorf("action: parse_bet | result: fail | client_id: %v | line: %v",
					c.config.ID, line)
				continue
			}
			bet := Bet{
				Agency:    c.config.ID,
				FirstName: parts[0],
				LastName:  parts[1],
				Document:  parts[2],
				Birthdate: parts[3],
				Number:    parts[4],
			}

			betSize := SerializedBetSize(bet)
			if len(batch) > 0 {
				betSize++ // separador '\n'
			}

			// Manda el batch si agregar la apuesta supera 8KB o se alcanzó maxAmount
			if len(batch) > 0 && (currentPayloadSize+betSize > MaxPayloadSize || len(batch) == c.config.MaxBatchSize) {
				if err := c.sendBatchAndReceive(batch); err != nil {
					return err
				}
				batch = batch[:0]
				currentPayloadSize = 0
				betSize = SerializedBetSize(bet)
			}

			batch = append(batch, bet)
			currentPayloadSize += betSize
		}

		// Manda el batch restante al terminar el archivo
		if !hasMore {
			if len(batch) > 0 {
				if err := c.sendBatchAndReceive(batch); err != nil {
					return err
				}
			}
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Errorf("action: read_file | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return err
	}
	return nil
}

// sendBatchAndReceive manda un batch al servidor y espera su confirmación.
func (c *Client) sendBatchAndReceive(batch []Bet) error {
	if err := SendBatch(c.conn, batch); err != nil {
		log.Errorf("action: send_batch | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return err
	}

	response, err := ReceiveResponse(c.conn)
	if err != nil {
		log.Errorf("action: receive_response | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return err
	}

	if response == "OK" {
		log.Infof("action: apuesta_enviada | result: success | cantidad: %v",
			len(batch))
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | client_id: %v | cantidad: %v",
			c.config.ID, len(batch))
	}
	return nil
}
