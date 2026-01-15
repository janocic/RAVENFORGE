FROM python:3.11-slim

LABEL org.opencontainers.image.title="triage-prioritize"
LABEL org.opencontainers.image.description="Incident triage and prioritization"
LABEL org.opencontainers.image.vendor="RavenForge"

WORKDIR /app

# Copy SDK
COPY sdk/libs/python /app/sdk

# Copy tool
COPY tools/triage/triage-prioritize/main.py /app/

# Create non-root user
RUN useradd -r -u 1000 ravenforge && \
    chown -R ravenforge:ravenforge /app
USER ravenforge

ENTRYPOINT ["python", "/app/main.py"]
