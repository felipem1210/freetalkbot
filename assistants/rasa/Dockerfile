FROM khalosa/rasa-aarch64:3.5.2

WORKDIR /app

USER root

RUN pip install spacy && python -m spacy download en_core_web_md

RUN python -c "import spacy; spacy.load('en_core_web_md')"

COPY ./ /app

EXPOSE 5005

CMD ["rasa", "run", "--enable-api", "--cors", "*", "--debug"]
