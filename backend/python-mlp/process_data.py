from collections import defaultdict
import json
import math
import random
import re
from string import ascii_uppercase

upi_id_collection = [
    "VPA 7088449889@axl GEETA KANYAL WO GOVIND SINGH",
    "VPA Q121794777@ybl INDRAJEET PANDEY",
    "VPA leelachalal4@oksbi LILAWATI DEVI",
    "VPA Q371186215@ybl NARENDRA PANT",
    "VPA 87456239087@axl CHIRAG  BISHT",
    "VPA 76361846599@axl VIRENDRA TOMAR",
    "VPA mukeshsingh22@oksbi MUKESH SINGH",
    "VPA paytmqr5ehfhj@ptys ANUJ KUMAR",
    "VPA gpay-1125-@okbizaxis VARIETY STORE",
    "VPA Q97866018@ybl MRS PREMA KHAMPA",
    "VPA rehan26keitw-1@okaxis MR MOHD ASIF",
    "VPA 8826125150@@ibl NIRAJ KUMAR",
    "VPA Q597760547@ybl RIHAN AZAM",
    "VPA vanshsaraswat29@okici VANSH SARASWAT",
    "VPA deepaksane0@oksbi DEEPAK SAMPATRAO SANE",
    "VPA 012waseemsheikh@oksbi SHEIKH WASEEM SHEIKH NASIM"
    "VPA Q093998869@ybl KIRAN BHAURAO AMBULKAR",
    "VPA gpay-1124santoshnagesh@okbizaxis SANTOSH NAGESH",
    "VPA 9326934213@ptaxis RAJENDRAKUMAR BABURAOJI",
    "VPA Mswipe.14.87461546589@kotak PRASHANT KIRANA STORE",
    "VPA Q327126986@ybl ANUJ NARIYAL PANI AND FRUITS",
    "VPA pantkartik21@oksbi KARTIK PANT",
    "VPA Vyapar.1721827459@hdfcbank ASHISH SINGH",
    "VPA paytmqr11352543@paytm ANNAPURNA MISHTHAN BHANDAR",
    "VPA 881030983@ibl KULDEEP KUMAR",
    "VPA Q597760547@ybl CHANDRASHEKHAR",
    "VPA paytmqr5ehfhj@ptys MADAN SAH",
    "VPA Getepay.u38238237@icici NEHA KHANKA BHASERA",
    "VPA paytmqr1q132iz@paytm BHAVIKA DAILY NEEDS",
    "VPA paytmqr5eq9v@ptys BOUTIQUE APARTMENTS",
    "VPA gpatle72@okaxis MR BASANT KUMAR SO MAHESH KUMAR",
    "VPA ajith7beb-4@okicici VENKATESH MURTHY",
    "VPA 7517317041@ybl MR MITHUN SHRIRAMJI",
    "VPA getepay.dnsablqr484706@icici NEW SHARAN MOBILE",
    "VPA paytmqr59bdrf@paytm URMILA PREMCHAND SAH",
    "VPA Q998954481@ybl JAYANT RAMESH JOSHI",
    "VPA paytmqr5vqc2o@ptys VICKY MOHAN KEWLANI",
    "VPA Q810790460@ybl MAN SINGH CHAUHAN SO",
    "VPA paytmqr18zinsni0t@paytm SURENDRA NARAYAN RAU",
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
    "Ashu": "VPA divyanshkapri12-1@okaxis DIVYANSH KAPRI",
    "Vishakha": "VPA 7467802062@ybl VISHAKHA MARKUN",
    "Lavi": "VPA 7900680687@axl SHATAKSHI JOSHI",
    "Dasila": "VPA dasila.m24@oksbi MOHIT DASILA",
    "Dhawal": "VPA 7618138198@ybl DHAWAL KANDPAL",
    "Zerodha": "VPA zerodhamf@hdfcbank ICCL ZERODHA COIN",
    "Transfer : Mutual Funds": "VPA zerodhamf@hdfcbank ICCL ZERODHA COIN",
    "Transfer : Stocks": "VPA indmoneys@hdfcbank INDMONEY PRIVATE LIMITED",
    "INDmoney": "VPA indmoneys@hdfcbank INDMONEY PRIVATE LIMITED",
    "Maa": [
        "VPA sheelakapri5@hdfcbank SHEELA KAPRI",
        "VPA 9458306660@ybl SHEELA KAPRI",
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
        "VPA 9410774005@ybl GOKUL CHANDRA KAPRI",
        "VPA 9410774005@ibl GOKUL CHANDRA KAPRI",
    ],
    "Hotel": upi_id_collection,
    "Travel": upi_id_collection,
    "Transfer : PNB (Savings)": "VPA 9997167687@ybl RISHABH KAPRI S O GOKUL CHANDRA KAP",
    "Transfer : HDFC Credit Card": "VPA cred.club@axisb CRED Club",
    "Transfer : HDFC Swiggy Credit Card": "VPA cred.club@axisb CRED Club",
}

account_name_to_num = {
    "HDFC (Salary)": 8936,
    "PNB (Savings)": 2086,
    "Kotak (Savings)": 6318,
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
