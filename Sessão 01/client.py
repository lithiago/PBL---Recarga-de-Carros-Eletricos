import socket


def main():
    client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    
    try:
        client.connect(('localhost', 5000))  # Conecta ao servidor
        print("Conectado ao servidor!")
    except Exception as e:
        print(f"Erro ao conectar ao servidor: {e}")

    client.close()  # Fecha a conexão

    """ username = input("User: ")
    print("\n Conectado")
    
    thread1 = threading.Thread(target=receiveMessage, args=[client])
    thread2 = threading.Thread(target=sendMessage, args=[client, username])
    
    thread1.start()
    thread2.start()
    
    
    
    
def receiveMessage(client):
    while True:
        try:
            msg = client.recv(2048).decode('utf-8')
            print(msg+'\n')
        except:
            print("Não foi possível permanecer concetado ao servidor!\n")
            print("Pressione Enter para continuar")
            client.close()
            break
def sendMessage(client, username):
    while True:
        try:
            msg = input("\n")
            client.send(f'{username} || {msg}'.encode('utf-8'))
        except:
            return """


main()