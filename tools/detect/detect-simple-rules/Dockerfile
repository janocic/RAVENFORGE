FROM python:3.11-slim

LABEL org.opencontainers.image.title="detect-simple-rules"
LABEL org.opencontainers.image.description="Rule-based event detection engine"
LABEL org.opencontainers.image.vendor="RavenForge"

WORKDIR /app

# Install PyYAML for custom rules
RUN pip install --no-cache-dir pyyaml

# Copy SDK
COPY sdk/libs/python /app/sdk

# Copy tool
COPY tools/detect/detect-simple-rules/main.py /app/

# Create non-root user
RUN useradd -r -u 1000 ravenforge && \
    chown -R ravenforge:ravenforge /app
USER ravenforge

ENTRYPOINT ["python", "/app/main.py"]
