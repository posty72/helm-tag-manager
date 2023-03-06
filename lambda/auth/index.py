import os


def lambda_handler(event, context):
    auth_header = event['headers']['authorization']
    token = f"Bearer {os.environ['API_KEY']}"

    return {
        "isAuthorized": auth_header == token,
        "context": {
            "exampleKey": "exampleValue",
        }
    }
