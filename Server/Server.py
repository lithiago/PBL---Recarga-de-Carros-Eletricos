import socket
from threading import Thread
import os

class Server:
    
    def __init__(self, HOST, PORT):
        # Create a new socket. AF_INET is the address family for IPv4
        # SOCK_STREAM is the socket type for TCP.
        self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.socket.bind((HOST, PORT))
        
        # Enable a server to accept connections
        self.socket.listen()
        print("SERVIDOR ESPERANDO CONEXÃO")
        # Accept a connection. Returns (connm address). Conn is a new
        # socket object used to send and receive data on the connection.
        # Address is the address of the other connection.
        client_socket, address = self.socket.accept()
        print(f"Connection from: {str(address)}")

        self.talk_to_client(client_socket)
        
    def talk_to_client(self, client_socket):
        # Create a thread and start the thread's activity
        Thread(target= self.receive_message, args= (client_socket,)).start()
        self.send_message(client_socket)
    
    def send_message(self, client_socket):
        while True:
            server_message = input("")
            # The encode function converts the string into bytes so we can send the bytes down th socket
            client_socket.send(server_message.encode())
            
    def receive_message(self, client_socket):
        while True:
            # Receive a data from the socket. 1024 is the buffer size, the max amount of data to be received at once.
            # Returns a bytes object. A returned empy bytes object indicates that the client has disconnected.
            client_message = client_socket.recv(1024).decode()
            if(client_message.strip() ==  "bye" or not client_message.strip()):
                os._exit(0)
            # Add a red color to the cliente message
            print("\033[1;31;40m" + "Server: " + client_message + "\033[0m")
            
Server("127.0.0.1", 8080)