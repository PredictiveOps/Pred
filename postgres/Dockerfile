# PostgreSQL 18 Dockerfile for multi-tenant predictive maintenance system
FROM postgres:18-alpine

# Set environment variables for Postgres
ENV POSTGRES_INITDB_ARGS="-c shared_preload_libraries=pg_stat_statements" \
    POSTGRES_HOST_AUTH_METHOD=md5

# Copy initialization scripts
COPY ./init /docker-entrypoint-initdb.d/

# Set permissions on init scripts
RUN chmod +x /docker-entrypoint-initdb.d/*.sh

# Create a non-root user for running PostgreSQL (optional, postgres user already exists in image)
# The official postgres image already runs as the postgres user for security

# Expose PostgreSQL port
EXPOSE 5432

# Health check to verify PostgreSQL is ready
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
    CMD pg_isready -U postgres

# Run PostgreSQL
CMD ["postgres"]
