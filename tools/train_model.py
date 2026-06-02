#!/usr/bin/env python3
"""Train a complexity classifier for Aperture's Tier 3 ML strategy.

Usage:
    python train_model.py --input labeled_data.csv --output model_weights.json

Input CSV format:
    text,complexity
    "Hello there!",trivial
    "Write a sorting function in Python",complex
    "Prove this theorem...",expert

Output: JSON file with vocabulary, weights, and bias for the Go ML strategy.
"""

import argparse
import csv
import json
import math
from collections import Counter

VOCAB = [
    # Code & generation
    "function", "class", "code", "write", "implement", "create", "generate",
    # Reasoning
    "explain", "analyze", "compare", "why", "how", "evaluate", "assess",
    # Math
    "prove", "proof", "theorem", "derive", "integral", "equation", "solve", "calculate",
    # Greetings
    "hello", "hi", "hey", "thanks", "bye", "help",
    # Debugging
    "fix", "debug", "refactor", "bug", "error", "test",
    # Architecture
    "review", "design", "architecture", "system", "api", "database",
    # Optimization
    "optimize", "performance", "clean", "improve",
    # Data/ML
    "data", "model", "train", "learn", "predict",
    # Types
    "define", "type", "interface", "struct", "module", "package",
    # Security/Legal
    "contract", "legal", "nda", "agreement", "compliance",
    "secure", "encrypt", "auth", "token", "password",
]

COMPLEXITY_MAP = {
    "trivial": 0,
    "simple": 1,
    "moderate": 2,
    "complex": 3,
    "expert": 4,
}


def extract_features(text: str) -> list[float]:
    """Extract word-count features from text."""
    text = text.lower()
    return [float(text.count(word)) for word in VOCAB]


def softmax(x: list[float]) -> list[float]:
    """Compute softmax probabilities."""
    max_x = max(x)
    exp = [math.exp(v - max_x) for v in x]
    total = sum(exp)
    return [v / total for v in exp]


def train(data: list[tuple[str, int]], lr: float = 0.01, epochs: int = 100):
    """Train a simple logistic regression model.

    Uses gradient descent with cross-entropy loss.
    Returns (weights, bias) where weights is [5][n_features] and bias is [5].
    """
    n_features = len(VOCAB)
    n_classes = 5

    # Initialize weights (small random values)
    import random
    random.seed(42)
    weights = [[random.uniform(-0.1, 0.1) for _ in range(n_features)] for _ in range(n_classes)]
    bias = [0.0] * n_classes

    for epoch in range(epochs):
        total_loss = 0.0
        for text, label in data:
            features = extract_features(text)

            # Forward pass
            logits = [bias[i] + sum(features[j] * weights[i][j] for j in range(n_features)) for i in range(n_classes)]
            probs = softmax(logits)

            # Cross-entropy loss
            loss = -math.log(max(probs[label], 1e-10))
            total_loss += loss

            # Gradient descent
            for i in range(n_classes):
                target = 1.0 if i == label else 0.0
                error = probs[i] - target
                bias[i] -= lr * error
                for j in range(n_features):
                    weights[i][j] -= lr * error * features[j]

        if epoch % 20 == 0:
            avg_loss = total_loss / len(data)
            print(f"Epoch {epoch}: loss={avg_loss:.4f}")

    return weights, bias


def main():
    parser = argparse.ArgumentParser(description="Train Aperture ML complexity classifier")
    parser.add_argument("--input", "-i", required=True, help="Input CSV file (text,complexity)")
    parser.add_argument("--output", "-o", default="model_weights.json", help="Output JSON file")
    parser.add_argument("--lr", type=float, default=0.01, help="Learning rate")
    parser.add_argument("--epochs", type=int, default=100, help="Training epochs")
    args = parser.parse_args()

    data = []
    with open(args.input, "r", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            text = row.get("text", row.get("prompt", ""))
            complexity = row.get("complexity", row.get("label", "moderate"))
            if complexity in COMPLEXITY_MAP:
                data.append((text, COMPLEXITY_MAP[complexity]))

    if len(data) < 10:
        print(f"Warning: only {len(data)} labeled examples. Need more data for reliable training.")

    print(f"Training on {len(data)} examples, {len(VOCAB)} features, {args.epochs} epochs")
    weights, bias = train(data, lr=args.lr, epochs=args.epochs)

    model = {
        "vocab": VOCAB,
        "weights": weights,
        "bias": bias,
    }

    with open(args.output, "w", encoding="utf-8") as f:
        json.dump(model, f, indent=2)

    print(f"Model saved to {args.output}")

    # Quick accuracy check
    correct = 0
    for text, label in data:
        features = extract_features(text)
        logits = [bias[i] + sum(features[j] * weights[i][j] for j in range(len(VOCAB))) for i in range(5)]
        probs = softmax(logits)
        predicted = max(range(5), key=lambda i: probs[i])
        if predicted == label:
            correct += 1

    accuracy = correct / len(data) * 100 if data else 0
    print(f"Training accuracy: {accuracy:.1f}%")


if __name__ == "__main__":
    main()
