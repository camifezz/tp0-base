import socket
import logging
import signal
from .protocol import (
    receive_message, parse_batch, send_response,
    MSG_TYPE_BATCH, MSG_TYPE_FIN, MSG_TYPE_WINNER_QUERY
)
from .utils import store_bets, load_bets, has_won

TOTAL_AGENCIES = 5


class Server:
    def __init__(self, port, listen_backlog):
        self._shutting_down = False
        self._fins_received = 0
        self._lottery_done = False

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
        Atiende a un cliente. Distingue dos tipos de conexión según el primer mensaje:
        - BET_BATCH: fase de apuestas seguida de FIN y consulta de ganadores.
        - WINNER_QUERY: reconexión para consultar ganadores (cuando se recibió NOT_READY).
        """
        try:
            msg_type, body = receive_message(client_sock)

            if msg_type == MSG_TYPE_BATCH:
                self.__handle_betting_phase(client_sock, body)
                self.__handle_winner_query(client_sock)
            elif msg_type == MSG_TYPE_WINNER_QUERY:
                agency_id = body.decode('utf-8')
                self.__respond_winner_query(client_sock, agency_id)
        except (OSError, ConnectionError, ValueError) as e:
            logging.error(f'action: handle_client | result: fail | error: {e}')
        finally:
            try:
                client_sock.close()
                logging.info('action: close_client_socket | result: success')
            except OSError as e:
                logging.error(f'action: close_client_socket | result: fail | error: {e}')

    def __handle_betting_phase(self, client_sock, first_body):
        """
        Recibe batches de apuestas y los almacena. Cuando llega el mensaje FIN,
        incrementa el contador de agencias finalizadas y corre el sorteo si son todas.
        """
        # Procesa el primer batch ya leído
        bets = parse_batch(first_body)
        self.__store_and_respond(client_sock, bets)

        # Sigue recibiendo hasta el FIN
        while True:
            msg_type, body = receive_message(client_sock)
            if msg_type == MSG_TYPE_FIN:
                self._fins_received += 1
                if self._fins_received == TOTAL_AGENCIES:
                    self.__run_lottery()
                break
            elif msg_type == MSG_TYPE_BATCH:
                bets = parse_batch(body)
                self.__store_and_respond(client_sock, bets)

    def __store_and_respond(self, client_sock, bets):
        """Almacena un batch de apuestas y responde OK o ERR al cliente."""
        try:
            store_bets(bets)
            logging.info(f'action: apuesta_recibida | result: success | cantidad: {len(bets)}')
            send_response(client_sock, 'OK')
        except Exception as e:
            logging.error(f'action: apuesta_recibida | result: fail | cantidad: {len(bets)} | error: {e}')
            send_response(client_sock, 'ERR')

    def __run_lottery(self):
        """Ejecuta el sorteo una vez que todas las agencias finalizaron el envío."""
        logging.info('action: sorteo | result: success')
        self._lottery_done = True

    def __handle_winner_query(self, client_sock):
        """Recibe la consulta de ganadores y responde según el estado del sorteo."""
        msg_type, body = receive_message(client_sock)
        if msg_type == MSG_TYPE_WINNER_QUERY:
            agency_id = body.decode('utf-8')
            self.__respond_winner_query(client_sock, agency_id)

    def __respond_winner_query(self, client_sock, agency_id):
        """
        Responde a una consulta de ganadores.
        Si el sorteo no ocurrió aún, responde NOT_READY.
        Si ya ocurrió, responde con los DNIs ganadores de la agencia separados por coma.
        """
        if not self._lottery_done:
            send_response(client_sock, 'NOT_READY')
            return

        winners = [
            bet.document
            for bet in load_bets()
            if bet.agency == agency_id and has_won(bet)
        ]
        send_response(client_sock, ','.join(winners))

    def __accept_new_connection(self):
        """
        Accept new connections.

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned.
        """
        logging.info('action: accept_connections | result: in_progress')
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c
