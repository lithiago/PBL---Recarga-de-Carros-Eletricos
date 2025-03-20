import socket
import time

# Set the host and port of the server
HOST = 'servidor'  # This will resolve to the 'server' container's name in Docker
PORT = 65432      # Port to connect to

# Create a TCP/IP socket
with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
    connected = False
    while not connected:
        try:
            s.connect((HOST, PORT))  # Try to connect to the server
            connected = True
        except socket.error as e:
            print(f"Connection failed: {e}. Retrying...")
            time.sleep(2)  # Wait before retrying

    # Send a message to the server
    s.sendall(b"Hello from the client!")

    # Receive a response from the server
    data = s.recv(1024)
    print(f"Received from server: {data.decode()}")
