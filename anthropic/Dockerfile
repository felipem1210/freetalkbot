FROM python:3.12-alpine

# Copy the requirements.txt file
COPY requirements.txt .

# Install any needed packages specified in requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

# Set the working directory
WORKDIR /app

# Copy the current directory contents into the container at /app
COPY . .

# Make port 8088 available to the world outside this container
EXPOSE 8088

# Run app.py when the container launches
CMD ["python", "app.py"]