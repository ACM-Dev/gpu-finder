FROM python:3.12-slim

RUN pip install --no-cache-dir "boto3[crt]" textual rich

WORKDIR /app
COPY gpu_capacity_finder.py /app/gpu_capacity_finder.py

# Output files land here — mount a host dir to persist them
VOLUME ["/app/output"]

ENTRYPOINT ["python3", "/app/gpu_capacity_finder.py"]
