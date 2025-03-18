import socket
from threading import Thread
import requests
import math
import time
import random
import json

### USAR MUTEX
class Client:
    def __init__(self, HOST, PORT):
        # O SOCKET AF_INET INFORMA QUE ESTAMOS TRABALHANDO COM OS ENDEREÇOS DA FAMÍLIA IPV4. O SOCKET_STREAM DEFINE UM PROTOCOLO TCP
        self.socket_client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.socket_client.connect((HOST, PORT))
        self.bateria = 100
        self.longitude = 0
        self.latitude = 0
        self.consumo_kw = random.randrange(0,1, 0.1)
        

    def get_local_ip():
        local_hostname = socket.gethostname()
        ip_addresses = socket.gethostbyname_ex(local_hostname)[2]
        filtered_ips = [ip for ip in ip_addresses if not ip.startswith("127.")]
        return filtered_ips[0] if filtered_ips else None  # Retorna None se não houver IP válido
    
    def send_data(self):
        data = json.dumps({"latitude": self.latitude, "longitude": self.longitude, "bateria": self.bateria})  # Converte para JSON
        self.socket_client.sendall(data.encode())  # Converte para bytes e envia
        
    def atualiza_localizacao(self):
        client_ip = self.get_local_ip()
        if self.latitude == 0 and self.longitude == 0:
            # Se for IP privado, obtenha o IP público
            if client_ip.startswith(("192.", "10.", "127.", "172.")):
                ip_response = requests.get("https://api64.ipify.org?format=json")
                client_ip = ip_response.json().get("ip", client_ip)
            # Chamar a API ipinfo.io
            url = f"https://ipinfo.io/{client_ip}?token={API_TOKEN}"
            response = requests.get(url)
            data = response.json() 
            localizacao = data['loc'].split(",")
            self.latitude = float(localizacao[0])
            self.longitude = float(localizacao[1])
            
        velocidade_kmh = 50
        intervalo_segundos = 1  # Atualização a cada 1 segundo
        fator_grau_por_metro = 1 / 111320  # Conversão de metros para graus de latitude
        
        while True:
            direcao = random.uniform(0, 360)  # Direção aleatória (0° a 360°)
            deslocamento_metros = (velocidade_kmh * 1000 / 3600) * intervalo_segundos  # m/s
            deslocamento_graus = deslocamento_metros * fator_grau_por_metro  # Conversão

             # Atualiza a posição com base na direção
            self.latitude += deslocamento_graus * math.cos(math.radians(direcao))
            self.longitude += deslocamento_graus * math.sin(math.radians(direcao))
            time.sleep(intervalo_segundos)
            self.bateria -= deslocamento_metros / 1000 * self.consumo_kwh   

            
            