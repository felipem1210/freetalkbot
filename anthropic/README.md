# Anthropic Assistant

Here are some examples of things that can be done with Anthropic LLM models. In this code we are implementing two tools, so Anthropic can check the web for recent solutions and can get data when a customer asks for "order information".

## Development

You can raise up the python Flask application:

1. Install dependencies `pip3 install -r requirements.txt`
2. Run `python3 app.py``
3. Test the endpoint

```sh
curl -X POST http://localhost:8000/chat -d '{"sender": "12345568@whatsapp.net", "text": "Hey can you help me?"}' -H "Content-Type: application/json"

[
  {
    "recipient_id": "12345568@whatsapp.net",
    "text": "Hello! Of course, I'd be happy to help you. How can I assist you today? Are you looking for information about our tech products or do you need help with an order?"
  }
]
```

**Disclaimer :** All this implementation is just an example and for development purposes. Implementation of an assistant with Anthropic should be treated as a separate component of the system.