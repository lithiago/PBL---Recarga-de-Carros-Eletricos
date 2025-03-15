import requests
import socket

# Chave da API ipinfo.io
API_TOKEN = "a8c315fd547e31"

HOST = "0.0.0.0"  
PORT = 5000       

server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
server.bind((HOST, PORT))
server.listen(5)

print(f"Servidor rodando em {HOST}:{PORT}...")

while True:
    client_socket, client_address = server.accept()
    client_ip = client_address[0]
    print(f"Nova conexão de: {client_ip}")

    # Se for IP privado, obtenha o IP público
    if client_ip.startswith(("192.", "10.", "127.", "172.")):
        ip_response = requests.get("https://api64.ipify.org?format=json")
        client_ip = ip_response.json().get("ip", client_ip)

    # Chamar a API ipinfo.io
    url = f"https://ipinfo.io/{client_ip}?token={API_TOKEN}"
    response = requests.get(url)

    if response.status_code == 200:
        data = response.json()
        city = data['city']
        region = data['region']
        country = data['country']
        latitude = data['loc']
        print(f"Localização de {client_ip}: {city}, {region}, {country}, {latitude}")
    else:
        print("Erro ao obter localização")

    client_socket.close()
