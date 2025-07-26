from timeit import timeit
import time
import json
import pickle
from datetime import datetime
from typing import TypedDict

import matplotlib.pyplot as plt
import numpy as np
from sentence_transformers import SentenceTransformer
from sklearn.metrics import PredictionErrorDisplay
from sklearn.preprocessing import LabelEncoder, StandardScaler
from sklearn.utils.multiclass import unique_labels
from sklearn.model_selection import KFold

# model = SentenceTransformer("all-MiniLM-L6-v2")
# model = SentenceTransformer("all-mpnet-base-v2")


# Row major implementation of MLP
# For column major transpose the y_true and X
# Choosing which implementation is purely design choice
class MLP:
    def __init__(
        self, layers: list[int], learning_rate: float = 1.0, decay: float = 0.0, l1_l2_lambdas = {}
    ):
        """Initialise a MLP neural net
        layers: len(layers) denote how many total layers (including input & output)
        each int number of layers denote the number of neurons for that layer

        learning_rate: the rate at which the network learns (used in gradient descent)
        """
        # for reproducibility
        # np.random.seed(42)

        self.scaling_factor = 0.01
        self.layers = layers
        # print("Inside __init__", layers, learning_rate)

        self.learning_rate = learning_rate
        self.current_learning_rate = learning_rate
        self.decay = decay
        self.iterations = 0
        self.l1_weight_lambda = l1_l2_lambdas["l1w"] if "l1w" in l1_l2_lambdas else 0
        self.l1_bias_lambda = l1_l2_lambdas["l1b"] if "l1b" in l1_l2_lambdas else 0
        self.l2_weight_lambda = l1_l2_lambdas["l2w"] if "l11" in l1_l2_lambdas else 0
        self.l2_bias_lambda = l1_l2_lambdas["l2b"] if "l1b" in l1_l2_lambdas else 0
        self.weights = {}
        self.biases = {}
        self.activations = {}
        # moment vectors
        self.mv1_weights = {}
        self.mv2_weights = {}
        self.mv1_biases = {}
        self.mv2_biases = {}
        self.beta1 = 0.9
        # self.beta2 = 0.999
        self.beta2 = 0.999
        self.epsilon = 1e-8
        self.time_step = 0
        self.z = {}

        for i in range(1, len(layers)):
            # print(f"layer-{i}", "neurons:", layers[i], "previous:", layers[i - 1])
            self.weights[i] = (
                np.random.randn(layers[i - 1], layers[i])
                * np.sqrt(2 / layers[i])
                # np.random.randn(layers[i], layers[i - 1]) * self.scaling_factor
            )
            self.biases[i] = np.zeros((1, layers[i]))
            self.mv1_weights[i] = np.zeros_like(self.weights[i])
            self.mv2_weights[i] = np.zeros_like(self.weights[i])
            self.mv1_biases[i] = np.zeros_like(self.biases[i])
            self.mv2_biases[i] = np.zeros_like(self.biases[i])
            # print(
            #     "Shapes ->",
            #     "Weights:",
            #     self.weights[i].shape,
            #     "Biases:",
            #     self.biases[i].shape,
            # )
        # print("Weights:", self.weights, "\nBiases:", self.biases)

    def set_parameters(self, data):
        self.layers = data["layer_sizes"]
        self.weights = {
            i+1: np.array(w) for i, w in enumerate(data["weights"])
        }
        self.biases = {
            i+1: np.array(b) for i, b in enumerate(data["biases"])
        }

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
        # since max value will be 0 and all other values will be negative, exponentiation of 0 is 1, so bounded values will be between 0 and 1
        # https://youtu.be/omz_NdFgWyU?feature=shared&t=1408

        # print("Using softmax for:", X)
        # axis=1 tells numpy to only max/sum along the row instead of the whole matrix, axis=0 is column wise
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

    def feedforward(self, X, isPredict=False):
        """Forward pass through the network
        x shape: (number_of_samples, input_size/features)
        number_of_samples is the total number of training data
        input_size is number of neurons
        """

        # print("feedforward", X, X.shape)
        # for input layer activation is the input vector provided
        self.activations[0] = X
        # output_activated will have the final output after softmax
        output_activated = X

        for i in range(1, len(self.layers)):
            # weight and bias matrices of current layer
            W = self.weights[i]
            b = self.biases[i]
            # print(W.shape, b.shape)

            Z = np.dot(output_activated, W) + b
            # output matrix of the layer without activation
            self.z[i] = Z

            # print(
            #     "dot product:",
            #     output,
            #     np.shape(output),
            # )
            output_activated = (
                self.softmax(Z) if i == len(self.layers) - 1 else self.relu(Z)
            )
            # output matrix of the layer after activation
            self.activations[i] = output_activated
        # if isPredict:
            # print("TEST Weights:", self.weights)
            # print("TEST Biases:", self.biases)
            # print("TEST Z:", self.z)
            # print("TEST Activations:", self.activations)

        return output_activated

    def binary_cross_entropy_loss(self, y_pred, y_true):
        """This is useful for calculating loss for neural net that have binary classification
        For neural nets with multiple category classification, use category cross entropy loss
        y_pred: the predicted values, shape: (number_of_samples, output_size)
        y_true: the true labels, shape: (number_of_samples, output_size)
        """
        # n is the total number of sample and since our y_true has the shape (number_of_samples, output_size) we take the first element of y_true.shape
        n = y_true.shape[0]
        # print("binary_cross_entropy_loss:", n)

        # clipping by 1e-8 to y_pred because if loss comes out to be 0 then log(0) = infinite causing average loss to be infinite
        y_pred_clipped = np.clip(y_pred, 1e-8, 1 - 1e-8)
        # axis=1 tells numpy to only sum along the row instead of the whole matrix, axis=0 is column wise
        # bce = -1/n * np.sum(y_true * np.log(y_pred_clipped) + (1 - y_true) * np.log(1 - y_pred_clipped), axis=1, keepdims=True)
        bce = -np.mean(
            y_true * np.log(y_pred_clipped) + (1 - y_true) * np.log(1 - y_pred_clipped)
        )
        # print(bce, bce_sum)
        return bce

    def compute_loss(self, y_pred, y_true):
        m = y_true.shape[0]
        # axis=1 tells numpy to only sum along the row instead of the whole matrix, axis=0 is column wise
        loss =  -np.sum(y_true * np.log(y_pred + 1e-8)) / m

        # regularization
        regularization_loss = 0
        for i in range(len(self.layers)):
            if self.l1_weight_lambda > 0:
                regularization_loss += self.l1_weight_lambda * np.sum(np.abs(self.weights[i]))

            if self.l2_weight_lambda > 0:
                regularization_loss += self.l2_weight_lambda * np.sum(self.weights[i] * self.weights[i])

            if self.l1_bias_lambda > 0:
                regularization_loss += self.l1_bias_lambda * np.sum(np.abs(self.biases[i]))

            if self.l2_bias_lambda > 0:
                regularization_loss += self.l1_bias_lambda * np.sum(self.biases[i] * self.biases[i])

        return loss + regularization_loss

    def backpropagate(self, X, y_pred, y_true):
        """Backward pass through the network
        X shape: (number_of_samples, input_size)
        y: the expected labels

        dA = derivative of the next layer
        dZ = derivative of the weighted sum
        dW = derivative of the
        """

        # print("Inside backpropagate:", "\nX:", X, "\nOutput:", y_pred, "\nY:", y_true)
        m = X.shape[0]
        weight_gradients = {}
        bias_gradients = {}

        # Derivative of sigmoid + cross entropy loss
        # See: https://www.python-unleashed.com/post/derivation-of-the-binary-cross-entropy-loss-gradient
        # This is the output layers derivate
        dA = y_pred - y_true

        # We need to find the gradients for each layer as we move backwards
        for i in reversed(range(1, len(self.layers))):
            # print("Layer weights & biases", i)
            if i == len(self.layers) - 1:
                # We already created the derivative of the output layer
                dZ = dA
            else:
                # derivative of other layers is the multiplication of derivative of activation function with the derivative of the next layer (dA)
                dZ = dA * self.relu_derivative(self.z[i])

            # Derivative of f(x) = x.y => y
            # Derivative of multiplication w.r.t. inputs of a single neuron is the input to that neuron from previous layer (which is the self.activations[i-1]) 
            #  multipled by the next layer derivative (dZ)
            # We are transposing to match the shape
            dW = np.dot(self.activations[i - 1].T, dZ)
            # @TODO: understand this
            db = np.mean(dZ, axis=0, keepdims=True)

            # We are calculating the derivative for the current layer now and updating it to dA to use in previous layers
            # This is the derivative of multiplication w.r.t. weights
            dA = np.dot(dZ, self.weights[i].T)

            weight_gradients[i] = dW
            bias_gradients[i] = db

        return weight_gradients, bias_gradients

    def pre_update(self):
        if self.decay:
            self.current_learning_rate = self.learning_rate * (1.0 / (1.0 + self.decay * self.iterations))

    def update_params(self, weight_gradients, bias_gradients):
        for i in reversed(range(1, len(self.layers))):
            # Update using the stochastic gradient descent
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
            self.mv1_weights[i] = (self.beta1 * self.mv1_weights[i] + (1 - self.beta1) * weight_gradients[i])
            self.mv1_biases[i] = (self.beta1 * self.mv1_biases[i] + (1 - self.beta1) * bias_gradients[i])

            self.mv2_weights[i] = self.beta2 * self.mv2_weights[i] + (1 - self.beta2) * (weight_gradients[i] ** 2)
            self.mv2_biases[i] = self.beta2 * self.mv2_biases[i] + (1 - self.beta2) * (bias_gradients[i] ** 2)

            # self. iteration is 0 at the first time
            mv1_weights_hat = self.mv1_weights[i] / (1 - self.beta1 ** (self.iterations + 1))
            mv1_biases_hat = self.mv1_biases[i] / (1 - self.beta1 ** (self.iterations + 1))

            mv2_weights_hat = self.mv2_weights[i] / (1 - self.beta2 ** (self.iterations + 1))
            mv2_biases_hat = self.mv2_biases[i] / (1 - self.beta2 ** (self.iterations + 1))

            mv2_weights_hat = np.maximum(mv2_weights_hat, 1e-8)
            mv2_biases_hat = np.maximum(mv2_biases_hat, 1e-8)

            self.weights[i] += -self.current_learning_rate * mv1_weights_hat / (np.sqrt(mv2_weights_hat) + self.epsilon)
            self.biases[i] += -self.current_learning_rate * mv1_biases_hat / (np.sqrt(mv2_biases_hat) + self.epsilon)

    def calculate_accuracy(self, y_pred, y_true):
        predictions = np.argmax(y_pred, axis=1)
        # print("Predictions", predictions, y_pred)
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
        # lowest_loss = 999999
        # self.best_weights = {}
        # self.best_biases = {}
        self.losses = []
        for epoch in range(1, epochs + 1):
            # loss = self.compute_loss(y_pred, y_true)

            # Very crude implementation to make the network learn
            # for i in range(1, len(self.layers)):
            #     self.weights[i] += self.learning_rate * (
            #         np.random.randn(self.layers[i - 1], self.layers[i])
            #     )
            #     self.biases[i] += self.learning_rate * np.random.randn(
            #         1, self.layers[i]
            #     )
            #
            # y_pred = self.feedforward(X)
            #
            # loss = self.binary_cross_entropy_loss(y_pred, y_true)
            # # loss = self.compute_loss(y_pred, y_true)
            # accuracy = self.calculate_binary_accuracy(y_pred, y_true)
            # # accuracy = self.calculate_accuracy(y_pred, y_true)
            #
            # if loss < lowest_loss:
            #     # print("binary_loss:", binary_loss, "accuracy:", accuracy)
            #     for i in range(1, len(self.layers)):
            #         self.best_weights[i] = self.weights[i].copy()
            #         self.best_biases[i] = self.biases[i].copy()
            #     lowest_loss = loss
            # else:
            #     for i in range(1, len(self.layers)):
            #         self.weights[i] = self.best_weights[i].copy()
            #         self.biases[i] = self.best_biases[i].copy()
            #
            # if epoch % 100 == 0:
            #     print(f"Epoch {epoch}, Loss: {loss:.4f}, Accuracy: {accuracy}")
            # if epoch == epochs:
            #     print(f"Epoch {epoch}, Loss: {loss:.4f}, Accuracy: {accuracy}")
            #     print("Weights:", self.weights)
            #     print("Biases:", self.biases)
            #     print("Z:", self.z)
            #     print("Activations:", self.activations)

            y_pred = self.feedforward(X)
            # loss = self.binary_cross_entropy_loss(y_pred, y_true)
            loss = self.compute_loss(y_pred, y_true)
            self.losses.append(loss)
            accuracy = self.calculate_accuracy(y_pred, y_true)
            weight_gradients, bias_gradients = self.backpropagate(X, y_pred, y_true)
            self.pre_update()
            # self.update_params(weight_gradients, bias_gradients)
            self.adam_optimiser(weight_gradients, bias_gradients)
            self.iterations += 1

            # print("Gradients:", weight_gradients, bias_gradients)
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
                # plt.show()

    def predict(self, X):
        output = self.feedforward(X)
        # print("Inside predict:", X.shape, output.shape)
        # print("Predicted:", output)
        # print(
        #     "Argmax1:",
        #     np.argmax(output, axis=1, keepdims=True),
        #     np.argmax(output, axis=1).shape,
        # )
        predicted_indices = np.argmax(output, axis=1)

        # return (output > 0.5).astype(int)
        return output, predicted_indices


# two neurons in input because for XOR we have two binary inputs
# neurons = [2, 4, 4, 1]
# X = np.array([[0, 0], [0, 1], [1, 0], [1, 1]])
# expected = np.array([[0], [1], [1], [0]])  # shape (4, 1)
# mlp = MLP(neurons, 0.1)
# mlp.print_shapes()
# mlp.train(X, expected, 5000)
# predictions = mlp.predict(X)
# print("Predictions:", predictions)

class HyperParameters(TypedDict):
    hidden_layers: list[int]
    learning_rate: float
    decay: float
    l1_l2_lambdas: dict[str, float]
    epochs: int

class PennywiseMLP:
    def __init__(self, type, is_new=True, model="all-MiniLM-L6-v2"):
        # Only initialise the labels and extra params when training a new model
        # load_model method will handle loading these params for existing model
        self.type = type
        self.model = SentenceTransformer(model)
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
            ) = self.get_labels_and_email(type)
            self.account_to_index = {
                account: i for i, account in enumerate(unique_accounts)
            }
            # Email embeddings using SBERT: shape (num_samples, 384)
            signed_logs = []
            for amount in amounts:
                signed_logs.append(np.sign(amount) * np.log1p(abs(amount)))

            self.min_signed_log = np.min(signed_logs)
            self.max_signed_log = np.max(signed_logs)

            inputs = []
            for i in range(len(emails)):
                # print("email:", emails[i])
                # print("label", labels[i])
                # print("account_label:", account_labels[i])
                # print("amount:", amounts[i])
                email_vector = self.model.encode(emails[i])
                payee_vector = self.model.encode(payee_labels[i])
                account_vector = self.one_hot_encode_account(account_labels[i])
                amount_normalized = (
                    2
                    * (signed_logs[i] - self.min_signed_log)
                    / (self.max_signed_log - self.min_signed_log)
                    - 1
                )
                transaction_type = "outflow"
                if amounts[i] > 0:
                    transaction_type = "inflow"
                if self.type == "payee":
                    input_vector = np.concatenate(
                        [email_vector, [amount_normalized], account_vector]
                    )
                    # input_vector = model.encode(f"Email text: {emails[i]}, Account: {account_labels[i]}, Amount: ₹{amounts[i]}")
                    # input_vector = model.encode(f"email={emails[i]},account={account_labels[i]},type={transaction_type}")
                elif self.type == "category":
                    input_vector = np.concatenate([email_vector, payee_vector, [amount_normalized], account_vector])
                    # input_vector = model.encode(f"Email text: {emails[i]}, Payee: {payee_labels[i]}, Account: {account_labels[i]}, Amount: ₹{amounts[i]}")
                    # input_vector = model.encode(f"email={emails[i]},payee={payee_labels[i]},account={account_labels[i]},type={transaction_type}")
                elif self.type == "account":
                    input_vector = np.concatenate(
                        [email_vector, [amount_normalized]]
                    )
                    # input_vector = model.encode(f"Email text: {emails[i]}, Amount: ₹{amounts[i]}")
                else:
                    raise Exception("Wrong type")
                # print("Amount:", amount_normalized)
                inputs.append(input_vector)
                # breakpoint += 1

            self.X = np.array(inputs)
            self.Y = self.one_hot_encode_labels(labels)
            print("X shape", self.X.shape, "Y shape:", self.Y.shape)

    def one_hot_encode_account(self, account):
        vec = np.zeros(len(self.account_to_index))
        if account in self.account_to_index:
            vec[self.account_to_index[account]] = 1.0
        return vec

    def get_labels_and_email(self, label_type: str) -> tuple[list[str], list[str], list[str], list[str], list[str], list[str], list[float], int]:
        """
        Extract labels and email data from normalized JSON file.

        Args:
            label_type: The type of label to extract from the data

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
        try:
            with open("./normalized_with_email.json", "r") as file:
                email_data = json.load(file)
        except FileNotFoundError:
            raise FileNotFoundError("normalized_with_email.json not found")
        except json.JSONDecodeError:
            raise ValueError("Invalid JSON format in normalized_with_email.json")

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
        Y = np.eye(len(self.label_encoder.classes_))[y_indices].T # type: ignore
        return Y.T

    def load_model(self, path="pennywise_mlp.parms"):
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
                    self.Y = self.one_hot_encode_labels(data["extras"]["labels"])
                else:
                    raise Exception("extra config not present in saved model")
        except FileNotFoundError:
            raise FileNotFoundError(f"{path} not found")

    def save_model(self, path="pennywise_mlp.parms"):
        model_data = {
            "mlp": {
                "weights": [w.tolist() for w in self.mlp.weights.values()],
                "biases": [b.tolist() for b in self.mlp.biases.values()],
                "layer_sizes": self.mlp.layers,
            },
            "extras": {
                "labels": self.label_encoder.classes_.tolist() if isinstance(self.label_encoder.classes_, np.ndarray) else None,
                "account_to_index": self.account_to_index,
                "min_signed_log": self.min_signed_log,
                "max_signed_log": self.max_signed_log,
            }
        }
        with open(path, "wb") as f:
            pickle.dump(model_data, f)

    def k_fold_train_validate(self, hyper_parameters: list[HyperParameters], cv_folds=5):
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

        # self.mlp = MLP(neurons, learning_rate, decay, l1_l2_lambdas=l1_l2_lambdas)
        print(f"Evaluating {len(hyper_parameters)} hyper parameters with {cv_folds}-fold cross-validation")
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
                    l1_l2_lambdas=params["l1_l2_lambdas"]
                )
                mlp.train(X=X_train, y_true=Y_train, epochs=params["epochs"])
                preds, _ = mlp.predict(X_val) # predict on validation inputs
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
                "mlp": mlp
            }
            all_results.append(param_result)

        best_result = max(all_results, key=lambda x: x["mean_accuracy"])
        print("\n" + "=" * 60)
        print("RESULTS SUMMARY:")
        print("=" * 60)

        sorted_results = sorted(all_results, key=lambda x: x["mean_accuracy"], reverse=True)
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
        print(f"  Mean accuracy: {best_result['mean_accuracy']:.4f} ± {best_result['std_accuracy']:.4f}")

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
            l1_l2_lambdas=hyper_parameters["l1_l2_lambdas"]
        )
        self.mlp.train(self.X, self.Y, hyper_parameters["epochs"])

    def predict(self, type, email_text, amount, account=None, payee=None):
        email_vec = self.model.encode(email_text)
        account_vec = self.one_hot_encode_account(account)
        amount_signed_log = np.sign(amount) * np.log1p(abs(amount))
        amount_norm = 2 * (amount_signed_log - self.min_signed_log) / (self.max_signed_log - self.min_signed_log) - 1
        if type == "payee" and account is not None:
            input_vec = np.concatenate([email_vec, [amount_norm], account_vec])
        elif type == "category" and payee is not None and account is not None:
            payee_vec = self.model.encode(payee)
            input_vec = np.concatenate([email_vec, payee_vec, [amount_norm], account_vec])
        elif type == "account":
            input_vec = np.concatenate([email_vec, [amount_norm]])
        else:
            print("Wrong MLP type")
            return
        raw_output, predicted_indices = self.mlp.predict(input_vec)
        print(input_vec.shape, raw_output.shape, predicted_indices)
        confidences = np.max(raw_output, axis=1)
        predicted_label = self.label_encoder.inverse_transform(predicted_indices)
        return {
            "label": predicted_label[0],
            "confidence": float(f"{confidences[0]:.2f}")
        }

    def test(self, test_data, key):
        print(":::::TEST:::::", self.Y.shape)
        X_test = []
        breakpoint = 0
        for data in test_data:
            if breakpoint > 1:
                break
            email_vec = self.model.encode(data[key])
            payee_vec = self.model.encode(data["payee"])
            account_vec = self.one_hot_encode_account(data["account"])
            signed_log = np.sign(data["amount"]) * np.log1p(abs(data["amount"]))
            amount_normalized = 2 * (signed_log - self.min_signed_log) / (self.max_signed_log - self.min_signed_log) - 1
            transaction_type = "outflow"
            if data["amount"] > 0:
                transaction_type = "inflow"

            if self.type == "payee":
                input_vector = np.concatenate([email_vec, [amount_normalized], account_vec])
                # input_vector = self.model.encode(f"Email text: {data[key]}, Account: {data["account"]}, Amount: ₹{data["amount"]}")
                # input_vector = self.model.encode(f"email={data[key]},account={data["account"]},type={transaction_type}")
            elif self.type == "category":
                input_vector = np.concatenate([email_vec, payee_vec, [amount_normalized], account_vec])
                # input_vector = self.model.encode(f"Email text: {data[key]}, Payee: {data["payee"]}, Account: {data["account"]}, Amount: ₹{data["amount"]}")
                # input_vector = self.model.encode(f"email={data[key]},payee={data["payee"]},account={data["account"]},type={transaction_type}")
            elif self.type == "account":
                input_vector = np.concatenate([email_vec, [amount_normalized]])
                # input_vector = self.model.encode(f"Email text: {data[key]}, Amount: ₹{data["amount"]}")
            else:
                raise Exception("Wrong type")
            X_test.append(input_vector)
            # breakpoint += 1

        X_test = np.array(X_test)
        print(X_test.shape)
        raw_output, predicted_indices = self.mlp.predict(X_test)
        print("Predictions shape:", predicted_indices.shape)
        print("Predicted indices", predicted_indices)

        predicted_labels = self.label_encoder.inverse_transform(predicted_indices)
        print("Predicted labels", predicted_labels)

        print("raw_output:", raw_output.shape, predicted_indices.shape)

        confidences = np.max(raw_output, axis=1)
        print("Confidences", confidences, confidences.shape)
        for i in range(raw_output.shape[0]):
            probs = raw_output[i]
            # print("probs:", probs)
            top_k = probs.argsort()[::-1][:3]
            # print("top_k:", top_k)
            # for idx in top_k:
            #     label = self.label_encoder.inverse_transform([idx])[0]
            #     print(f"{label}: Confidence {probs[idx]:.2f}")
            # print()
            # predicted_index = predicted_indices[i]
            # confidence = confidences[i]
            # print("Index:", predicted_index, "Confidence:", confidence)
            # predicted_label = self.label_encoder.inverse_transform([top_idx])[0]
            # print(f"Transaction {i}: {predicted_label} (confidence: {confidence:.2f})")

        predicted = 0
        confident_predictions = 0
        correct = 0
        for i, label in enumerate(predicted_labels):
            confidence = confidences[i] * 100
            if confidence >= 70:
                confident_predictions += 1
                if label == test_data[i][self.type]:
                    correct += 1
            print(
                f"Predicted {self.type}: {label} and Expected: {test_data[i][self.type]} with Confidence: {confidence:.2f}"
            )

        coverage = confident_predictions / len(test_data)
        accuracy = correct / len(test_data)
        # print(f"Accuracy {self.type}: {predicted/len(test_data)*100:.2f}")
        print(f"Coverage: {coverage*100:.2f}%")
        print(f"High-confidence Accuracy:{accuracy*100:.2f}%")

        # true_indices = np.argmax(self.Y, axis=1)
        # pred_indices = np.argmax(raw_output, axis=1)
        # cm = self.mlp.confusion_matrix(pred_indices, true_indices, num_classes=62)
        # print("Confusion matrix", cm)
        # metrics = self.mlp.classification_metrics(cm)
        # precision, recall, f1, accuracy = self.mlp.compute_metrics(cm)
        # print(metrics)
        # print(np.unique(pred_indices, return_counts=True))
        # print(f"Accuracy: {accuracy:.4f}")
        # print(f"Macro Precision: {np.mean(precision):.4f}")
        # print(f"Macro Recall: {np.mean(recall):.4f}")
        # print(f"Macro F1: {np.mean(f1):.4f}")


file = open("./test_data.json")
test_emails = json.load(file)
# file = open("./normalized_with_email.json")
# test_emails = json.load(file)

# payee_mlp = PennywiseMLP("payee", [128, 80, 40], learning_rate=0.00011)
# payee_mlp = PennywiseMLP("payee", [512, 256], learning_rate=0.01, decay=0.001, l1_l2_lambdas={"l2w": 0.0005, "l2b": 0.0005})
# payee_mlp.train(500)
# payee_mlp.test(test_emails, "email_text")

payee_params: list[HyperParameters] = [
    # {
    #     "hidden_layers": [512],
    #     "learning_rate": 0.01,
    #     "decay": 0.001,
    #     "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
    #     "epochs": 500,
    # },
    # {
    #     "hidden_layers": [512],
    #     "learning_rate": 0.001,
    #     "decay": 0.0001,
    #     "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
    #     "epochs": 1000,
    # },
    {
        "hidden_layers": [1024],
        "learning_rate": 0.001,
        "decay": 0.0001,
        "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
        "epochs": 1000,
    },
    # {
    #     "hidden_layers": [1024, 512],
    #     "learning_rate": 0.01,
    #     "decay": 0.001,
    #     "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
    #     "epochs": 500,
    # },
    # {
    #     "hidden_layers": [1024, 512, 256],
    #     "learning_rate": 0.01,
    #     "decay": 0.001,
    #     "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
    #     "epochs": 500,
    # },
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
    }

]

category_params: list[HyperParameters] = [
    # {
    #     "hidden_layers": [512, 256],
    #     "learning_rate": 0.01,
    #     "decay": 0.001,
    #     "l1_l2_lambdas": {"l2w": 0.00005, "l2b": 0.00005},
    #     "epochs": 1000,
    # },
    # {
    #     "hidden_layers": [512, 256],
    #     "learning_rate": 0.001,
    #     "decay": 0.0001,
    #     "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
    #     "epochs": 1000,
    # },
    {
        "hidden_layers": [1024, 512],
        "learning_rate": 0.01,
        "decay": 0.001,
        "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
        "epochs": 1000,
    },
    # {
    #     "hidden_layers": [1024, 512],
    #     "learning_rate": 0.001,
    #     "decay": 0.0001,
    #     "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
    #     "epochs": 500,
    # },
    # {
    #     "hidden_layers": [1024, 512, 256],
    #     "learning_rate": 0.01,
    #     "decay": 0.001,
    #     "l1_l2_lambdas": {"l2w": 0.0005, "l2b": 0.0005},
    #     "epochs": 1000,
    # },
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
    }
]

def predict_payee():
    payee_mlp = PennywiseMLP(type="payee", is_new=False, model="all-mpnet-base-v2")
    # payee_mlp.train(hyper_parameters=payee_params[2])
    # payee_mlp.save_model(path="pennywise_payee_mlp.parms")
    payee_mlp.load_model(path="pennywise_payee_mlp.parms")
    # prediction = payee_mlp.predict(
    #     type="payee",
    #     email_text="Dear Customer, Rs.5979.00 has been debited from account 8936 to VPA 0790187A0169637.bqr@kotak ALLIANCE AIR AVIATION LIMITED on 12-07-25.",
    #     amount=-5979,
    #     account="HDFC (Salary)"
    # )
    # prediction = payee_mlp.predict(
    #     type="payee",
    #     email_text="Dear Customer, Rs. 3000.00 is successfully credited to your account **8936 by VPA 9997167687@ybl RISHABH KAPRI S O GOKUL CHANDRA KAP on 12-07-25.",
    #     amount="3000",
    #     account="HDFC"
    # )
    # print(prediction)
    payee_mlp.test(test_data=test_emails, key="email_text")
    # results = payee_mlp.k_fold_train_validate(payee_params, cv_folds=5)

def predict_category():
    category_mlp = PennywiseMLP("category", is_new=False)
    category_mlp.load_model(path="pennywise_category_mlp.parms")
    # prediction = category_mlp.predict(
    #     type="category",
    #     payee="Petrol Pump",
    #     email_text="Dear Customer, Rs.1000.00 has been debited from account 8936 to VPA Q536830324@ybl OM DIESELS on 12-07-25.",
    #     amount=-1000,
    #     account="HDFC (Salary)"
    # )
    # print(prediction)
    category_mlp.test(test_data=test_emails, key="email_text")

def predict_account():
    account_mlp = PennywiseMLP("account", is_new=False)
    account_mlp.load_model(path="pennywise_account_mlp.parms")
    # prediction = account_mlp.predict(
    #     type="account",
    #     email_text="Dear Customer, Rs.15000.00 is successfully credited to your account **8936 by VPA 9458306660@ybl SHEELA KAPRI on 02-07-25.",
    #     amount=15000
    # )
    # @TODO: add training data for this
    prediction = account_mlp.predict(
        type="account",
        email_text="Dear Customer, Rs. 1.00 is successfully credited to your account **8936 by VPA 9997167687@ybl RISHABH KAPRI S O GOKUL CHANDRA KAP on 12-07-25.",
        amount=3000
    )
    print(prediction)


def train_payee():
    mlp = PennywiseMLP(type="payee", is_new=True, model="all-mpnet-base-v2")
    mlp.load_model(path="pennywise_payee_mlp.parms")
    # results = mlp.k_fold_train_validate(hyper_parameters=payee_params, cv_folds=5)
    # mlp.train(hyper_parameters=results["best_params"])
    # mlp.train(hyper_parameters=payee_params[0]) # best params with all-mpnet-base-v2
    mlp.test(test_data=test_emails, key="email_text")
    # mlp.save_model(path="pennywise_payee_mlp.parms")

def train_category():
    mlp = PennywiseMLP(type="category", is_new=True)
    mlp.load_model(path="pennywise_category_mlp.parms")
    # results = mlp.k_fold_train_validate(hyper_parameters=category_params, cv_folds=5)
    # mlp.train(hyper_parameters=results["best_params"])
    # mlp.train(hyper_parameters=category_params[1]) # best with all-mpnet-base-v2 and sbert + onehot
    mlp.test(test_data=test_emails, key="email_text")
    # mlp.save_model(path="pennywise_category_mlp.parms")

# predict_payee()
# predict_category()
# predict_account()
train_payee()
train_category()

# category_mlp = PennywiseMLP("category")
# category_mlp.k_fold_train_validate(hyper_parameters=category_params, cv_folds=5)
# category_mlp = PennywiseMLP("category", 0.0001, [256, 128, 64])
# category_mlp = PennywiseMLP("category", [1024, 512, 256], learning_rate=0.01, decay=0.001, l1_l2_lambdas={"l2w": 0.0005, "l2b": 0.0005})
# category_mlp.train(hyper_parameters=category_params[2])
# category_mlp.save_model(path="pennywise_category_mlp.parms")
# category_mlp.test(test_emails, "email_text")

account_params: list[HyperParameters] = [
    {
        "hidden_layers": [256],
        "learning_rate": 0.01,
        "decay": 0.001,
        "l1_l2_lambdas": {"l2w": 0.00005, "l2b": 0.00005},
        "epochs": 500,
    }
]
# account_mlp = PennywiseMLP("account", 0.0011, [128])
# account_mlp.train(1000)
# account_mlp.test(test_emails, "email_text")
# account_mlp = PennywiseMLP("account")
# account_mlp.train(hyper_parameters=account_params[0])
# account_mlp.save_model(path="pennywise_account_mlp.parms")
