import struct
from .utils import Bet

HEADER_SIZE = 4

# Tipos de mensaje del protocolo
MSG_TYPE_BATCH = b'B'
MSG_TYPE_FIN = b'F'
MSG_TYPE_WINNER_QUERY = b'W'


def _recv_exactly(sock, n):
    """Lee exactamente n bytes del socket, iterando para evitar short reads."""
    data = b''
    while len(data) < n:
        chunk = sock.recv(n - len(data))
        if not chunk:
            raise ConnectionError("Connection closed unexpectedly")
        data += chunk
    return data


def _send_all(sock, data):
    """Envía todos los bytes al socket, iterando para evitar short writes."""
    total = 0
    while total < len(data):
        sent = sock.send(data[total:])
        if sent == 0:
            raise ConnectionError("Connection closed unexpectedly")
        total += sent


def receive_message(sock):
    """
    Recibe un mensaje del cliente.
    Formato: [1 byte tipo][4 bytes longitud big-endian][body]
    Retorna (tipo, body) donde tipo es bytes de 1 caracter y body es bytes.
    """
    msg_type = _recv_exactly(sock, 1)
    header = _recv_exactly(sock, HEADER_SIZE)
    length = struct.unpack('!I', header)[0]
    if length == 0:
        return msg_type, b''
    body = _recv_exactly(sock, length)
    return msg_type, body


def parse_batch(body):
    """
    Parsea el body de un mensaje de tipo BET_BATCH.
    Cada línea tiene el formato: agencia|nombre|apellido|documento|nacimiento|numero
    Retorna una lista de Bet.
    """
    bets = []
    for line in body.decode('utf-8').split('\n'):
        line = line.strip()
        if not line:
            continue
        fields = line.split('|')
        agency, first_name, last_name, document, birthdate, number = fields
        bets.append(Bet(agency, first_name, last_name, document, birthdate, number))
    return bets


def send_response(sock, msg):
    """
    Envía una respuesta al cliente.
    Formato: [4 bytes longitud big-endian][body]
    """
    if isinstance(msg, str):
        msg = msg.encode('utf-8')
    header = struct.pack('!I', len(msg))
    _send_all(sock, header + msg)
