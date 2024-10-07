from flask import Flask, jsonify, request
from consult import anthropic_chat, chat_with_user
from flask import Flask
import os

app = Flask(__name__)


if os.getenv("ENABLE_PROMPT_CACHING") is None:
    raise ValueError("Please set the ENABLE_PROMPT_CACHING environment variable")

enable_prompt_caching = bool(os.getenv("ENABLE_PROMPT_CACHING"))


# Define routes
@app.route("/chat", methods=["POST"])
def implement_chat():
    if request.is_json:
        data = request.get_json()
        app.logger.info(data)
        system_prompt, messages = chat_with_user(
            # get body of the request
            data.get("text"),
            enable_prompt_caching,
        )
        app.logger.info(data.get("text"))
        response = anthropic_chat(system_prompt, messages, enable_prompt_caching)
        return jsonify([{"recipient_id": data.get("sender"), "text": response}])
    else:
        return jsonify({"error": "Content must be json"}), 400


@app.errorhandler(400)
def bad_request(error):
    return jsonify({"error": f"400 - Bad Rquest: {error}"}), 400


# Manejador personalizado para el error 404 (opcional)
@app.errorhandler(404)
def not_found(error):
    return jsonify({"error": f"404 - resource not found: {error}"}), 404


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8088, debug=True)
