#!/usr/bin/env python3
"""
Model Evaluation Script for Fraud Detection

This script evaluates trained models and generates detailed performance reports.

Usage:
    python evaluate_model.py --model-path MODEL_PATH --test-data TEST_DATA_PATH
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
import matplotlib.pyplot as plt
import seaborn as sns
from sklearn.metrics import (accuracy_score, classification_report,
                             confusion_matrix, f1_score, precision_score,
                             recall_score, roc_auc_score, roc_curve,
                             precision_recall_curve)
import xgboost as xgb

warnings.filterwarnings("ignore")

def load_model(model_path: str):
    """Load trained model and preprocessing objects"""
    print(f"Loading model from {model_path}")
    
    with open(model_path, 'rb') as f:
        model_data = pickle.load(f)
    
    return model_data

def load_test_data(test_data_path: str):
    """Load test data"""
    print(f"Loading test data from {test_data_path}")
    
    if test_data_path.endswith('.csv'):
        df = pd.read_csv(test_data_path)
    elif test_data_path.endswith('.pkl'):
        with open(test_data_path, 'rb') as f:
            data = pickle.load(f)
        if isinstance(data, dict):
            return data['X_test'], data['y_test']
        else:
            return data
    else:
        raise ValueError("Unsupported test data format")
    
    # Assume target column is the last one
    target_column = df.columns[-1]
    X = df.iloc[:, :-1]
    y = df.iloc[:, -1]
    
    return X, y

def evaluate_model_performance(model, X_test, y_test, scaler=None):
    """Evaluate model and return metrics"""
    print("Evaluating model performance...")
    
    # Scale test data if scaler is provided
    if scaler:
        X_test = scaler.transform(X_test)
    
    # Make predictions
    y_pred = model.predict(X_test)
    y_pred_proba = model.predict_proba(X_test)[:, 1]
    
    # Calculate metrics
    metrics = {
        'accuracy': accuracy_score(y_test, y_pred),
        'precision': precision_score(y_test, y_pred),
        'recall': recall_score(y_test, y_pred),
        'f1_score': f1_score(y_test, y_pred),
        'roc_auc': roc_auc_score(y_test, y_pred_proba),
        'confusion_matrix': confusion_matrix(y_test, y_pred).tolist(),
        'classification_report': classification_report(y_test, y_pred, output_dict=True)
    }
    
    # Calculate ROC curve points
    fpr, tpr, _ = roc_curve(y_test, y_pred_proba)
    roc_curve_data = {
        'fpr': fpr.tolist(),
        'tpr': tpr.tolist()
    }
    
    # Calculate Precision-Recall curve points
    precision, recall, _ = precision_recall_curve(y_test, y_pred_proba)
    pr_curve_data = {
        'precision': precision.tolist(),
        'recall': recall.tolist()
    }
    
    return metrics, roc_curve_data, pr_curve_data, y_pred, y_pred_proba

def generate_confusion_matrix_plot(y_true, y_pred, output_path: str):
    """Generate and save confusion matrix plot"""
    plt.figure(figsize=(8, 6))
    cm = confusion_matrix(y_true, y_pred)
    
    sns.heatmap(cm, annot=True, fmt='d', cmap='Blues',
                xticklabels=['Not Fraud', 'Fraud'],
                yticklabels=['Not Fraud', 'Fraud'])
    plt.title('Confusion Matrix')
    plt.xlabel('Predicted')
    plt.ylabel('Actual')
    plt.tight_layout()
    plt.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close()

def generate_roc_curve_plot(roc_data, auc_score, output_path: str):
    """Generate and save ROC curve plot"""
    plt.figure(figsize=(8, 6))
    plt.plot(roc_data['fpr'], roc_data['tpr'], 
             color='darkorange', lw=2,
             label=f'ROC curve (AUC = {auc_score:.4f})')
    plt.plot([0, 1], [0, 1], color='navy', lw=2, linestyle='--')
    plt.xlim([0.0, 1.0])
    plt.ylim([0.0, 1.05])
    plt.xlabel('False Positive Rate')
    plt.ylabel('True Positive Rate')
    plt.title('Receiver Operating Characteristic (ROC) Curve')
    plt.legend(loc="lower right")
    plt.grid(True)
    plt.tight_layout()
    plt.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close()

def generate_precision_recall_curve_plot(pr_data, output_path: str):
    """Generate and save Precision-Recall curve plot"""
    plt.figure(figsize=(8, 6))
    plt.plot(pr_data['recall'], pr_data['precision'], 
             color='blue', lw=2, label='Precision-Recall curve')
    plt.xlim([0.0, 1.0])
    plt.ylim([0.0, 1.05])
    plt.xlabel('Recall')
    plt.ylabel('Precision')
    plt.title('Precision-Recall Curve')
    plt.legend(loc="lower left")
    plt.grid(True)
    plt.tight_layout()
    plt.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close()

def generate_feature_importance_plot(model, feature_names, output_path: str):
    """Generate and save feature importance plot"""
    if hasattr(model, 'feature_importances_'):
        importances = model.feature_importances_
        indices = np.argsort(importances)[::-1]
        
        plt.figure(figsize=(12, 8))
        plt.title('Feature Importance')
        plt.bar(range(len(importances)), importances[indices])
        plt.xticks(range(len(importances)), 
                   [feature_names[i] for i in indices], 
                   rotation=45, ha='right')
        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()
        
        return {
            'feature_names': [feature_names[i] for i in indices],
            'importances': importances[indices].tolist()
        }
    else:
        return None

def save_evaluation_report(metrics, roc_data, pr_data, feature_importance, 
                         output_dir: str, model_name: str):
    """Save comprehensive evaluation report"""
    print(f"Saving evaluation report to {output_dir}")
    
    os.makedirs(output_dir, exist_ok=True)
    
    # Create comprehensive report
    report = {
        'model_name': model_name,
        'evaluation_date': datetime.now().isoformat(),
        'performance_metrics': metrics,
        'roc_curve': roc_data,
        'precision_recall_curve': pr_data,
        'feature_importance': feature_importance,
        'summary': {
            'total_samples': len(metrics['classification_report']['0.0']) if '0.0' in metrics['classification_report'] else 'unknown',
            'fraud_cases_detected': int(metrics['confusion_matrix'][1][1]),
            'false_positives': int(metrics['confusion_matrix'][0][1]),
            'false_negatives': int(metrics['confusion_matrix'][1][0]),
            'true_negatives': int(metrics['confusion_matrix'][0][0]),
            'detection_rate': metrics['recall'],
            'false_positive_rate': metrics['confusion_matrix'][0][1] / (metrics['confusion_matrix'][0][0] + metrics['confusion_matrix'][0][1]) if (metrics['confusion_matrix'][0][0] + metrics['confusion_matrix'][0][1]) > 0 else 0
        }
    }
    
    # Save JSON report
    report_path = os.path.join(output_dir, f'{model_name}_evaluation_report.json')
    with open(report_path, 'w') as f:
        json.dump(report, f, indent=2)
    
    # Save text summary
    summary_path = os.path.join(output_dir, f'{model_name}_summary.txt')
    with open(summary_path, 'w') as f:
        f.write(f"Model Evaluation Report - {model_name}\n")
        f.write(f"Evaluation Date: {report['evaluation_date']}\n\n")
        
        f.write("Performance Metrics:\n")
        f.write(f"  Accuracy: {metrics['accuracy']:.4f}\n")
        f.write(f"  Precision: {metrics['precision']:.4f}\n")
        f.write(f"  Recall: {metrics['recall']:.4f}\n")
        f.write(f"  F1-Score: {metrics['f1_score']:.4f}\n")
        f.write(f"  ROC-AUC: {metrics['roc_auc']:.4f}\n\n")
        
        f.write("Confusion Matrix:\n")
        f.write(f"  True Negatives: {metrics['confusion_matrix'][0][0]}\n")
        f.write(f"  False Positives: {metrics['confusion_matrix'][0][1]}\n")
        f.write(f"  False Negatives: {metrics['confusion_matrix'][1][0]}\n")
        f.write(f"  True Positives: {metrics['confusion_matrix'][1][1]}\n\n")
        
        f.write("Summary:\n")
        f.write(f"  Detection Rate: {report['summary']['detection_rate']:.4f}\n")
        f.write(f"  False Positive Rate: {report['summary']['false_positive_rate']:.4f}\n")
        f.write(f"  Fraud Cases Detected: {report['summary']['fraud_cases_detected']}\n")
    
    print(f"Evaluation report saved to {report_path}")
    print(f"Summary saved to {summary_path}")

def main():
    parser = argparse.ArgumentParser(description='Evaluate fraud detection model')
    parser.add_argument('--model-path', type=str, required=True,
                       help='Path to trained model file')
    parser.add_argument('--test-data', type=str, required=True,
                       help='Path to test data')
    parser.add_argument('--output-dir', type=str, default='./evaluation_results',
                       help='Output directory for evaluation results')
    parser.add_argument('--model-name', type=str, default='fraud_model',
                       help='Name for the model in reports')
    
    args = parser.parse_args()
    
    # Load model and test data
    model_data = load_model(args.model_path)
    
    if isinstance(model_data, dict) and 'model' in model_data:
        model = model_data['model']
        scaler = model_data.get('scaler')
        feature_columns = model_data.get('feature_columns')
    else:
        model = model_data
        scaler = None
        feature_columns = None
    
    X_test, y_test = load_test_data(args.test_data)
    
    # If feature columns are specified, ensure we use only those
    if feature_columns:
        if isinstance(X_test, pd.DataFrame):
            X_test = X_test[feature_columns]
        elif hasattr(X_test, 'shape') and len(feature_columns) == X_test.shape[1]:
            pass  # Already correct shape
        else:
            print("Warning: Feature columns specified but couldn't be matched to test data")
    
    # Evaluate model
    metrics, roc_data, pr_data, y_pred, y_pred_proba = evaluate_model_performance(
        model, X_test, y_test, scaler
    )
    
    # Generate plots
    os.makedirs(args.output_dir, exist_ok=True)
    
    confusion_matrix_path = os.path.join(args.output_dir, f'{args.model_name}_confusion_matrix.png')
    generate_confusion_matrix_plot(y_test, y_pred, confusion_matrix_path)
    
    roc_curve_path = os.path.join(args.output_dir, f'{args.model_name}_roc_curve.png')
    generate_roc_curve_plot(roc_data, metrics['roc_auc'], roc_curve_path)
    
    pr_curve_path = os.path.join(args.output_dir, f'{args.model_name}_pr_curve.png')
    generate_precision_recall_curve_plot(pr_data, pr_curve_path)
    
    # Feature importance
    feature_importance = None
    if feature_columns:
        feature_importance_path = os.path.join(args.output_dir, f'{args.model_name}_feature_importance.png')
        feature_importance = generate_feature_importance_plot(model, feature_columns, feature_importance_path)
    
    # Save evaluation report
    save_evaluation_report(metrics, roc_data, pr_data, feature_importance,
                         args.output_dir, args.model_name)
    
    # Print summary
    print("\n=== Evaluation Summary ===")
    print(f"Accuracy: {metrics['accuracy']:.4f}")
    print(f"Precision: {metrics['precision']:.4f}")
    print(f"Recall: {metrics['recall']:.4f}")
    print(f"F1-Score: {metrics['f1_score']:.4f}")
    print(f"ROC-AUC: {metrics['roc_auc']:.4f}")
    print(f"\nConfusion Matrix:")
    print(f"TN: {metrics['confusion_matrix'][0][0]}, FP: {metrics['confusion_matrix'][0][1]}")
    print(f"FN: {metrics['confusion_matrix'][1][0]}, TP: {metrics['confusion_matrix'][1][1]}")
    
    print(f"\nEvaluation completed! Results saved to {args.output_dir}")

if __name__ == "__main__":
    main()