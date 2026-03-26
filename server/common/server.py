import socket
import logging
import signal
import threading
from .protocol import (
    receive_message, parse_batch, send_response,
    MSG_TYPE_BATCH, MSG_TYPE_FIN, MSG_TYPE_WINNER_QUERY
)
from .utils import store_bets, load_bets, has_won


class Server:
    def __init__(self, port, listen_backlog, total_agencies):
        self._shutting_down = False

        # Lock para proteger store_bets (no es thread-safe)
        self._store_lock = threading.Lock()

        # Barrera que bloquea cada thread hasta que todas las agencias
        # finalizaron el envío de apuestas. Al llegar la última, ejecuta
        # el sorteo automáticamente como acción de la barrera.
        self._barrier = threading.Barrier(total_agencies, action=self.__run_lottery)

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
        Acepta conexiones y lanza un thread por cliente hasta recibir SIGTERM.
        """
        threads = []
        while not self._shutting_down:
            try:
                client_sock = self.__accept_new_connection()
                t = threading.Thread(target=self.__handle_client_connection, args=(client_sock,))
                t.start()
                threads.append(t)
            except OSError as e:
                if self._shutting_down:
                    break
                logging.error(f'action: accept_connections | result: fail | error: {e}')

        for t in threads:
            t.join()

        logging.info('action: graceful_shutdown | result: success')

    def __handle_client_connection(self, client_sock):
        """
        Atiende a un cliente en su propio thread:
        recibe todos sus batches, espera en la barrera junto a las demás
        agencias y luego responde con los ganadores correspondientes.
        """
        try:
            msg_type, body = receive_message(client_sock)
            if msg_type != MSG_TYPE_BATCH:
                return

            agency_id = self.__handle_betting_phase(client_sock, body)

            # Bloquea hasta que todas las agencias terminaron.
            # La última en llegar ejecuta __run_lottery automáticamente.
            self._barrier.wait()

            self.__handle_winner_query(client_sock, agency_id)
        except threading.BrokenBarrierError:
            logging.error('action: barrier | result: fail | error: barrera rota')
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
        Recibe y almacena todos los batches de apuestas hasta recibir el FIN.
        Retorna el agency_id de la agencia conectada.
        """
        bets = parse_batch(first_body)
        agency_id = bets[0].agency if bets else None
        self.__store_and_respond(client_sock, bets)

        while True:
            msg_type, body = receive_message(client_sock)
            if msg_type == MSG_TYPE_FIN:
                agency_id = body.decode('utf-8')
                break
            elif msg_type == MSG_TYPE_BATCH:
                bets = parse_batch(body)
                self.__store_and_respond(client_sock, bets)

        return agency_id

    def __store_and_respond(self, client_sock, bets):
        """Almacena un batch de apuestas con lock y responde OK o ERR al cliente."""
        try:
            with self._store_lock:
                store_bets(bets)
            logging.info(f'action: apuesta_recibida | result: success | cantidad: {len(bets)}')
            send_response(client_sock, 'OK')
        except Exception as e:
            logging.error(f'action: apuesta_recibida | result: fail | cantidad: {len(bets)} | error: {e}')
            send_response(client_sock, 'ERR')

    def __run_lottery(self):
        """
        Ejecuta el sorteo. Es llamado automáticamente por la barrera
        cuando todas las agencias finalizaron el envío de apuestas.
        """
        logging.info('action: sorteo | result: success')

    def __handle_winner_query(self, client_sock, agency_id):
        """Recibe la consulta de ganadores y responde con los DNIs de la agencia."""
        msg_type, body = receive_message(client_sock)
        if msg_type == MSG_TYPE_WINNER_QUERY:
            agency_id = body.decode('utf-8')
            winners = [
                bet.document
                for bet in load_bets()
                if str(bet.agency) == agency_id and has_won(bet)
            ]
            send_response(client_sock, ','.join(winners))

    def __accept_new_connection(self):
        """
        Acepta una nueva conexión entrante y retorna el socket del cliente.
        """
        logging.info('action: accept_connections | result: in_progress')
        c, addr = self._server_socket.accept()
        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        return c
