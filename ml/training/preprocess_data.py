#!/usr/bin/env python3
"""
Data Preprocessing Script for Fraud Detection

This script handles data cleaning, feature engineering, and preparation
for model training.

Usage:
    python preprocess_data.py --input INPUT_FILE --output OUTPUT_DIR
"""

import argparse
import os
import pandas as pd
import numpy as np
from sklearn.preprocessing import StandardScaler, LabelEncoder
from datetime import datetime
import warnings

warnings.filterwarnings("ignore")

def load_data(input_path: str) -> pd.DataFrame:
    """Load raw data"""
    print(f"Loading data from {input_path}")
    
    if input_path.endswith('.csv'):
        return pd.read_csv(input_path)
    elif input_path.endswith('.json'):
        return pd.read_json(input_path)
    else:
        raise ValueError("Unsupported file format. Use CSV or JSON.")

def clean_data(df: pd.DataFrame) -> pd.DataFrame:
    """Clean and validate data"""
    print("Cleaning data...")
    
    # Remove duplicates
    initial_count = len(df)
    df = df.drop_duplicates()
    print(f"Removed {initial_count - len(df)} duplicate rows")
    
    # Handle missing values
    numeric_columns = df.select_dtypes(include=[np.number]).columns
    df[numeric_columns] = df[numeric_columns].fillna(df[numeric_columns].median())
    
    categorical_columns = df.select_dtypes(include=['object']).columns
    for col in categorical_columns:
        df[col] = df[col].fillna(df[col].mode().iloc[0] if not df[col].mode().empty else 'unknown')
    
    # Remove outliers using IQR method for numeric columns
    for col in numeric_columns:
        if col in ['amount', 'transaction_amount']:  # Amount-related columns
            Q1 = df[col].quantile(0.25)
            Q3 = df[col].quantile(0.75)
            IQR = Q3 - Q1
            lower_bound = Q1 - 1.5 * IQR
            upper_bound = Q3 + 1.5 * IQR
            
            # Cap outliers instead of removing them
            df[col] = np.clip(df[col], lower_bound, upper_bound)
    
    return df

def engineer_features(df: pd.DataFrame) -> pd.DataFrame:
    """Create new features"""
    print("Engineering features...")
    
    # Time-based features
    if 'timestamp' in df.columns:
        df['timestamp'] = pd.to_datetime(df['timestamp'])
        df['hour'] = df['timestamp'].dt.hour
        df['day_of_week'] = df['timestamp'].dt.dayofweek
        df['month'] = df['timestamp'].dt.month
        df['is_weekend'] = df['day_of_week'].isin([5, 6]).astype(int)
        df['is_night'] = ((df['hour'] >= 22) | (df['hour'] <= 6)).astype(int)
    
    # Amount-based features
    amount_col = 'amount' if 'amount' in df.columns else 'transaction_amount'
    if amount_col in df.columns:
        df['amount_log'] = np.log1p(df[amount_col])
        df['amount_sqrt'] = np.sqrt(df[amount_col])
        
        # Amount ratios
        if 'avg_amount_24h' in df.columns:
            df['amount_vs_avg_ratio'] = df[amount_col] / (df['avg_amount_24h'] + 1)
    
    # Customer behavior features
    if 'customer_age' in df.columns and amount_col in df.columns:
        df['amount_per_age'] = df[amount_col] / (df['customer_age'] + 1)
    
    if 'customer_tenure' in df.columns:
        df['tenure_years'] = df['customer_tenure'] / 365.25
    
    # Transaction frequency features
    if 'transaction_count_24h' in df.columns:
        df['high_frequency'] = (df['transaction_count_24h'] > 10).astype(int)
    
    # Geographic features
    if 'distance_from_home' in df.columns:
        df['is_far_from_home'] = (df['distance_from_home'] > 100).astype(int)
        df['distance_log'] = np.log1p(df['distance_from_home'])
    
    # Merchant features
    if 'merchant_category' in df.columns:
        # Risk score by merchant category (example)
        high_risk_categories = [1, 5, 8]  # Example high-risk categories
        df['high_risk_merchant'] = df['merchant_category'].isin(high_risk_categories).astype(int)
    
    return df

def encode_categorical_features(df: pd.DataFrame) -> tuple:
    """Encode categorical variables"""
    print("Encoding categorical features...")
    
    categorical_columns = df.select_dtypes(include=['object']).columns
    label_encoders = {}
    
    for col in categorical_columns:
        le = LabelEncoder()
        df[col] = le.fit_transform(df[col].astype(str))
        label_encoders[col] = le
    
    return df, label_encoders

def scale_features(df: pd.DataFrame, target_column: str = None) -> tuple:
    """Scale numerical features"""
    print("Scaling features...")
    
    if target_column and target_column in df.columns:
        features = df.drop(columns=[target_column])
        target = df[target_column]
    else:
        features = df
        target = None
    
    numeric_columns = features.select_dtypes(include=[np.number]).columns
    scaler = StandardScaler()
    features_scaled = features.copy()
    features_scaled[numeric_columns] = scaler.fit_transform(features[numeric_columns])
    
    if target is not None:
        return features_scaled, target, scaler
    else:
        return features_scaled, scaler

def validate_processed_data(df: pd.DataFrame) -> bool:
    """Validate processed data"""
    print("Validating processed data...")
    
    # Check for missing values
    missing_values = df.isnull().sum().sum()
    if missing_values > 0:
        print(f"Warning: {missing_values} missing values found")
        return False
    
    # Check for infinite values
    infinite_values = np.isinf(df.select_dtypes(include=[np.number])).sum().sum()
    if infinite_values > 0:
        print(f"Warning: {infinite_values} infinite values found")
        return False
    
    # Check data types
    if df.select_dtypes(include=['object']).shape[1] > 0:
        print("Warning: Categorical variables found but not encoded")
        return False
    
    print("Data validation passed")
    return True

def save_processed_data(df: pd.DataFrame, output_dir: str, label_encoders=None, scaler=None):
    """Save processed data and preprocessing objects"""
    print(f"Saving processed data to {output_dir}")
    
    os.makedirs(output_dir, exist_ok=True)
    
    # Save processed data
    output_path = os.path.join(output_dir, 'processed_data.csv')
    df.to_csv(output_path, index=False)
    
    # Save preprocessing objects
    if label_encoders:
        import pickle
        le_path = os.path.join(output_dir, 'label_encoders.pkl')
        with open(le_path, 'wb') as f:
            pickle.dump(label_encoders, f)
    
    if scaler:
        import pickle
        scaler_path = os.path.join(output_dir, 'scaler.pkl')
        with open(scaler_path, 'wb') as f:
            pickle.dump(scaler, f)
    
    # Save data info
    info = {
        'processed_date': datetime.now().isoformat(),
        'shape': df.shape,
        'columns': list(df.columns),
        'dtypes': df.dtypes.to_dict(),
        'missing_values': df.isnull().sum().to_dict()
    }
    
    import json
    info_path = os.path.join(output_dir, 'data_info.json')
    with open(info_path, 'w') as f:
        json.dump(info, f, indent=2, default=str)
    
    print(f"Processed data saved to {output_path}")
    print(f"Data shape: {df.shape}")

def main():
    parser = argparse.ArgumentParser(description='Preprocess fraud detection data')
    parser.add_argument('--input', type=str, required=True, help='Input data file path')
    parser.add_argument('--output', type=str, required=True, help='Output directory')
    parser.add_argument('--target', type=str, help='Target column name')
    
    args = parser.parse_args()
    
    # Load and process data
    df = load_data(args.input)
    print(f"Initial data shape: {df.shape}")
    
    # Clean data
    df = clean_data(df)
    print(f"After cleaning: {df.shape}")
    
    # Feature engineering
    df = engineer_features(df)
    print(f"After feature engineering: {df.shape}")
    
    # Encode categorical features
    df, label_encoders = encode_categorical_features(df)
    
    # Scale features
    if args.target and args.target in df.columns:
        features, target, scaler = scale_features(df, args.target)
        processed_df = pd.concat([features, target], axis=1)
    else:
        processed_df, scaler = scale_features(df)
    
    # Validate data
    if not validate_processed_data(processed_df):
        print("Data validation failed!")
        return 1
    
    # Save processed data
    save_processed_data(processed_df, args.output, label_encoders, scaler)
    
    print("Data preprocessing completed successfully!")
    return 0

if __name__ == "__main__":
    exit(main())