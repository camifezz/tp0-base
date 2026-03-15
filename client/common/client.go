package common

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
}

// Client Entity that encapsulates client behavior and resources
type Client struct {
	config       ClientConfig
	conn         net.Conn
	shuttingDown bool
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) *Client {
	client := &Client{
		config: config,
	}
	return client
}

// createClientSocket Initializes client socket.
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

// closeConnection Closes the current client socket if it exists.
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

// Shutdown Marks the client for shutdown and closes open resources.
func (c *Client) Shutdown() {
	log.Infof("action: shutdown_signal_received | result: success | client_id: %v", c.config.ID)
	c.shuttingDown = true
	c.closeConnection()
}

// StartClientLoop Send messages to the server until some time threshold is met
// or a shutdown is requested.
func (c *Client) StartClientLoop() {
	for msgID := 1; msgID <= c.config.LoopAmount; msgID++ {
		if c.shuttingDown {
			log.Infof("action: stop_loop | result: success | client_id: %v", c.config.ID)
			return
		}

		if err := c.createClientSocket(); err != nil {
			return
		}

		_, err := fmt.Fprintf(
			c.conn,
			"[CLIENT %v] Message N°%v\n",
			c.config.ID,
			msgID,
		)
		if err != nil {
			log.Errorf("action: send_message | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
			c.closeConnection()
			return
		}

		msg, err := bufio.NewReader(c.conn).ReadString('\n')
		c.closeConnection()

		if err != nil {
			log.Errorf("action: receive_message | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
			return
		}

		log.Infof("action: receive_message | result: success | client_id: %v | msg: %v",
			c.config.ID,
			msg,
		)

		if c.shuttingDown {
			log.Infof("action: stop_loop | result: success | client_id: %v", c.config.ID)
			return
		}

		time.Sleep(c.config.LoopPeriod)
	}

	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}
