import socket
from threading import Thread, Lock
import requests
import math
import time
import random
import json
from ipaddress import ip_network, ip_address

class Client:
    def __init__(self, HOST, PORT, API_TOKEN):
        self.lock = Lock()
        self.socket_client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.socket_client.connect((HOST, PORT))
        self.bateria = 100
        self.longitude = 0
        self.latitude = 0
        self.consumo_kw = random.uniform(0, 1)
        self.API_TOKEN = API_TOKEN

        self.handlers = {
            "recarga": self.solicita_recarga,
            "alerta": self.recebe_alerta,
        }
        
        self.talk_to_server()
        self.localizacao()

    def talk_to_server(self):
        Thread(target=self.receive_data, daemon=True).start()
        self.send_data()

    def localizacao(self):
        Thread(target=self.atualiza_localizacao, daemon=True).start()

    def get_local_ip(self):
        local_hostname = socket.gethostname()
        ip_addresses = socket.gethostbyname_ex(local_hostname)[2]
        filtered_ips = [ip for ip in ip_addresses if not ip.startswith("127.")]
        return filtered_ips[0] if filtered_ips else None

    def is_private(self, ip):
        private_ranges = [
            ip_network("192.168.0.0/16"),
            ip_network("10.0.0.0/8"),
            ip_network("172.16.0.0/12"),
            ip_network("127.0.0.0/8"),
        ]
        return any(ip_address(ip) in net for net in private_ranges)

    def send_data(self):
        with self.lock:
            data = json.dumps({
                "latitude": self.latitude, 
                "longitude": self.longitude, 
                "bateria": self.bateria
            })
        self.socket_client.sendall(data.encode())

    def receive_data(self):
        while True:
            server_data = self.socket_client.recv(1024).decode()
            try:
                data = json.loads(server_data)
                if "tipo" in data:
                    handler = self.handlers.get(data["tipo"], None)
                    if handler:
                        handler(data)
                    else:
                        print(f"Tipo de mensagem desconhecido: {data['tipo']}")
            except json.JSONDecodeError:
                print("Erro ao decodificar JSON recebido:", server_data)

    def atualiza_localizacao(self):
        client_ip = self.get_local_ip()
        if self.latitude == 0 and self.longitude == 0:
            if client_ip and self.is_private(client_ip):
                ip_response = requests.get("https://api64.ipify.org?format=json")
                client_ip = ip_response.json().get("ip", client_ip)
            
            url = f"https://ipinfo.io/{client_ip}?token={self.API_TOKEN}"
            response = requests.get(url)
            data = response.json()
            localizacao = data['loc'].split(",")
            self.latitude = float(localizacao[0])
            self.longitude = float(localizacao[1])
        
        velocidade_kmh = 50
        intervalo_segundos = 1  
        fator_grau_por_metro = 1 / 111320  

        while True:
            direcao = random.uniform(0, 360)
            deslocamento_metros = (velocidade_kmh * 1000 / 3600) * intervalo_segundos  
            deslocamento_graus = deslocamento_metros * fator_grau_por_metro  

            with self.lock:
                self.latitude += deslocamento_graus * math.cos(math.radians(direcao))
                self.longitude += deslocamento_graus * math.sin(math.radians(direcao))
                self.bateria -= deslocamento_metros / 1000 * self.consumo_kw  

            time.sleep(intervalo_segundos)

    def solicita_recarga(self, data=None):
        data = json.dumps({"tipo": "recarga", "bateria": self.bateria})
        self.socket_client.sendall(data.encode())

    def recebe_alerta(self, data):
        print("Alerta recebido:", data)
