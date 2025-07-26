from collections import defaultdict
import json
import math
import random
import re
from string import ascii_uppercase

upi_id_collection = [
    "VPA 9000000104@axl SAMPLE PERSON",
    "VPA Q000000134@ybl SAMPLE PERSON",
    "VPA sample123@oksbi SAMPLE PERSON",
    "VPA Q000000135@ybl SAMPLE PERSON",
    "VPA 90000001100@axl SAMPLE PERSON",
    "VPA 90000001080@axl SAMPLE PERSON",
    "VPA sample124@oksbi SAMPLE PERSON",
    "VPA paytmqr5ehfhj@ptys ANUJ KUMAR",
    "VPA gpay-1125-@okbizaxis VARIETY STORE",
    "VPA Q000000137@ybl SAMPLE PERSON",
    "VPA sample126-1@okaxis SAMPLE PERSON",
    "VPA 9000000140@@ibl SAMPLE PERSON",
    "VPA Q000000132@ybl SAMPLE PERSON",
    "VPA sample141@okici SAMPLE PERSON",
    "VPA sample121@oksbi SAMPLE PERSON",
    "VPA sample115@oksbi SAMPLE PERSON"
    "VPA Q000000133@ybl SAMPLE PERSON",
    "VPA gpay-sample127@okbizaxis SAMPLE PERSON",
    "VPA 9000000112@ptaxis SAMPLE PERSON",
    "VPA Mswipe.14.87461546589@kotak PRASHANT KIRANA STORE",
    "VPA Q327126986@ybl ANUJ NARIYAL PANI AND FRUITS",
    "VPA sample125@oksbi SAMPLE PERSON",
    "VPA Vyapar.sample131@hdfcbank SAMPLE PERSON",
    "VPA paytmqr11352543@paytm ANNAPURNA MISHTHAN BHANDAR",
    "VPA 900000111@ibl SAMPLE PERSON",
    "VPA Q000000132@ybl SAMPLE PERSON",
    "VPA paytmqr5ehfhj@ptys SAMPLE PERSON",
    "VPA Getepay.sample130@icici SAMPLE PERSON",
    "VPA paytmqr1q132iz@paytm BHAVIKA DAILY NEEDS",
    "VPA paytmqr5eq9v@ptys BOUTIQUE APARTMENTS",
    "VPA sample122@okaxis SAMPLE PERSON",
    "VPA sample119-4@okicici SAMPLE PERSON",
    "VPA 9000000106@ybl SAMPLE PERSON",
    "VPA getepay.dnsablqr484706@icici NEW SHARAN MOBILE",
    "VPA paytmqrsample129@paytm SAMPLE PERSON",
    "VPA Q000000138@ybl SAMPLE PERSON",
    "VPA paytmqrsample142@ptys SAMPLE PERSON",
    "VPA Q000000136@ybl SAMPLE PERSON",
    "VPA paytmqrsample128@paytm SAMPLE PERSON",
    "VPA paytmqr2810050501011kvp9f3dhaac@paytm N KUMAR RETAILS PRO",
]
payee_map = {
    "Adobe": "Adobe Systems Software",
    "Google cloud": "GOOGLE CLOUD CYBS SI",
    "Google play store": [
        "VPA playstore@axisbank GOOGLE INDIA DIGITAL SERVICES",
        "VPA playstore@axisbank Google Play",
    ],
    "1mg": "TATA 1MG HEALTHCARE",
    "Nykaa": "FSN Ecommerce Ventures",
    "Amazon": ["AMAZON PAY INDIA PRIVA", "AMAZONIN"],
    "Spotify": "SPOTIFY SI",
    "C7": "VPA paytmqr5g870d@ptys CORRIDOR SEVEN",
    "Bus": ["VPA ka01f9560@cnrb BMTC BUS KA01F9560"],
    "Friend 2": "VPA sample117-1@okaxis SAMPLE PERSON",
    "Friend 1": "VPA 9000000105@ybl SAMPLE PERSON",
    "Friend 3": "VPA 9000000109@axl SAMPLE PERSON",
    "Friend 4": "VPA sample120@oksbi SAMPLE PERSON",
    "Friend 5": "VPA 9000000107@ybl SAMPLE PERSON",
    "Zerodha": "VPA zerodhamf@hdfcbank ICCL ZERODHA COIN",
    "Transfer : Mutual Funds": "VPA zerodhamf@hdfcbank ICCL ZERODHA COIN",
    "Transfer : Stocks": "VPA indmoneys@hdfcbank INDMONEY PRIVATE LIMITED",
    "INDmoney": "VPA indmoneys@hdfcbank INDMONEY PRIVATE LIMITED",
    "Maa": [
        "VPA sample118@hdfcbank SAMPLE PERSON",
        "VPA 9000000102@ybl SAMPLE PERSON",
    ],
    "MMT": "MAKEMYTRIP INDIA PVT L ",
    "Gym": ["VPA gpay-11259176055@okbizaxis The Fitness Hub"],
    "Nakpro Protein": "NAKPRO",
    "Zomato/Swiggy": [
        "WWW SWIGGY COM",
        "ZOMATO",
        "Cashfree*SWIGGY LIMITE",
        "Payu*ZOMATO",
    ],
    "GoIbibo": "IBIBO GROUP PVT LTD",
    "Indigo": "INDIGO AINE",
    "Shop": upi_id_collection,
    "Cab": upi_id_collection,
    "Uber": upi_id_collection,
    "Pharmacy": [
        "VPA asterpharmacykarnataka@ybl ASTER PHARMACY KARNA",
    ],
    "Physiotherapy": ["theurbanphysiocare.67099515@hdfcbank THEURBANPHYSIOCARE"],
    "Restaurant": ["a2bkarnataka@ybl ADYAR ANANDA BHAVAN"],
    "Petrol Pump": [
        "VPA Q536830324@ybl OM DIESELS",
        "VPA B478563294@axl INDIAN OIL",
    ],
    "Airtel": [
        "Airtel Payments Ban",
        "Bharti Airtel Limited",
    ],
    "Papa": [
        "VPA 9000000101@ybl SAMPLE PERSON",
        "VPA 9000000101@ibl SAMPLE PERSON",
    ],
    "Hotel": upi_id_collection,
    "Travel": upi_id_collection,
    "Transfer : PNB (Savings)": "VPA 9000000100@ybl SAMPLE PERSON",
    "Transfer : HDFC Credit Card": "VPA cred.club@axisb CRED Club",
    "Transfer : HDFC Swiggy Credit Card": "VPA cred.club@axisb CRED Club",
}

account_name_to_num = {
    "HDFC (Salary)": 1111,
    "PNB (Savings)": 4444,
    "Kotak (Savings)": 5555,
    "PhonePe Wallet": "",
    "Steam": "",
    "Cash": "",
    "Mutual Funds": "",
    "Stocks": "",
    "SGB": "",
    "Emergency Fund (HDFC FD)": "",
    "Short Term Mutual Funds (~6 months)": "",
    "ELSS": "",
    "Jaggu Loan": "",
    "Dhawal Loan": "",
    "Bhokal Loan": "",
}
friends = [
    "Sumit Negi",
    "Sahil",
    "Gautam",
    "Bhavesh Bhatt",
    "Vishakha",
    "Vinay Thakur",
    "Lavi",
]

travel_payees = {}


def generate_payee_list():
    file = open("./normalized_with_amount.json")
    file_data = json.load(file)

    payee_set = set()
    for data in file_data:
        if data["payee"]:
            payee_set.add(data["payee"].lower())

    payees_list = sorted(list(payee_set))
    with open("payees_list.json", "w") as f:
        json.dump(payees_list, f)


def get_friend_upi(payee):
    upi_suffix = ["oksbi", "ybl", "axl", "pz", "okaxis", "hdfcbank", "icici"]
    index = math.floor(random.random() * len(upi_suffix))
    name_arr = payee.split(" ")
    random_chars = "".join(random.choice(ascii_uppercase) for _ in range(10))
    upi_id = (
        f"VPA {name_arr[-1].lower()}.{random_chars}@{upi_suffix[index]} {payee.upper()}"
    )

    return upi_id


def get_payee(transaction_payee):
    if transaction_payee in payee_map:
        payees = payee_map[transaction_payee]
        if isinstance(payees, list):
            index = math.floor(random.random() * len(payees))
            return payees[index]
        else:
            return payees
    elif transaction_payee == "Travel" or transaction_payee == "Hotel":
        return transaction_payee
    elif "Friend" in transaction_payee or transaction_payee in friends:
        if "Friend" in transaction_payee:
            possible_names = [
                "Vivek Singh",
                "Prashant Kumar",
                "Kartik Pant",
                "Lokesh Singh",
                "Aishwarya Patil",
            ]
            index = math.floor(random.random() * len(possible_names))
            return get_friend_upi(possible_names[index])
        else:
            return get_friend_upi(transaction_payee)
    else:
        return transaction_payee


def generate_email_text():
    file = open("./normalized_with_amount.json")
    file_data = json.load(file)

    for data in file_data:
        payee = get_payee(data["payee"])
        # masked_payee = "[PAYEE]"
        if "Credit Card" in data["account"]:
            card_suffix = 4432
            date = "-".join(data["date"].split("-")[::-1])
            if "Swiggy" in data["account"]:
                card_suffix = 8799
            email = f"Dear Card Member, Thank you for using your HDFC Bank Credit Card ending {card_suffix} for Rs {float(data["amount"]):.2f} at {payee} on {date}."
            data["email_text"] = email
        else:
            account_num = account_name_to_num[data["account"]]
            if account_num:
                type = "has been debited from account"
                from_to = "to"
                dateSplit = data["date"].split("-")[::-1]
                date = f"{dateSplit[0]}-{dateSplit[1]}-{dateSplit[2][-2:]}"
                if data["amount"] > 0:
                    type = "is successfully credited to your account"
                    from_to = "by"
                email = f"Dear Customer, Rs.{float(data["amount"]):.2f} {type} **{account_num} {from_to} {payee} on {date}."
                data["email_text"] = email

    with open("normalized_with_email.json", "w") as f:
        json.dump(file_data, f, ensure_ascii=False, indent=2)


def count_emails():
    count = 0
    file = open("./normalized_with_email.json")
    file_data = json.load(file)
    payee_map = {}
    category_map = {}
    account_map = {}
    payee_counts = defaultdict(int)
    category_counts = defaultdict(int)

    for data in file_data:
        if "email_text" in data:
            count += 1
            payee = data["payee"]
            category = data.get("category", "null")
            account = data["account"]

            payee_counts[payee] += 1
            category_counts[category] += 1

            if payee in payee_map:
                payee_map[payee] += 1
            else:
                payee_map[payee] = 1

            if category in category_map:
                category_map[category] += 1
            else:
                category_map[category] = 1

            if account in account_map:
                account_map[account] += 1
            else:
                account_map[account] = 1

    print(f"Total transactions are: {len(file_data)} and generated emails are {count}")
    print("Top payees:")
    for payee, count in sorted(payee_counts.items(), key=lambda x: -x[1]):
        print(f"{payee}: {count}")

    print("\nTop categories:")
    for cat, count in sorted(category_counts.items(), key=lambda x: -x[1]):
        print(f"{cat}: {count}")

generate_payee_list()
generate_email_text()
count_emails()
