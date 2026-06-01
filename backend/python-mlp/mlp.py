import json
import pickle
from typing import TypedDict

import matplotlib.pyplot as plt
import numpy as np
from sentence_transformers import SentenceTransformer
from sklearn.model_selection import KFold
from sklearn.preprocessing import LabelEncoder


# Row major implementation of MLP
# For column major transpose the y_true and X
# Choosing which implementation is purely design choice
class MLP:
    def __init__(
        self,
        layers: list[int],
        learning_rate: float = 1.0,
        decay: float = 0.0,
        l1_l2_lambdas={},
    ):
        """Initialise a MLP neural net
        layers: len(layers) denote how many total layers (including input & output)
        each int number of layers denote the number of neurons for that layer

        learning_rate: the rate at which the network learns (used in gradient descent)
        """
        self.scaling_factor = 0.01
        self.layers = layers

        self.learning_rate = learning_rate
        self.current_learning_rate = learning_rate
        self.decay = decay
        self.iterations = 0
        self.l1_weight_lambda = l1_l2_lambdas["l1w"] if "l1w" in l1_l2_lambdas else 0
        self.l1_bias_lambda = l1_l2_lambdas["l1b"] if "l1b" in l1_l2_lambdas else 0
        self.l2_weight_lambda = l1_l2_lambdas["l2w"] if "l2w" in l1_l2_lambdas else 0
        self.l2_bias_lambda = l1_l2_lambdas["l2b"] if "l2b" in l1_l2_lambdas else 0
        self.weights = {}
        self.biases = {}
        self.activations = {}
        # Adam optimiser moment vectors
        self.mv1_weights = {}
        self.mv2_weights = {}
        self.mv1_biases = {}
        self.mv2_biases = {}
        self.beta1 = 0.9
        self.beta2 = 0.999
        self.epsilon = 1e-8
        self.time_step = 0
        self.z = {}

        for i in range(1, len(layers)):
            self.weights[i] = (
                np.random.randn(layers[i - 1], layers[i])
                * np.sqrt(2 / layers[i])
            )
            self.biases[i] = np.zeros((1, layers[i]))
            self.mv1_weights[i] = np.zeros_like(self.weights[i])
            self.mv2_weights[i] = np.zeros_like(self.weights[i])
            self.mv1_biases[i] = np.zeros_like(self.biases[i])
            self.mv2_biases[i] = np.zeros_like(self.biases[i])

    def set_parameters(self, data):
        self.layers = data["layer_sizes"]
        self.weights = {i + 1: np.array(w) for i, w in enumerate(data["weights"])}
        self.biases = {i + 1: np.array(b) for i, b in enumerate(data["biases"])}

    def sigmoid(self, x):
        x = np.clip(x, -500, 500)
        return 1 / (1 + np.exp(-x))

    def relu(self, x):
        return np.maximum(0, x)

    def relu_derivative(self, z):
        # derivative will be applied to the output, z is the sum of the weighted inputs and bias
        return z > 0

    def softmax(self, X):
        # subtract the max value to bound the exponentiation between 0 and 1
        # https://youtu.be/omz_NdFgWyU?feature=shared&t=1408
        exp_x = np.exp(X - np.max(X, axis=1, keepdims=True))
        return exp_x / np.sum(exp_x, axis=1, keepdims=True)

    def print_shapes(self):
        for i in range(1, len(self.layers)):
            textInner = "Input"
            textOuter = f"Hidden-{i}"
            if i > 1:
                textInner = f"Hidden-{i-1}"
            if i == len(self.layers) - 1:
                textOuter = "Output"
            if i in self.z:
                print(f"{textInner} -> {textOuter} Z:", self.z[i].shape)
            if i in self.activations:
                print(
                    f"{textInner} -> {textOuter} activations:",
                    self.activations[i].shape,
                )

    def feedforward(self, X):
        """Forward pass through the network
        x shape: (number_of_samples, input_size/features)
        number_of_samples is the total number of training data
        input_size is number of neurons
        """
        # For input layer, activation is the input vector provided
        self.activations[0] = X
        output_activated = X

        for i in range(1, len(self.layers)):
            W = self.weights[i]
            b = self.biases[i]

            # Output of the layer without activation (weighted sum + bias)
            Z = np.dot(output_activated, W) + b
            self.z[i] = Z

            # Apply softmax on output layer, ReLU on hidden layers
            output_activated = (
                self.softmax(Z) if i == len(self.layers) - 1 else self.relu(Z)
            )
            self.activations[i] = output_activated

        return output_activated

    def binary_cross_entropy_loss(self, y_pred, y_true):
        """This is useful for calculating loss for neural net that have binary classification
        For neural nets with multiple category classification, use category cross entropy loss
        y_pred: the predicted values, shape: (number_of_samples, output_size)
        y_true: the true labels, shape: (number_of_samples, output_size)
        """
        n = y_true.shape[0]
        # Clip to avoid log(0) = -inf
        y_pred_clipped = np.clip(y_pred, 1e-8, 1 - 1e-8)
        bce = -np.mean(
            y_true * np.log(y_pred_clipped) + (1 - y_true) * np.log(1 - y_pred_clipped)
        )
        return bce

    def compute_loss(self, y_pred, y_true):
        m = y_true.shape[0]
        loss = -np.sum(y_true * np.log(y_pred + 1e-8)) / m

        regularization_loss = 0
        for i in range(len(self.layers)):
            if self.l1_weight_lambda > 0:
                regularization_loss += self.l1_weight_lambda * np.sum(
                    np.abs(self.weights[i])
                )

            if self.l2_weight_lambda > 0:
                regularization_loss += self.l2_weight_lambda * np.sum(
                    self.weights[i] * self.weights[i]
                )

            if self.l1_bias_lambda > 0:
                regularization_loss += self.l1_bias_lambda * np.sum(
                    np.abs(self.biases[i])
                )

            if self.l2_bias_lambda > 0:
                regularization_loss += self.l2_bias_lambda * np.sum(
                    self.biases[i] * self.biases[i]
                )

        return loss + regularization_loss

    def backpropagate(self, X, y_pred, y_true):
        """Backward pass through the network
        X shape: (number_of_samples, input_size)
        """
        m = X.shape[0]
        weight_gradients = {}
        bias_gradients = {}

        # Derivative of softmax + cross entropy loss
        # See: https://www.python-unleashed.com/post/derivation-of-the-binary-cross-entropy-loss-gradient
        dA = y_pred - y_true

        # Find gradients for each layer moving backwards
        for i in reversed(range(1, len(self.layers))):
            if i == len(self.layers) - 1:
                # Output layer derivative is already computed above
                dZ = dA
            else:
                # Hidden layer: chain rule with activation function derivative
                dZ = dA * self.relu_derivative(self.z[i])

            # Gradient w.r.t. weights: previous layer activations transposed * current dZ
            dW = np.dot(self.activations[i - 1].T, dZ)
            db = np.mean(dZ, axis=0, keepdims=True)

            # Gradient w.r.t. previous layer activations (for propagating further back)
            dA = np.dot(dZ, self.weights[i].T)

            weight_gradients[i] = dW
            bias_gradients[i] = db

        return weight_gradients, bias_gradients

    def pre_update(self):
        if self.decay:
            self.current_learning_rate = self.learning_rate * (
                1.0 / (1.0 + self.decay * self.iterations)
            )

    def update_params(self, weight_gradients, bias_gradients):
        """Update using stochastic gradient descent"""
        for i in reversed(range(1, len(self.layers))):
            self.weights[i] += -self.current_learning_rate * weight_gradients[i]
            self.biases[i] += -self.current_learning_rate * bias_gradients[i]

    def adam_optimiser(self, weight_gradients, bias_gradients):
        for i in reversed(range(1, len(self.layers))):
            # L1 & L2 regularization
            if self.l1_weight_lambda > 0:
                weight_gradients[i] += self.l1_weight_lambda * np.sign(self.weights[i])
            if self.l2_weight_lambda > 0:
                weight_gradients[i] += 2 * self.l2_weight_lambda * self.weights[i]
            if self.l1_bias_lambda > 0:
                bias_gradients[i] += self.l1_bias_lambda * np.sign(self.biases[i])
            if self.l2_bias_lambda > 0:
                bias_gradients[i] += 2 * self.l2_bias_lambda * self.biases[i]

            # Momentum terms
            self.mv1_weights[i] = (
                self.beta1 * self.mv1_weights[i]
                + (1 - self.beta1) * weight_gradients[i]
            )
            self.mv1_biases[i] = (
                self.beta1 * self.mv1_biases[i] + (1 - self.beta1) * bias_gradients[i]
            )

            self.mv2_weights[i] = self.beta2 * self.mv2_weights[i] + (
                1 - self.beta2
            ) * (weight_gradients[i] ** 2)
            self.mv2_biases[i] = self.beta2 * self.mv2_biases[i] + (1 - self.beta2) * (
                bias_gradients[i] ** 2
            )

            # self. iteration is 0 at the first time
            mv1_weights_hat = self.mv1_weights[i] / (
                1 - self.beta1 ** (self.iterations + 1)
            )
            mv1_biases_hat = self.mv1_biases[i] / (
                1 - self.beta1 ** (self.iterations + 1)
            )

            mv2_weights_hat = self.mv2_weights[i] / (
                1 - self.beta2 ** (self.iterations + 1)
            )
            mv2_biases_hat = self.mv2_biases[i] / (
                1 - self.beta2 ** (self.iterations + 1)
            )

            mv2_weights_hat = np.maximum(mv2_weights_hat, 1e-8)
            mv2_biases_hat = np.maximum(mv2_biases_hat, 1e-8)

            self.weights[i] += (
                -self.current_learning_rate
                * mv1_weights_hat
                / (np.sqrt(mv2_weights_hat) + self.epsilon)
            )
            self.biases[i] += (
                -self.current_learning_rate
                * mv1_biases_hat
                / (np.sqrt(mv2_biases_hat) + self.epsilon)
            )

    def calculate_accuracy(self, y_pred, y_true):
        predictions = np.argmax(y_pred, axis=1)
        true_classes = np.argmax(y_true, axis=1)
        return np.mean(predictions == true_classes)

    def calculate_binary_accuracy(self, y_pred, y_true, threshold=0.5):
        # convert probabilities to binary predictions
        predictions = (y_pred >= threshold).astype(int)

        accuracy = np.mean(predictions == y_true)
        return accuracy

    def confusion_matrix(self, y_pred, y_true, num_classes):
        cm = np.zeros((num_classes, num_classes), dtype=int)
        for t, p in zip(y_true, y_pred):
            cm[t][p] += 1
        return cm

    def classification_metrics(self, cm):
        metrics = {}
        for cls in range(len(cm)):
            TP = cm[cls, cls]
            FP = cm[:, cls].sum() - TP
            FN = cm[cls, :].sum() - TP
            TN = cm.sum() - (TP + FP + FN)

            precision = TP / (TP + FP + 1e-8)
            recall = TP / (TP + FN + 1e-8)
            f1 = 2 * precision * recall / (precision + recall + 1e-8)
            accuracy = (TP + TN) / (TP + TN + FP + FN)

            metrics[cls] = {
                "precision": round(precision, 3),
                "recall": round(recall, 3),
                "f1_score": round(f1, 3),
                "accuracy": round(accuracy, 3),
            }

        return metrics

    def compute_metrics(self, cm):
        epsilon = 1e-8
        TP = np.diag(cm)
        FP = np.sum(cm, axis=0) - TP
        FN = np.sum(cm, axis=1) - TP

        precision = TP / (TP + FP + epsilon)
        recall = TP / (TP + FN + epsilon)
        f1 = 2 * precision * recall / (precision + recall + epsilon)

        accuracy = np.sum(TP) / np.sum(cm)

        return precision, recall, f1, accuracy

    def train(self, X, y_true, epochs=1000):
        self.losses = []
        for epoch in range(1, epochs + 1):
            y_pred = self.feedforward(X)
            loss = self.compute_loss(y_pred, y_true)
            self.losses.append(loss)
            accuracy = self.calculate_accuracy(y_pred, y_true)
            weight_gradients, bias_gradients = self.backpropagate(X, y_pred, y_true)
            self.pre_update()
            self.adam_optimiser(weight_gradients, bias_gradients)
            self.iterations += 1

            if epoch % 100 == 0:
                print(
                    f"epoch {epoch}, "
                    + f"acc: {accuracy*100:.3f}, "
                    + f"loss: {loss:.3f}, "
                    + f"lr: {self.current_learning_rate:.3f} "
                )
            if epoch == epochs - 1:
                plt.plot(self.losses)
                plt.title("Losses Curve")

    def predict(self, X):
        output = self.feedforward(X)
        predicted_indices = np.argmax(output, axis=1)
        return output, predicted_indices


class HyperParameters(TypedDict):
    hidden_layers: list[int]
    learning_rate: float
    decay: float
    l1_l2_lambdas: dict[str, float]
    epochs: int


class PennywiseMLP:
    def __init__(self, mlp_type, is_new=True, model="all-MiniLM-L6-v2", data_path=None):
        # load_model method will handle loading these params for existing model
        self.mlp_type = mlp_type
        self.model = SentenceTransformer(model, trust_remote_code=True)
        if is_new:
            (
                emails,
                labels,
                payee_labels,
                account_labels,
                dates,
                unique_accounts,
                amounts,
                unique_labels,
            ) = self.get_labels_and_email(mlp_type, data_path=data_path)
            self.account_to_index = {
                account: i for i, account in enumerate(unique_accounts)
            }
            # Email embeddings using SBERT + signed log-scaled amounts
            signed_logs = []
            for amount in amounts:
                signed_logs.append(np.sign(amount) * np.log1p(abs(amount)))

            self.min_signed_log = np.min(signed_logs)
            self.max_signed_log = np.max(signed_logs)

            inputs = []
            for i in range(len(emails)):
                input_vector = self._build_input_vector(
                    email_text=emails[i],
                    amount=amounts[i],
                    account=account_labels[i],
                    payee=payee_labels[i],
                )
                inputs.append(input_vector)

            self.X = np.array(inputs)
            self.Y = self.one_hot_encode_labels(labels)
            print("X shape", self.X.shape, "Y shape:", self.Y.shape)

    def one_hot_encode_account(self, account):
        vec = np.zeros(len(self.account_to_index))
        if account in self.account_to_index:
            vec[self.account_to_index[account]] = 1.0
        return vec

    def _build_input_vector(self, email_text, amount, account=None, payee=None):
        """Single source of truth for constructing MLP input vectors."""
        email_vec = self.model.encode(email_text)
        account_vec = self.one_hot_encode_account(account)
        signed_log = np.sign(amount) * np.log1p(abs(amount))
        amount_norm = (
            2
            * (signed_log - self.min_signed_log)
            / (self.max_signed_log - self.min_signed_log)
            - 1
        )

        if self.mlp_type == "payee" and account is not None:
            return np.concatenate([email_vec, [amount_norm], account_vec])
        elif self.mlp_type == "category" and payee is not None and account is not None:
            payee_vec = self.model.encode(payee)
            return np.concatenate([email_vec, payee_vec, [amount_norm], account_vec])
        elif self.mlp_type == "account":
            return np.concatenate([email_vec, [amount_norm]])
        else:
            raise ValueError(f"Invalid mlp_type '{self.mlp_type}' or missing required fields")

    def get_labels_and_email(self, label_type: str, data_path: str | None = None) -> tuple[
        list[str],
        list[str],
        list[str],
        list[str],
        list[str],
        list[str],
        list[float],
        int,
    ]:
        """
        Extract labels and email data from normalized JSON file.

        Args:
            label_type: The type of label to extract from the data
            data_path: Path to the training data JSON file (default: ./data/normalized_with_email.json)

        Returns:
            Tuple containing:
            - emails: List of email texts
            - labels: List of labels of the specified type
            - payee_labels: List of payee labels
            - account_labels: List of account labels
            - dates: List of dates
            - sorted_unique_accounts: Sorted list of unique accounts
            - amounts: List of amounts
            - unique_label_count: Number of unique labels
        """
        # Initialize lists to store extracted data
        emails = []
        labels = []
        payee_labels = []
        account_labels = []
        dates = []
        amounts = []

        # Sets to track unique values
        unique_labels = set()
        unique_accounts = set()

        # Load and process data
        resolved_path = data_path or "./data/normalized_with_email.json"
        try:
            with open(resolved_path, "r") as file:
                email_data = json.load(file)
        except FileNotFoundError:
            raise FileNotFoundError(f"{resolved_path} not found")
        except json.JSONDecodeError:
            raise ValueError(f"Invalid JSON format in {resolved_path}")

        # Process each record
        for data in email_data:
            # Skip records without email text
            if "email_text" not in data:
                continue

            # Extract label, defaulting to "null" if None
            label = data.get(label_type) or "null"

            # Append all required data
            emails.append(data["email_text"])
            labels.append(label)
            payee_labels.append(data.get("payee", ""))
            account_labels.append(data.get("account", ""))
            dates.append(data.get("date", ""))
            amounts.append(data.get("amount", 0.0))

            # Track unique values
            unique_labels.add(label)
            unique_accounts.add(data.get("account", ""))

        return (
            emails,
            labels,
            payee_labels,
            account_labels,
            dates,
            sorted(unique_accounts),
            amounts,
            len(unique_labels),
        )

    def one_hot_encode_labels(self, labels):
        if not hasattr(self, "label_encoder"):
            self.label_encoder: LabelEncoder = LabelEncoder()
        y_indices = self.label_encoder.fit_transform(labels)
        Y = np.eye(len(self.label_encoder.classes_))[y_indices].T  # type: ignore
        return Y.T

    def load_model(self, path):
        if path is None:
            raise Exception("No model path provided")
        try:
            with open(path, "rb") as f:
                data = pickle.load(f)
                self.mlp = MLP(layers=[])
                if "mlp" in data:
                    print(data["mlp"]["layer_sizes"])
                    self.mlp.set_parameters(data["mlp"])
                else:
                    raise Exception("mlp data not present in saved model")
                if "extras" in data:
                    self.account_to_index = data["extras"]["account_to_index"]
                    self.min_signed_log = data["extras"]["min_signed_log"]
                    self.max_signed_log = data["extras"]["max_signed_log"]
                    # Restore label encoder without re-fitting to preserve saved class mappings
                    if not hasattr(self, "label_encoder"):
                        self.label_encoder = LabelEncoder()
                    self.label_encoder.classes_ = np.array(data["extras"]["labels"])
                    self.Y = np.eye(len(self.label_encoder.classes_))
                else:
                    raise Exception("extra config not present in saved model")
        except FileNotFoundError:
            raise FileNotFoundError(f"{path} not found")

    def save_model(self, path):
        if path is None:
            raise Exception("No path provided to save model")
        model_data = {
            "mlp": {
                "weights": [w.tolist() for w in self.mlp.weights.values()],
                "biases": [b.tolist() for b in self.mlp.biases.values()],
                "layer_sizes": self.mlp.layers,
            },
            "extras": {
                "labels": (
                    self.label_encoder.classes_.tolist()
                    if isinstance(self.label_encoder.classes_, np.ndarray)
                    else None
                ),
                "account_to_index": self.account_to_index,
                "min_signed_log": self.min_signed_log,
                "max_signed_log": self.max_signed_log,
            },
        }
        with open(path, "wb") as f:
            pickle.dump(model_data, f)

    def k_fold_train_validate(
        self, hyper_parameters: list[HyperParameters], cv_folds=5
    ):
        """
        Train multiple hyper parameters using K-fold cross-validation and return the best one.
        Args:
            hyper_parameters: List of network hyper parameters to evaluate
            cv_folds: Number of cross-validation folds

        Returns:
        dict with best parameters and all results
        """
        kf = KFold(n_splits=cv_folds, shuffle=True, random_state=42)
        all_results = []

        print(
            f"Evaluating {len(hyper_parameters)} hyper parameters with {cv_folds}-fold cross-validation"
        )
        print("=" * 60)

        for param_idx, params in enumerate(hyper_parameters):
            print(f"\nParams {param_idx + 1}/{len(hyper_parameters)}:")
            print(f"  Hidden layers: {params['hidden_layers']}")
            print(f"  Learning rate: {params['learning_rate']}")
            print(f"  Decay: {params['decay']}")
            print(f"  L1/L2 lambdas: {params['l1_l2_lambdas']}")

            fold_accuracies = []

            mlp = None

            for fold_idx, (train_idx, val_idx) in enumerate(kf.split(self.X)):
                X_train, X_val = self.X[train_idx], self.X[val_idx]
                Y_train, Y_val = self.Y[train_idx], self.Y[val_idx]

                layers = [self.X.shape[1], *params["hidden_layers"], self.Y.shape[1]]
                mlp = MLP(
                    layers=layers,
                    learning_rate=params["learning_rate"],
                    decay=params["decay"],
                    l1_l2_lambdas=params["l1_l2_lambdas"],
                )
                mlp.train(X=X_train, y_true=Y_train, epochs=params["epochs"])
                preds, _ = mlp.predict(X_val)  # predict on validation inputs
                accuracy = mlp.calculate_accuracy(preds, Y_val)
                fold_accuracies.append(accuracy)

                print(f"    Fold {fold_idx + 1}: {accuracy*100:.4f}")

            mean_accuracy = np.mean(fold_accuracies)
            std_accuracy = np.std(fold_accuracies)

            param_result = {
                "param_index": param_idx,
                "params": params,
                "fold_accuracies": fold_accuracies,
                "mean_accuracy": mean_accuracy,
                "std_accuracy": std_accuracy,
                "mlp": mlp,
            }
            all_results.append(param_result)

        best_result = max(all_results, key=lambda x: x["mean_accuracy"])
        print("\n" + "=" * 60)
        print("RESULTS SUMMARY:")
        print("=" * 60)

        sorted_results = sorted(
            all_results, key=lambda x: x["mean_accuracy"], reverse=True
        )
        for i, result in enumerate(sorted_results):
            rank = i + 1
            param_idx = result["param_index"]
            mean_acc = result["mean_accuracy"]
            std_acc = result["std_accuracy"]
            print(f"{rank}. Param {param_idx + 1}: {mean_acc:.4f} ± {std_acc:.4f}")

        print(f"\nBEST CONFIGURATION (Config {best_result['param_index'] + 1}):")
        print(f"  Hidden layers: {best_result['params']['hidden_layers']}")
        print(f"  Learning rate: {best_result['params']['learning_rate']}")
        print(f"  Decay: {best_result['params']['decay']}")
        print(f"  L1/L2 lambdas: {best_result['params']['l1_l2_lambdas']}")
        print(
            f"  Mean accuracy: {best_result['mean_accuracy']:.4f} ± {best_result['std_accuracy']:.4f}"
        )

        return {
            "best_params": best_result["params"],
            "best_mlp": best_result["mlp"],
            "best_result": best_result,
            "all_results": all_results,
        }

    def train(self, hyper_parameters: HyperParameters):
        layers = [self.X.shape[1], *hyper_parameters["hidden_layers"], self.Y.shape[1]]
        self.mlp = MLP(
            layers=layers,
            learning_rate=hyper_parameters["learning_rate"],
            decay=hyper_parameters["decay"],
            l1_l2_lambdas=hyper_parameters["l1_l2_lambdas"],
        )
        self.mlp.train(self.X, self.Y, hyper_parameters["epochs"])

    def predict(self, mlp_type, email_text, amount, account=None, payee=None):
        input_vec = self._build_input_vector(
            email_text=email_text,
            amount=amount,
            account=account,
            payee=payee,
        )
        raw_output, predicted_indices = self.mlp.predict(input_vec)
        print(input_vec.shape, raw_output.shape, predicted_indices)
        confidences = np.max(raw_output, axis=1)
        predicted_label = self.label_encoder.inverse_transform(predicted_indices)
        return {
            "label": predicted_label[0],
            "confidence": float(f"{confidences[0]:.2f}"),
        }

    def test(self, test_data, key):
        print(":::::TEST:::::", self.Y.shape)
        X_test = []
        for data in test_data:
            input_vector = self._build_input_vector(
                email_text=data[key],
                amount=data["amount"],
                account=data.get("account"),
                payee=data.get("payee"),
            )
            X_test.append(input_vector)

        X_test = np.array(X_test)
        print(X_test.shape)
        raw_output, predicted_indices = self.mlp.predict(X_test)
        print("Predictions shape:", predicted_indices.shape)
        print("Predicted indices", predicted_indices)

        predicted_labels = self.label_encoder.inverse_transform(predicted_indices)
        print("Predicted labels", predicted_labels)

        print("raw_output:", raw_output.shape, predicted_indices.shape)

        confidences = np.max(raw_output, axis=1)

        confident_predictions = 0
        correct = 0
        for i, label in enumerate(predicted_labels):
            confidence = confidences[i] * 100
            if confidence >= 70:
                confident_predictions += 1
                if label == test_data[i][self.mlp_type]:
                    correct += 1
            print(
                f"Predicted {self.mlp_type}: {label} and Expected: {test_data[i][self.mlp_type]} with Confidence: {confidence:.2f}"
            )

        coverage = confident_predictions / len(test_data)
        accuracy = correct / len(test_data)
        print(f"Coverage: {coverage*100:.2f}%")
        print(f"High-confidence Accuracy:{accuracy*100:.2f}%")



PARAMS: dict[str, list[HyperParameters]] = {
    "payee": [
        {
            "hidden_layers": [1024],
            "learning_rate": 0.001,
            "decay": 0.0001,
            "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
            "epochs": 1000,
        },
        {
            "hidden_layers": [1536, 1024, 512],
            "learning_rate": 5e-4,
            "decay": 0.00001,
            "l1_l2_lambdas": {"l2w": 1e-5, "l2b": 1e-5},
            "epochs": 500,
        },
        {
            "hidden_layers": [1024, 1024, 512],
            "learning_rate": 5e-4,
            "decay": 0.001,
            "l1_l2_lambdas": {"l2w": 1e-5, "l2b": 1e-5},
            "epochs": 500,
        },
    ],
    "category": [
        {
            "hidden_layers": [1024, 512],
            "learning_rate": 0.01,
            "decay": 0.001,
            "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
            "epochs": 1000,
        },
        {
            "hidden_layers": [1536, 1024, 512, 256],
            "learning_rate": 5e-4,
            "decay": 0.0001,
            "l1_l2_lambdas": {"l2w": 1e-5, "l2b": 1e-5},
            "epochs": 1000,
        },
        {
            "hidden_layers": [1024, 1024, 512],
            "learning_rate": 5e-3,
            "decay": 0.0001,
            "l1_l2_lambdas": {"l2w": 1e-5, "l2b": 1e-5},
            "epochs": 500,
        },
    ],
    "account": [
        {
            "hidden_layers": [256],
            "learning_rate": 0.01,
            "decay": 0.001,
            "l1_l2_lambdas": {"l2w": 0.00005, "l2b": 0.00005},
            "epochs": 500,
        }
    ],
}

MODEL_PATHS: dict[str, str] = {
    "payee": "pennywise_payee_mlp.parms",
    "category": "pennywise_category_mlp.parms",
    "account": "pennywise_account_mlp.parms",
}

DEFAULT_MODELS: dict[str, str] = {
    "payee": "all-mpnet-base-v2",
    "category": "all-MiniLM-L6-v2",
    "account": "all-MiniLM-L6-v2",
}


def load_test_data(path="./data/test_data.json"):
    with open(path) as f:
        return json.load(f)


def run_train(mlp_type: str, params_index: int, model: str | None, save: bool):
    """Train a new model from scratch and optionally save it."""
    params_list = PARAMS[mlp_type]
    if params_index < 0 or params_index >= len(params_list):
        print(f"Invalid --params-index {params_index}. Available: 0-{len(params_list) - 1}")
        print("Configurations:")
        for i, p in enumerate(params_list):
            print(f"  [{i}] layers={p['hidden_layers']}, lr={p['learning_rate']}, epochs={p['epochs']}")
        return

    sbert_model = model or DEFAULT_MODELS[mlp_type]
    mlp = PennywiseMLP(mlp_type=mlp_type, is_new=True, model=sbert_model)
    mlp.train(hyper_parameters=params_list[params_index])

    test_data = load_test_data()
    mlp.test(test_data=test_data, key="email_text")

    if save:
        mlp.save_model(path=MODEL_PATHS[mlp_type])
        print(f"Model saved to {MODEL_PATHS[mlp_type]}")


def run_test(mlp_type: str, model: str | None):
    """Load an existing model and run test evaluation."""
    sbert_model = model or DEFAULT_MODELS[mlp_type]
    mlp = PennywiseMLP(mlp_type=mlp_type, is_new=False, model=sbert_model)
    mlp.load_model(path=MODEL_PATHS[mlp_type])

    test_data = load_test_data()
    mlp.test(test_data=test_data, key="email_text")


def run_predict(mlp_type: str, email_text: str, amount: float, model: str | None,
                account: str | None = None, payee: str | None = None):
    """Load an existing model and predict on a single input."""
    sbert_model = model or DEFAULT_MODELS[mlp_type]
    mlp = PennywiseMLP(mlp_type=mlp_type, is_new=False, model=sbert_model)
    mlp.load_model(path=MODEL_PATHS[mlp_type])

    prediction = mlp.predict(
        mlp_type=mlp_type,
        email_text=email_text,
        amount=amount,
        account=account,
        payee=payee,
    )
    print(prediction)


def run_kfold(mlp_type: str, model: str | None, folds: int):
    """Run K-fold cross-validation across all param configs for a type."""
    sbert_model = model or DEFAULT_MODELS[mlp_type]
    mlp = PennywiseMLP(mlp_type=mlp_type, is_new=True, model=sbert_model)
    results = mlp.k_fold_train_validate(hyper_parameters=PARAMS[mlp_type], cv_folds=folds)
    return results


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="Pennywise MLP training and prediction CLI")
    subparsers = parser.add_subparsers(dest="action", required=True)

    # Common args
    type_choices = ["payee", "category", "account"]

    # --- train ---
    train_parser = subparsers.add_parser("train", help="Train a new model from scratch")
    train_parser.add_argument("type", choices=type_choices)
    train_parser.add_argument("--params-index", type=int, default=0,
                              help="Index into the params list for this type (default: 0)")
    train_parser.add_argument("--model", type=str, default=None,
                              help="Sentence transformer model name (default: per-type default)")
    train_parser.add_argument("--save", action="store_true",
                              help="Save the trained model to disk")

    # --- test ---
    test_parser = subparsers.add_parser("test", help="Test an existing saved model")
    test_parser.add_argument("type", choices=type_choices)
    test_parser.add_argument("--model", type=str, default=None)

    # --- predict ---
    predict_parser = subparsers.add_parser("predict", help="Predict on a single email input")
    predict_parser.add_argument("type", choices=type_choices)
    predict_parser.add_argument("--email", type=str, required=True, help="Email text")
    predict_parser.add_argument("--amount", type=float, required=True, help="Transaction amount")
    predict_parser.add_argument("--account", type=str, default=None, help="Account name")
    predict_parser.add_argument("--payee", type=str, default=None, help="Payee name (for category prediction)")
    predict_parser.add_argument("--model", type=str, default=None)

    # --- kfold ---
    kfold_parser = subparsers.add_parser("kfold", help="Run K-fold cross-validation on all param configs")
    kfold_parser.add_argument("type", choices=type_choices)
    kfold_parser.add_argument("--folds", type=int, default=5, help="Number of CV folds (default: 5)")
    kfold_parser.add_argument("--model", type=str, default=None)

    args = parser.parse_args()

    if args.action == "train":
        run_train(args.type, args.params_index, args.model, args.save)
    elif args.action == "test":
        run_test(args.type, args.model)
    elif args.action == "predict":
        run_predict(args.type, args.email, args.amount, args.model, args.account, args.payee)
    elif args.action == "kfold":
        run_kfold(args.type, args.model, args.folds)
