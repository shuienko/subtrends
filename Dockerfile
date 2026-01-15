# Build stage
FROM python:3.12-slim AS builder

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir --user -r requirements.txt

# Runtime stage
FROM python:3.12-slim

WORKDIR /app

# Create non-root user for security
RUN groupadd -r subtrends && useradd -r -g subtrends subtrends

# Copy installed packages from builder
COPY --from=builder /root/.local /home/subtrends/.local

# Copy application code
COPY *.py ./

# Create data directory with proper permissions
RUN mkdir -p /app/data && chown -R subtrends:subtrends /app

# Switch to non-root user
USER subtrends

# Add local packages to PATH
ENV PATH=/home/subtrends/.local/bin:$PATH
ENV PYTHONUNBUFFERED=1

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD python -c "import sys; sys.exit(0)"

# Run the bot
CMD ["python", "main.py"]
