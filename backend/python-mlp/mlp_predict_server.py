import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse
import shutil
import threading
import uuid as uuid_mod
from datetime import datetime

import numpy as np
import os

from mlp import PennywiseMLP
from prepare_training_data import fetch_predictions, predictions_to_training_data, save_json
import utils

# Use /data volume on Railway (persistent), fall back to local paths for dev
VOLUME_DIR = os.environ.get("VOLUME_DIR", "/data" if os.path.isdir("/data") else ".")
DEFAULT_DATA_PATH = os.path.join(VOLUME_DIR, "data", "normalized_with_email.json")

HOST = "0.0.0.0"
PORT = int(os.environ.get("PORT", 8000))
BACKUPS_DIR = os.path.join(VOLUME_DIR, "backups")

# Model type -> (parms file, embedding model)
MODEL_CONFIG = {
    "payee": {
        "path": os.path.join(VOLUME_DIR, "pennywise_payee_mlp.parms"),
        "embedding_model": "all-mpnet-base-v2",
    },
    "category": {
        "path": os.path.join(VOLUME_DIR, "pennywise_category_mlp.parms"),
        "embedding_model": "all-MiniLM-L6-v2",
    },
    "account": {
        "path": os.path.join(VOLUME_DIR, "pennywise_account_mlp.parms"),
        "embedding_model": "all-MiniLM-L6-v2",
    },
}

DEFAULT_HYPERPARAMS = {
    "payee": {
        "hidden_layers": [1024],
        "learning_rate": 5e-4,
        "decay": 1e-4,
        "l1_l2_lambdas": {},
        "epochs": 500,
    },
    "category": {
        "hidden_layers": [1024, 512],
        "learning_rate": 5e-3,
        "decay": 1e-4,
        "l1_l2_lambdas": {},
        "epochs": 500,
    },
    "account": {
        "hidden_layers": [256],
        "learning_rate": 0.01,
        "decay": 0.001,
        "l1_l2_lambdas": {},
        "epochs": 500,
    },
}

# In-memory job tracker for background training
retrain_jobs: dict[str, dict] = {}


def backup_model(model_type: str) -> str | None:
    """Back up the current .parms file. Returns the backup path or None."""
    config = MODEL_CONFIG[model_type]
    src = config["path"]
    if not os.path.exists(src):
        return None
    os.makedirs(BACKUPS_DIR, exist_ok=True)
    timestamp = datetime.now().strftime("%Y%m%dT%H%M%S")
    backup_path = os.path.join(BACKUPS_DIR, f"{model_type}_{timestamp}.parms")
    shutil.copy2(src, backup_path)
    return backup_path


def run_retrain(job_id: str, types: list[str], data_path: str, hyperparams_override: dict | None):
    """Background training function. Updates retrain_jobs with progress."""
    job = retrain_jobs[job_id]
    job["status"] = "running"
    results = {}

    for model_type in types:
        job["current_type"] = model_type
        try:
            config = MODEL_CONFIG[model_type]
            hp = (hyperparams_override or {}).get(model_type, DEFAULT_HYPERPARAMS[model_type])

            backup_path = backup_model(model_type)
            if backup_path:
                print(f"[retrain] Backed up {config['path']} -> {backup_path}")

            print(f"[retrain] Training {model_type} model...")
            mlp = PennywiseMLP(
                model_type,
                is_new=True,
                model=config["embedding_model"],
                data_path=data_path,
            )
            mlp.train(hp)
            mlp.save_model(config["path"])
            print(f"[retrain] Saved {model_type} model to {config['path']}")

            results[model_type] = {
                "status": "success",
                "backup": backup_path,
                "samples": int(mlp.X.shape[0]),
                "classes": int(mlp.Y.shape[1]),
            }
        except Exception as e:
            print(f"[retrain] Error training {model_type}: {e}")
            results[model_type] = {"status": "error", "error": str(e)}

    job["status"] = "completed"
    job["current_type"] = None
    job["results"] = results
    job["completed_at"] = datetime.now().isoformat()


class MLPHandler(BaseHTTPRequestHandler):

    def log_message(self, format, *args):
        """Write request logs to stdout instead of stderr so Railway doesn't flag them as errors."""
        print(f"{self.address_string()} - - [{self.log_date_time_string()}] {format % args}")

    def _read_body(self):
        content_length = int(self.headers["Content-Length"])
        return json.loads(self.rfile.read(content_length).decode("utf-8"))

    def _json_response(self, status, data):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())

    def do_POST(self):
        parsed_path = urlparse(self.path)

        if parsed_path.path == "/predict":
            try:
                data = self._read_body()
                print(data)
                type = data.get("type")
                email_text = data.get("email_text")
                amount = data.get("amount")
                account = data.get("account")
                payee = data.get("payee")

                if type not in MODEL_CONFIG:
                    raise Exception("Wrong type")

                config = MODEL_CONFIG[type]
                print("inside predict", config["path"])
                mlp = PennywiseMLP(type, is_new=False, model=config["embedding_model"])
                mlp.load_model(path=config["path"])
                prediction = mlp.predict(
                    type=type,
                    email_text=email_text,
                    amount=amount,
                    account=account,
                    payee=payee,
                )
                print(prediction)
                self._json_response(200, prediction)

            except Exception as e:
                print(str(e))
                self._json_response(400, {"error": str(e)})

        elif parsed_path.path == "/retrain":
            try:
                data = self._read_body()
                types = data.get("types", ["payee", "category", "account"])
                data_path = data.get("data_path", DEFAULT_DATA_PATH)
                hyperparams = data.get("hyperparameters")

                # Validate types
                invalid = [t for t in types if t not in MODEL_CONFIG]
                if invalid:
                    raise Exception(f"Invalid model types: {invalid}")

                if not os.path.exists(data_path):
                    raise Exception(f"Data file not found: {data_path}")

                # Start background training
                job_id = str(uuid_mod.uuid4())[:8]
                retrain_jobs[job_id] = {
                    "status": "queued",
                    "types": types,
                    "data_path": data_path,
                    "current_type": None,
                    "results": {},
                    "started_at": datetime.now().isoformat(),
                    "completed_at": None,
                }

                thread = threading.Thread(
                    target=run_retrain,
                    args=(job_id, types, data_path, hyperparams),
                    daemon=True,
                )
                thread.start()

                self._json_response(202, {
                    "job_id": job_id,
                    "status": "queued",
                    "types": types,
                    "message": f"Retraining started. Poll GET /retrain/{job_id} for status.",
                })

            except Exception as e:
                print(str(e))
                self._json_response(400, {"error": str(e)})

        elif parsed_path.path == "/rollback":
            try:
                data = self._read_body()
                model_type = data.get("type")
                backup_file = data.get("backup_file")

                if model_type not in MODEL_CONFIG:
                    raise Exception(f"Invalid type: {model_type}")

                backup_path = os.path.join(BACKUPS_DIR, backup_file)
                if not os.path.exists(backup_path):
                    raise Exception(f"Backup not found: {backup_file}")

                # Sanity check: backup file should match the model type
                if not backup_file.startswith(model_type + "_"):
                    raise Exception(f"Backup file {backup_file} doesn't match type {model_type}")

                dest = MODEL_CONFIG[model_type]["path"]
                shutil.copy2(backup_path, dest)
                self._json_response(200, {
                    "message": f"Rolled back {model_type} to {backup_file}",
                    "model_path": dest,
                })

            except Exception as e:
                print(str(e))
                self._json_response(400, {"error": str(e)})

        elif parsed_path.path == "/fetch":
            try:
                data = self._read_body()
                api_url = data.get("api_url")
                budget_id = data.get("budget_id")

                if not api_url or not budget_id:
                    raise Exception("api_url and budget_id are required")

                output_path = data.get("output", DEFAULT_DATA_PATH)

                print(f"[fetch] Fetching predictions from {api_url} (budget: {budget_id})...")
                predictions = fetch_predictions(api_url, budget_id)
                print(f"[fetch] Received {len(predictions)} predictions")

                training_data = predictions_to_training_data(predictions)
                print(f"[fetch] Converted {len(training_data)} records")

                save_json(training_data, output_path)

                # Correction stats
                total = len(predictions)
                corrected = sum(1 for p in predictions if p.get("hasUserCorrected"))
                uncorrected = total - corrected

                self._json_response(200, {
                    "message": f"Fetched and saved {len(training_data)} training records to {output_path}",
                    "total_predictions": total,
                    "correct_predictions": uncorrected,
                    "user_corrected": corrected,
                    "training_records": len(training_data),
                    "output_path": output_path,
                })

            except Exception as e:
                print(f"[fetch] Error: {e}")
                self._json_response(400, {"error": str(e)})

        elif parsed_path.path == "/embeddings":
            try:
                data = self._read_body()
                content = data.get("content")
                embedding = utils.create_embedding(content)
                self._json_response(200, embedding)
            except Exception as e:
                self._json_response(400, {"error": str(e)})
        else:
            self.send_response(404)
            self.end_headers()

    def do_GET(self):
        parsed_path = urlparse(self.path)

        if parsed_path.path == "/health":
            self._json_response(200, {"status": "healthy"})

        elif parsed_path.path.startswith("/retrain/"):
            job_id = parsed_path.path.split("/retrain/")[1]
            job = retrain_jobs.get(job_id)
            if not job:
                self._json_response(404, {"error": f"Job {job_id} not found"})
            else:
                self._json_response(200, job)

        elif parsed_path.path == "/backups":
            if not os.path.exists(BACKUPS_DIR):
                self._json_response(200, {"backups": {}})
                return

            backups: dict[str, list[str]] = {}
            for f in sorted(os.listdir(BACKUPS_DIR)):
                if f.endswith(".parms"):
                    model_type = f.rsplit("_", 1)[0]
                    backups.setdefault(model_type, []).append(f)
            self._json_response(200, {"backups": backups})

        else:
            self.send_response(404)
            self.end_headers()


if __name__ == "__main__":
    server = HTTPServer((HOST, PORT), MLPHandler)
    print("Server running on http://" + HOST + ":" + str(PORT))
    print("Endpoints:")
    print("  POST /predict      - Predict payee/category/account")
    print("  POST /fetch        - Fetch predictions from API and save as training data")
    print("  POST /retrain      - Start retraining (background)")
    print("  GET  /retrain/:id  - Check retraining job status")
    print("  POST /rollback     - Restore a backed-up model")
    print("  GET  /backups      - List available model backups")
    print("  GET  /health       - Health check")
    server.serve_forever()