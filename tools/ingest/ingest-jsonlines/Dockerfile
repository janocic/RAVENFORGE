FROM python:3.11-slim

WORKDIR /app

# Copy SDK
COPY sdk/libs/python/rfsdk.py /app/sdk/rfsdk.py

# Copy tool
COPY tools/ingest/ingest-jsonlines/main.py /app/main.py

# No additional dependencies needed

ENTRYPOINT ["python", "/app/main.py"]
