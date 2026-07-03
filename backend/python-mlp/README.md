# python-mlp (deprecated)

> **Status: deprecated.** The MLP prediction path has been replaced by [`cipher`](../cipher)'s pipeline (Ollama extraction → payee rules → pgvector → LLM fallback). This service is kept for reference and rollback.

MLP + sentence-transformer models that predicted account, payee, and category from parsed bank-email text. `go-gmail` used to call `POST /predict` three times sequentially (account → payee → category) with confidence gating.

## Endpoints

Plain `http.server`-based API (`mlp_predict_server.py`):

- `POST /predict` — inference
- `POST /retrain`, `GET /retrain/:id` — retraining jobs
- `POST /fetch`, `POST /augment`, `POST /rollback` — training-data management
- `GET /backups`, `GET /health`

## Run

```bash
pip install -r requirements.txt
python mlp_predict_server.py   # PORT env var; optional VOLUME_DIR
```

Models load from `.parms` files; the Docker entrypoint seeds `/data` on first run. Not part of the docker-compose stack.
