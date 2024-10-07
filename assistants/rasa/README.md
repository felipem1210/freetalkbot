# RASA assistant

[Rasa](https://rasa.com/) is a framework to create your own assistants with models based on NLU/NLP technologies. Currently only Rasa Open Source is integrated.

The assistant is configured to be a "remind me to call bot", inspired in the [example](https://github.com/RasaHQ/rasa/tree/main/examples/reminderbot) provided by rasa. The files in this folder are for NLP training of the assistant.
Checkout the following resources to get more knowledge about RASA.

* [Documentation](https://rasa.com/docs/rasa/training-data-format)
* [Youtube Channel](https://www.youtube.com/@RasaHQ)

If you modify any of the rasa files you will need to retrain the assistant, you can do it with `make rasa-train`

**Disclaimer :** All this implementation is just an example and for development purposes. Implementation of an assistant with RASA should be treated as a separate component of the system.