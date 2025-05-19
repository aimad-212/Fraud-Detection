# ML Training Container for Fraud Detection
FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    wget \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Install Python ML libraries
RUN pip install --no-cache-dir \
    pandas==2.1.3 \
    numpy==1.26.1 \
    scikit-learn==1.3.2 \
    lightgbm==4.1.0 \
    xgboost==2.0.0 \
    kaggle==1.6.3 \
    skl2onnx==1.16.0 \
    onnxmltools==1.12.0 \
    onnxruntime==1.16.3 \
    matplotlib==3.8.2 \
    seaborn==0.13.0 \
    jupyter==1.0.0 \
    notebook==7.0.6

# Create directories
RUN mkdir -p /app/data /app/models /app/scripts /app/notebooks

# Set working directory
WORKDIR /app

# Copy training scripts
COPY scripts/ ./scripts/

# Default command
CMD ["python", "--version"]