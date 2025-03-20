import socket

# Set the host and port
HOST = '0.0.0.0'  # Listen on all available interfaces
PORT = 65432       # Port to bind the server

# Create a TCP/IP socket
with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
    s.bind((HOST, PORT))
    s.listen()

    print(f"Server is listening on {HOST}:{PORT}...")

    conn, addr = s.accept()
    with conn:
        print(f"Connected by {addr}")

        # Receive the data from the client
        data = conn.recv(1024)
        if data:
            print(f"Received from client: {data.decode()}")

            # Send a response back to the client
            conn.sendall(b"Hello from server!")
