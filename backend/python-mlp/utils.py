import json

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

