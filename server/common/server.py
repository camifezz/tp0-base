import socket
import logging
import signal


class Server:
    def __init__(self, port, listen_backlog):
        # Atributo para saber si se tiene que cerrar o no la conexion
        self._shutting_down = False

        # Initialize server socket
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)

        signal.signal(signal.SIGTERM, self._handle_sigterm)

    def _handle_sigterm(self, signum, frame):
        """
        Handler de SIGTERM.

        Marca el servidor en estado de apagado y cierra el socket listener
        para detener la aceptación de nuevas conexiones y permitir que el
        loop principal finalice de manera ordenada.
        """
        logging.info('action: shutdown_signal_received | result: success | signal: SIGTERM')
        self._shutting_down = True

        try:
            self._server_socket.close()
            logging.info('action: close_server_socket | result: success')
        except OSError as e:
            logging.error(f'action: close_server_socket | result: fail | error: {e}')

    def run(self):
        """
        Server loop

        Accepts new connections and communicates with clients until a
        SIGTERM is received. On shutdown, the listener socket is closed
        gracefully.
        """
        while not self._shutting_down:
            try:
                client_sock = self.__accept_new_connection()
                self.__handle_client_connection(client_sock)
            except OSError as e:
                if self._shutting_down:
                    break
                logging.error(f'action: accept_connections | result: fail | error: {e}')

        logging.info('action: graceful_shutdown | result: success')

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket.

        If a problem arises in the communication with the client, the
        client socket will also be closed.
        """
        try:
            msg = client_sock.recv(1024).rstrip().decode('utf-8')
            addr = client_sock.getpeername()
            logging.info(f'action: receive_message | result: success | ip: {addr[0]} | msg: {msg}')
            client_sock.send(f"{msg}\n".encode('utf-8'))
        except OSError as e:
            logging.error(f'action: receive_message | result: fail | error: {e}')
        finally:
            try:
                client_sock.close()
                logging.info('action: close_client_socket | result: success')
            except OSError as e:
                logging.error(f'action: close_client_socket | result: fail | error: {e}')

    def __accept_new_connection(self):
        """
        Accept new connections.

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned.
        """
        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c

