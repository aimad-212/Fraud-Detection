#!/usr/bin/env python3
"""
Fraud Detection Model Training Script

This script trains a fraud detection model using XGBoost and saves it in multiple formats:
1. Pickle format for Python inference
2. ONNX format for cross-platform inference

Usage:
    python train_fraud_model.py [--data-path DATA_PATH] [--output-dir OUTPUT_DIR]
"""

import argparse
import json
import os
import pickle
import sys
import warnings
from datetime import datetime
from pathlib import Path

import numpy as np
import pandas as pd
import xgboost as xgb
from sklearn.metrics import (accuracy_score, classification_report,
                             confusion_matrix, f1_score, precision_score,
                             recall_score, roc_auc_score)
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import StandardScaler

# Suppress warnings
warnings.filterwarnings("ignore")

def load_data(data_path: str = None) -> pd.DataFrame:
    """Load fraud detection dataset"""
    if data_path and os.path.exists(data_path):
        print(f"Loading data from {data_path}")
        return pd.read_csv(data_path)
    
    # Generate synthetic data if no data file provided
    print("Generating synthetic fraud detection data...")
    np.random.seed(42)
    n_samples = 10000
    
    # Generate features
    data = {
        'amount': np.random.exponential(100, n_samples),
        'hour': np.random.randint(0, 24, n_samples),
        'day_of_week': np.random.randint(0, 7, n_samples),
        'merchant_category': np.random.randint(1, 10, n_samples),
        'customer_age': np.random.randint(18, 80, n_samples),
        'customer_tenure': np.random.randint(0, 365, n_samples),
        'transaction_count_24h': np.random.randint(0, 50, n_samples),
        'avg_amount_24h': np.random.exponential(80, n_samples),
        'distance_from_home': np.random.exponential(50, n_samples),
        'is_foreign': np.random.choice([0, 1], n_samples, p=[0.9, 0.1]),
        'is_online': np.random.choice([0, 1], n_samples, p=[0.3, 0.7]),
    }
    
    df = pd.DataFrame(data)
    
    # Generate fraud labels with some logic
    fraud_prob = (
        (df['amount'] > 500) * 0.3 +
        (df['hour'] < 6) * 0.2 +
        (df['is_foreign'] == 1) * 0.4 +
        (df['distance_from_home'] > 100) * 0.3 +
        (df['transaction_count_24h'] > 20) * 0.2
    )
    fraud_prob = np.clip(fraud_prob, 0, 1)
    df['is_fraud'] = np.random.random(n_samples) < fraud_prob
    
    print(f"Generated {len(df)} samples with {df['is_fraud'].sum()} fraud cases")
    return df

def preprocess_data(df: pd.DataFrame) -> tuple:
    """Preprocess the data for training"""
    print("Preprocessing data...")
    
    # Feature engineering
    df['amount_log'] = np.log1p(df['amount'])
    df['amount_hour_ratio'] = df['amount'] / (df['hour'] + 1)
    df['age_amount_ratio'] = df['amount'] / (df['customer_age'] + 1)
    
    # Select features for training
    feature_columns = [
        'amount', 'amount_log', 'hour', 'day_of_week', 'merchant_category',
        'customer_age', 'customer_tenure', 'transaction_count_24h',
        'avg_amount_24h', 'distance_from_home', 'is_foreign', 'is_online',
        'amount_hour_ratio', 'age_amount_ratio'
    ]
    
    X = df[feature_columns].copy()
    y = df['is_fraud'].copy()
    
    # Handle missing values
    X = X.fillna(X.median())
    
    # Scale features
    scaler = StandardScaler()
    X_scaled = scaler.fit_transform(X)
    
    return X_scaled, y, scaler, feature_columns

def train_model(X_train, y_train, X_val, y_val) -> xgb.XGBClassifier:
    """Train XGBoost model"""
    print("Training XGBoost model...")
    
    # Calculate scale_pos_weight for imbalanced dataset
    scale_pos_weight = (len(y_train) - y_train.sum()) / y_train.sum()
    
    model = xgb.XGBClassifier(
        n_estimators=100,
        max_depth=6,
        learning_rate=0.1,
        subsample=0.8,
        colsample_bytree=0.8,
        random_state=42,
        scale_pos_weight=scale_pos_weight,
        eval_metric='auc'
    )
    
    model.fit(
        X_train, y_train,
        eval_set=[(X_val, y_val)],
        verbose=False
    )
    
    return model

def evaluate_model(model, X_test, y_test) -> dict:
    """Evaluate model performance"""
    print("Evaluating model...")
    
    y_pred = model.predict(X_test)
    y_pred_proba = model.predict_proba(X_test)[:, 1]
    
    metrics = {
        'accuracy': accuracy_score(y_test, y_pred),
        'precision': precision_score(y_test, y_pred),
        'recall': recall_score(y_test, y_pred),
        'f1_score': f1_score(y_test, y_pred),
        'roc_auc': roc_auc_score(y_test, y_pred_proba),
        'confusion_matrix': confusion_matrix(y_test, y_pred).tolist(),
        'classification_report': classification_report(y_test, y_pred, output_dict=True)
    }
    
    return metrics

def save_model(model, scaler, feature_columns, metrics, output_dir: str):
    """Save model and metadata"""
    print(f"Saving model to {output_dir}...")
    
    os.makedirs(output_dir, exist_ok=True)
    
    # Save as pickle
    pickle_path = os.path.join(output_dir, 'fraud_model.pkl')
    with open(pickle_path, 'wb') as f:
        pickle.dump({
            'model': model,
            'scaler': scaler,
            'feature_columns': feature_columns
        }, f)
    
    # Save as ONNX
    try:
        import onnx
        from onnxmltools import convert_xgboost
        from skl2onnx import convert_sklearn
        
        # Convert to ONNX
        onnx_model = convert_xgboost(
            model, initial_types=[('input', 'float32', [None, len(feature_columns)])]
        )
        
        onnx_path = os.path.join(output_dir, 'fraud_model.onnx')
        with open(onnx_path, 'wb') as f:
            f.write(onnx_model.SerializeToString())
        
        print(f"Model saved as ONNX: {onnx_path}")
    except ImportError:
        print("ONNX conversion skipped (onnxmltools not installed)")
    
    # Save metadata
    metadata = {
        'model_name': 'fraud-detection-xgb',
        'version': '1.0.0',
        'training_date': datetime.now().isoformat(),
        'algorithm': 'xgboost',
        'features': len(feature_columns),
        'feature_columns': feature_columns,
        'metrics': metrics,
        'training_data': 'synthetic' if not os.path.exists('data/raw/creditcard.csv') else 'creditcardfraud'
    }
    
    metadata_path = os.path.join(output_dir, 'fraud-detection-xgb-1.0.0-metadata.json')
    with open(metadata_path, 'w') as f:
        json.dump(metadata, f, indent=2)
    
    print(f"Model saved as pickle: {pickle_path}")
    print(f"Metadata saved: {metadata_path}")

def main():
    parser = argparse.ArgumentParser(description='Train fraud detection model')
    parser.add_argument('--data-path', type=str, help='Path to training data CSV')
    parser.add_argument('--output-dir', type=str, default='./models', 
                       help='Output directory for model files')
    parser.add_argument('--test-size', type=float, default=0.2, 
                       help='Test set proportion')
    
    args = parser.parse_args()
    
    # Load and preprocess data
    df = load_data(args.data_path)
    X, y, scaler, feature_columns = preprocess_data(df)
    
    # Split data
    X_train, X_temp, y_train, y_temp = train_test_split(
        X, y, test_size=args.test_size * 2, random_state=42, stratify=y
    )
    X_val, X_test, y_val, y_test = train_test_split(
        X_temp, y_temp, test_size=0.5, random_state=42, stratify=y_temp
    )
    
    print(f"Training set: {X_train.shape}")
    print(f"Validation set: {X_val.shape}")
    print(f"Test set: {X_test.shape}")
    print(f"Fraud cases - Train: {y_train.sum()}, Val: {y_val.sum()}, Test: {y_test.sum()}")
    
    # Train model
    model = train_model(X_train, y_train, X_val, y_val)
    
    # Evaluate model
    metrics = evaluate_model(model, X_test, y_test)
    
    # Print metrics
    print("\n=== Model Performance ===")
    print(f"Accuracy: {metrics['accuracy']:.4f}")
    print(f"Precision: {metrics['precision']:.4f}")
    print(f"Recall: {metrics['recall']:.4f}")
    print(f"F1-Score: {metrics['f1_score']:.4f}")
    print(f"ROC-AUC: {metrics['roc_auc']:.4f}")
    
    # Save model
    output_dir = os.path.abspath(args.output_dir)
    save_model(model, scaler, feature_columns, metrics, output_dir)
    
    print("\nTraining completed successfully!")

if __name__ == "__main__":
    main()