import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse

import numpy as np

from mlp import PennywiseMLP
import utils

HOST = "0.0.0.0"
PORT = 8000

class MLPHandler(BaseHTTPRequestHandler):

    def do_POST(self):
        parsed_path = urlparse(self.path)

        if parsed_path.path == "/predict":
            try:
                content_length = int(self.headers["Content-Length"])
                post_data = self.rfile.read(content_length)

                data = json.loads(post_data.decode("utf-8"))
                print(data)
                type = data.get("type")
                email_text = data.get("email_text")
                amount = data.get("amount")
                account = data.get("account")
                payee = data.get("payee")
                embedding_model = "all-MiniLM-L6-v2"

                path = ""
                if type == "payee":
                    path = "pennywise_payee_mlp.parms"
                    embedding_model = "all-mpnet-base-v2"
                elif type == "category":
                    path = "pennywise_category_mlp.parms"
                elif type == "account":
                    path = "pennywise_account_mlp.parms"
                else:
                    raise Exception("Wrong type")

                print("inside predict", path)
                mlp = PennywiseMLP(type, is_new=False, model=embedding_model)
                mlp.load_model(path=path)
                prediction = mlp.predict(
                    type=type,
                    email_text=email_text,
                    amount=amount,
                    account=account,
                    payee=payee,
                )
                print(prediction)
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(prediction).encode())

            except Exception as e:
                print(str(e))
                self.send_response(400)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                error_res = {"error": str(e)}
                self.wfile.write(json.dumps(error_res).encode())

        elif parsed_path.path == "/embeddings":
            try:
                content_length = int(self.headers["Content-Length"])
                post_data = self.rfile.read(content_length)
                data = json.loads(post_data.decode("utf-8"))
                content = data.get("content")
                embedding = utils.create_embedding(content)
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(embedding).encode())
            except Exception as e:
                self.send_response(400)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                error_res = {"error": str(e)}
                self.wfile.write(json.dumps(error_res).encode())
        else:
            self.send_response(404)
            self.end_headers()

    def do_GET(self):
        # Simple health check
        if self.path == '/health':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({'status': 'healthy'}).encode())
        else:
            self.send_response(404)
            self.end_headers()

if __name__ == "__main__":
    server = HTTPServer((HOST, PORT), MLPHandler)
    print("Server running on http://localhost:8000")
    print("POST to /predict with JSON: {type: account|payee|category, email_text: <parsed_email>, amount: <parsed_amount>, account: <account_name (when type is payee)>, payee: <payee_name (when type is category)>}")
    print("POST to /embed with JSON: {content: <content_to_embed>}")
    server.serve_forever()
