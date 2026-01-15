FROM python:3.11-slim

LABEL org.opencontainers.image.title="enrich-geoip"
LABEL org.opencontainers.image.description="GeoIP enrichment for IP addresses"
LABEL org.opencontainers.image.vendor="RavenForge"

WORKDIR /app

# Install maxminddb for GeoIP database support
RUN pip install --no-cache-dir maxminddb

# Copy SDK
COPY sdk/libs/python /app/sdk

# Copy tool
COPY tools/enrich/enrich-geoip/main.py /app/

# Create non-root user
RUN useradd -r -u 1000 ravenforge && \
    chown -R ravenforge:ravenforge /app
USER ravenforge

ENTRYPOINT ["python", "/app/main.py"]
