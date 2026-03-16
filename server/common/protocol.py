import struct
from .utils import Bet

HEADER_SIZE = 4


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


def receive_bet(sock):
    """
    Recibe una apuesta del cliente.
    Protocolo: [4 bytes big-endian con longitud][agencia|nombre|apellido|documento|nacimiento|numero]
    """
    header = _recv_exactly(sock, HEADER_SIZE)
    length = struct.unpack('!I', header)[0]
    body = _recv_exactly(sock, length).decode('utf-8')

    fields = body.split('|')
    agency, first_name, last_name, document, birthdate, number = fields
    return Bet(agency, first_name, last_name, document, birthdate, number)


def send_response(sock, ok):
    """
    Envía una respuesta al cliente.
    Protocolo: [4 bytes big-endian con longitud]["OK" o "ERR"]
    """
    msg = b'OK' if ok else b'ERR'
    header = struct.pack('!I', len(msg))
    _send_all(sock, header + msg)
