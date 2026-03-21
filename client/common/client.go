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

// StartClientLoop abre una única conexión, lee el archivo CSV de apuestas en
// batches y los envía al servidor. Por cada batch espera la confirmación del servidor.
func (c *Client) StartClientLoop() {
	if err := c.createClientSocket(); err != nil {
		return
	}
	defer c.closeConnection()

	file, err := os.Open(c.config.DataFilePath)
	if err != nil {
		log.Errorf("action: open_file | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	batch := make([]Bet, 0, c.config.MaxBatchSize)

	// Procesa el archivo línea a línea, acumulando apuestas en batches
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
			batch = append(batch, Bet{
				Agency:    c.config.ID,
				FirstName: parts[0],
				LastName:  parts[1],
				Document:  parts[2],
				Birthdate: parts[3],
				Number:    parts[4],
			})
		}

		// Envía el batch cuando está lleno o cuando se terminó el archivo
		if len(batch) == c.config.MaxBatchSize || (!hasMore && len(batch) > 0) {
			if err := c.sendBatchAndReceive(batch); err != nil {
				return
			}
			batch = batch[:0]
		}

		if !hasMore {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Errorf("action: read_file | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
	}
}

// sendBatchAndReceive envía un batch al servidor y espera su confirmación.
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
