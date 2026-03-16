package common

import (
	"net"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

type ClientConfig struct {
	ID            string
	ServerAddress string
	Bet           Bet
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

func (c *Client) StartClientLoop() {
	if err := c.createClientSocket(); err != nil {
		return
	}
	defer c.closeConnection()

	if c.shuttingDown {
		return
	}

	if err := SendBet(c.conn, c.config.Bet); err != nil {
		log.Errorf("action: send_bet | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return
	}

	response, err := ReceiveResponse(c.conn)
	if err != nil {
		log.Errorf("action: receive_response | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return
	}

	if response == "OK" {
		log.Infof("action: apuesta_enviada | result: success | dni: %v | numero: %v",
			c.config.Bet.Document, c.config.Bet.Number)
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | client_id: %v | response: %v",
			c.config.ID, response)
	}
}
