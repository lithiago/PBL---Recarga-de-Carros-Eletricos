import socket
from threading import Thread
import os

class Client:
    def __init__(self, HOST, PORT):
        self.socket = socket.socket()
        self.socket.connect((HOST, PORT))
        
        self.talk_to_server()
        
    def talk_to_server(self):
        Thread(target= self.receive_message).start()
        self.send_message()
        
    def send_message(self):
        while True:
            client_message = input("")
            # The encode function converts the string into bytes so we can send the bytes down th socket
            self.socket.send(client_message.encode())
            
    def receive_message(self):
        while True:
            # Receive a data from the socket. 1024 is the buffer size, the max amount of data to be received at once.
            # Returns a bytes object. A returned empy bytes object indicates that the client has disconnected.
            server_message = self.socket.recv(1024).decode()
            if(server_message.strip() ==  "bye" or not server_message.strip()):
                os._exit(0)
            # Add a red color to the cliente message
            print("\033[1;31;40m" + "Client: " + server_message + "\033[0m")
            
Client("127.0.0.1", 8080)