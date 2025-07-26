import json

import numpy as np
from scipy.stats import yeojohnson_normplot
from sentence_transformers import SentenceTransformer
from sklearn.metrics import classification_report
from sklearn.metrics.pairwise import cosine_similarity
from sklearn.model_selection import train_test_split
from sklearn.neighbors import KNeighborsClassifier

model = SentenceTransformer("all-MiniLM-L6-v2")

embeddings = np.load("./payee_embeddings.npy")


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


class SBERT:
    def __init__(self, type):
        self.type = type
        (emails, labels) = get_labels_and_email(type)
        embeddings = model.encode(emails)

        # X_train, X_test, y_train, y_test = train_test_split(embeddings, labels, test_size=0.33, random_state=42)

        # knn = KNeighborsClassifier(n_neighbors=3)
        # knn.fit(X_train, y_train)
        #
        # y_pred = knn.predict(X_test)

        # print("Classification Report:")
        # print(classification_report(y_test, y_pred))
        self.knn = KNeighborsClassifier(n_neighbors=1)
        self.knn.fit(embeddings, labels)

    def test(self):
        file = open("./test_data.json")
        test_emails = json.load(file)

        correct = 0
        for email_dict in test_emails:
            new_embedding = model.encode([email_dict["email_text"]])
            predicted = self.knn.predict(new_embedding)[0]
            prob = self.knn.predict_proba(new_embedding)[0]
            class_probs = dict(zip(self.knn.classes_, prob))
            confidence = class_probs[predicted]
            if predicted == email_dict[self.type]:
                correct += 1
            print(
                f"\033[92mPredicted: \033[0m{predicted} \033[0mand \033[31mExpected: \033[0m{email_dict[self.type]} with Confidence: {confidence:.2f}"
            )
        print(
            f"\nCorrect: {correct} out of Total: {len(test_emails)} with precentage: {(correct/len(test_emails)) * 100:.2f}\n"
        )


# def search_payee(query, expected):
#     query_embedding = model.encode([query])
#
#     scores = cosine_similarity(query_embedding, embeddings)[0]
#     top_indices = scores.argsort()[-3:][::-1]
#     print(top_indices)
#
#     results = []
#     for i in top_indices:
#         results.append([payees[i], float(scores[i])])
#
#     print(results, expected)

SBERT("payee").test()
SBERT("category").test()
SBERT("account").test()

# search_payee("Steam Purchase", "Steam")
# search_payee("AMAZON PAY INDIA PRIVA", "Amazon")
# search_payee("Adobe Systems Software", "Google Cloud")
# search_payee("VPA cred.club@axisb CRED Club", "Cred/Transfer: Credit Card")
# search_payee("VPA 7088449889@axl GEETA KANYAL WO GOVIND SINGH", "Shop")
# search_payee("VPA Q121794777@ybl INDRAJEET PANDEY", "Shop")
# search_payee("OM DIESELS", "Petrol Pump")
