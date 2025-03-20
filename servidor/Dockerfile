# Use the official Python image from Docker Hub
FROM python:3.9-slim

# Set the working directory
WORKDIR /app

# Copy the server code into the container
COPY servidor.py /app/

# Expose port 65432 to allow the client to connect
EXPOSE 65432

# Command to run the server
CMD ["python", "servidor.py"]
