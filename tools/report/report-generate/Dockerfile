FROM python:3.11-slim

LABEL org.opencontainers.image.title="report-generate"
LABEL org.opencontainers.image.description="Security report generator"
LABEL org.opencontainers.image.vendor="RavenForge"

WORKDIR /app

# Copy SDK
COPY sdk/libs/python /app/sdk

# Copy tool
COPY tools/report/report-generate/main.py /app/

# Create non-root user
RUN useradd -r -u 1000 ravenforge && \
    chown -R ravenforge:ravenforge /app
USER ravenforge

ENTRYPOINT ["python", "/app/main.py"]
