import json

from sentence_transformers import SentenceTransformer


def get_labels_and_email(type):
    labels = []
    emails = []
    file = open("./normalized_with_email.json")
    file_data = json.load(file)

    for data in file_data:
        if "email_text" in data:
            if data[type] is None:
                labels.append("null")
            else:
                labels.append(data[type])
            emails.append(data["email_text"])
    return (emails, labels)


def create_embedding(text: str):
    """
    Takes a string and returns an embedding vector of it using nomic-embed-text-v1.5.
    """""
    model = SentenceTransformer("nomic-ai/nomic-embed-text-v1.5", trust_remote_code=True)
    embedding = model.encode(text)
    return embedding.tolist()


def get_embeddings(account, payee, category, amount, note):
    input = f"Payee: {payee}, Category: {category}, Account: {account}, Amount: {amount}, Note: {note}"
    model = SentenceTransformer("all-MiniLM-L6-v2")
    embedding = model.encode(input)
