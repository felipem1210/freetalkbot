from anthropic.types import ToolParam, MessageParam


def send_text_to_user(text):
    # Sends a text to the user
    # We'll just print out the text to keep things simple:
    return text


def get_customer_info(email):
    purchases = []
    customer_data = {
        "email": email,
        "purchases": [
            {"id": 1, "product": "computer mouse"},
            {"id": 2, "product": "screen protector"},
            {"id": 3, "product": "usb charging cable"},
        ],
    }
    for purchase in customer_data["purchases"]:
        purchases.append(purchase["product"])

    return f"Customer purchased: {", ".join(purchases)}"


def get_tools():
    return [
        {
            "name": "send_text_to_user",
            "description": "Sends a text message to a user unless the user is asking for information about a product",
            "input_schema": {
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "The piece of text to be sent to the user via text message",
                    },
                },
                "required": ["text"],
            },
        },
        {
            "name": "get_customer_info",
            "description": "gets information on a customer based on the customer's email.  Response includes email, and previous purchases. Only call this tool once a user has provided you with their email",
            "input_schema": {
                "type": "object",
                "properties": {
                    "email": {
                        "type": "string",
                        "description": "The email of the user in question.",
                    },
                },
                "required": ["email"],
            },
        },
    ]
